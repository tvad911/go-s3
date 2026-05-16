# PLAN: GoS3 Security Audit

> **Type:** Security Audit & Penetration Test Plan
> **Created:** 2026-05-16
> **Status:** Ready
> **Agent:** security-auditor
> **Codebase size:** ~15,000 LOC (Go)

---

## Tổng quan

GoS3 là S3-compatible object storage server với Web Console. Audit này cover toàn bộ attack surface theo OWASP và S3-specific security vectors.

### Attack Surface Map

```
┌──────────────────────────────────────────────────────────────┐
│                    INTERNET / CLIENT                         │
├──────────────┬────────────────┬──────────────────────────────┤
│   S3 API     │   Admin API    │   Web Console                │
│   (SigV4)    │   (SigV4/JWT)  │   (/api/v1 + /_ui)           │
├──────────────┴────────────────┴──────────────────────────────┤
│                   MIDDLEWARE CHAIN                            │
│  RealIP → RequestID → Logger → Recoverer → RateLimit         │
│  → CustomDomain → CORS → Auth (SigV4) → Handler              │
├──────────────────────────────────────────────────────────────┤
│              HANDLER LAYER                                    │
│  Bucket/Object CRUD │ Multipart │ PostObject │ Admin │ Auth   │
├──────────────────────────────────────────────────────────────┤
│              STORAGE & AUTH ENGINE                             │
│  Local Backend (filesystem) │ BBolt metadata │ Policy Engine   │
├──────────────────────────────────────────────────────────────┤
│              OS / FILESYSTEM / NETWORK                        │
└──────────────────────────────────────────────────────────────┘
```

---

## Phase 1: Path Traversal & Filesystem (CRITICAL)

> **Risk:** HIGH — Object key được nhúng trực tiếp vào filesystem path

### 1.1 — Object Key Path Traversal (`internal/storage/local/local.go`)

**Mục tiêu:** Xác minh key chứa `../` hoặc `..%2f` KHÔNG escape được khỏi `dataDir`.

| # | Test Case | Input Key | Expected |
|---|-----------|-----------|----------|
| 1 | Basic traversal | `../../../etc/passwd` | Reject hoặc sanitize |
| 2 | URL-encoded traversal | `..%2f..%2f..%2fetc%2fpasswd` | Reject |
| 3 | Backslash traversal (Windows) | `..\..\..\etc\passwd` | Reject |
| 4 | Null byte injection | `file.txt\x00.jpg` | Reject |
| 5 | Double-encoded | `%252e%252e%252f` | Reject |
| 6 | Unicode normalization | `..／etc/passwd` (fullwidth ／) | Reject |
| 7 | Extremely long key (>1024) | `a` × 2000 | 400 KeyTooLongError |
| 8 | Key với leading `/` | `/absolute/path` | Sanitize, không escape |

**Affected code:**
- `local.go:222` — `filepath.Join(b.dataDir, "buckets", bucket, "objects", key+"@"+versionId)`
- `local.go:265` — GetObject path
- `local.go:373` — DeleteObject path
- `local.go:589` — CompleteMultipartUpload path

**Root cause analysis:**
- `filepath.Join` tự clean `..` nhưng cần verify nó KHÔNG escape `dataDir`
- Cần `filepath.Clean` + kiểm tra `strings.HasPrefix(cleaned, dataDir)` sau join

**Fix strategy:**
```go
func (b *Backend) safePath(bucket, key string) (string, error) {
    p := filepath.Join(b.dataDir, "buckets", bucket, "objects", key)
    p = filepath.Clean(p)
    if !strings.HasPrefix(p, filepath.Clean(b.dataDir)) {
        return "", fmt.Errorf("path traversal detected: %s", key)
    }
    return p, nil
}
```

**VERIFY:** `go test ./internal/storage/local/ -run TestPathTraversal -v`

### 1.2 — Bucket Name Traversal

