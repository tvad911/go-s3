package client

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

func hashSHA256(data []byte) string {
	h := sha256.New()
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

func hmacSHA256(key []byte, data string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(data))
	return h.Sum(nil)
}

func getSignatureKey(key, dateStamp, regionName, serviceName string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+key), dateStamp)
	kRegion := hmacSHA256(kDate, regionName)
	kService := hmacSHA256(kRegion, serviceName)
	kSigning := hmacSHA256(kService, "aws4_request")
	return kSigning
}

// SignRequestV4 signs an HTTP request with AWS Signature Version 4.
func SignRequestV4(req *http.Request, accessKey, secretKey, region, service string, signTime time.Time) {
	amzDate := signTime.UTC().Format("20060102T150405Z")
	dateStamp := signTime.UTC().Format("20060102")

	// 1. Prepare Request (Host, X-Amz-Date, X-Amz-Content-Sha256)
	if req.Header.Get("Host") == "" {
		req.Header.Set("Host", req.URL.Host)
	}
	req.Header.Set("x-amz-date", amzDate)

	payloadHash := req.Header.Get("x-amz-content-sha256")
	if payloadHash == "" {
		if req.Body == nil {
			payloadHash = hashSHA256([]byte(""))
		} else {
			// Read body to hash, then restore
			bodyBytes, _ := io.ReadAll(req.Body)
			payloadHash = hashSHA256(bodyBytes)
			req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		}
		req.Header.Set("x-amz-content-sha256", payloadHash)
	}

	// 2. Canonical Request
	var canonicalURI string
	if req.URL.Opaque != "" {
		canonicalURI = req.URL.Opaque
	} else {
		canonicalURI = req.URL.EscapedPath()
		if canonicalURI == "" {
			canonicalURI = "/"
		}
	}

	canonicalQueryString := strings.ReplaceAll(req.URL.Query().Encode(), "+", "%20")

	var headerNames []string
	for k := range req.Header {
		headerNames = append(headerNames, strings.ToLower(k))
	}
	sort.Strings(headerNames)

	var signedHeaders string
	var canonicalHeaders string
	for i, k := range headerNames {
		v := req.Header.Get(k)
		// space trimming for v is technically required but skipping for brevity
		canonicalHeaders += k + ":" + strings.TrimSpace(v) + "\n"
		signedHeaders += k
		if i < len(headerNames)-1 {
			signedHeaders += ";"
		}
	}

	canonicalRequest := fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s",
		req.Method,
		canonicalURI,
		canonicalQueryString,
		canonicalHeaders,
		signedHeaders,
		payloadHash)

	// 3. String to Sign
	stringToSign := fmt.Sprintf("AWS4-HMAC-SHA256\n%s\n%s/%s/%s/aws4_request\n%s",
		amzDate, dateStamp, region, service, hashSHA256([]byte(canonicalRequest)))

	// 4. Calculate Signature
	signingKey := getSignatureKey(secretKey, dateStamp, region, service)
	signature := hex.EncodeToString(hmacSHA256(signingKey, stringToSign))

	// 5. Add Authorization Header
	credential := fmt.Sprintf("%s/%s/%s/%s/aws4_request", accessKey, dateStamp, region, service)
	authHeader := fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s, SignedHeaders=%s, Signature=%s",
		credential, signedHeaders, signature)

	req.Header.Set("Authorization", authHeader)
}
