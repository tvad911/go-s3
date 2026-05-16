package s3_test

import (
	"net/http"
	"testing"

	"gos3/internal/s3"
)

func TestErrorCodes(t *testing.T) {
	tests := []struct {
		name       string
		err        s3.Error
		wantCode   string
		wantStatus int
	}{
		{"AccessDenied", s3.ErrAccessDenied, "AccessDenied", http.StatusForbidden},
		{"NoSuchBucket", s3.ErrNoSuchBucket, "NoSuchBucket", http.StatusNotFound},
		{"NoSuchKey", s3.ErrNoSuchKey, "NoSuchKey", http.StatusNotFound},
		{"BucketAlreadyExists", s3.ErrBucketAlreadyExists, "BucketAlreadyExists", http.StatusConflict},
		{"BucketNotEmpty", s3.ErrBucketNotEmpty, "BucketNotEmpty", http.StatusConflict},
		{"InvalidBucketName", s3.ErrInvalidBucketName, "InvalidBucketName", http.StatusBadRequest},
		{"InvalidRange", s3.ErrInvalidRange, "InvalidRange", http.StatusRequestedRangeNotSatisfiable},
		{"InvalidPart", s3.ErrInvalidPart, "InvalidPart", http.StatusBadRequest},
		{"InvalidPartOrder", s3.ErrInvalidPartOrder, "InvalidPartOrder", http.StatusBadRequest},
		{"NoSuchUpload", s3.ErrNoSuchUpload, "NoSuchUpload", http.StatusNotFound},
		{"MalformedXML", s3.ErrMalformedXML, "MalformedXML", http.StatusBadRequest},
		{"InternalError", s3.ErrInternalError, "InternalError", http.StatusInternalServerError},
		{"SlowDown", s3.ErrSlowDown, "SlowDown", http.StatusTooManyRequests},
		{"InsufficientStorage", s3.ErrInsufficientStorage, "InsufficientStorage", http.StatusInsufficientStorage},
		{"SignatureDoesNotMatch", s3.ErrSignatureDoesNotMatch, "SignatureDoesNotMatch", http.StatusForbidden},
		{"NotImplemented", s3.ErrNotImplemented, "NotImplemented", http.StatusNotImplemented},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Code != tt.wantCode {
				t.Errorf("Code = %q, want %q", tt.err.Code, tt.wantCode)
			}
			if tt.err.HTTPStatus != tt.wantStatus {
				t.Errorf("HTTPStatus = %d, want %d", tt.err.HTTPStatus, tt.wantStatus)
			}
			if tt.err.Message == "" {
				t.Error("Message should not be empty")
			}
			// Error() should return the message
			if tt.err.Error() != tt.err.Message {
				t.Errorf("Error() = %q, want %q", tt.err.Error(), tt.err.Message)
			}
		})
	}
}

func TestErrorWithResource(t *testing.T) {
	err := s3.ErrNoSuchBucket.WithResource("my-bucket")

	if err.Resource != "my-bucket" {
		t.Errorf("Resource = %q, want %q", err.Resource, "my-bucket")
	}
	// Original should not be mutated
	if s3.ErrNoSuchBucket.Resource != "" {
		t.Error("WithResource mutated the original error")
	}
}