| # | Test Case | Bucket Name | Expected |
|---|-----------|-------------|----------|
| 1 | Dots in bucket name | `../../etc` | Reject (validation) |
| 2 | Slashes in bucket name | `a/b/c` | Reject (validation) |
| 3 | Empty bucket name | `` | Reject |

**Note:** `s3.ValidateBucketName()` đã cover phần lớn. Cần verify nó được gọi TRƯỚC khi tạo path.

**VERIFY:** `go test ./internal/s3/ -run TestValidateBucketName -v` (đã có từ Phase B)

### 1.3 — Multipart Upload Path Safety

| # | Test Case | Expected |
|---|-----------|----------|
| 1 | UploadID chứa `../` | Reject — uploadID phải là UUID |
| 2 | PartNumber cực lớn (999999) | Reject hoặc bound check |
| 3 | Temp file cleanup sau crash | Orphan files bị xóa khi startup |

**VERIFY:** `go test ./internal/storage/local/ -run TestMultipartPathSafety -v`

---

## Phase 2: Authentication Bypass (CRITICAL)

### 2.1 — SigV4 Edge Cases (`internal/auth/sigv4.go`)

| # | Test Case | Expected |
|---|-----------|----------|
| 1 | Empty Authorization header | ErrAuthHeaderMissing |
| 2 | Authorization = "AWS4-HMAC-SHA256 " (trailing space, no params) | ErrAuthHeaderMalformed |
| 3 | Credential với region khác | Reject (region mismatch) |
| 4 | SignedHeaders thiếu "host" | Reject |
| 5 | Duplicate Authorization header | Dùng cái đầu tiên |
| 6 | Signature = all zeros | Reject (constant-time compare) |
| 7 | AccessKeyID tồn tại nhưng user disabled | Reject |
| 8 | Presigned URL với negative X-Amz-Expires | Reject |
| 9 | Presigned URL với X-Amz-Expires > 604800 (7 days) | Reject |

**VERIFY:** `go test ./internal/auth/ -run TestSigV4Edge -v`

### 2.2 — JWT Session Security (`internal/auth/session.go`)

| # | Test Case | Expected |
|---|-----------|----------|
| 1 | Token với `alg: none` | Reject |
| 2 | Token signed với wrong key | Reject |
| 3 | Expired token | ErrTokenExpired |
| 4 | Token với `exp` = 0 (epoch) | Reject |
| 5 | Token body tampered (change username) | Signature mismatch |
| 6 | Session revoked (deleted from DB) | Reject |
| 7 | Empty token string | Reject |
| 8 | Token chỉ có 2 parts (no signature) | Reject |
| 9 | Replay token sau logout | Reject (session deleted) |

**VERIFY:** `go test ./internal/auth/ -run TestJWTSecurity -v`

### 2.3 — Login Brute Force (`internal/handler/auth_handler.go`)

| # | Test Case | Expected |
|---|-----------|----------|
| 1 | 100 consecutive wrong passwords | Rate limit hoặc lockout |
| 2 | Timing oracle: valid vs invalid username | Response time ~equal |
| 3 | SQL-style injection in username | No crash, clean error |
| 4 | Service Account login với disabled SA | Reject |
| 5 | Login response không leak user existence | Same error message |

> **Observation:** Login handler hiện KHÔNG có brute-force protection riêng (rate limit chung từ middleware nhưng per-IP, không per-user). **Cần thêm login-specific rate limiting.**

**VERIFY:** `go test ./internal/handler/ -run TestLoginBruteForce -v`

---

## Phase 3: Authorization & Privilege Escalation (HIGH)

### 3.1 — Admin API Without Auth

| # | Test Case | Expected |
|---|-----------|----------|
| 1 | GET /_admin/users không có auth | 403 |
| 2 | POST /_admin/users với non-root JWT | 403 |
| 3 | PUT /_admin/users/root với regular user | 403 |
| 4 | DELETE /_admin/users/root (root xóa chính mình) | 403 hoặc reject |
| 5 | JWT token cho disabled user → access admin | 403 |
| 6 | GetUser response có chứa secretKey/passwordHash không? | KHÔNG |

