# GoS3 Test Suite Plan

> **Project Type:** BACKEND (Go)
> **Scope:** S3 API tests, CLI client tests, Authorization & Permission tests
> **Không viết code** — chỉ lập kế hoạch

---

## Tổng quan

Hiện tại GoS3 **không có bất kỳ file `*_test.go` nào**. Chỉ có 1 file `test/smoke.sh` (shell script dùng awscli).
Plan này chia test thành 3 nhóm chính, mỗi nhóm có unit test + integration test.

---

## Success Criteria

- [ ] `go test ./...` pass 100%
- [ ] Coverage ≥ 60% cho các package `handler`, `auth`, `storage/local`
- [ ] Smoke test (`test/smoke.sh`) pass với server thật
- [ ] Tất cả S3 error codes trả đúng HTTP status + XML body
- [ ] CLI commands hoạt động đúng với cả valid và invalid input

---

## Tech Stack

| Tool | Mục đích |
|------|----------|
| `testing` (stdlib) | Unit test framework |
| `net/http/httptest` | Integration test cho HTTP handlers |
| `os.MkdirTemp` | Tạo temp directory cho test storage |
| `t.TempDir()` | Auto-cleanup temp dirs |
| `testing/fstest` | Nếu cần mock filesystem |
| `bash` + `awscli` | Smoke/E2E test |

**Không dùng:** testify, gomock, hay bất kỳ framework ngoài nào.

---

## File Structure

```
internal/
├── auth/
│   ├── sigv4_test.go         # SigV4 signature verification
│   ├── policy_test.go        # Policy evaluation logic
│   ├── engine_test.go        # Auth engine (IAM + Bucket policy combined)
│   ├── iam_test.go           # Password hashing, user context
│   └── service_account_test.go # Key generation, expiry
│
├── handler/
│   ├── bucket_test.go        # CreateBucket, DeleteBucket, HeadBucket, ListBuckets
│   ├── object_test.go        # PutObject, GetObject, HeadObject, DeleteObject, CopyObject
│   ├── multipart_test.go     # Multipart upload flow
│   ├── error_test.go         # S3 XML error responses
│   └── admin_test.go         # Admin API (user CRUD, server info)
│
├── storage/
│   ├── local/
│   │   ├── local_test.go     # Backend interface implementation
│   │   └── disk_test.go      # Disk space checking
│   └── metadata/
│       └── bbolt_test.go     # Metadata CRUD operations
│
├── s3/
│   ├── bucket_name_test.go   # Bucket name validation
│   └── errors_test.go        # Error code mapping
│
├── middleware/
│   ├── auth_test.go          # Auth middleware integration
│   └── ratelimit_test.go     # Rate limiting behavior
│
cli/
│   ├── cp_test.go            # Upload/download/copy commands
│   ├── ls_test.go            # List buckets/objects
│   └── mb_test.go            # Make/remove bucket
│
client/
│   ├── sigv4_test.go         # Client-side request signing
│   └── client_test.go        # Client struct + request building
│
test/
├── smoke.sh                  # (đã có) AWS CLI smoke test
├── smoke_auth.sh             # Authorization smoke test
└── smoke_cli.sh              # CLI client smoke test
```

---

## Task Breakdown

### Phase A: Test Infrastructure (helpers dùng chung)

#### A.1 — Test helper: tạo storage backend tạm
- **INPUT:** Không
- **OUTPUT:** File `internal/handler/testutil_test.go` chứa helper tạo `LocalBackend` + bbolt DB trong `t.TempDir()`
- **VERIFY:** Helper compile thành công, dùng được trong các test khác

#### A.2 — Test helper: tạo HTTP handler cho integration test
- **INPUT:** A.1
- **OUTPUT:** Helper function `setupTestServer(t) (*httptest.Server, cleanup)` — khởi tạo router + handler + storage đầy đủ
- **VERIFY:** `httptest.Server` trả 200 cho `GET /_health`

---

### Phase B: S3 API Tests (Unit + Integration)

#### B.1 — Bucket Name Validation (`internal/s3/bucket_name_test.go`)
- **Deps:** Không
- **INPUT:** Hàm `validateBucketName()`
- **OUTPUT:** Table-driven test: valid names, quá ngắn, quá dài, uppercase, ký tự đặc biệt, consecutive dots, bắt đầu/kết thúc bằng hyphen
- **VERIFY:** `go test ./internal/s3/ -run TestValidateBucketName -v`

