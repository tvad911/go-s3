package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"
)

func getSignatureKey(key, dateStamp, regionName, serviceName string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+key), dateStamp)
	kRegion := hmacSHA256(kDate, regionName)
	kService := hmacSHA256(kRegion, serviceName)
	kSigning := hmacSHA256(kService, "aws4_request")
	return kSigning
}

func hmacSHA256(key []byte, data string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(data))
	return h.Sum(nil)
}

func main() {
	bucket := "test-post-bucket-go"
	key := "hello.txt"
	content := []byte("Hello from POST upload via Go!")

	accessKey := "minioadmin"
	secretKey := "minioadmin"
	region := "us-east-1"
	service := "s3"

	t := time.Now().UTC()
	amzDate := t.Format("20060102T150405Z")
	dateStamp := t.Format("20060102")

	credential := fmt.Sprintf("%s/%s/%s/%s/aws4_request", accessKey, dateStamp, region, service)

	policyJSON := fmt.Sprintf(`{"expiration": "%s", "conditions": [{"bucket": "%s"}, ["starts-with", "$key", ""], {"x-amz-credential": "%s"}, {"x-amz-algorithm": "AWS4-HMAC-SHA256"}, {"x-amz-date": "%s"}]}`, t.Add(time.Hour).Format(time.RFC3339Nano), bucket, credential, amzDate)
	policyB64 := base64.StdEncoding.EncodeToString([]byte(policyJSON))

	signingKey := getSignatureKey(secretKey, dateStamp, region, service)
	signature := hex.EncodeToString(hmacSHA256(signingKey, policyB64))

	// Create multipart form
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	w.WriteField("key", key)
	w.WriteField("x-amz-credential", credential)
	w.WriteField("x-amz-algorithm", "AWS4-HMAC-SHA256")
	w.WriteField("x-amz-date", amzDate)
	w.WriteField("policy", policyB64)
	w.WriteField("x-amz-signature", signature)
	w.WriteField("success_action_status", "201")

	fw, _ := w.CreateFormFile("file", key)
	fw.Write(content)
	w.Close()

	// Send POST request
	req, _ := http.NewRequest("POST", "http://localhost:9099/"+bucket, &b)
	req.Header.Set("Content-Type", w.FormDataContentType())

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("Status: %d\n", resp.StatusCode)
	fmt.Printf("Response: %s\n", string(body))
}