**VERIFY:** `go test ./internal/handler/ -run TestAdminAuth -v`

### 3.2 — Bucket Policy Bypass

| # | Test Case | Expected |
|---|-----------|----------|
| 1 | User A tạo bucket, User B truy cập | 403 (default deny) |
| 2 | Public bucket policy → anonymous GET | 200 |
| 3 | Public bucket + Deny specific key | 403 cho key đó |
| 4 | User inject policy cho bucket không thuộc mình | 403 |
| 5 | Wildcard resource `arn:aws:s3:::*` | Chỉ match bucket thuộc user |

**VERIFY:** `go test ./internal/auth/ -run TestPolicyBypass -v`

### 3.3 — IDOR (Insecure Direct Object Reference)

| # | Test Case | Expected |
|---|-----------|----------|
| 1 | Service Account A access SA of User B | 403 |
| 2 | Modify service-account-id trong URL | 403 nếu không thuộc user |
| 3 | Enumerate user list (non-root) | 403 |

**VERIFY:** `go test ./internal/handler/ -run TestIDOR -v`

---

## Phase 4: Input Validation & Injection (MEDIUM)

### 4.1 — XML External Entity (XXE)

| # | Test Case | Input | Expected |
|---|-----------|-------|----------|
| 1 | XXE trong DeleteObjects | `<!DOCTYPE foo [<!ENTITY xxe SYSTEM "file:///etc/passwd">]>` | Reject |
| 2 | XXE trong CompleteMultipartUpload | Same | Reject |
| 3 | XXE trong PutBucketCors | Same | Reject |
| 4 | XML bomb (billion laughs) | `<!ENTITY lol ...>` nested | Reject (limit body) |

**Note:** Go's `encoding/xml` KHÔNG process DTD/external entities by default → **likely safe**. Nhưng cần test confirm.

**VERIFY:** `go test ./internal/handler/ -run TestXXE -v`

### 4.2 — PostObject (Form Upload) Validation

| # | Test Case | Expected |
|---|-----------|----------|
| 1 | `success_action_redirect` chứa javascript: URL | Reject hoặc validate |
| 2 | `key` chứa path traversal `../` | Sanitize |
| 3 | Multipart form không có "file" field | 400 |
| 4 | Policy expiration đã qua | Reject |
| 5 | Policy base64 không hợp lệ | 400 |
| 6 | Multipart form cực lớn (> MaxObjectSize) | Reject |

**Observation:** `post_object.go:59` dùng `io.ReadAll(part)` cho form fields → **unbounded read**. Cần `io.LimitReader`.

**VERIFY:** `go test ./internal/handler/ -run TestPostObjectSecurity -v`

### 4.3 — HTTP Header Injection

| # | Test Case | Expected |
|---|-----------|----------|
| 1 | Object key chứa `\r\n` (CRLF injection) | Sanitize |
| 2 | User metadata value chứa CRLF | Sanitize |
| 3 | Content-Type header injection | Sanitize |

**VERIFY:** `go test ./internal/handler/ -run TestHeaderInjection -v`

### 4.4 — Open Redirect via success_action_redirect

| # | Test Case | Expected |
|---|-----------|----------|
| 1 | Redirect tới `http://evil.com` | Chỉ cho phép same-origin hoặc reject |
| 2 | Redirect tới `javascript:alert(1)` | Reject |

**VERIFY:** `go test ./internal/handler/ -run TestOpenRedirect -v`

---

## Phase 5: Denial of Service (MEDIUM)

### 5.1 — Resource Exhaustion

| # | Test Case | Expected |
|---|-----------|----------|
| 1 | Slowloris (send headers very slowly) | ReadHeaderTimeout kicks in |
| 2 | 10,000 concurrent connections | Rate limiter blocks |
| 3 | Upload content-length: 999TB, send 1 byte | Timeout / reject |
| 4 | Create 100,000 buckets | No crash, maybe limit |
| 5 | ListObjects trên bucket 1M objects | Pagination, không OOM |
| 6 | Multipart upload 10,000 parts | Bound check |
| 7 | Initiate multipart nhưng never complete (orphan) | Cleanup worker xóa |

