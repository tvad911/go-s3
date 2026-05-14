package client

import (
	"context"
	"io"
	"net/http"
)

// DoAdminRequest executes an admin API request to GoS3 server.
func (c *Client) DoAdminRequest(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	u := c.buildURL("", "")
	// remove trailing slash
	if len(u) > 0 && u[len(u)-1] == '/' {
		u = u[:len(u)-1]
	}
	u = u + "/_admin" + path

	req, err := http.NewRequest(method, u, body)
	if err != nil {
		return nil, err
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return c.doRequest(ctx, req)
}
