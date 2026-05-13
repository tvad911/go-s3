package s3

import (
	"strings"
	"unicode"
)

// ValidateBucketName checks if a bucket name follows the S3 naming rules.
func ValidateBucketName(name string) error {
	if len(name) < 3 || len(name) > 63 {
		return ErrInvalidBucketName
	}
	if strings.Contains(name, "..") {
		return ErrInvalidBucketName
	}
	if strings.HasPrefix(name, "-") || strings.HasSuffix(name, "-") {
		return ErrInvalidBucketName
	}

	for _, c := range name {
		if unicode.IsUpper(c) {
			return ErrInvalidBucketName
		}
		if !unicode.IsLower(c) && !unicode.IsDigit(c) && c != '-' && c != '.' {
			return ErrInvalidBucketName
		}
	}

	return nil
}