**VERIFY:** `go test ./internal/handler/ -run TestDoS -v` (subset — full DoS cần load testing)

### 5.2 — Disk Exhaustion

| # | Test Case | Expected |
|---|-----------|----------|
| 1 | PutObject khi disk < threshold | 507 InsufficientStorage |
| 2 | UploadPart khi disk full | 507 |
| 3 | Multipart complete tạo file lớn khi disk gần full | Reject trước khi bắt đầu |

**VERIFY:** `go test ./internal/storage/local/ -run TestDiskExhaustion -v`

---

## Phase 6: Cryptographic & Secret Management (MEDIUM)

### 6.1 — Secret Exposure

| # | Test Case | Expected |
|---|-----------|----------|
| 1 | GET /admin/users → response có `secretKey`? | KHÔNG |
| 2 | GET /admin/users → response có `passwordHash`? | KHÔNG |
| 3 | Error response leak internal path? | KHÔNG |
| 4 | Log output chứa secret key? | KHÔNG |
| 5 | Config dump (ServerInfo) chứa credentials? | KHÔNG |
| 6 | Service Account create response chứa secret? | CHỈ 1 LẦN |

**VERIFY:** `go test ./internal/handler/ -run TestSecretExposure -v`

### 6.2 — Cryptographic Strength

| # | Check | Expected |
|---|-------|----------|
| 1 | JWT signing key >= 32 bytes | Yes (DefaultSessionConfig) |
| 2 | SigV4 signature uses constant-time compare | Yes (subtle.ConstantTimeCompare) |
| 3 | JWT signature uses hmac.Equal | Yes |
| 4 | Password hashing uses bcrypt | Yes (bcrypt.DefaultCost) |
| 5 | AccessKey generation uses crypto/rand | Yes |
| 6 | ETag = MD5 (acceptable for checksum, not security) | Yes — acceptable |

---

## Phase 7: CORS & Cross-Origin (LOW-MEDIUM)

### 7.1 — CORS Misconfiguration

| # | Test Case | Expected |
|---|-----------|----------|
| 1 | Origin: * cho private bucket | Không nên — phải match config |
| 2 | CORS headers trên error response | Có (per AWS spec) |
| 3 | Preflight với custom headers | Trả Access-Control-Allow-Headers |
| 4 | null Origin | Reject |
| 5 | Wildcard subdomain reflection | Không reflect user input |

**VERIFY:** `go test ./internal/middleware/ -run TestCORS -v`

---

## Phase 8: Web Console Security (LOW-MEDIUM)

### 8.1 — Cookie Security

| # | Check | Status |
|---|-------|--------|
| 1 | HttpOnly cookie | Yes |
| 2 | SameSite=Strict | Yes |
| 3 | Secure flag (HTTPS only) | Configurable (CookieSecure) — default false |
| 4 | Cookie path = "/" | Yes |

### 8.2 — Static File Serving (Web UI)

| # | Test Case | Expected |
|---|-----------|----------|
| 1 | `/_ui/../../../etc/passwd` | 404 (embed.FS là read-only) |
| 2 | `/_ui/%2e%2e/` | 404 |
| 3 | Static files chứa XSS? | Audit frontend code |

---

## Phase 9: Known Vulnerabilities to Fix

> Phát hiện từ code review — cần fix trước khi deploy production.

### 9.1 — CRITICAL: PostObject unbounded read

**File:** `internal/handler/post_object.go:59`
```go
value, err := io.ReadAll(part) // UNBOUNDED — DoS vector
```
**Fix:** `io.LimitReader(part, 1<<20)` (1MB limit cho form fields)

### 9.2 — MEDIUM: PostObject open redirect

