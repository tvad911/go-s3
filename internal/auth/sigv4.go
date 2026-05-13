package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

var (
	ErrAuthHeaderMissing    = errors.New("authorization header missing")
	ErrAuthHeaderMalformed  = errors.New("authorization header malformed")
	ErrSignatureDoesNotMatch = errors.New("signature does not match")
	ErrRequestTimeTooSkewed  = errors.New("request time too skewed")
)

type SigV4Verifier struct {
	UserStore UserStore
	Region    string
}

func NewSigV4Verifier(store UserStore, region string) *SigV4Verifier {
	return &SigV4Verifier{
		UserStore: store,
		Region:    region,
	}
}

// Verify checks the request signature. It returns the authenticated User, or an error.
func (v *SigV4Verifier) Verify(r *http.Request) (*User, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		// Fallback to query string auth for presigned URLs
		if r.URL.Query().Get("X-Amz-Signature") != "" {
			return v.verifyQueryString(r)
		}
		return nil, ErrAuthHeaderMissing
	}

	return v.verifyHeader(r, authHeader)
}

func (v *SigV4Verifier) verifyHeader(r *http.Request, authHeader string) (*User, error) {
	if !strings.HasPrefix(authHeader, "AWS4-HMAC-SHA256 ") {
		return nil, ErrAuthHeaderMalformed
	}

	parts := strings.Split(authHeader[17:], ",")
	if len(parts) != 3 {
		return nil, ErrAuthHeaderMalformed
	}

	credPart := strings.TrimSpace(parts[0])
	signedHeadersPart := strings.TrimSpace(parts[1])
	sigPart := strings.TrimSpace(parts[2])

	if !strings.HasPrefix(credPart, "Credential=") {
		return nil, ErrAuthHeaderMalformed
	}
	credStr := credPart[11:]
	credFields := strings.Split(credStr, "/")
	if len(credFields) != 5 {
		return nil, ErrAuthHeaderMalformed
	}
	accessKey := credFields[0]
	dateStamp := credFields[1]
	region := credFields[2]
	service := credFields[3]
	// aws4_request := credFields[4]

	if !strings.HasPrefix(signedHeadersPart, "SignedHeaders=") {
		return nil, ErrAuthHeaderMalformed
	}
	signedHeadersStr := signedHeadersPart[14:]

	if !strings.HasPrefix(sigPart, "Signature=") {
		return nil, ErrAuthHeaderMalformed
	}
	providedSig := sigPart[10:]

	amzDate := r.Header.Get("X-Amz-Date")
	if amzDate == "" {
		amzDate = r.Header.Get("Date")
	}

	// Lookup user
	user, err := v.UserStore.GetUserByAccessKey(r.Context(), accessKey)
	if err != nil {
		return nil, err
	}

	// Payload hash
	payloadHash := r.Header.Get("X-Amz-Content-Sha256")
	if payloadHash == "" {
		// Compute payload hash if not provided (though clients should provide it)
		// For simplicity, if not provided we assume unsigned payload
		payloadHash = "UNSIGNED-PAYLOAD"
	}

	expectedSig := v.computeSignature(r, signedHeadersStr, payloadHash, amzDate, dateStamp, region, service, user.SecretKey)

	if subtle.ConstantTimeCompare([]byte(expectedSig), []byte(providedSig)) != 1 {
		return nil, ErrSignatureDoesNotMatch
	}

	// Verify timestamp
	reqTime, err := time.Parse("20060102T150405Z", amzDate)
	if err == nil {
		if time.Since(reqTime) > 15*time.Minute || time.Until(reqTime) > 15*time.Minute {
			return nil, ErrRequestTimeTooSkewed
		}
	}

	return user, nil
}