#### B.2 — S3 Error Response (`internal/handler/error_test.go`)
- **Deps:** Không
- **INPUT:** Hàm `WriteError()`
- **OUTPUT:** Test verify XML output format, HTTP status codes, `x-amz-request-id` header
- **VERIFY:** Response body parse được thành `<Error>` XML struct

#### B.3 — Bucket Handlers (`internal/handler/bucket_test.go`)
- **Deps:** A.1, A.2
- **Cases:**

| # | Test Case | Method | Expected |
|---|-----------|--------|----------|
| 1 | CreateBucket thành công | `PUT /test-bucket` | 200, `Location` header |
| 2 | CreateBucket trùng tên | `PUT /test-bucket` x2 | 409 `BucketAlreadyOwnedByYou` |
| 3 | CreateBucket tên không hợp lệ | `PUT /AB` | 400 `InvalidBucketName` |
| 4 | DeleteBucket rỗng | `DELETE /test-bucket` | 204 |
| 5 | DeleteBucket còn object | `DELETE /bucket-with-obj` | 409 `BucketNotEmpty` |
| 6 | DeleteBucket không tồn tại | `DELETE /no-bucket` | 404 `NoSuchBucket` |
| 7 | HeadBucket tồn tại | `HEAD /test-bucket` | 200 |
| 8 | HeadBucket không tồn tại | `HEAD /nope` | 404 |
| 9 | ListBuckets | `GET /` | 200, XML `ListAllMyBucketsResult` |

- **VERIFY:** `go test ./internal/handler/ -run TestBucket -v`

#### B.4 — Object Handlers (`internal/handler/object_test.go`)
- **Deps:** A.1, A.2
- **Cases:**

| # | Test Case | Expected |
|---|-----------|----------|
| 1 | PutObject + GetObject roundtrip | Content match, ETag match |
| 2 | PutObject với metadata (`x-amz-meta-*`) | HeadObject trả metadata |
| 3 | PutObject với Content-Type | GetObject trả đúng Content-Type |
| 4 | GetObject Range request | 206, đúng bytes, `Content-Range` header |
| 5 | GetObject key không tồn tại | 404 `NoSuchKey` |
| 6 | GetObject bucket không tồn tại | 404 `NoSuchBucket` |
| 7 | HeadObject | 200, đủ headers, không có body |
| 8 | DeleteObject | 204, GetObject sau đó → 404 |
| 9 | DeleteObjects (batch) | XML `DeleteResult`, mixed success/error |
| 10 | CopyObject cùng bucket | 200, `CopyObjectResult` XML |
| 11 | CopyObject khác bucket | 200, metadata preserved |
| 12 | PutObject overwrite | ETag thay đổi, content mới |
| 13 | GetObject If-None-Match (304) | 304 Not Modified |
| 14 | PutObject + verify Accept-Ranges header | `Accept-Ranges: bytes` |
| 15 | PutObject large (streaming, > 1MB) | ETag correct, không OOM |

- **VERIFY:** `go test ./internal/handler/ -run TestObject -v`

#### B.5 — Multipart Upload (`internal/handler/multipart_test.go`)
- **Deps:** A.1, A.2
- **Cases:**

| # | Test Case | Expected |
|---|-----------|----------|
| 1 | Full flow: Create → Upload 3 parts → Complete | Object tồn tại, ETag = `md5-3` format |
| 2 | AbortMultipartUpload | Parts bị xóa, upload ID invalid |
| 3 | ListParts | Trả đúng part info |
| 4 | ListMultipartUploads | Trả upload đang active |
| 5 | Complete với part order sai | 400 `InvalidPartOrder` |
| 6 | Complete với ETag sai | 400 `InvalidPart` |
| 7 | UploadPart partNumber ngoài range | 400 |
| 8 | Upload part cho uploadId không tồn tại | 404 `NoSuchUpload` |

- **VERIFY:** `go test ./internal/handler/ -run TestMultipart -v`

#### B.6 — ListObjects V1 + V2 (`internal/handler/bucket_test.go`)
- **Deps:** A.1, A.2
- **Cases:**

