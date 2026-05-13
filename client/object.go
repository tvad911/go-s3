package client

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
)

// ObjectInfo represents metadata about an S3 object.
type ObjectInfo struct {
	Key          string
	Size         int64
	ETag         string
	LastModified string
	ContentType  string
	Metadata     map[string]string
}

// PutObjectOptions holds options for PutObject.
type PutObjectOptions struct {
	ContentType string
	UserMeta    map[string]string
}

// PutObject uploads an object from an io.Reader.
func (c *Client) PutObject(ctx context.Context, bucket, key string, body io.Reader, size int64, opts PutObjectOptions) error {
	u := c.buildURL(bucket, key)
	req, err := http.NewRequest("PUT", u, body)
	if err != nil {
		return err
	}

	if size >= 0 {
		req.ContentLength = size
	}
	if opts.ContentType != "" {
		req.Header.Set("Content-Type", opts.ContentType)
	}
	for k, v := range opts.UserMeta {
		req.Header.Set("x-amz-meta-"+k, v)
	}

	resp, err := c.doRequest(ctx, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return c.parseErrorResponse(resp)
	}
	return nil
}

// FPutObject uploads an object from a local file.
func (c *Client) FPutObject(ctx context.Context, bucket, key string, filePath string, opts PutObjectOptions) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return err
	}

	return c.PutObject(ctx, bucket, key, file, stat.Size(), opts)
}

// GetObjectOptions holds options for GetObject.
type GetObjectOptions struct{}

// GetObject downloads an object and returns an io.ReadCloser.
// The caller is responsible for closing the returned reader.
func (c *Client) GetObject(ctx context.Context, bucket, key string, opts GetObjectOptions) (io.ReadCloser, ObjectInfo, error) {
	u := c.buildURL(bucket, key)
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, ObjectInfo{}, err
	}

	resp, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, ObjectInfo{}, err
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		return nil, ObjectInfo{}, c.parseErrorResponse(resp)
	}

	info := ObjectInfo{
		Key:          key,
		Size:         resp.ContentLength,
		ETag:         resp.Header.Get("ETag"),
		LastModified: resp.Header.Get("Last-Modified"),
		ContentType:  resp.Header.Get("Content-Type"),
		Metadata:     make(map[string]string),
	}

	for k, v := range resp.Header {
		if len(k) > 10 && k[:10] == "X-Amz-Meta" {
			info.Metadata[k] = v[0]
		}
	}

	return resp.Body, info, nil
}

// FGetObject downloads an object to a local file.
func (c *Client) FGetObject(ctx context.Context, bucket, key string, filePath string, opts GetObjectOptions) error {
	reader, _, err := c.GetObject(ctx, bucket, key, opts)
	if err != nil {
		return err
	}
	defer reader.Close()

	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, reader)
	return err
}

// StatObject retrieves object metadata.
func (c *Client) StatObject(ctx context.Context, bucket, key string) (ObjectInfo, error) {
	u := c.buildURL(bucket, key)
	req, err := http.NewRequest("HEAD", u, nil)
	if err != nil {
		return ObjectInfo{}, err
	}

	resp, err := c.doRequest(ctx, req)
	if err != nil {
		return ObjectInfo{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ObjectInfo{}, c.parseErrorResponse(resp)
	}

	info := ObjectInfo{
		Key:          key,
		Size:         resp.ContentLength,
		ETag:         resp.Header.Get("ETag"),
		LastModified: resp.Header.Get("Last-Modified"),
		ContentType:  resp.Header.Get("Content-Type"),
		Metadata:     make(map[string]string),
	}

	for k, v := range resp.Header {
		if len(k) > 10 && k[:10] == "X-Amz-Meta" {
			info.Metadata[k] = v[0]
		}
	}

	return info, nil
}

// RemoveObject deletes an object.
func (c *Client) RemoveObject(ctx context.Context, bucket, key string) error {
	u := c.buildURL(bucket, key)
	req, err := http.NewRequest("DELETE", u, nil)
	if err != nil {
		return err
	}

	resp, err := c.doRequest(ctx, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return c.parseErrorResponse(resp)
	}
	return nil
}

// CopyObject copies an object.
func (c *Client) CopyObject(ctx context.Context, srcBucket, srcKey, destBucket, destKey string) error {
	u := c.buildURL(destBucket, destKey)
	req, err := http.NewRequest("PUT", u, nil)
	if err != nil {
		return err
	}

	req.Header.Set("x-amz-copy-source", fmt.Sprintf("/%s/%s", srcBucket, srcKey))

	resp, err := c.doRequest(ctx, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return c.parseErrorResponse(resp)
	}
	return nil
}

// RemoveObjects deletes multiple objects.
func (c *Client) RemoveObjects(ctx context.Context, bucket string, keys []string) error {
	// For simplicity in Phase 5, we can just call RemoveObject in a loop, or implement the XML bulk delete.
	// We'll just loop for now, real implementation would use POST /bucket?delete.
	for _, key := range keys {
		if err := c.RemoveObject(ctx, bucket, key); err != nil {
			return err
		}
	}
	return nil
}
