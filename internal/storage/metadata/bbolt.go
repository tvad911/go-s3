package metadata

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.etcd.io/bbolt"

	"gos3/internal/auth"
	"gos3/internal/s3"
	"gos3/internal/storage"
)

var (
	bucketBuckets = []byte("buckets")
	bucketUploads  = []byte("uploads")
	bucketUsers    = []byte("users")
	bucketPolicies = []byte("policies")
	bucketCORS      = []byte("cors")
	bucketLifecycle = []byte("lifecycle")
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
		if _, err := tx.CreateBucketIfNotExists(bucketUsers); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(bucketPolicies); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(bucketCORS); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(bucketLifecycle); err != nil {
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

func (s *bboltStore) CreateBucket(name, region, owner, acl string) error {
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
			ACL:          acl,
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

func (s *bboltStore) UpdateBucket(bucket *storage.BucketInfo) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketBuckets)
		if b.Get([]byte(bucket.Name)) == nil {
			return ErrBucketNotFound
		}
		data, err := json.Marshal(bucket)
		if err != nil {
			return err
		}
		return b.Put([]byte(bucket.Name), data)
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
		if meta.IsLatest || meta.VersionID == "" {
			if err := b.Put([]byte(key), data); err != nil {
				return err
			}
		}

		vb, err := tx.CreateBucketIfNotExists([]byte("versions:" + bucket))
		if err != nil {
			return err
		}
		versionKey := fmt.Sprintf("%s\x00%s", key, meta.VersionID)
		return vb.Put([]byte(versionKey), data)
	})
}

func (s *bboltStore) GetObject(bucket, key, versionId string) (*storage.ObjectMeta, error) {
	var meta storage.ObjectMeta
	err := s.db.View(func(tx *bbolt.Tx) error {
		if versionId == "" {
			b := tx.Bucket([]byte("objects:" + bucket))
			if b == nil {
				return ErrBucketNotFound
			}
			data := b.Get([]byte(key))
			if data == nil {
				return ErrObjectNotFound
			}
			return json.Unmarshal(data, &meta)
		} else {
			vb := tx.Bucket([]byte("versions:" + bucket))
			if vb == nil {
				return ErrObjectNotFound
			}
			versionKey := fmt.Sprintf("%s\x00%s", key, versionId)
			data := vb.Get([]byte(versionKey))
			if data == nil {
				return ErrObjectNotFound
			}
			return json.Unmarshal(data, &meta)
		}
	})
	if err != nil {
		return nil, err
	}
	return &meta, nil
}

func (s *bboltStore) DeleteObject(bucket, key, versionId string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("objects:" + bucket))
		if b == nil {
			return ErrBucketNotFound
		}
		
		if versionId == "" {
			b.Delete([]byte(key))
			vb := tx.Bucket([]byte("versions:" + bucket))
			if vb != nil {
				c := vb.Cursor()
				prefix := []byte(key + "\x00")
				for k, _ := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, _ = c.Next() {
					vb.Delete(k)
				}
			}
			return nil
		}

		vb := tx.Bucket([]byte("versions:" + bucket))
		if vb != nil {
			versionKey := fmt.Sprintf("%s\x00%s", key, versionId)
			vb.Delete([]byte(versionKey))
			
			c := vb.Cursor()
			prefix := []byte(key + "\x00")
			var latestMeta *storage.ObjectMeta
			var latestData []byte
			for k, v := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, v = c.Next() {
				var m storage.ObjectMeta
				if err := json.Unmarshal(v, &m); err == nil {
					if latestMeta == nil || m.LastModified.After(latestMeta.LastModified) {
						latestMeta = &m
						latestData = v
					}
				}
			}
			
			if latestMeta != nil {
				latestMeta.IsLatest = true
				latestData, _ = json.Marshal(latestMeta)
				b.Put([]byte(key), latestData)
				vb.Put([]byte(fmt.Sprintf("%s\x00%s", key, latestMeta.VersionID)), latestData)
			} else {
				b.Delete([]byte(key))
			}
		}
		return nil
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
				Key:            keyStr,
				VersionID:      meta.VersionID,
				IsLatest:       meta.IsLatest,
				IsDeleteMarker: meta.IsDeleteMarker,
				LastModified:   meta.LastModified,
				ETag:           meta.ETag,
				Size:           meta.Size,
				StorageClass:   meta.StorageClass,
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