| # | Test Case | Expected |
|---|-----------|----------|
| 1 | ListObjects V1 basic | XML với Contents |
| 2 | ListObjects V1 với prefix | Chỉ trả objects match prefix |
| 3 | ListObjects V1 với delimiter `/` | CommonPrefixes cho "folders" |
| 4 | ListObjects V1 pagination (marker) | IsTruncated + NextMarker |
| 5 | ListObjects V2 basic | XML với `list-type=2` |
| 6 | ListObjects V2 continuation-token | Pagination hoạt động |
| 7 | ListObjects bucket rỗng | 200, Contents rỗng |
| 8 | ListObjects max-keys=1 | IsTruncated=true nếu >1 object |

- **VERIFY:** `go test ./internal/handler/ -run TestListObjects -v`

#### B.7 — Storage Backend (`internal/storage/local/local_test.go`)
- **Deps:** Không (test trực tiếp Backend interface)
- **Cases:**

| # | Test Case | Expected |
|---|-----------|----------|
| 1 | CreateBucket + BucketExists | true |
| 2 | PutObject streaming | File trên disk, ETag = MD5 |
| 3 | GetObject streaming | io.Reader trả đúng content |
| 4 | Atomic write (crash simulation) | Không có file corrupt |
| 5 | CopyObject | Destination tồn tại, source giữ nguyên |
| 6 | DeleteBucket non-empty | Error |
| 7 | ListObjects prefix scan | Đúng kết quả |

- **VERIFY:** `go test ./internal/storage/local/ -v`

#### B.8 — Metadata Store (`internal/storage/metadata/bbolt_test.go`)
- **Deps:** Không
- **Cases:** CRUD bucket metadata, object metadata, multipart metadata, tags
- **VERIFY:** `go test ./internal/storage/metadata/ -v`

---

### Phase C: Authorization & Permission Tests

#### C.1 — SigV4 Signature (`internal/auth/sigv4_test.go`)
- **Deps:** Không
- **Cases:**

| # | Test Case | Expected |
|---|-----------|----------|
| 1 | Valid signature (header auth) | Pass, user extracted |
| 2 | Invalid access key | `InvalidAccessKeyId` |
| 3 | Wrong secret key → wrong signature | `SignatureDoesNotMatch` |
| 4 | Expired request (>15 min) | Reject |
| 5 | Valid presigned URL | Pass |
| 6 | Expired presigned URL | Reject |
| 7 | UNSIGNED-PAYLOAD | Pass (skip payload hash) |
| 8 | Missing Authorization header | Anonymous request |

- **VERIFY:** `go test ./internal/auth/ -run TestSigV4 -v`

#### C.2 — Policy Evaluation (`internal/auth/policy_test.go`)
- **Deps:** Không
- **Cases:**

| # | Test Case | Expected |
|---|-----------|----------|
| 1 | Allow `s3:GetObject` on `arn:aws:s3:::bucket/*` | true |
| 2 | Deny overrides Allow | false |
| 3 | No matching statement → default deny | false |
| 4 | Wildcard action `s3:*` | true cho mọi action |
| 5 | Wildcard principal `*` | true cho mọi user |
| 6 | Principal match by username ARN | true |
| 7 | Principal match by AccessKeyID ARN | true |
| 8 | Resource prefix wildcard `bucket/prefix/*` | true cho keys dưới prefix |
| 9 | Policy nil → deny | false |

- **VERIFY:** `go test ./internal/auth/ -run TestEvaluatePolicy -v`

#### C.3 — Auth Engine (`internal/auth/engine_test.go`)
- **Deps:** C.2
- **Cases:**

| # | Test Case | Expected |
|---|-----------|----------|
| 1 | Root user → always allowed | true |
| 2 | User với IAM policy Allow | true |
| 3 | User không có policy → deny | false |
| 4 | Bucket policy Allow cho anonymous | true |
| 5 | IAM Allow + Bucket Deny → deny | false (Deny wins) |
| 6 | IAM Deny + Bucket Allow → deny | false |
| 7 | Disabled user | false (check ở middleware) |

- **VERIFY:** `go test ./internal/auth/ -run TestEngine -v`

#### C.4 — Service Account (`internal/auth/service_account_test.go`)
- **Deps:** Không
- **Cases:**

| # | Test Case | Expected |
|---|-----------|----------|
| 1 | GenerateAccessKey uniqueness | 2 keys khác nhau |
| 2 | GenerateSecretKey length | 40 chars |
| 3 | IsExpired chưa hết hạn | false |
| 4 | IsExpired đã hết hạn | true |
| 5 | IsExpired không có ExpiresAt | false |