**File:** `internal/handler/post_object.go:152`
```go
http.Redirect(w, r, redirect, http.StatusSeeOther) // No URL validation
```
**Fix:** Validate `redirect` is same-origin or within allowed list.

### 9.3 — MEDIUM: GetUser leaks sensitive fields

**File:** `internal/handler/admin.go:130`
```go
json.NewEncoder(w).Encode(user) // May include SecretKey/PasswordHash
```
**Fix:** Strip `SecretKey` and `PasswordHash` before encode (ListUsers already does, GetUser does not).

### 9.4 — LOW: Service Account secret key comparison

**File:** `internal/handler/auth_handler.go:66`
```go
if sa.SecretKey == req.Password { // NOT constant-time
```
**Fix:** Use `subtle.ConstantTimeCompare()`.

### 9.5 — LOW: Missing path traversal guard in storage

**File:** `internal/storage/local/local.go:222,265,373`
- `filepath.Join` cleans `..` but DOES NOT validate final path stays within `dataDir`.
- Chi router strips leading `/` and Go HTTP normalizes paths, but **defense-in-depth** needs explicit check.

---

## Execution Order

### Priority

```
CRITICAL  → Phase 1 (Path Traversal) + Phase 2 (Auth Bypass) + Fix 9.1
HIGH      → Phase 3 (Privilege Escalation) + Fix 9.3
MEDIUM    → Phase 4 (Injection) + Phase 5 (DoS) + Phase 6 (Secrets)
LOW       → Phase 7 (CORS) + Phase 8 (Web Console)
```

### Test Files to Create

| Phase | Test File | Package |
|-------|-----------|---------|
| 1 | `internal/storage/local/security_test.go` | `local_test` |
| 2 | `internal/auth/security_test.go` | `auth_test` |
| 3 | `internal/handler/admin_security_test.go` | `handler_test` |
| 4 | `internal/handler/injection_test.go` | `handler_test` |
| 5 | `internal/handler/dos_test.go` | `handler_test` |
| 6 | `internal/handler/secret_exposure_test.go` | `handler_test` |
| 7 | `internal/middleware/cors_test.go` | `middleware_test` |

### Production Fixes

| ID | Severity | File | Fix |
|----|----------|------|-----|
| 9.1 | CRITICAL | `post_object.go:59` | `io.LimitReader(part, 1<<20)` |
| 9.2 | MEDIUM | `post_object.go:152` | Validate redirect URL |
| 9.3 | MEDIUM | `admin.go:130` | Strip sensitive fields |
| 9.4 | LOW | `auth_handler.go:66` | `subtle.ConstantTimeCompare` |
| 9.5 | LOW | `local.go` | Add `safePath()` guard |

### Verification Command

```bash
# Run all security tests
go test ./internal/storage/local/ -run "TestPathTraversal|TestDiskExhaustion|TestMultipartPathSafety" -v
go test ./internal/auth/ -run "TestSigV4Edge|TestJWTSecurity" -v
go test ./internal/handler/ -run "TestAdminAuth|TestXXE|TestPostObject|TestHeaderInjection|TestDoS|TestSecretExposure" -v
go test ./internal/middleware/ -run "TestCORS" -v

# Full regression (must pass after fixes)
go test ./internal/... ./client/ -race -count=1
```

---

## Checklist

- [ ] Phase 1: Path Traversal tests pass
- [ ] Phase 2: Auth bypass tests pass
- [ ] Phase 3: Privilege escalation tests pass
- [ ] Phase 4: Injection tests pass
- [ ] Phase 5: DoS protection verified
- [ ] Phase 6: Secret exposure audit clean
- [ ] Phase 7: CORS configuration reviewed
- [ ] Phase 8: Web Console security checked
- [ ] Fix 9.1: PostObject LimitReader
- [ ] Fix 9.2: PostObject redirect validation
- [ ] Fix 9.3: GetUser strip sensitive fields
- [ ] Fix 9.4: SA login constant-time compare
- [ ] Fix 9.5: Storage safePath guard
- [ ] Full regression pass (123+ tests)