func (s *bboltStore) ListObjectVersions(bucket, prefix, delimiter, keyMarker, versionIdMarker string, maxKeys int) ([]storage.ObjectInfo, []string, string, string, error) {
	var objects []storage.ObjectInfo
	var commonPrefixes []string
	var nextKeyMarker string
	var nextVersionIdMarker string

	err := s.db.View(func(tx *bbolt.Tx) error {
		vb := tx.Bucket([]byte("versions:" + bucket))
		if vb == nil {
			return ErrBucketNotFound
		}

		c := vb.Cursor()
		var k, v []byte
		
		if keyMarker != "" {
			k, v = c.Seek([]byte(keyMarker))
		} else if prefix != "" {
			k, v = c.Seek([]byte(prefix))
		} else {
			k, v = c.First()
		}

		prefixes := make(map[string]bool)

		for k != nil {
			keyParts := strings.SplitN(string(k), "\x00", 2)
			if len(keyParts) != 2 {
				k, v = c.Next()
				continue
			}
			keyStr := keyParts[0]

			if prefix != "" && !strings.HasPrefix(keyStr, prefix) {
				break
			}

			if delimiter != "" {
				rem := keyStr[len(prefix):]
				if idx := strings.Index(rem, delimiter); idx >= 0 {
					pfx := prefix + rem[:idx+len(delimiter)]
					if !prefixes[pfx] {
						prefixes[pfx] = true
						commonPrefixes = append(commonPrefixes, pfx)
						
						if len(objects)+len(commonPrefixes) >= maxKeys {
							nextKeyMarker = pfx
							return nil
						}
					}
					prefixEnd := []byte(pfx)
					prefixEnd[len(prefixEnd)-1]++
					k, v = c.Seek(prefixEnd)
					continue
				}
			}

			var keyVersions []storage.ObjectMeta
			for k != nil && strings.HasPrefix(string(k), keyStr+"\x00") {
				var meta storage.ObjectMeta
				if err := json.Unmarshal(v, &meta); err == nil {
					keyVersions = append(keyVersions, meta)
				}
				k, v = c.Next()
			}

			sort.Slice(keyVersions, func(i, j int) bool {
				return keyVersions[i].LastModified.After(keyVersions[j].LastModified)
			})

			for _, meta := range keyVersions {
				if keyMarker == keyStr && versionIdMarker != "" {
					if meta.VersionID == versionIdMarker {
						versionIdMarker = ""
					}
					continue
				}
				
				objects = append(objects, storage.ObjectInfo{
					Key:            keyStr,
					VersionID:      meta.VersionID,
					IsLatest:       meta.IsLatest,
					IsDeleteMarker: meta.IsDeleteMarker,
					LastModified:   meta.LastModified,
					ETag:           meta.ETag,
					Size:           meta.Size,
					StorageClass:   meta.StorageClass,
				})

				if len(objects)+len(commonPrefixes) >= maxKeys {
					nextKeyMarker = keyStr
					nextVersionIdMarker = meta.VersionID
					return nil
				}
			}
		}
		return nil
	})

	return objects, commonPrefixes, nextKeyMarker, nextVersionIdMarker, err
}

func (s *bboltStore) CreateMultipartUpload(bucket, key string, meta storage.ObjectMeta) (string, error) {
	uploadID := uuid.New().String()

	err := s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketUploads)

		info := storage.UploadInfo{
			Key:       key,
			UploadID:  uploadID,
			Initiated: time.Now().UTC(),
			Owner:     "admin",
			Initiator: "admin",
		}

		data, err := json.Marshal(info)
		if err != nil {
			return err
		}

		// Key format for uploads bucket: bucket:key:uploadID
		k := fmt.Sprintf("%s:%s:%s", bucket, key, uploadID)
		if err := b.Put([]byte(k), data); err != nil {
			return err
		}

		// Save metadata
		metaData, err := json.Marshal(meta)
		if err != nil {
			return err
		}
		mk := fmt.Sprintf("meta:%s:%s:%s", bucket, key, uploadID)
		if err := b.Put([]byte(mk), metaData); err != nil {
			return err
		}

		_, err = tx.CreateBucketIfNotExists([]byte("parts:" + uploadID))
		return err
	})

	return uploadID, err
}

func (s *bboltStore) GetMultipartUpload(bucket, key, uploadID string) (*storage.ObjectMeta, error) {
	var meta storage.ObjectMeta
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketUploads)
		mk := fmt.Sprintf("meta:%s:%s:%s", bucket, key, uploadID)
		data := b.Get([]byte(mk))
		if data == nil {
			return ErrUploadNotFound
		}
		return json.Unmarshal(data, &meta)
	})
	return &meta, err
}

