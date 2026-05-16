package handler_test

import (
	"bytes"
	"encoding/xml"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"gos3/client"
	"gos3/internal/s3"
)

// --- B.5: Multipart Upload Handlers ---

func TestMultipart_FullLifecycle(t *testing.T) {
	env := setupTestEnv(t)
	doSigned(t, env, "PUT", "/mp-bucket", nil).Body.Close()

	// 1. CreateMultipartUpload
	resp := doSigned(t, env, "POST", "/mp-bucket/bigfile.bin?uploads", nil)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("CreateMultipartUpload: expected 200, got %d: %s", resp.StatusCode, body)
	}

	var initResult s3.InitiateMultipartUploadResult
	if err := xml.Unmarshal(body, &initResult); err != nil {
		t.Fatalf("parse InitiateMultipartUploadResult: %v", err)
	}
	if initResult.UploadId == "" {
		t.Fatal("UploadId should not be empty")
	}
	uploadID := initResult.UploadId

	// 2. UploadPart (2 parts)
	part1Data := bytes.Repeat([]byte("A"), 1024)
	part2Data := bytes.Repeat([]byte("B"), 512)

	part1ETag := uploadPart(t, env, "/mp-bucket/bigfile.bin", uploadID, 1, part1Data)
	part2ETag := uploadPart(t, env, "/mp-bucket/bigfile.bin", uploadID, 2, part2Data)

	// 3. ListParts
	resp = doSigned(t, env, "GET", "/mp-bucket/bigfile.bin?uploadId="+uploadID, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("ListParts: expected 200, got %d: %s", resp.StatusCode, body)
	}

	// 4. CompleteMultipartUpload
	completeXML := buildCompleteXML([]partEntry{
		{Number: 1, ETag: part1ETag},
		{Number: 2, ETag: part2ETag},
	})

	resp = doSigned(t, env, "POST", "/mp-bucket/bigfile.bin?uploadId="+uploadID, completeXML)
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("CompleteMultipartUpload: expected 200, got %d: %s", resp.StatusCode, body)
	}

	var completeResult s3.CompleteMultipartUploadResult
	if err := xml.Unmarshal(body, &completeResult); err != nil {
		t.Fatalf("parse CompleteMultipartUploadResult: %v", err)
	}

	// ETag for multipart should be in format: md5-N
	if !strings.Contains(completeResult.ETag, "-2") {
		t.Errorf("multipart ETag should end with '-2', got %q", completeResult.ETag)
	}

	// 5. Verify object is readable
	resp = doSigned(t, env, "GET", "/mp-bucket/bigfile.bin", nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GetObject after complete: expected 200, got %d", resp.StatusCode)
	}
	got, _ := io.ReadAll(resp.Body)
	expected := append(part1Data, part2Data...)
	if !bytes.Equal(got, expected) {
		t.Errorf("content length mismatch: got %d, want %d", len(got), len(expected))
	}
}

func TestMultipart_Abort(t *testing.T) {
	env := setupTestEnv(t)
	doSigned(t, env, "PUT", "/abort-bucket", nil).Body.Close()

	// Create upload
	resp := doSigned(t, env, "POST", "/abort-bucket/aborted.bin?uploads", nil)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	var initResult s3.InitiateMultipartUploadResult
	xml.Unmarshal(body, &initResult)
	uploadID := initResult.UploadId

	// Upload a part
	uploadPart(t, env, "/abort-bucket/aborted.bin", uploadID, 1, []byte("partial"))

	// Abort
	resp = doSigned(t, env, "DELETE", "/abort-bucket/aborted.bin?uploadId="+uploadID, nil)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("AbortMultipartUpload: expected 204, got %d", resp.StatusCode)
	}

	// ListParts on aborted upload should fail
	resp = doSigned(t, env, "GET", "/abort-bucket/aborted.bin?uploadId="+uploadID, nil)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("ListParts after abort: expected 404, got %d", resp.StatusCode)
	}
}

func TestMultipart_NoSuchUpload(t *testing.T) {
	env := setupTestEnv(t)
	doSigned(t, env, "PUT", "/nsu-bucket", nil).Body.Close()

	resp := doSigned(t, env, "GET", "/nsu-bucket/file.txt?uploadId=fake-id", nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 for fake uploadId, got %d", resp.StatusCode)
	}
}

func TestMultipart_ListMultipartUploads(t *testing.T) {
	env := setupTestEnv(t)
	doSigned(t, env, "PUT", "/lmu-bucket", nil).Body.Close()

	// Create 2 uploads
	doSigned(t, env, "POST", "/lmu-bucket/file1.txt?uploads", nil).Body.Close()
	doSigned(t, env, "POST", "/lmu-bucket/file2.txt?uploads", nil).Body.Close()

	resp := doSigned(t, env, "GET", "/lmu-bucket/?uploads", nil)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("ListMultipartUploads: expected 200, got %d: %s", resp.StatusCode, body)
	}
}

// --- Helper types and functions ---

type partEntry struct {
	Number int
	ETag   string
}

// uploadPart sends a PUT request to upload a part and returns the ETag.
func uploadPart(t *testing.T, env *testEnv, objectPath, uploadID string, partNum int, data []byte) string {
	t.Helper()

	url := env.Server.URL + objectPath + "?partNumber=" + itoa(partNum) + "&uploadId=" + uploadID
	req, _ := http.NewRequest("PUT", url, bytes.NewReader(data))
	client.SignRequestV4(req, rootAccessKey(), rootSecretKey(), "us-east-1", "s3", time.Now())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("UploadPart %d: %v", partNum, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("UploadPart %d: expected 200, got %d: %s", partNum, resp.StatusCode, body)
	}

	etag := resp.Header.Get("ETag")
	// Strip quotes if present
	etag = strings.Trim(etag, `"`)
	return etag
}

// buildCompleteXML creates the CompleteMultipartUpload XML body.
func buildCompleteXML(parts []partEntry) []byte {
	var buf bytes.Buffer
	buf.WriteString("<CompleteMultipartUpload>")
	for _, p := range parts {
		buf.WriteString("<Part>")
		buf.WriteString("<PartNumber>" + itoa(p.Number) + "</PartNumber>")
		buf.WriteString("<ETag>" + p.ETag + "</ETag>")
		buf.WriteString("</Part>")
	}
	buf.WriteString("</CompleteMultipartUpload>")
	return buf.Bytes()
}

func itoa(i int) string {
	return strconv.Itoa(i)
}