- **VERIFY:** `go test ./internal/auth/ -run TestServiceAccount -v`

#### C.5 — Auth Middleware Integration (`internal/middleware/auth_test.go`)
- **Deps:** A.2, C.1
- **Cases:**

| # | Test Case | Expected |
|---|-----------|----------|
| 1 | Request có SigV4 hợp lệ | 200, user in context |
| 2 | Request không có auth header | Anonymous (hoặc 403 tùy route) |
| 3 | Request SigV4 sai key | 403 `InvalidAccessKeyId` |
| 4 | Request SigV4 sai signature | 403 `SignatureDoesNotMatch` |
| 5 | Presigned URL hợp lệ | 200 |
| 6 | Presigned URL hết hạn | 403 |

- **VERIFY:** `go test ./internal/middleware/ -run TestAuth -v`

#### C.6 — Rate Limiting (`internal/middleware/ratelimit_test.go`)
- **Deps:** Không
- **Cases:**

| # | Test Case | Expected |
|---|-----------|----------|
| 1 | Dưới limit | 200 |
| 2 | Vượt limit | 429 `SlowDown`, có `Retry-After` |
| 3 | Khác IP → limit riêng | Mỗi IP có bucket riêng |

- **VERIFY:** `go test ./internal/middleware/ -run TestRateLimit -v`

#### C.7 — Admin API Authorization (`internal/handler/admin_test.go`)
- **Deps:** A.2, C.1
- **Cases:**

| # | Test Case | Expected |
|---|-----------|----------|
| 1 | Root user CRUD users | 200 |
| 2 | Non-root user gọi admin API | 403 |
| 3 | Create user → login bằng key mới | 200 |
| 4 | Delete user → login fail | 403 |
| 5 | Rotate key → old key fail, new key pass | 200 / 403 |

- **VERIFY:** `go test ./internal/handler/ -run TestAdmin -v`

---

### Phase D: CLI Client Tests

#### D.1 — Client SigV4 Signing (`client/sigv4_test.go`)
- **Deps:** Không
- **Cases:**

| # | Test Case | Expected |
|---|-----------|----------|
| 1 | Sign GET request | Authorization header đúng format |
| 2 | Sign PUT request với body | Payload hash included |
| 3 | Sign presigned URL | Query params đầy đủ |
| 4 | Roundtrip: client sign → server verify | Pass |

- **VERIFY:** `go test ./client/ -run TestSigV4 -v`

#### D.2 — Client Library (`client/client_test.go`)
- **Deps:** A.2 (cần test server)
- **Cases:**

| # | Test Case | Expected |
|---|-----------|----------|
| 1 | MakeBucket + ListBuckets | Bucket xuất hiện trong list |
| 2 | PutObject + GetObject | Content match |
| 3 | FPutObject (upload file) | File trên server, size match |
| 4 | FGetObject (download file) | File local, content match |
| 5 | StatObject | Metadata đúng |
| 6 | RemoveObject | Object bị xóa |
| 7 | CopyObject | Destination tồn tại |
| 8 | ListObjects iterator | Trả đúng danh sách |
| 9 | PresignGetObject | URL valid, download được |
| 10 | RemoveBucket | Bucket bị xóa |

- **VERIFY:** `go test ./client/ -v`

#### D.3 — CLI Smoke Test (`test/smoke_cli.sh`)
- **Deps:** Server chạy thật
- **Cases:**

