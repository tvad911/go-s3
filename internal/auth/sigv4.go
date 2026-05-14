package auth

import (
	"context"
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
	"strconv"
	"time"
)

var (
	ErrAuthHeaderMissing    = errors.New("authorization header missing")
	ErrAuthHeaderMalformed  = errors.New("authorization header malformed")
	ErrSignatureDoesNotMatch = errors.New("signature does not match")
	ErrRequestTimeTooSkewed  = errors.New("request time too skewed")
)

type SigV4Verifier struct {
	UserStore    UserStore
	SAStore      ServiceAccountStore
	Region       string
}

func NewSigV4Verifier(store UserStore, saStore ServiceAccountStore, region string) *SigV4Verifier {
	return &SigV4Verifier{
		UserStore: store,
		SAStore:   saStore,
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

	// Lookup user by access key.
	// Try service accounts first, then fall back to legacy user table.
	user, secretKey, err := v.lookupByAccessKey(r.Context(), accessKey)
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

	expectedSig := v.computeSignature(r, signedHeadersStr, payloadHash, amzDate, dateStamp, region, service, secretKey)

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

// lookupByAccessKey resolves an access key to a User and its secret key.
// Priority: ServiceAccountStore first, then UserStore (backward compat).
func (v *SigV4Verifier) lookupByAccessKey(ctx context.Context, accessKey string) (*User, string, error) {
	// Try service accounts first
	if v.SAStore != nil {
		sa, err := v.SAStore.GetServiceAccountByAccessKey(ctx, accessKey)
		if err == nil {
			if sa.Disabled || sa.IsExpired() {
				return nil, "", ErrAuthHeaderMissing
			}
			// Resolve the parent user
			parentUser, err := v.UserStore.GetUserByUsername(ctx, sa.ParentUser)
			if err != nil {
				return nil, "", err
			}
			if parentUser.Disabled {
				return nil, "", ErrAuthHeaderMissing
			}

			// Create a copy of the user to avoid mutating the original
			user := *parentUser
			// Merge SA policies with parent user policies
			if len(sa.Policies) > 0 {
				user.Policies = append([]string(nil), parentUser.Policies...)
				user.Policies = append(user.Policies, sa.Policies...)
			}

			return &user, sa.SecretKey, nil
		}
	}
	// Fallback to legacy user table (backward compatibility for root_access_key)
	user, err := v.UserStore.GetUserByAccessKey(ctx, accessKey)
	if err != nil {
		return nil, "", err
	}
	return user, user.SecretKey, nil
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

	user, secretKey, err := v.lookupByAccessKey(r.Context(), accessKey)
	if err != nil {
		return nil, err
	}

	payloadHash := "UNSIGNED-PAYLOAD"
	
	// Create a copy of the request to strip X-Amz-Signature for canonical request
	reqCopy := r.Clone(r.Context())
	newQ := reqCopy.URL.Query()
	newQ.Del("X-Amz-Signature")
	reqCopy.URL.RawQuery = strings.ReplaceAll(newQ.Encode(), "+", "%20")

	expectedSig := v.computeSignature(reqCopy, signedHeadersStr, payloadHash, amzDate, dateStamp, region, service, secretKey)

	if subtle.ConstantTimeCompare([]byte(expectedSig), []byte(providedSig)) != 1 {
		return nil, ErrSignatureDoesNotMatch
	}

	// Verify Expiry
	expiresStr := q.Get("X-Amz-Expires")
	if expiresStr == "" {
		return nil, ErrAuthHeaderMalformed
	}
	expires, err := strconv.ParseInt(expiresStr, 10, 64)
	if err != nil || expires < 1 || expires > 604800 { // Max 7 days
		return nil, ErrAuthHeaderMalformed
	}

	reqTime, err := time.Parse("20060102T150405Z", amzDate)
	if err != nil {
		return nil, ErrAuthHeaderMalformed
	}

	if time.Since(reqTime) > time.Duration(expires)*time.Second || time.Until(reqTime) > 15*time.Minute {
		return nil, ErrRequestTimeTooSkewed
	}

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

// VerifyPostPolicy verifies an HTML form upload policy signature.
// policyB64 is the exact string from the 'policy' form field.
func (v *SigV4Verifier) VerifyPostPolicy(ctx context.Context, credential, date, policyB64, signature string) (*User, error) {
	if credential == "" || date == "" || policyB64 == "" || signature == "" {
		return nil, ErrAuthHeaderMalformed
	}

	credFields := strings.Split(credential, "/")
	if len(credFields) != 5 {
		return nil, ErrAuthHeaderMalformed
	}
	accessKey := credFields[0]
	dateStamp := credFields[1]
	region := credFields[2]
	service := credFields[3]

	// Must match the date provided in X-Amz-Date form field (YYYYMMDDTHHMMSSZ)
	if !strings.HasPrefix(date, dateStamp) {
		return nil, ErrAuthHeaderMalformed
	}

	user, secretKey, err := v.lookupByAccessKey(ctx, accessKey)
	if err != nil {
		return nil, err
	}

	signingKey := getSignatureKey(secretKey, dateStamp, region, service)
	
	// For POST uploads, the string to sign is literally the base64-encoded policy
	expectedSig := hex.EncodeToString(hmacSHA256(signingKey, policyB64))

	if subtle.ConstantTimeCompare([]byte(expectedSig), []byte(signature)) != 1 {
		return nil, ErrSignatureDoesNotMatch
	}

	return user, nil
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

// GeneratePresignedURL generates a presigned URL for the given parameters.
func GeneratePresignedURL(method, endpoint, region, accessKey, secretKey, bucket, key string, expires int64) string {
	now := time.Now().UTC()
	amzDate := now.Format("20060102T150405Z")
	dateStamp := now.Format("20060102")
	service := "s3"

	// Construct path
	path := "/"
	if bucket != "" {
		path += bucket
		if key != "" {
			path += "/" + key
		}
	}

	credential := fmt.Sprintf("%s/%s/%s/%s/aws4_request", accessKey, dateStamp, region, service)
	signedHeaders := "host"

	q := url.Values{}
	q.Set("X-Amz-Algorithm", "AWS4-HMAC-SHA256")
	q.Set("X-Amz-Credential", credential)
	q.Set("X-Amz-Date", amzDate)
	q.Set("X-Amz-Expires", strconv.FormatInt(expires, 10))
	q.Set("X-Amz-SignedHeaders", signedHeaders)

	// Host logic: assuming endpoint contains the host
	u, _ := url.Parse(endpoint)
	host := u.Host
	if host == "" {
		host = endpoint // fallback
	}

	canonicalURI := path
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

	signingKey := getSignatureKey(secretKey, dateStamp, region, service)
	signature := hex.EncodeToString(hmacSHA256(signingKey, stringToSign))

	q.Set("X-Amz-Signature", signature)

	// Format final URL
	endpoint = strings.TrimSuffix(endpoint, "/")
	finalURL := fmt.Sprintf("%s%s?%s", endpoint, path, strings.ReplaceAll(q.Encode(), "+", "%20"))
	return finalURL
}
