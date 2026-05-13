package client

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
)

// MakeBucket creates a new bucket.
func (c *Client) MakeBucket(ctx context.Context, bucket string) error {
	u := c.buildURL(bucket, "")
	req, err := http.NewRequest("PUT", u, nil)
	if err != nil {
		return err
	}

	resp, err := c.doRequest(ctx, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return c.parseErrorResponse(resp)
	}
	return nil
}

// RemoveBucket deletes an empty bucket.
func (c *Client) RemoveBucket(ctx context.Context, bucket string) error {
	u := c.buildURL(bucket, "")
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

// BucketExists checks if a bucket exists.
func (c *Client) BucketExists(ctx context.Context, bucket string) (bool, error) {
	u := c.buildURL(bucket, "")
	req, err := http.NewRequest("HEAD", u, nil)
	if err != nil {
		return false, err
	}

	resp, err := c.doRequest(ctx, req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return true, nil
	}
	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	return false, c.parseErrorResponse(resp)
}

// ListBuckets returns a list of all buckets.
func (c *Client) ListBuckets(ctx context.Context) ([]Bucket, error) {
	u := c.buildURL("", "")
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.parseErrorResponse(resp)
	}

	var res ListAllMyBucketsResult
	if err := xml.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}

	var buckets []Bucket
	for _, b := range res.Buckets.Buckets {
		buckets = append(buckets, Bucket{
			Name:         b.Name,
			CreationDate: b.CreationDate,
		})
	}

	return buckets, nil
}

// parseErrorResponse attempts to parse an S3 XML error, otherwise returns a generic error.
func (c *Client) parseErrorResponse(resp *http.Response) error {
	bodyBytes, _ := io.ReadAll(resp.Body)
	var errResp ErrorResponse
	if err := xml.Unmarshal(bodyBytes, &errResp); err == nil && errResp.Code != "" {
		errResp.HTTPStatus = resp.StatusCode
		return errResp
	}
	return fmt.Errorf("HTTP error %d: %s", resp.StatusCode, string(bodyBytes))
}
