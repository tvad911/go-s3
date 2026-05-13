package client

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
)

const minPartSize = 5 * 1024 * 1024 // 5MB

// InitiateMultipartUploadResult represents the XML response.
type InitiateMultipartUploadResult struct {
	XMLName  xml.Name `xml:"InitiateMultipartUploadResult"`
	Bucket   string   `xml:"Bucket"`
	Key      string   `xml:"Key"`
	UploadId string   `xml:"UploadId"`
}

// CompleteMultipartUpload represents the XML request to complete a multipart upload.
type CompleteMultipartUpload struct {
	XMLName xml.Name            `xml:"CompleteMultipartUpload"`
	Parts   []CompletedPart `xml:"Part"`
}

// CompletedPart represents a part that has been uploaded.
type CompletedPart struct {
	PartNumber int    `xml:"PartNumber"`
	ETag       string `xml:"ETag"`
}

// PutObjectMultipart uploads a large file using S3 multipart upload.
func (c *Client) PutObjectMultipart(ctx context.Context, bucket, key string, filePath string, opts PutObjectOptions) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return err
	}
	totalSize := stat.Size()

	if totalSize < minPartSize {
		// Fallback to normal upload
		return c.PutObject(ctx, bucket, key, file, totalSize, opts)
	}

	// 1. Initiate Multipart Upload
	u := c.buildURL(bucket, key) + "?uploads"
	req, err := http.NewRequest("POST", u, nil)
	if err != nil {
		return err
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
	
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		return c.parseErrorResponse(resp)
	}

	var initRes InitiateMultipartUploadResult
	if err := xml.NewDecoder(resp.Body).Decode(&initRes); err != nil {
		resp.Body.Close()
		return err
	}
	resp.Body.Close()

	uploadID := initRes.UploadId

	// 2. Upload Parts
	// For simplicity, we upload parts sequentially in Phase 5.
	// Parallel upload can be added later.
	partSize := int64(minPartSize)
	var completedParts []CompletedPart
	
	for partNum := 1; ; partNum++ {
		offset := int64(partNum-1) * partSize
		if offset >= totalSize {
			break
		}

		size := partSize
		if offset+size > totalSize {
			size = totalSize - offset
		}

		partBody := io.NewSectionReader(file, offset, size)
		
		pu := c.buildURL(bucket, key) + fmt.Sprintf("?partNumber=%d&uploadId=%s", partNum, uploadID)
		preq, err := http.NewRequest("PUT", pu, partBody)
		if err != nil {
			return err // Should realistically abort multipart upload here
		}
		preq.ContentLength = size

		presp, err := c.doRequest(ctx, preq)
		if err != nil {
			return err
		}

		if presp.StatusCode != http.StatusOK {
			presp.Body.Close()
			return c.parseErrorResponse(presp)
		}

		etag := presp.Header.Get("ETag")
		presp.Body.Close()

		completedParts = append(completedParts, CompletedPart{
			PartNumber: partNum,
			ETag:       etag,
		})
	}

	// 3. Complete Multipart Upload
	compReq := CompleteMultipartUpload{Parts: completedParts}
	var compBuf bytes.Buffer
	compBuf.WriteString(xml.Header)
	xml.NewEncoder(&compBuf).Encode(compReq)

	cu := c.buildURL(bucket, key) + fmt.Sprintf("?uploadId=%s", uploadID)
	creq, err := http.NewRequest("POST", cu, &compBuf)
	if err != nil {
		return err
	}
	creq.ContentLength = int64(compBuf.Len())
	creq.Header.Set("Content-Type", "application/xml")

	cresp, err := c.doRequest(ctx, creq)
	if err != nil {
		return err
	}
	defer cresp.Body.Close()

	if cresp.StatusCode != http.StatusOK {
		return c.parseErrorResponse(cresp)
	}

	return nil
}
