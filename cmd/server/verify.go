package main

import (
	"crypto/md5"
	"encoding/hex"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"gos3/internal/config"
	"gos3/internal/storage/metadata"
)

func runVerify(cfg *config.Config) {
	slog.Info("starting data verification")

	metaStore, err := metadata.NewBboltStore(filepath.Join(cfg.Storage.DataDir, "meta.db"))
	if err != nil {
		slog.Error("failed to open metadata store", "error", err)
		os.Exit(1)
	}
	defer metaStore.Close()

	buckets, err := metaStore.ListBuckets()
	if err != nil {
		slog.Error("failed to list buckets", "error", err)
		os.Exit(1)
	}

	var totalObjects, corruptObjects, missingObjects int64

	for _, bucket := range buckets {
		slog.Info("verifying bucket", "bucket", bucket.Name)

		objs, _, _, err := metaStore.ListObjects(bucket.Name, "", "", "", 1000000)
		if err != nil {
			slog.Error("failed to list objects", "bucket", bucket.Name, "error", err)
			continue
		}

		for _, obj := range objs {
			totalObjects++
			// Clean ETag (remove quotes and multipart suffix)
			expectedETag := strings.Trim(obj.ETag, `"`)
			if strings.Contains(expectedETag, "-") {
				// Multipart upload ETag is MD5 of parts' MD5s, can't easily verify the full file without part sizes
				// Skip full MD5 verify for multipart for now
				slog.Debug("skipping multipart object verify", "bucket", bucket.Name, "key", obj.Key)
				continue
			}

			// Encode key to filesystem path format
			safeKey := strings.ReplaceAll(obj.Key, "/", string(os.PathSeparator))
			objPath := filepath.Join(cfg.Storage.DataDir, "buckets", bucket.Name, "objects", safeKey)

			f, err := os.Open(objPath)
			if err != nil {
				slog.Error("missing object file", "bucket", bucket.Name, "key", obj.Key, "path", objPath)
				missingObjects++
				continue
			}

			h := md5.New()
			if _, err := io.Copy(h, f); err != nil {
				f.Close()
				slog.Error("failed to read object", "bucket", bucket.Name, "key", obj.Key, "error", err)
				corruptObjects++
				continue
			}
			f.Close()

			actualETag := hex.EncodeToString(h.Sum(nil))
			if actualETag != expectedETag {
				slog.Error("object corruption detected", "bucket", bucket.Name, "key", obj.Key, "expected", expectedETag, "actual", actualETag)
				corruptObjects++
			}
		}
	}

	slog.Info("data verification completed", "total", totalObjects, "missing", missingObjects, "corrupt", corruptObjects)

	if missingObjects > 0 || corruptObjects > 0 {
		os.Exit(1)
	}
}
