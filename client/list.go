package client

import (
	"context"
	"encoding/xml"
	"net/http"
	"net/url"
)

// ListObjectsOptions holds options for listing objects.
type ListObjectsOptions struct {
	Prefix    string
	Delimiter string
	Recursive bool
}

// ListObjectsV2 lists objects using the V2 API, returning a channel of Objects.
func (c *Client) ListObjectsV2(ctx context.Context, bucket string, opts ListObjectsOptions) <-chan Object {
	ch := make(chan Object)

	go func() {
		defer close(ch)

		var continuationToken string
		for {
			u := c.buildURL(bucket, "")
			parsed, _ := url.Parse(u)
			q := parsed.Query()
			q.Set("list-type", "2")

			if opts.Prefix != "" {
				q.Set("prefix", opts.Prefix)
			}
			if !opts.Recursive && opts.Delimiter == "" {
				opts.Delimiter = "/"
			}
			if opts.Delimiter != "" {
				q.Set("delimiter", opts.Delimiter)
			}
			if continuationToken != "" {
				q.Set("continuation-token", continuationToken)
			}
			parsed.RawQuery = q.Encode()

			req, err := http.NewRequest("GET", parsed.String(), nil)
			if err != nil {
				return // silently abort on error for simple channel design, or we could pass err to another channel
			}

			resp, err := c.doRequest(ctx, req)
			if err != nil {
				return
			}

			if resp.StatusCode != http.StatusOK {
				resp.Body.Close()
				return
			}

			var res ListBucketResultV2
			if err := xml.NewDecoder(resp.Body).Decode(&res); err != nil {
				resp.Body.Close()
				return
			}
			resp.Body.Close()

			for _, obj := range res.Contents {
				select {
				case <-ctx.Done():
					return
				case ch <- obj:
				}
			}

			// Add common prefixes as "directory" objects
			for _, cp := range res.CommonPrefixes {
				select {
				case <-ctx.Done():
					return
				case ch <- Object{Key: cp.Prefix}: // size 0 indicates prefix
				}
			}

			if !res.IsTruncated {
				break
			}
			continuationToken = res.NextContinuationToken
		}
	}()

	return ch
}

// ListObjects lists objects using the V1 API.
func (c *Client) ListObjects(ctx context.Context, bucket string, opts ListObjectsOptions) <-chan Object {
	ch := make(chan Object)

	go func() {
		defer close(ch)

		var marker string
		for {
			u := c.buildURL(bucket, "")
			parsed, _ := url.Parse(u)
			q := parsed.Query()

			if opts.Prefix != "" {
				q.Set("prefix", opts.Prefix)
			}
			if !opts.Recursive && opts.Delimiter == "" {
				opts.Delimiter = "/"
			}
			if opts.Delimiter != "" {
				q.Set("delimiter", opts.Delimiter)
			}
			if marker != "" {
				q.Set("marker", marker)
			}
			parsed.RawQuery = q.Encode()

			req, err := http.NewRequest("GET", parsed.String(), nil)
			if err != nil {
				return
			}

			resp, err := c.doRequest(ctx, req)
			if err != nil {
				return
			}

			if resp.StatusCode != http.StatusOK {
				resp.Body.Close()
				return
			}

			var res ListBucketResult
			if err := xml.NewDecoder(resp.Body).Decode(&res); err != nil {
				resp.Body.Close()
				return
			}
			resp.Body.Close()

			for _, obj := range res.Contents {
				select {
				case <-ctx.Done():
					return
				case ch <- obj:
				}
			}

			for _, cp := range res.CommonPrefixes {
				select {
				case <-ctx.Done():
					return
				case ch <- Object{Key: cp.Prefix}:
				}
			}

			if !res.IsTruncated {
				break
			}
			marker = res.NextMarker
			if marker == "" && len(res.Contents) > 0 {
				marker = res.Contents[len(res.Contents)-1].Key
			}
		}
	}()

	return ch
}