```bash
# Bucket
gos3c mb s3://cli-test-bucket
gos3c ls                          # → thấy cli-test-bucket
gos3c ls s3://cli-test-bucket/    # → empty

# Object
echo "hello cli" > /tmp/cli-test.txt
gos3c cp /tmp/cli-test.txt s3://cli-test-bucket/test.txt
gos3c ls s3://cli-test-bucket/    # → thấy test.txt
gos3c stat s3://cli-test-bucket/test.txt  # → metadata
gos3c cat s3://cli-test-bucket/test.txt   # → "hello cli"
gos3c cp s3://cli-test-bucket/test.txt /tmp/cli-downloaded.txt
diff /tmp/cli-test.txt /tmp/cli-downloaded.txt  # → identical

# Copy + Move
gos3c cp s3://cli-test-bucket/test.txt s3://cli-test-bucket/copy.txt
gos3c mv s3://cli-test-bucket/copy.txt s3://cli-test-bucket/moved.txt
gos3c ls s3://cli-test-bucket/    # → test.txt + moved.txt (no copy.txt)

# Presign
gos3c presign s3://cli-test-bucket/test.txt --expires 60
# → URL, curl nó phải download được

# Recursive
mkdir -p /tmp/cli-upload-dir && echo "a" > /tmp/cli-upload-dir/a.txt && echo "b" > /tmp/cli-upload-dir/b.txt
gos3c cp --recursive /tmp/cli-upload-dir/ s3://cli-test-bucket/dir/
gos3c ls s3://cli-test-bucket/dir/  # → a.txt, b.txt

# Cleanup
gos3c rm --recursive s3://cli-test-bucket/
gos3c rb s3://cli-test-bucket
```

- **VERIFY:** Script exit code 0

#### D.4 — Auth Smoke Test (`test/smoke_auth.sh`)
- **Deps:** Server chạy thật
- **Cases:**

```bash
# 1. Tạo user mới (root)
gos3c admin user add testuser

# 2. Login bằng key mới → list buckets OK
# (export key mới, chạy aws s3 ls)

# 3. Tạo bucket bằng root, set policy read-only cho testuser
# PUT bucket policy JSON: Allow s3:GetObject cho testuser

# 4. testuser GET object → 200
# 5. testuser PUT object → 403 AccessDenied
# 6. testuser DELETE object → 403 AccessDenied

# 7. Sai key → 403 InvalidAccessKeyId
# 8. Đúng key sai secret → 403 SignatureDoesNotMatch

# 9. Cleanup
gos3c admin user rm testuser
```

- **VERIFY:** Mỗi step assert đúng HTTP status

---

### Phase E: Edge Cases & Regression

#### E.1 — Encoding & Special Characters
- Object key với spaces, unicode, URL-encoded chars
- Bucket name edge cases (3 chars, 63 chars)
- `x-amz-meta-*` với giá trị chứa special chars

#### E.2 — Concurrent Access
- 10 goroutines PUT cùng key → last writer wins, không corrupt
- 10 goroutines PUT khác key → tất cả thành công
- Concurrent ListObjects trong khi PutObject → không crash

#### E.3 — Large Object
- PutObject 50MB → streaming, server không OOM
- Multipart Upload 3 parts x 20MB → Complete thành công

---

## Thứ tự thực hiện

```
Phase A (helpers)
  ├── A.1 → A.2
  │
Phase B (S3 API) — có thể song song sau A
  ├── B.1 (bucket name)     ← không deps
  ├── B.2 (error response)  ← không deps
  ├── B.7 (storage backend) ← không deps
  ├── B.8 (metadata store)  ← không deps
  ├── B.3 (bucket handler)  ← deps A
  ├── B.4 (object handler)  ← deps A
  ├── B.5 (multipart)       ← deps A
  └── B.6 (list objects)    ← deps A
  │
Phase C (auth) — có thể song song với B
  ├── C.1 (sigv4)           ← không deps
  ├── C.2 (policy)          ← không deps
  ├── C.4 (service account) ← không deps
  ├── C.3 (engine)          ← deps C.2
  ├── C.5 (auth middleware)  ← deps A, C.1
  ├── C.6 (rate limit)      ← không deps
  └── C.7 (admin auth)      ← deps A, C.1
  │
Phase D (CLI) — sau B + C
  ├── D.1 (client sigv4)    ← không deps
  ├── D.2 (client lib)      ← deps A
  ├── D.3 (cli smoke)       ← cần server
  └── D.4 (auth smoke)      ← cần server
  │
Phase E (edge cases) — cuối cùng
  └── E.1, E.2, E.3
```

---

## Phase X: Verification Checklist

- [ ] `go test ./...` — tất cả pass
- [ ] `go test ./... -race` — không có data race
- [ ] `go test ./... -cover` — coverage ≥ 60% cho handler, auth, storage
- [ ] `bash test/smoke.sh` — pass
- [ ] `bash test/smoke_cli.sh` — pass
- [ ] `bash test/smoke_auth.sh` — pass
- [ ] `go vet ./...` — không warning
- [ ] `golangci-lint run` — pass

---

*Plan created: 2026-05-15*