func (s *bboltStore) DeleteMultipartUpload(bucket, key, uploadID string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketUploads)
		k := fmt.Sprintf("%s:%s:%s", bucket, key, uploadID)
		mk := fmt.Sprintf("meta:%s:%s:%s", bucket, key, uploadID)

		if b.Get([]byte(k)) == nil {
			return ErrUploadNotFound
		}

		b.Delete([]byte(k))
		b.Delete([]byte(mk))

		tx.DeleteBucket([]byte("parts:" + uploadID))
		return nil
	})
}

func (s *bboltStore) PutObjectPart(uploadID string, partNum int, partInfo storage.PartInfo) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("parts:" + uploadID))
		if b == nil {
			return ErrUploadNotFound
		}

		data, err := json.Marshal(partInfo)
		if err != nil {
			return err
		}

		// Pad partNum so it sorts correctly
		k := fmt.Sprintf("%05d", partNum)
		return b.Put([]byte(k), data)
	})
}

func (s *bboltStore) ListObjectParts(uploadID string, partNumberMarker, maxParts int) ([]storage.PartInfo, int, error) {
	var parts []storage.PartInfo
	var nextMarker int

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("parts:" + uploadID))
		if b == nil {
			return ErrUploadNotFound
		}

		c := b.Cursor()
		k, v := c.First()
		if partNumberMarker > 0 {
			k, v = c.Seek([]byte(fmt.Sprintf("%05d", partNumberMarker)))
			if k != nil && string(k) == fmt.Sprintf("%05d", partNumberMarker) {
				k, v = c.Next()
			}
		}

		for ; k != nil; k, v = c.Next() {
			var part storage.PartInfo
			if err := json.Unmarshal(v, &part); err != nil {
				return err
			}
			parts = append(parts, part)

			if len(parts) >= maxParts {
				// Look ahead to see if there's more
				k2, _ := c.Next()
				if k2 != nil {
					nextMarker = part.PartNumber
				}
				break
			}
		}
		return nil
	})

	return parts, nextMarker, err
}

func (s *bboltStore) ListMultipartUploads(bucket, prefix, delimiter, keyMarker, uploadIDMarker string, maxUploads int) ([]storage.UploadInfo, []string, string, string, error) {
	var uploads []storage.UploadInfo
	var commonPrefixes []string
	var nextKeyMarker, nextUploadIDMarker string

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketUploads)
		c := b.Cursor()

		prefixBytes := []byte(bucket + ":")
		if prefix != "" {
			prefixBytes = []byte(bucket + ":" + prefix)
		}

		k, v := c.Seek(prefixBytes)

		// Very simplified marker logic for the stub
		// A full implementation would handle keyMarker & uploadIDMarker precisely.

		prefixes := make(map[string]bool)

		for ; k != nil && bytes.HasPrefix(k, []byte(bucket+":")); k, v = c.Next() {
			// key format: bucket:key:uploadID
			parts := strings.Split(string(k), ":")
			if len(parts) != 3 || parts[0] != "meta" && string(k) != parts[0]+":"+parts[1]+":"+parts[2] {
				// Skip meta records
				continue
			}
			keyStr := parts[1]

			if delimiter != "" {
				rem := keyStr[len(prefix):]
				if idx := strings.Index(rem, delimiter); idx >= 0 {
					pfx := prefix + rem[:idx+len(delimiter)]
					if !prefixes[pfx] {
						prefixes[pfx] = true
						commonPrefixes = append(commonPrefixes, pfx)
						if len(uploads)+len(commonPrefixes) >= maxUploads {
							nextKeyMarker = keyStr
							return nil
						}
					}
					continue
				}
			}

			var info storage.UploadInfo
			if err := json.Unmarshal(v, &info); err != nil {
				return err
			}
			uploads = append(uploads, info)

			if len(uploads)+len(commonPrefixes) >= maxUploads {
				nextKeyMarker = info.Key
				nextUploadIDMarker = info.UploadID
				return nil
			}
		}

		return nil
	})

	return uploads, commonPrefixes, nextKeyMarker, nextUploadIDMarker, err
}

// UserStore implementation

func (s *bboltStore) GetUserByAccessKey(ctx context.Context, accessKey string) (*auth.User, error) {
	var matched *auth.User
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketUsers)
		return b.ForEach(func(k, v []byte) error {
			var u auth.User
			if err := json.Unmarshal(v, &u); err == nil {
				if u.AccessKeyID == accessKey {
					matched = &u
					return errors.New("found")
				}
			}
			return nil
		})
	})
	if matched != nil {
		return matched, nil
	}
	if err != nil && err.Error() == "found" {
		return matched, nil
	}
	return nil, auth.ErrUserNotFound
}

