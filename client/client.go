package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Config represents the client configuration.
type Config struct {
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	Region          string
	UseSSL          bool
	PathStyle       bool // true = path-style (endpoint/bucket/key)
}

// Client is a GoS3 (and S3-compatible) client.
type Client struct {
	cfg        Config
	httpClient *http.Client
	endpoint   *url.URL
}

// New creates a new GoS3 client.
func New(cfg Config) (*Client, error) {
	if cfg.Endpoint == "" {
		return nil, fmt.Errorf("endpoint is required")
	}
	if !strings.HasPrefix(cfg.Endpoint, "http://") && !strings.HasPrefix(cfg.Endpoint, "https://") {
		if cfg.UseSSL {
			cfg.Endpoint = "https://" + cfg.Endpoint
		} else {
			cfg.Endpoint = "http://" + cfg.Endpoint
		}
	}

	u, err := url.Parse(cfg.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("invalid endpoint: %w", err)
	}

	if cfg.Region == "" {
		cfg.Region = "us-east-1"
	}

	return &Client{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		endpoint:   u,
	}, nil
}

// buildURL constructs the URL for a request, respecting PathStyle.
func (c *Client) buildURL(bucket, key string) string {
	u := *c.endpoint // copy

	// Always use PathStyle for this minimal implementation
	// If path style: http://endpoint/bucket/key
	// If not path style: http://bucket.endpoint/key
	// S3 defaults to virtual-hosted style now, but our server expects path style or custom handling.
	// For simplicity, let's just stick to PathStyle=true semantics unless Host is manipulated.

	path := "/"
	if bucket != "" {
		if !c.cfg.PathStyle {
			// virtual-hosted style
			u.Host = bucket + "." + u.Host
			if key != "" {
				path += key
			}
		} else {
			// path style
			path += bucket
			if key != "" {
				path += "/" + key
			}
		}
	}
	u.Path = path
	return u.String()
}

// doRequest signs and executes the HTTP request.
func (c *Client) doRequest(ctx context.Context, req *http.Request) (*http.Response, error) {
	req = req.WithContext(ctx)

	// Sign the request
	if c.cfg.AccessKeyID != "" && c.cfg.SecretAccessKey != "" {
		SignRequestV4(req, c.cfg.AccessKeyID, c.cfg.SecretAccessKey, c.cfg.Region, "s3", time.Now())
	}

	return c.httpClient.Do(req)
}
