package local

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
)

// AWSChunkedReader is an io.Reader that decodes AWS signature V4 chunked streams.
type AWSChunkedReader struct {
	reader   *bufio.Reader
	chunk    io.Reader
	chunkEOF bool
	eof      bool
	readErr  error
}

// NewAWSChunkedReader creates a new reader for AWS chunked data.
func NewAWSChunkedReader(r io.Reader) *AWSChunkedReader {
	return &AWSChunkedReader{
		reader: bufio.NewReader(r),
	}
}

func (cr *AWSChunkedReader) Read(p []byte) (int, error) {
	if cr.readErr != nil {
		return 0, cr.readErr
	}
	if cr.eof {
		return 0, io.EOF
	}

	for {
		if cr.chunk != nil {
			n, err := cr.chunk.Read(p)
			if err == io.EOF {
				cr.chunk = nil
				cr.chunkEOF = true
				continue
			}
			if err != nil {
				cr.readErr = err
				return n, err
			}
			return n, nil
		}

		if cr.chunkEOF {
			// Read trailing \r\n after chunk
			trail := make([]byte, 2)
			if _, err := io.ReadFull(cr.reader, trail); err != nil {
				cr.readErr = err
				return 0, err
			}
			cr.chunkEOF = false
		}

		// Read next chunk header
		// Format: {hex-size};chunk-signature={sig}\r\n
		line, err := cr.reader.ReadSlice('\n')
		if err != nil {
			cr.readErr = err
			return 0, err
		}

		// Remove \r\n
		line = bytes.TrimRight(line, "\r\n")

		// Split by ;
		parts := bytes.SplitN(line, []byte(";"), 2)
		if len(parts) == 0 {
			cr.readErr = errors.New("invalid chunk header format")
			return 0, cr.readErr
		}

		sizeHex := string(parts[0])
		var parsedSize int64
		_, err = fmt.Sscanf(sizeHex, "%x", &parsedSize)
		if err != nil {
			cr.readErr = fmt.Errorf("invalid chunk size: %s", sizeHex)
			return 0, cr.readErr
		}

		if parsedSize == 0 {
			// Last chunk is 0, followed by optional trailers and \r\n
			cr.eof = true
			return 0, io.EOF
		}

		// Setup new chunk reader for exactly parsedSize bytes
		cr.chunk = io.LimitReader(cr.reader, parsedSize)
	}
}

// IsAWSChunked checks if the request indicates AWS chunked uploading.
func IsAWSChunked(contentSha256 string) bool {
	return contentSha256 == "STREAMING-AWS4-HMAC-SHA256-PAYLOAD" ||
		contentSha256 == "STREAMING-AWS4-HMAC-SHA256-EVENTS"
}
