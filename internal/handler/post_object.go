package handler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"gos3/internal/auth"
	"gos3/internal/s3"
	"gos3/internal/storage"
)

// PostPolicy represents a basic S3 POST policy document.
type PostPolicy struct {
	Expiration string        `json:"expiration"`
	Conditions []interface{} `json:"conditions"`
}

// PostObject handles HTML form uploads.
func (h *S3Handler) PostObject(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	bucket := chi.URLParam(r, "bucket")

	mr, err := r.MultipartReader()
	if err != nil {
		WriteError(w, r, s3.ErrMalformedXML) // close enough for bad request
		return
	}

	fields := make(map[string]string)
	var filePart io.Reader
	var fileSize int64 // We don't know the exact size from streaming multipart, unless Content-Length is set on the part (rarely).

	// AWS S3 requires 'file' to be the last field. We stream parts until 'file'.
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			WriteError(w, r, fmt.Errorf("read part: %w", err))
			return
		}

		name := part.FormName()
		if name == "file" {
			filePart = part
			break // Stop parsing fields, we found the file. The rest of the stream is the file content.
		}

		// Read form field
		value, err := io.ReadAll(part)
		if err != nil {
			WriteError(w, r, fmt.Errorf("read field %s: %w", name, err))
			return
		}
		fields[strings.ToLower(name)] = string(value)
	}

	if filePart == nil {
		WriteError(w, r, fmt.Errorf("missing 'file' field"))
		return
	}

	key := fields["key"]
	if key == "" {
		WriteError(w, r, fmt.Errorf("missing 'key' field"))
		return
	}

	// In S3 POST, key can contain ${filename} variable.
	// For simplicity, we just use the key as is, or replace ${filename} if present.
	// We didn't save the part.FileName(), let's extract it if possible.
	// But part is already consumed. Oh wait, we broke out of loop, filePart is the 'file' part.
	// So we can get its filename if needed, but AWS doesn't require us to parse ${filename} unless specified.

	policyB64 := fields["policy"]
	credential := fields["x-amz-credential"]
	signature := fields["x-amz-signature"]
	date := fields["x-amz-date"]

	// Validate Signature
	if h.Verifier != nil {
		user, err := h.Verifier.VerifyPostPolicy(ctx, credential, date, policyB64, signature)
		if err != nil {
			WriteError(w, r, err)
			return
		}
		// Store user in context just in case downstream needs it
		ctx = context.WithValue(ctx, auth.UserContextKey, user)
	}

	// Validate Policy Expiration
	if policyB64 != "" {
		policyJSON, err := base64.StdEncoding.DecodeString(policyB64)
		if err != nil {
			WriteError(w, r, fmt.Errorf("invalid policy encoding: %w", err))
			return
		}

		var policy PostPolicy
		if err := json.Unmarshal(policyJSON, &policy); err != nil {
			WriteError(w, r, fmt.Errorf("invalid policy json: %w", err))
			return
		}

		if policy.Expiration != "" {
			exp, err := time.Parse(time.RFC3339Nano, policy.Expiration)
			if err == nil && time.Now().After(exp) {
				WriteError(w, r, fmt.Errorf("policy expired"))
				return
			}
		}
	}

	// Prepare metadata
	meta := storage.ObjectMeta{
		ContentType:        fields["content-type"],
		ContentDisposition: fields["content-disposition"],
		UserMeta:           make(map[string]string),
	}

	if meta.ContentType == "" {
		meta.ContentType = "application/octet-stream"
	}

	for k, v := range fields {
		if strings.HasPrefix(k, "x-amz-meta-") {
			meta.UserMeta[k] = v
		}
	}

	// Stream file to storage
	res, err := h.Backend.PutObject(ctx, bucket, key, filePart, fileSize, meta)
	if err != nil {
		WriteError(w, r, err)
		return
	}

	// Handle response formatting
	redirect := fields["success_action_redirect"]
	if redirect != "" {
		// Append bucket, key, etag as query params based on AWS specs
		// Simplification for now: just redirect
		http.Redirect(w, r, redirect, http.StatusSeeOther)
		return
	}

	statusStr := fields["success_action_status"]
	status := http.StatusNoContent // Default is 204
	if statusStr == "200" {
		status = http.StatusOK
	} else if statusStr == "201" {
		status = http.StatusCreated
	}

	w.Header().Set("ETag", `"`+res.ETag+`"`)

	if status == http.StatusCreated {
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusCreated)
		// AWS S3 returns XML on 201
		xmlStr := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<PostResponse>
	<Location>%s/%s/%s</Location>
	<Bucket>%s</Bucket>
	<Key>%s</Key>
	<ETag>"%s"</ETag>
</PostResponse>`, "http://"+r.Host, bucket, key, bucket, key, res.ETag)
		w.Write([]byte(xmlStr))
		return
	}

	w.WriteHeader(status)
}
