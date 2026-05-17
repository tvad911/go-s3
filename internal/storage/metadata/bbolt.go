package metadata

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	bucketBuckets         = []byte("buckets")
	bucketUploads         = []byte("uploads")
	bucketUsers           = []byte("users")
	bucketPolicies        = []byte("policies")
	bucketCORS            = []byte("cors")
	bucketLifecycle       = []byte("lifecycle")
	bucketWebsite         = []byte("website")
	bucketServiceAccounts = []byte("service_accounts")
	bucketAuditLogs       = []byte("audit_logs")
	bucketIAMPolicies     = []byte("iam_policies")
	bucketCustomDomains   = []byte("custom_domains")
	bucketNotifications   = []byte("notifications")
	bucketSettings        = []byte("settings")
	bucketSessions        = []byte("sessions")
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
		if _, err := tx.CreateBucketIfNotExists(bucketWebsite); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(bucketServiceAccounts); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(bucketAuditLogs); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(bucketIAMPolicies); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(bucketCustomDomains); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(bucketNotifications); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(bucketSettings); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(bucketSessions); err != nil {
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

func (s *bboltStore) CreateBucket(name, region, owner, acl string, objectLockEnabled bool) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketBuckets)
		if b.Get([]byte(name)) != nil {
			return ErrBucketExists
		}

		info := storage.BucketInfo{
			Name:              name,
			CreationDate:      time.Now().UTC(),
			Region:            region,
			Owner:             owner,
			ACL:               acl,
			ObjectLockEnabled: objectLockEnabled,
		}
		if objectLockEnabled {
			info.Versioning = "Enabled"
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

func (s *bboltStore) UpdateUserPolicies(ctx context.Context, username string, policies []string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketUsers)
		v := b.Get([]byte(username))
		if v == nil {
			return auth.ErrUserNotFound
		}
		var user auth.User
		if err := json.Unmarshal(v, &user); err != nil {
			return err
		}
		user.Policies = policies
		data, err := json.Marshal(&user)
		if err != nil {
			return err
		}
		return b.Put([]byte(username), data)
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

// IAMPolicyStore implementation

func (s *bboltStore) GetIAMPolicy(ctx context.Context, name string) (*auth.Policy, error) {
	var policy auth.Policy
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketIAMPolicies)
		v := b.Get([]byte(name))
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

func (s *bboltStore) PutIAMPolicy(ctx context.Context, name string, policy *auth.Policy) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketIAMPolicies)
		data, err := json.Marshal(policy)
		if err != nil {
			return err
		}
		return b.Put([]byte(name), data)
	})
}

func (s *bboltStore) DeleteIAMPolicy(ctx context.Context, name string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketIAMPolicies)
		if b.Get([]byte(name)) == nil {
			return auth.ErrPolicyNotFound
		}
		return b.Delete([]byte(name))
	})
}

func (s *bboltStore) ListIAMPolicies(ctx context.Context) ([]string, error) {
	var names []string
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketIAMPolicies)
		return b.ForEach(func(k, v []byte) error {
			names = append(names, string(k))
			return nil
		})
	})
	return names, err
}

// CORS implementations

func (s *bboltStore) GetBucketCORS(ctx context.Context, bucket string) (*s3.CORSConfiguration, error) {
	var cors s3.CORSConfiguration
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketCORS)
		v := b.Get([]byte(bucket))
		if v == nil {
			return errors.New("NoSuchCORSConfiguration")
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

// Website operations

func (s *bboltStore) GetBucketWebsite(ctx context.Context, bucket string) (*s3.WebsiteConfiguration, error) {
	var website s3.WebsiteConfiguration
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketWebsite)
		data := b.Get([]byte(bucket))
		if data == nil {
			return s3.ErrNoSuchWebsiteConfiguration
		}
		return json.Unmarshal(data, &website)
	})
	if err != nil {
		return nil, err
	}
	return &website, nil
}

func (s *bboltStore) PutBucketWebsite(ctx context.Context, bucket string, website *s3.WebsiteConfiguration) error {
	data, err := json.Marshal(website)
	if err != nil {
		return err
	}
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketWebsite)
		return b.Put([]byte(bucket), data)
	})
}

func (s *bboltStore) DeleteBucketWebsite(ctx context.Context, bucket string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket(bucketWebsite).Delete([]byte(bucket))
	})
}

// Custom Domain operations

func (s *bboltStore) PutCustomDomain(ctx context.Context, domain string, bucket string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketCustomDomains)
		return b.Put([]byte(domain), []byte(bucket))
	})
}

func (s *bboltStore) GetCustomDomain(ctx context.Context, domain string) (string, error) {
	var bucket string
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketCustomDomains)
		data := b.Get([]byte(domain))
		if data == nil {
			return errors.New("custom domain not found")
		}
		bucket = string(data)
		return nil
	})
	if err != nil {
		return "", err
	}
	return bucket, nil
}

