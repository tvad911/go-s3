package metadata

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.etcd.io/bbolt"

	"gos3/internal/storage"
)

var (
	bucketBuckets = []byte("buckets")
	bucketUploads = []byte("uploads")
)

type bboltStore struct {
	db *bbolt.DB
}

// NewBboltStore creates a new bbolt-backed metadata store.
func NewBboltStore(path string) (Store, error) {
	db, err := bbolt.Open(path, 0600, &bbolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("failed to open bbolt: %w", err)
	}

	// Initialize top-level buckets
	err = db.Update(func(tx *bbolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists(bucketBuckets); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(bucketUploads); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to init bbolt: %w", err)
	}

	return &bboltStore{db: db}, nil
}

func (s *bboltStore) Close() error {
	return s.db.Close()
}

func (s *bboltStore) CreateBucket(name, region, owner string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketBuckets)
		if b.Get([]byte(name)) != nil {
			return ErrBucketExists
		}

		info := storage.BucketInfo{
			Name:         name,
			CreationDate: time.Now().UTC(),
			Region:       region,
			Owner:        owner,
		}
		data, err := json.Marshal(info)
		if err != nil {
			return err
		}

		if err := b.Put([]byte(name), data); err != nil {
			return err
		}

		// Create objects bucket
		_, err = tx.CreateBucketIfNotExists([]byte("objects:" + name))
		return err
	})
}

func (s *bboltStore) DeleteBucket(name string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketBuckets)
		if b.Get([]byte(name)) == nil {
			return ErrBucketNotFound
		}

		// Check if objects bucket exists and is empty
		objB := tx.Bucket([]byte("objects:" + name))
		if objB != nil {
			k, _ := objB.Cursor().First()
			if k != nil {
				return errors.New("BucketNotEmpty") // Should match s3 error in handler
			}
			if err := tx.DeleteBucket([]byte("objects:" + name)); err != nil {
				return err
			}
		}

		return b.Delete([]byte(name))
	})
}

func (s *bboltStore) GetBucket(name string) (*storage.BucketInfo, error) {
	var info storage.BucketInfo
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketBuckets)
		data := b.Get([]byte(name))
		if data == nil {
			return ErrBucketNotFound
		}
		return json.Unmarshal(data, &info)
	})
	if err != nil {
		return nil, err
	}
	return &info, nil
}

func (s *bboltStore) ListBuckets() ([]storage.BucketInfo, error) {
	var buckets []storage.BucketInfo
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketBuckets)
		return b.ForEach(func(k, v []byte) error {
			var info storage.BucketInfo
			if err := json.Unmarshal(v, &info); err != nil {
				return err
			}
			buckets = append(buckets, info)
			return nil
		})
	})
	return buckets, err
}

func (s *bboltStore) PutObject(bucket, key string, meta storage.ObjectMeta) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("objects:" + bucket))
		if b == nil {
			return ErrBucketNotFound
		}
		data, err := json.Marshal(meta)
		if err != nil {
			return err
		}
		return b.Put([]byte(key), data)
	})
}

func (s *bboltStore) GetObject(bucket, key string) (*storage.ObjectMeta, error) {
	var meta storage.ObjectMeta
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("objects:" + bucket))
		if b == nil {
			return ErrBucketNotFound
		}
		data := b.Get([]byte(key))
		if data == nil {
			return ErrObjectNotFound
		}
		return json.Unmarshal(data, &meta)
	})
	if err != nil {
		return nil, err
	}
	return &meta, nil
}

func (s *bboltStore) DeleteObject(bucket, key string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("objects:" + bucket))
		if b == nil {
			return ErrBucketNotFound
		}
		return b.Delete([]byte(key))
	})
}

func (s *bboltStore) ListObjects(bucket, prefix, delimiter, marker string, maxKeys int) ([]storage.ObjectInfo, []string, string, error) {
	var objects []storage.ObjectInfo
	var commonPrefixes []string
	var nextMarker string

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("objects:" + bucket))
		if b == nil {
			return ErrBucketNotFound
		}

		c := b.Cursor()
		var k, v []byte

		if marker != "" {
			// Seek to the key AFTER marker
			k, v = c.Seek([]byte(marker))
			if k != nil && string(k) == marker {
				k, v = c.Next()
			}
		} else if prefix != "" {
			k, v = c.Seek([]byte(prefix))
		} else {
			k, v = c.First()
		}

		prefixes := make(map[string]bool)

		for ; k != nil; k, v = c.Next() {
			keyStr := string(k)

			if prefix != "" && !strings.HasPrefix(keyStr, prefix) {
				break
			}

			if delimiter != "" {
				// Strip prefix to find delimiter
				rem := keyStr[len(prefix):]
				if idx := strings.Index(rem, delimiter); idx >= 0 {
					pfx := prefix + rem[:idx+len(delimiter)]
					if !prefixes[pfx] {
						prefixes[pfx] = true
						commonPrefixes = append(commonPrefixes, pfx)

						if len(objects)+len(commonPrefixes) >= maxKeys {
							nextMarker = pfx
							return nil
						}
					}
					// Skip to next key that doesn't start with this prefix
					// Not perfectly optimized in bbolt without a custom index, but works
					continue
				}
			}

			var meta storage.ObjectMeta
			if err := json.Unmarshal(v, &meta); err != nil {
				return err
			}

			objects = append(objects, storage.ObjectInfo{
				Key:          keyStr,
				LastModified: meta.LastModified,
				ETag:         meta.ETag,
				Size:         meta.Size,
				StorageClass: meta.StorageClass,
			})

			if len(objects)+len(commonPrefixes) >= maxKeys {
				nextMarker = keyStr
				return nil
			}
		}

		return nil
	})

	return objects, commonPrefixes, nextMarker, err
}

func (s *bboltStore) CreateMultipartUpload(bucket, key string, meta storage.ObjectMeta) (string, error) {
	// Not implemented completely yet for simplicity in this example
	return "", errors.New("not implemented")
}

func (s *bboltStore) GetMultipartUpload(bucket, key, uploadID string) (*storage.ObjectMeta, error) {
	return nil, errors.New("not implemented")
}

func (s *bboltStore) DeleteMultipartUpload(bucket, key, uploadID string) error {
	return errors.New("not implemented")
}

func (s *bboltStore) PutObjectPart(uploadID string, partNum int, partInfo storage.PartInfo) error {
	return errors.New("not implemented")
}

func (s *bboltStore) ListObjectParts(uploadID string, partNumberMarker, maxParts int) ([]storage.PartInfo, int, error) {
	return nil, 0, errors.New("not implemented")
}

func (s *bboltStore) ListMultipartUploads(bucket, prefix, delimiter, keyMarker, uploadIDMarker string, maxUploads int) ([]storage.UploadInfo, []string, string, string, error) {
	return nil, nil, "", "", errors.New("not implemented")
}