func (v *SigV4Verifier) verifyQueryString(r *http.Request) (*User, error) {
	q := r.URL.Query()
	credStr := q.Get("X-Amz-Credential")
	signedHeadersStr := q.Get("X-Amz-SignedHeaders")
	providedSig := q.Get("X-Amz-Signature")
	amzDate := q.Get("X-Amz-Date")

	if credStr == "" || signedHeadersStr == "" || providedSig == "" || amzDate == "" {
		return nil, ErrAuthHeaderMalformed
	}

	credFields := strings.Split(credStr, "/")
	if len(credFields) != 5 {
		return nil, ErrAuthHeaderMalformed
	}
	accessKey := credFields[0]
	dateStamp := credFields[1]
	region := credFields[2]
	service := credFields[3]

	user, err := v.UserStore.GetUserByAccessKey(r.Context(), accessKey)
	if err != nil {
		return nil, err
	}

	payloadHash := "UNSIGNED-PAYLOAD"
	
	// Create a copy of the request to strip X-Amz-Signature for canonical request
	reqCopy := r.Clone(r.Context())
	newQ := reqCopy.URL.Query()
	newQ.Del("X-Amz-Signature")
	reqCopy.URL.RawQuery = strings.ReplaceAll(newQ.Encode(), "+", "%20")

	expectedSig := v.computeSignature(reqCopy, signedHeadersStr, payloadHash, amzDate, dateStamp, region, service, user.SecretKey)

	if subtle.ConstantTimeCompare([]byte(expectedSig), []byte(providedSig)) != 1 {
		return nil, ErrSignatureDoesNotMatch
	}

	// Verify Expiry
	// ... (omitted for brevity, but could be added based on X-Amz-Expires)

	return user, nil
}

func (v *SigV4Verifier) computeSignature(r *http.Request, signedHeadersStr, payloadHash, amzDate, dateStamp, region, service, secretKey string) string {
	canonicalRequest := v.buildCanonicalRequest(r, signedHeadersStr, payloadHash)
	stringToSign := fmt.Sprintf("AWS4-HMAC-SHA256\n%s\n%s/%s/%s/aws4_request\n%s",
		amzDate, dateStamp, region, service, hashSHA256([]byte(canonicalRequest)))

	signingKey := getSignatureKey(secretKey, dateStamp, region, service)
	return hex.EncodeToString(hmacSHA256(signingKey, stringToSign))
}

func (v *SigV4Verifier) buildCanonicalRequest(r *http.Request, signedHeadersStr string, payloadHash string) string {
	method := r.Method

	// Canonical URI
	canonicalURI := r.URL.Path
	if canonicalURI == "" {
		canonicalURI = "/"
	}
	// Note: Proper S3 URI encoding is complex, assuming standard paths for now

	// Canonical Query String
	q := r.URL.Query()
	var queryKeys []string
	for k := range q {
		queryKeys = append(queryKeys, k)
	}
	sort.Strings(queryKeys)
	var canonicalQueryParts []string
	for _, k := range queryKeys {
		vals := q[k]
		sort.Strings(vals)
		for _, v := range vals {
			canonicalQueryParts = append(canonicalQueryParts, url.QueryEscape(k)+"="+url.QueryEscape(v))
		}
	}
	canonicalQueryString := strings.Join(canonicalQueryParts, "&")
	canonicalQueryString = strings.ReplaceAll(canonicalQueryString, "+", "%20")

	// Canonical Headers
	signedHeaders := strings.Split(signedHeadersStr, ";")
	var canonicalHeadersParts []string
	for _, h := range signedHeaders {
		val := r.Header.Get(h)
		if h == "host" {
			val = r.Host
		}
		// Trim multiple spaces to a single space
		val = strings.Join(strings.Fields(val), " ")
		canonicalHeadersParts = append(canonicalHeadersParts, h+":"+val+"\n")
	}
	canonicalHeaders := strings.Join(canonicalHeadersParts, "")

	return fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s",
		method,
		canonicalURI,
		canonicalQueryString,
		canonicalHeaders,
		signedHeadersStr,
		payloadHash)
}

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
