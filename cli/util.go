package cli

import (
	"fmt"
	"strings"
)

// parseS3URI parses an s3:// URI into bucket and key.
func parseS3URI(uri string) (string, string, error) {
	if !strings.HasPrefix(uri, "s3://") {
		return "", "", fmt.Errorf("invalid S3 URI: %s (must start with s3://)", uri)
	}

	path := uri[5:]
	if path == "" {
		return "", "", fmt.Errorf("invalid S3 URI: %s", uri)
	}

	parts := strings.SplitN(path, "/", 2)
	bucket := parts[0]
	key := ""
	if len(parts) > 1 {
		key = parts[1]
	}

	return bucket, key, nil
}
