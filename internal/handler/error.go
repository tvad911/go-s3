package handler

import (
	"encoding/xml"
	"net/http"

	"gos3/internal/s3"
)

// WriteError formats and writes an S3 XML error response.
func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	s3Err, ok := err.(s3.Error)
	if !ok {
		s3Err = s3.ErrInternalError
	}

	// Set RequestID if available in context (from middleware)
	if reqID := RequestIDFromContext(r.Context()); reqID != "" {
		s3Err.RequestId = reqID
		w.Header().Set("x-amz-request-id", reqID)
	}

	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(s3Err.HTTPStatus)

	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")

	// Add XML header
	w.Write([]byte(xml.Header))
	_ = enc.Encode(s3Err)
}
