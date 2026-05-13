package client

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
	"encoding/hex"
)

// PresignGetObject generates a presigned URL for downloading an object.
func (c *Client) PresignGetObject(bucket, key string, expires time.Duration) (string, error) {
	return c.presign("GET", bucket, key, expires)
}

// PresignPutObject generates a presigned URL for uploading an object.
func (c *Client) PresignPutObject(bucket, key string, expires time.Duration) (string, error) {
	return c.presign("PUT", bucket, key, expires)
}

func (c *Client) presign(method, bucket, key string, expires time.Duration) (string, error) {
	if c.cfg.AccessKeyID == "" || c.cfg.SecretAccessKey == "" {
		return "", fmt.Errorf("credentials required for presigning")
	}

	u := c.buildURL(bucket, key)
	parsed, err := url.Parse(u)
	if err != nil {
		return "", err
	}

	now := time.Now().UTC()
	amzDate := now.Format("20060102T150405Z")
	dateStamp := now.Format("20060102")
	service := "s3"
	region := c.cfg.Region

	credential := fmt.Sprintf("%s/%s/%s/%s/aws4_request", c.cfg.AccessKeyID, dateStamp, region, service)
	signedHeaders := "host"

	q := parsed.Query()
	q.Set("X-Amz-Algorithm", "AWS4-HMAC-SHA256")
	q.Set("X-Amz-Credential", credential)
	q.Set("X-Amz-Date", amzDate)
	q.Set("X-Amz-Expires", strconv.FormatInt(int64(expires.Seconds()), 10))
	q.Set("X-Amz-SignedHeaders", signedHeaders)

	host := parsed.Host
	if host == "" {
		host = parsed.String()
	}

	canonicalURI := parsed.EscapedPath()
	if canonicalURI == "" {
		canonicalURI = "/"
	}

	canonicalQueryString := strings.ReplaceAll(q.Encode(), "+", "%20")
	canonicalHeaders := "host:" + host + "\n"
	payloadHash := "UNSIGNED-PAYLOAD"

	canonicalRequest := fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s",
		method,
		canonicalURI,
		canonicalQueryString,
		canonicalHeaders,
		signedHeaders,
		payloadHash)

	stringToSign := fmt.Sprintf("AWS4-HMAC-SHA256\n%s\n%s/%s/%s/aws4_request\n%s",
		amzDate, dateStamp, region, service, hashSHA256([]byte(canonicalRequest)))

	signingKey := getSignatureKey(c.cfg.SecretAccessKey, dateStamp, region, service)
	signature := hex.EncodeToString(hmacSHA256(signingKey, stringToSign))

	q.Set("X-Amz-Signature", signature)
	parsed.RawQuery = strings.ReplaceAll(q.Encode(), "+", "%20")

	return parsed.String(), nil
}