func (s *bboltStore) GetUserByUsername(ctx context.Context, username string) (*auth.User, error) {
	var matched *auth.User
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketUsers)
		v := b.Get([]byte(username))
		if v == nil {
			return auth.ErrUserNotFound
		}
		var u auth.User
		if err := json.Unmarshal(v, &u); err != nil {
			return err
		}
		matched = &u
		return nil
	})
	return matched, err
}

func (s *bboltStore) ListUsers(ctx context.Context) ([]*auth.User, error) {
	var users []*auth.User
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketUsers)
		return b.ForEach(func(k, v []byte) error {
			var u auth.User
			if err := json.Unmarshal(v, &u); err != nil {
				return err
			}
			users = append(users, &u)
			return nil
		})
	})
	return users, err
}

func (s *bboltStore) CreateUser(ctx context.Context, user *auth.User) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketUsers)
		if b.Get([]byte(user.Username)) != nil {
			return auth.ErrUserExists
		}
		data, err := json.Marshal(user)
		if err != nil {
			return err
		}
		return b.Put([]byte(user.Username), data)
	})
}

func (s *bboltStore) UpdateUser(ctx context.Context, user *auth.User) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketUsers)
		if b.Get([]byte(user.Username)) == nil {
			return auth.ErrUserNotFound
		}
		data, err := json.Marshal(user)
		if err != nil {
			return err
		}
		return b.Put([]byte(user.Username), data)
	})
}

func (s *bboltStore) DeleteUser(ctx context.Context, username string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketUsers)
		if b.Get([]byte(username)) == nil {
			return auth.ErrUserNotFound
		}
		return b.Delete([]byte(username))
	})
}

// PolicyStore implementation

func (s *bboltStore) GetBucketPolicy(ctx context.Context, bucket string) (*auth.Policy, error) {
	var policy auth.Policy
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketPolicies)
		v := b.Get([]byte(bucket))
		if v == nil {
			return auth.ErrPolicyNotFound
		}
		return json.Unmarshal(v, &policy)
	})
	if err != nil {
		return nil, err
	}
	return &policy, nil
}

func (s *bboltStore) PutBucketPolicy(ctx context.Context, bucket string, policy *auth.Policy) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketPolicies)
		data, err := json.Marshal(policy)
		if err != nil {
			return err
		}
		return b.Put([]byte(bucket), data)
	})
}

func (s *bboltStore) DeleteBucketPolicy(ctx context.Context, bucket string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketPolicies)
		if b.Get([]byte(bucket)) == nil {
			return auth.ErrPolicyNotFound
		}
		return b.Delete([]byte(bucket))
	})
}

// CORS implementations

func (s *bboltStore) GetBucketCORS(ctx context.Context, bucket string) (*s3.CORSConfiguration, error) {
	var cors s3.CORSConfiguration
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketCORS)
		v := b.Get([]byte(bucket))
		if v == nil {
			return s3.ErrNoSuchBucket // Wait, usually NoSuchCORSConfiguration, we'll map later
		}
		return json.Unmarshal(v, &cors)
	})
	if err != nil {
		return nil, err
	}
	return &cors, nil
}

func (s *bboltStore) PutBucketCORS(ctx context.Context, bucket string, cors *s3.CORSConfiguration) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketCORS)
		data, err := json.Marshal(cors)
		if err != nil {
			return err
		}
		return b.Put([]byte(bucket), data)
	})
}

func (s *bboltStore) DeleteBucketCORS(ctx context.Context, bucket string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketCORS)
		return b.Delete([]byte(bucket))
	})
}

// Lifecycle operations

func (s *bboltStore) GetBucketLifecycle(ctx context.Context, bucket string) (*s3.LifecycleConfiguration, error) {
	var lifecycle s3.LifecycleConfiguration
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketLifecycle)
		data := b.Get([]byte(bucket))
		if data == nil {
			return s3.ErrNoSuchLifecycleConfiguration
		}
		return json.Unmarshal(data, &lifecycle)
	})
	if err != nil {
		return nil, err
	}
	return &lifecycle, nil
}

func (s *bboltStore) PutBucketLifecycle(ctx context.Context, bucket string, lifecycle *s3.LifecycleConfiguration) error {
	data, err := json.Marshal(lifecycle)
	if err != nil {
		return err
	}
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketLifecycle)
		return b.Put([]byte(bucket), data)
	})
}

func (s *bboltStore) DeleteBucketLifecycle(ctx context.Context, bucket string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket(bucketLifecycle).Delete([]byte(bucket))
	})
}