func (s *bboltStore) DeleteCustomDomain(ctx context.Context, domain string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket(bucketCustomDomains).Delete([]byte(domain))
	})
}

func (s *bboltStore) GetBucketCustomDomains(ctx context.Context, bucket string) ([]string, error) {
	var domains []string
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketCustomDomains)
		return b.ForEach(func(k, v []byte) error {
			if string(v) == bucket {
				domains = append(domains, string(k))
			}
			return nil
		})
	})
	return domains, err
}

// Notification operations

func (s *bboltStore) GetBucketNotification(ctx context.Context, bucket string) (*s3.NotificationConfiguration, error) {
	var notification s3.NotificationConfiguration
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketNotifications)
		data := b.Get([]byte(bucket))
		if data == nil {
			return errors.New("notification configuration not found")
		}
		return json.Unmarshal(data, &notification)
	})
	if err != nil {
		return nil, err
	}
	return &notification, nil
}

func (s *bboltStore) PutBucketNotification(ctx context.Context, bucket string, config *s3.NotificationConfiguration) error {
	data, err := json.Marshal(config)
	if err != nil {
		return err
	}
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketNotifications)
		return b.Put([]byte(bucket), data)
	})
}

func (s *bboltStore) DeleteBucketNotification(ctx context.Context, bucket string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket(bucketNotifications).Delete([]byte(bucket))
	})
}

// ServiceAccountStore implementation

func (s *bboltStore) CreateServiceAccount(ctx context.Context, sa *auth.ServiceAccount) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketServiceAccounts)
		if b.Get([]byte(sa.AccessKeyID)) != nil {
			return fmt.Errorf("service account with access key %s already exists", sa.AccessKeyID)
		}
		data, err := json.Marshal(sa)
		if err != nil {
			return err
		}
		return b.Put([]byte(sa.AccessKeyID), data)
	})
}

func (s *bboltStore) GetServiceAccountByAccessKey(ctx context.Context, accessKey string) (*auth.ServiceAccount, error) {
	var sa auth.ServiceAccount
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketServiceAccounts)
		v := b.Get([]byte(accessKey))
		if v == nil {
			return auth.ErrUserNotFound
		}
		return json.Unmarshal(v, &sa)
	})
	if err != nil {
		return nil, err
	}
	return &sa, nil
}

func (s *bboltStore) ListServiceAccountsByUser(ctx context.Context, parentUser string) ([]*auth.ServiceAccount, error) {
	var accounts []*auth.ServiceAccount
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketServiceAccounts)
		return b.ForEach(func(k, v []byte) error {
			var sa auth.ServiceAccount
			if err := json.Unmarshal(v, &sa); err != nil {
				return err
			}
			if sa.ParentUser == parentUser {
				// Clear secret key from list responses
				sa.SecretKey = ""
				accounts = append(accounts, &sa)
			}
			return nil
		})
	})
	return accounts, err
}

func (s *bboltStore) DeleteServiceAccount(ctx context.Context, id string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketServiceAccounts)
		// Find by ID (iterate since key is AccessKeyID, not ID)
		var keyToDelete []byte
		err := b.ForEach(func(k, v []byte) error {
			var sa auth.ServiceAccount
			if err := json.Unmarshal(v, &sa); err != nil {
				return err
			}
			if sa.ID == id {
				keyToDelete = k
			}
			return nil
		})
		if err != nil {
			return err
		}
		if keyToDelete == nil {
			return auth.ErrUserNotFound
		}
		return b.Delete(keyToDelete)
	})
}

func (s *bboltStore) DisableServiceAccount(ctx context.Context, id string, disabled bool) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketServiceAccounts)
		var found *auth.ServiceAccount
		var foundKey []byte
		err := b.ForEach(func(k, v []byte) error {
			var sa auth.ServiceAccount
			if err := json.Unmarshal(v, &sa); err != nil {
				return err
			}
			if sa.ID == id {
				found = &sa
				foundKey = k
			}
			return nil
		})
		if err != nil {
			return err
		}
		if found == nil {
			return auth.ErrUserNotFound
		}
		found.Disabled = disabled
		data, err := json.Marshal(found)
		if err != nil {
			return err
		}
		return b.Put(foundKey, data)
	})
}

func (s *bboltStore) UpdateServiceAccountPolicies(ctx context.Context, id string, policies []string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketServiceAccounts)
		var found *auth.ServiceAccount
		var foundKey []byte
		err := b.ForEach(func(k, v []byte) error {
			var sa auth.ServiceAccount
			if err := json.Unmarshal(v, &sa); err != nil {
				return err
			}
			if sa.ID == id {
				found = &sa
				foundKey = k
			}
			return nil
		})
		if err != nil {
			return err
		}
		if found == nil {
			return auth.ErrUserNotFound
		}
		found.Policies = policies
		data, err := json.Marshal(found)
		if err != nil {
			return err
		}
		return b.Put(foundKey, data)
	})
}

