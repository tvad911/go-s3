package handler

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

// EventPayload represents the payload sent to a webhook
type EventPayload struct {
	EventName string    `json:"eventName"`
	Bucket    string    `json:"bucket"`
	Key       string    `json:"key"`
	Size      int64     `json:"size,omitempty"`
	ETag      string    `json:"etag,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	RequestID string    `json:"requestId"`
}

// fireWebhooks triggers any configured webhooks for the bucket and event
func (h *S3Handler) fireWebhooks(ctx context.Context, eventName, bucket, key string, size int64, etag string) {
	// Look up notification configuration
	config, err := h.Backend.GetBucketNotification(context.Background(), bucket)
	if err != nil || config == nil || len(config.Webhooks) == 0 {
		return
	}

	reqID := RequestIDFromContext(ctx)
	if reqID == "" {
		reqID = "internal-event"
	}

	payload := EventPayload{
		EventName: eventName,
		Bucket:    bucket,
		Key:       key,
		Size:      size,
		ETag:      etag,
		Timestamp: time.Now(),
		RequestID: reqID,
	}

	payloadBytes, _ := json.Marshal(payload)

	// Fire in background
	go func() {
		client := &http.Client{Timeout: 10 * time.Second}
		for _, wh := range config.Webhooks {
			// Check if event matches
			match := false
			for _, ev := range wh.Events {
				// simple prefix matching, e.g., "s3:ObjectCreated:*"
				if ev == "*" || ev == eventName {
					match = true
					break
				}
				if strings.HasSuffix(ev, "*") && strings.HasPrefix(eventName, strings.TrimSuffix(ev, "*")) {
					match = true
					break
				}
			}
			if !match {
				continue
			}

			// Send webhook
			req, err := http.NewRequest("POST", wh.URL, bytes.NewReader(payloadBytes))
			if err != nil {
				continue
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Amz-Sns-Message-Type", "Notification") // For S3 compatibility-ish

			// Add signature if secret is provided
			if wh.Secret != "" {
				mac := hmac.New(sha256.New, []byte(wh.Secret))
				mac.Write(payloadBytes)
				signature := hex.EncodeToString(mac.Sum(nil))
				req.Header.Set("X-GoS3-Signature", "sha256="+signature)
			}

			// Execute (ignore response for now)
			resp, err := client.Do(req)
			if err == nil {
				resp.Body.Close()
			}
		}
	}()
}
