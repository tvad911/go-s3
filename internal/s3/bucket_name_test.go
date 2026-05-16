package s3_test

import (
	"strings"
	"testing"

	"gos3/internal/s3"
)

func TestValidateBucketName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		// Valid cases
		{"valid simple", "my-bucket", false},
		{"valid 3 chars", "abc", false},
		{"valid with numbers", "bucket-123", false},
		{"valid with dots", "my.bucket.name", false},
		{"valid 63 chars", strings.Repeat("a", 63), false},

		// Too short / too long
		{"too short 2 chars", "ab", true},
		{"too short 1 char", "a", true},
		{"too short empty", "", true},
		{"too long 64 chars", strings.Repeat("a", 64), true},

		// Uppercase
		{"uppercase letter", "MyBucket", true},
		{"all uppercase", "BUCKET", true},

		// Starts/ends with hyphen
		{"starts with hyphen", "-bucket", true},
		{"ends with hyphen", "bucket-", true},

		// Consecutive dots
		{"consecutive dots", "my..bucket", true},

		// Special characters
		{"underscore", "my_bucket", true},
		{"space", "my bucket", true},
		{"exclamation", "my!bucket", true},
		{"at sign", "my@bucket", true},

		// Valid edge cases
		{"dots and hyphens", "my-bucket.v2", false},
		{"all digits", "123456", false},
		{"digit start", "3bucket", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := s3.ValidateBucketName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateBucketName(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if err != nil && tt.wantErr {
				// Verify it returns the correct S3 error type
				if err != s3.ErrInvalidBucketName {
					t.Errorf("ValidateBucketName(%q) returned %v, want ErrInvalidBucketName", tt.input, err)
				}
			}
		})
	}
}