// GetBucketStats returns the total number of objects and bytes for a bucket.
func (db *bboltStore) GetBucketStats(name string) (int64, int64, error) {
	var objects, bytes int64
	err := db.db.View(func(tx *bbolt.Tx) error {
		bucketKey := []byte("objects:" + name)
		b := tx.Bucket(bucketKey)
		if b == nil {
			return nil // empty bucket
		}

		return b.ForEach(func(k, v []byte) error {
			// Object keys might include versions, let's just count total non-delete-marker bytes
			var meta storage.ObjectMeta
			if err := json.Unmarshal(v, &meta); err == nil {
				if !meta.IsDeleteMarker {
					objects++
					bytes += meta.Size
				}
			}
			return nil
		})
	})
	return objects, bytes, err
}

// ==================== Audit Logs ====================

func (b *bboltStore) RecordAuditLog(ctx context.Context, log *AuditLog) error {
	if log.ID == "" {
		log.ID = uuid.New().String()
	}
	if log.Timestamp.IsZero() {
		log.Timestamp = time.Now()
	}

	return b.db.Update(func(tx *bbolt.Tx) error {
		bkt := tx.Bucket(bucketAuditLogs)
		if bkt == nil {
			return errors.New("audit_logs bucket not found")
		}

		// Use RFC3339Nano for sorting chronologically
		key := []byte(log.Timestamp.UTC().Format(time.RFC3339Nano) + "_" + log.ID)
		val, err := json.Marshal(log)
		if err != nil {
			return err
		}
		return bkt.Put(key, val)
	})
}

func (b *bboltStore) ListAuditLogs(ctx context.Context, limit int) ([]AuditLog, error) {
	var logs []AuditLog
	err := b.db.View(func(tx *bbolt.Tx) error {
		bkt := tx.Bucket(bucketAuditLogs)
		if bkt == nil {
			return nil
		}

		c := bkt.Cursor()
		// Start from the last item and go backwards (newest first)
		for k, v := c.Last(); k != nil; k, v = c.Prev() {
			var log AuditLog
			if err := json.Unmarshal(v, &log); err != nil {
				continue
			}
			logs = append(logs, log)
			if limit > 0 && len(logs) >= limit {
				break
			}
		}
		return nil
	})
	return logs, err
}

func (s *bboltStore) BackupTo(w io.Writer) error {
	return s.db.View(func(tx *bbolt.Tx) error {
		_, err := tx.WriteTo(w)
		return err
	})
}

// SettingStore implementation

func (s *bboltStore) GetSetting(ctx context.Context, key string) (string, error) {
	var val string
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketSettings)
		v := b.Get([]byte(key))
		if v == nil {
			return errors.New("setting not found")
		}
		val = string(v)
		return nil
	})
	return val, err
}

func (s *bboltStore) PutSetting(ctx context.Context, key string, value string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketSettings)
		return b.Put([]byte(key), []byte(value))
	})
}

func (s *bboltStore) DeleteSetting(ctx context.Context, key string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketSettings)
		return b.Delete([]byte(key))
	})
}

func (s *bboltStore) ListSettings(ctx context.Context) (map[string]string, error) {
	settings := make(map[string]string)
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketSettings)
		return b.ForEach(func(k, v []byte) error {
			settings[string(k)] = string(v)
			return nil
		})
	})
	return settings, err
}

// SessionStore implementation

func (s *bboltStore) CreateSession(ctx context.Context, session *auth.Session) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketSessions)
		data, err := json.Marshal(session)
		if err != nil {
			return err
		}
		return b.Put([]byte(session.ID), data)
	})
}

func (s *bboltStore) GetSession(ctx context.Context, sessionID string) (*auth.Session, error) {
	var session auth.Session
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketSessions)
		v := b.Get([]byte(sessionID))
		if v == nil {
			return errors.New("session not found")
		}
		return json.Unmarshal(v, &session)
	})
	if err != nil {
		return nil, err
	}
	return &session, nil
}

func (s *bboltStore) DeleteSession(ctx context.Context, sessionID string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketSessions)
		return b.Delete([]byte(sessionID))
	})
}

func (s *bboltStore) ListSessions(ctx context.Context, username string) ([]*auth.Session, error) {
	var sessions []*auth.Session
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketSessions)
		return b.ForEach(func(k, v []byte) error {
			var sess auth.Session
			if err := json.Unmarshal(v, &sess); err == nil {
				if sess.Username == username {
					sessions = append(sessions, &sess)
				}
			}
			return nil
		})
	})
	return sessions, err
}
