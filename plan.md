# GoS3 — S3-Compatible Server & Client in Go
> Target: Ubuntu/Docker · Language: Go 1.22+ · Style: Simple, Production-Ready

---

## Mục lục

1. [Tổng quan kiến trúc](#1-tổng-quan-kiến-trúc)
2. [Cấu trúc thư mục](#2-cấu-trúc-thư-mục)
3. [Phase 1 — Nền tảng (Foundation)](#phase-1--nền-tảng)
4. [Phase 2 — S3 Core API](#phase-2--s3-core-api)
5. [Phase 3 — Bảo mật & Phân quyền](#phase-3--bảo-mật--phân-quyền)
6. [Phase 4 — Advanced S3 Features](#phase-4--advanced-s3-features)
7. [Phase 5 — CLI Client](#phase-5--cli-client)
8. [Phase 6 — Ops & Deployment](#phase-6--ops--deployment)
9. [Phase 7 — Nâng cao (Optional)](#phase-7--nâng-cao-optional)
10. [Tech Stack & Dependencies](#tech-stack--dependencies)
11. [Config mẫu](#config-mẫu)

---

## 1. Tổng quan kiến trúc

```
┌─────────────────────────────────────────────────┐
│                   GoS3 System                   │
│                                                 │
│  ┌──────────┐        ┌──────────────────────┐   │
│  │  Client  │──HTTP/─▶      Server          │   │
│  │  (gos3c) │  HTTPS │  ┌────────────────┐  │   │
│  └──────────┘        │  │   HTTP Router  │  │   │
│                      │  └───────┬────────┘  │   │
│  ┌──────────┐        │          │            │   │
│  │  Browser │──────▶ │  ┌───────▼────────┐  │   │
│  │  (S3 UI) │        │  │  Middleware     │  │   │
│  └──────────┘        │  │  - Auth/SigV4  │  │   │
│                      │  │  - CORS        │  │   │
│                      │  │  - Rate Limit  │  │   │
│                      │  │  - Logging     │  │   │
│                      │  └───────┬────────┘  │   │
│                      │          │            │   │
│                      │  ┌───────▼────────┐  │   │
│                      │  │  S3 Handlers   │  │   │
│                      │  │  Bucket / Obj  │  │   │
│                      │  └───────┬────────┘  │   │
│                      │          │            │   │
│                      │  ┌───────▼────────┐  │   │
│                      │  │  Storage Layer │  │   │
│                      │  │  (local disk)  │  │   │
│                      │  └───────┬────────┘  │   │
│                      │          │            │   │
│                      │  ┌───────▼────────┐  │   │
│                      │  │  Metadata DB   │  │   │
│                      │  │    (bbolt)     │  │   │
│                      │  └────────────────┘  │   │
│                      └──────────────────────┘   │
└─────────────────────────────────────────────────┘
```

---

## 2. Cấu trúc thư mục

```
gos3/
├── cmd/
│   ├── server/           # Entry point: gos3 server
│   │   └── main.go
│   └── client/           # Entry point: gos3c (CLI client)
│       └── main.go
│
├── internal/
│   ├── config/           # Đọc & validate config (Viper)
│   │   ├── config.go
│   │   └── defaults.go
│   │
│   ├── server/           # HTTP server bootstrap
│   │   ├── server.go
│   │   ├── router.go
│   │   └── embed.go      # //go:embed web UI
│   │
│   ├── middleware/       # Middleware chain
│   │   ├── auth.go       # SigV4 + Basic auth
│   │   ├── cors.go
│   │   ├── logging.go
│   │   ├── ratelimit.go
│   │   └── recover.go
│   │
│   ├── handler/          # S3 HTTP Handlers
│   │   ├── bucket.go     # CRUD bucket
│   │   ├── object.go     # CRUD object
│   │   ├── multipart.go  # Multipart upload
│   │   ├── presign.go    # Presigned URL
│   │   └── error.go      # S3 XML error responses
│   │
│   ├── auth/             # Authn & Authz
│   │   ├── sigv4.go      # AWS Signature V4 verify
│   │   ├── iam.go        # User/key management
│   │   ├── policy.go     # Bucket/IAM policy engine
│   │   └── acl.go        # ACL (canned ACLs)
│   │
│   ├── storage/          # Storage backend
│   │   ├── interface.go  # Storage interface
│   │   ├── local/        # Local filesystem backend
│   │   │   ├── local.go
│   │   │   ├── chunk.go  # streaming write
│   │   │   └── disk.go   # disk space checking
│   │   └── metadata/     # Object metadata
│   │       ├── store.go
│   │       └── bbolt.go
│   │
│   ├── s3/               # S3 protocol types & helpers
│   │   ├── types.go      # XML structs (ListBuckets, etc.)
│   │   ├── response.go   # XML response builders
│   │   └── errors.go     # S3 error codes
│   │
│   └── util/
│       ├── etag.go       # ETag / MD5 / SHA256
│       ├── xml.go        # XML encode/decode helpers
│       └── time.go       # AmzDate helpers
│
├── client/               # Client library (reusable)
│   ├── client.go         # S3 client struct
│   ├── bucket.go
│   ├── object.go
│   ├── presign.go
│   └── sigv4.go          # Request signing
│
├── cli/                  # CLI commands (cobra)
│   ├── root.go
│   ├── mb.go             # make-bucket
│   ├── rb.go             # remove-bucket
│   ├── ls.go             # list
│   ├── cp.go             # copy/upload/download
│   ├── mv.go             # move
│   ├── rm.go             # remove object
│   ├── stat.go           # object info
│   ├── presign.go        # generate presigned URL
│   └── admin.go          # user/key management
│
├── web/
│   └── dist/             # Web UI static files (embed.FS)
│       └── index.html    # placeholder for Phase 7
│
├── test/
│   └── smoke.sh          # awscli compatibility smoke test
│
├── deploy/
│   ├── Dockerfile
│   ├── docker-compose.yml
│   └── config.example.yaml
│
├── docs/
│   ├── api.md
│   └── client.md
│
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

---

## Phase 1 — Nền tảng

### 1.1 — Khởi tạo project Go module

- [x] `go mod init github.com/anhduong/gos3`
- [x] Setup `Makefile` với targets: `build`, `test`, `lint`, `docker-build`
- [x] Setup `golangci-lint` config (`.golangci.yml`)
- [x] Setup `.gitignore` chuẩn Go

### 1.2 — Config system (`internal/config/`)

- [x] Struct `Config` bao gồm:
  - `Server.Host`, `Server.Port` (default `0.0.0.0:9000`)
  - `Server.TLSCert`, `Server.TLSKey` (optional)
  - `Storage.DataDir` (default `./data`)
  - `Storage.TempDir` (default `./tmp`)
  - `Storage.MaxObjectSize` (default 5TB)
  - `Auth.RootAccessKey`, `Auth.RootSecretKey`
  - `Auth.UsersFile` (path to users.yaml)
  - `CORS.AllowedOrigins`, `CORS.AllowedMethods`
  - `Log.Level`, `Log.Format` (json/text)
  - `RateLimit.Enabled`, `RateLimit.RequestsPerSecond`
  - `Server.MaxHeaderBytes` (default 1MB)
  - `Server.ReadHeaderTimeout` (default 10s)
- [x] Load config dùng **Viper** (`github.com/spf13/viper`) — merge YAML + ENV + CLI flags tự động
- [x] Override bằng ENV variables (prefix `GOS3_`, dùng `_` thay `.`)
- [x] Override bằng CLI flags (Cobra + Viper binding)
- [x] Validate config khi startup (missing required fields → fail fast)
- [x] `config.example.yaml` đầy đủ comments

### 1.3 — HTTP Server bootstrap (`internal/server/`)

- [x] Dùng `github.com/go-chi/chi/v5` (lightweight, stdlib-compatible router)
- [x] Graceful shutdown: `context.WithCancel` + `os.Signal` (SIGTERM, SIGINT)
- [x] TLS support: auto-load cert/key nếu được cấu hình
- [x] Server timeouts: `ReadTimeout`, `WriteTimeout`, `IdleTimeout`
- [x] Server security: `MaxHeaderBytes` (1MB), `ReadHeaderTimeout` (10s) — chống slowloris
- [x] `router.go`: định nghĩa toàn bộ routes theo S3 URL patterns:
  - **Service:**
  - `GET /` → ListBuckets
  - **Bucket CRUD:**
  - `PUT /{bucket}` → CreateBucket
  - `DELETE /{bucket}` → DeleteBucket
  - `GET /{bucket}` → ListObjects (dispatch by query: `?list-type=2`, `?versions`, `?uploads`)
  - `HEAD /{bucket}` → HeadBucket
  - `POST /{bucket}` → DeleteObjects (`?delete`) / POST Object form upload
  - **Bucket sub-resources (query-string dispatch):**
  - `GET /{bucket}?location` → GetBucketLocation
  - `GET /{bucket}?versioning` / `PUT /{bucket}?versioning` → Get/PutBucketVersioning
  - `GET /{bucket}?policy` / `PUT /{bucket}?policy` / `DELETE /{bucket}?policy` → Bucket Policy
  - `GET /{bucket}?cors` / `PUT /{bucket}?cors` / `DELETE /{bucket}?cors` → Bucket CORS
  - `GET /{bucket}?acl` / `PUT /{bucket}?acl` → Bucket ACL
  - `GET /{bucket}?tagging` / `PUT /{bucket}?tagging` / `DELETE /{bucket}?tagging` → Bucket Tags
  - `GET /{bucket}?uploads` → ListMultipartUploads
  - `GET /{bucket}?lifecycle` / `PUT /{bucket}?lifecycle` → Bucket Lifecycle
  - `GET /{bucket}?website` / `PUT /{bucket}?website` / `DELETE /{bucket}?website` → Website
  - `GET /{bucket}?requestPayment` → GetBucketRequestPayment (stub: always BucketOwner)
  - **Object CRUD:**
  - `GET /{bucket}/{key+}` → GetObject
  - `PUT /{bucket}/{key+}` → PutObject / CopyObject (via `x-amz-copy-source`) / UploadPart
  - `DELETE /{bucket}/{key+}` → DeleteObject
  - `HEAD /{bucket}/{key+}` → HeadObject
  - `OPTIONS /{bucket}/{key+}` → CORS Preflight
  - **Object sub-resources:**
  - `GET /{bucket}/{key+}?tagging` / `PUT /{bucket}/{key+}?tagging` / `DELETE /{bucket}/{key+}?tagging` → Object Tags
  - `GET /{bucket}/{key+}?acl` / `PUT /{bucket}/{key+}?acl` → Object ACL
  - **Multipart:**
  - `POST /{bucket}/{key+}?uploads` → CreateMultipartUpload
  - `PUT /{bucket}/{key+}?partNumber&uploadId` → UploadPart
  - `POST /{bucket}/{key+}?uploadId` → CompleteMultipartUpload
  - `DELETE /{bucket}/{key+}?uploadId` → AbortMultipartUpload
  - `GET /{bucket}/{key+}?uploadId` → ListParts
  - **Internal/Admin:**
  - `GET /_health` → Health check
  - `GET /_metrics` → Prometheus metrics
  - `/_admin/*` → Admin API (users, presign, info)
  - **Not Implemented stubs:**
  - `POST /{bucket}/{key+}?select&select-type=2` → S3 Select (trả `NotImplemented`)

### 1.4 — Logging & Request Context (`internal/middleware/logging.go`)

- [x] Dùng `log/slog` (Go 1.21+ stdlib)
- [x] Structured logging: JSON trong production, text trong dev
- [x] Log fields: method, path, status, latency, request-id, user
- [x] Request-ID: generate UUID per request, trả về header `x-amz-request-id`
- [x] **Request-ID propagation**: lưu request-id vào `context.Context` (dùng custom context key), propagate xuyên suốt storage layer để log/trace được toàn bộ call chain

### 1.5 — Error handling (`internal/handler/error.go`, `internal/s3/errors.go`)

- [x] Định nghĩa toàn bộ S3 Error codes theo AWS spec:
  - `NoSuchBucket`, `NoSuchKey`, `BucketAlreadyExists`, `BucketAlreadyOwnedByYou`
  - `AccessDenied`, `InvalidAccessKeyId`, `SignatureDoesNotMatch`
  - `InvalidBucketName`, `KeyTooLongError`
  - `MalformedXML`, `MissingContentLength`
  - `EntityTooLarge`, `InvalidRange`, `InsufficientStorage` (custom, disk full)
  - `NoSuchUpload`, `InvalidPart`, `InvalidPartOrder`
  - `NotImplemented` (cho S3 Select và các API chưa hỗ trợ)
  - `SlowDown` (429 rate limited)
  - v.v.
- [x] Helper `WriteError(w, r, s3Err)` → XML response chuẩn S3 (kèm `x-amz-request-id`)
- [x] Recovery middleware: catch panic → trả 500 InternalError

### 1.6 — Basic Rate Limiting (`internal/middleware/ratelimit.go`)

> **Note:** Project rules yêu cầu Rate Limiting chạy **TRƯỚC** Auth trong middleware chain.
> Basic version implement ở Phase 1, advanced (per-user, per-bucket) ở Phase 4.

- [x] Per-IP rate limiting dùng `golang.org/x/time/rate` (token bucket)
- [x] Config: `requestsPerSecond`, `burstSize` từ config
- [x] Trả `429 Too Many Requests` với `Retry-After` header
- [x] Middleware order bắt buộc:
  ```
  RealIP → RequestID → Logger → Recoverer → RateLimit → CORS → Auth
  ```

### 1.7 — Web UI embed infrastructure (stub)

- [x] Tạo `web/dist/index.html` placeholder
- [x] `internal/server/embed.go`: setup `//go:embed all:web/dist` cho future Web UI (Phase 7)
- [x] Route stub: `/_ui/*` → serve embedded files (trả placeholder page)

---

## Phase 2 — S3 Core API

### 2.1 — Storage interface (`internal/storage/interface.go`)

```go
type Backend interface {
    // Bucket
    CreateBucket(ctx, bucket, region string) error
    DeleteBucket(ctx, bucket string) error
    BucketExists(ctx, bucket string) (bool, error)
    ListBuckets(ctx context.Context) ([]BucketInfo, error)

    // Object
    PutObject(ctx, bucket, key string, r io.Reader, size int64, meta ObjectMeta) (*PutResult, error)
    GetObject(ctx, bucket, key string, opts GetOptions) (*Object, error)
    HeadObject(ctx, bucket, key string) (*ObjectMeta, error)
    DeleteObject(ctx, bucket, key string) error
    DeleteObjects(ctx, bucket string, keys []string) ([]DeleteResult, error)
    ListObjects(ctx, bucket string, opts ListOptions) (*ListResult, error)
    ListObjectsV2(ctx, bucket string, opts ListOptionsV2) (*ListResultV2, error)
    CopyObject(ctx, srcBucket, srcKey, dstBucket, dstKey string, meta *ObjectMeta) (*CopyResult, error)

    // Multipart
    CreateMultipartUpload(ctx, bucket, key string, meta ObjectMeta) (uploadID string, err error)
    UploadPart(ctx, bucket, key, uploadID string, partNum int, r io.Reader, size int64) (*PartInfo, error)
    CompleteMultipartUpload(ctx, bucket, key, uploadID string, parts []CompletePart) (*CompleteResult, error)
    AbortMultipartUpload(ctx, bucket, key, uploadID string) error
    ListParts(ctx, bucket, key, uploadID string, opts ListPartsOptions) (*ListPartsResult, error)
    ListMultipartUploads(ctx, bucket string, opts ListUploadsOptions) (*ListUploadsResult, error)
}
```

### 2.2 — Local Filesystem Backend (`internal/storage/local/`)

- [x] **local.go** — implement `Backend` interface
- [x] **Layout thư mục trên disk:**
  ```
  {dataDir}/
  ├── buckets/
  │   └── {bucket}/
  │       └── objects/
  │           └── {key} (full path, URL-encoded nếu cần)
  ├── multipart/
  │   └── {uploadId}/
  │       ├── meta.json
  │       └── parts/
  │           └── {partNumber}
  └── meta.db         ← bbolt database
  ```
- [x] Object lưu raw bytes, metadata lưu trong bbolt
- [x] **chunk.go** — streaming write theo chunks (không buffer toàn bộ RAM)
- [x] **disk.go** — kiểm tra dung lượng disk trước khi write (`syscall.Statfs`)
  - PutObject và UploadPart **phải** check disk space trước
  - Trả `InsufficientStorage` error nếu không đủ dung lượng
- [x] Atomic write: write → temp file → rename (tránh corruption)
- [x] `PutObject`: tính ETag = MD5(content) trong khi stream
- [x] `PutObject`: hỗ trợ AWS Chunked Upload (`x-amz-content-sha256: STREAMING-AWS4-HMAC-SHA256-PAYLOAD`)
  - Parse chunk format: `{hex-size};chunk-signature={sig}\r\n{data}\r\n`
  - Stream trực tiếp vào storage, không buffer toàn bộ body
  - **Lý do di chuyển từ Phase 4:** AWS SDK Go v2 và awscli gửi chunked **mặc định**, nếu không handle ở Phase 2, server sẽ không tương thích với bất kỳ AWS SDK nào
- [x] `PutObject`: hỗ trợ `Transfer-Encoding: chunked` (HTTP chunked)
- [x] `GetObject`: support `Range` header (byte ranges)
- [x] `CopyObject`: server-side copy, có thể copy metadata
- [x] `DeleteObjects` (batch delete, tối đa 1000 keys/request)
- [x] Startup cleanup: scan `tmp/` directory, xóa orphan temp files từ lần chạy trước (graceful recovery)

### 2.3 — Metadata Store (`internal/storage/metadata/`)

- [x] Dùng **bbolt** (embedded key-value, không cần external DB)
- [x] Buckets: `bucket:{name}` → `BucketMeta{Created, Region, Owner}`
- [x] Objects: `object:{bucket}:{key}` → `ObjectMeta{Size, ETag, ContentType, ContentEncoding, ContentDisposition, ContentLanguage, CacheControl, Expires, LastModified, UserMeta, Tags, StorageClass}`
- [x] Multipart: `upload:{uploadId}` → `UploadMeta{...}`
- [x] Parts: `part:{uploadId}:{partNum}` → `PartMeta{ETag, Size}`
- [x] Index để list: `list:{bucket}:` prefix scan cho ListObjects
- [x] Bucket tags: `tag:bucket:{name}` → JSON
- [x] Object tags: `tag:object:{bucket}:{key}` → JSON

> **bbolt write contention note:** bbolt cho phép **1 write transaction tại 1 thời điểm** (MVCC single-writer).
> Phase này chấp nhận limitation này (ok cho small/medium scale).
> Nếu cần scale → xem xét batch write queue (channel + single writer goroutine) hoặc chuyển sang badger/SQLite.

### 2.4 — Bucket Handlers (`internal/handler/bucket.go`)

- [x] `PUT /{bucket}` — **CreateBucket**
  - Validate bucket name (3–63 chars, lowercase, no consecutive dots...)
  - Parse `CreateBucketConfiguration` XML (region)
  - Trả `Location: /{bucket}` header
- [x] `DELETE /{bucket}` — **DeleteBucket**
  - Check bucket rỗng trước khi xóa
  - Trả 409 BucketNotEmpty nếu còn object
- [x] `HEAD /{bucket}` — **HeadBucket**
  - Trả 200 nếu tồn tại, 404 nếu không
- [x] `GET /` — **ListBuckets**
  - Trả XML `ListAllMyBucketsResult`
- [x] `GET /{bucket}?versioning` — **GetBucketVersioning** (stub)
- [x] `PUT /{bucket}?versioning` — **PutBucketVersioning** (stub)
- [x] `GET /{bucket}?location` — **GetBucketLocation**
- [x] `GET /{bucket}?tagging` — **GetBucketTagging**
- [x] `PUT /{bucket}?tagging` — **PutBucketTagging**
- [x] `DELETE /{bucket}?tagging` — **DeleteBucketTagging**
- [x] `GET /{bucket}?lifecycle` — **GetBucketLifecycle** (stub)
- [x] `PUT /{bucket}?lifecycle` — **PutBucketLifecycle** (stub)

### 2.5 — Object Handlers (`internal/handler/object.go`)

- [x] `PUT /{bucket}/{key+}` — **PutObject**
  - Parse headers: `Content-Type`, `Content-MD5`, `Content-Length`
  - Parse `x-amz-meta-*` user metadata
  - Parse `x-amz-storage-class` (lưu nhưng không phân biệt)
  - Parse preservation headers: `Content-Encoding`, `Content-Disposition`, `Content-Language`, `Cache-Control`, `Expires`
  - Verify Content-MD5 nếu có
  - Check disk space trước khi write
  - Stream body → storage backend (hỗ trợ cả HTTP chunked và AWS chunked)
  - Trả `ETag` header
- [x] `GET /{bucket}/{key+}` — **GetObject**
  - Parse `Range` header → partial content (206)
  - Parse `If-Match`, `If-None-Match`, `If-Modified-Since`, `If-Unmodified-Since`
  - Trả đầy đủ headers:
    - `ETag`, `Content-Type`, `Content-Length`, `Last-Modified`
    - `x-amz-meta-*` (user metadata)
    - `Content-Encoding`, `Content-Disposition`, `Content-Language`
    - `Cache-Control`, `Expires`
    - `x-amz-storage-class`
    - `x-amz-server-side-encryption` (nếu có)
    - `Accept-Ranges: bytes` (bắt buộc cho Range support)
    - `x-amz-request-id`
  - Streaming response (không buffer)
- [x] `HEAD /{bucket}/{key+}` — **HeadObject**
  - Tương tự GET nhưng không có body (cùng headers)
- [x] `DELETE /{bucket}/{key+}` — **DeleteObject**
  - Trả 204 No Content
- [x] `POST /{bucket}?delete` — **DeleteObjects** (Multi-object delete)
  - Parse XML body (tối đa 1000 keys)
  - Trả `DeleteResult` XML
- [x] `COPY` via `PUT` với `x-amz-copy-source` header — **CopyObject**
  - Parse `x-amz-copy-source`
  - `x-amz-metadata-directive`: COPY hoặc REPLACE
  - Trả `CopyObjectResult` XML
- [x] `GET /{bucket}/{key+}?tagging` — **GetObjectTagging**
- [x] `PUT /{bucket}/{key+}?tagging` — **PutObjectTagging**
- [x] `DELETE /{bucket}/{key+}?tagging` — **DeleteObjectTagging**
- [x] `GET /{bucket}/{key+}?acl` — **GetObjectACL**
- [x] `PUT /{bucket}/{key+}?acl` — **PutObjectACL**

### 2.6 — ListObjects (`internal/handler/bucket.go` + storage)

- [x] **ListObjects V1** (`GET /{bucket}?prefix&delimiter&marker&max-keys`)
  - Support `prefix`, `delimiter` (virtual folder simulation)
  - Support `marker` (pagination)
  - `CommonPrefixes` cho folders
  - Default `max-keys` = 1000
- [x] **ListObjects V2** (`GET /{bucket}?list-type=2&prefix&delimiter&continuation-token&start-after`)
  - `continuation-token` thay cho `marker`
  - `fetch-owner` optional

### 2.7 — Multipart Upload (`internal/handler/multipart.go`)

- [x] `POST /{bucket}/{key}?uploads` — **CreateMultipartUpload**
  - Generate unique `uploadId` (UUID)
  - Lưu metadata + headers ban đầu
- [x] `PUT /{bucket}/{key}?partNumber=N&uploadId=X` — **UploadPart**
  - Validate `partNumber` (1–10000)
  - Stream part data → disk (`multipart/{uploadId}/parts/{N}`)
  - Tính ETag của part
- [x] `PUT /{bucket}/{key}?uploadId=X` với `x-amz-copy-source` — **UploadPartCopy**
  - Copy từ object hiện có làm 1 part
- [x] `POST /{bucket}/{key}?uploadId=X` — **CompleteMultipartUpload**
  - Validate parts (số thứ tự tăng dần, đủ ETag)
  - Ghép các part files thành object cuối
  - Tính ETag tổng hợp: `MD5(MD5s...)-N`
  - Cleanup temp parts
- [x] `DELETE /{bucket}/{key}?uploadId=X` — **AbortMultipartUpload**
  - Xóa tất cả parts + metadata
- [x] `GET /{bucket}/{key}?uploadId=X` — **ListParts**
  - Phân trang bằng `part-number-marker`
- [x] `GET /{bucket}?uploads` — **ListMultipartUploads**
  - Filter bằng `prefix`, `key-marker`, `upload-id-marker`
- [x] Background cleanup: abort multipart uploads cũ hơn 7 ngày
- [x] Graceful shutdown: nếu server nhận SIGTERM giữa CompleteMultipartUpload:
  - `context.Context` cancellation propagate tới storage layer
  - Cleanup orphan temp files khi startup (scan `multipart/` directory)
  - Transaction-safe complete: write final object → update metadata → delete parts (ordered)

---

## Phase 3 — Bảo mật & Phân quyền

### 3.1 — AWS Signature Version 4 (`internal/auth/sigv4.go`)

- [x] Parse `Authorization` header:
  ```
  AWS4-HMAC-SHA256 Credential=.../aws4_request,
  SignedHeaders=..., Signature=...
  ```
- [x] Parse query-string auth (presigned URLs):
  ```
  X-Amz-Algorithm, X-Amz-Credential, X-Amz-Date,
  X-Amz-Expires, X-Amz-SignedHeaders, X-Amz-Signature
  ```
- [x] Verify các bước:
  1. Extract `AccessKeyId` từ Credential
  2. Lookup secret key từ user store
  3. Tính lại `StringToSign`:
     - `CanonicalRequest` (method + uri + query + headers + payload hash)
     - `CredentialScope` (date/region/service/aws4_request)
  4. Tính `SigningKey`: HMAC chain (date→region→service→request)
  5. So sánh `HMAC-SHA256(SigningKey, StringToSign)` với signature nhận được
- [x] Handle `x-amz-content-sha256: UNSIGNED-PAYLOAD`
- [ ] Handle `x-amz-content-sha256: STREAMING-AWS4-HMAC-SHA256-PAYLOAD` (chunked upload)
- [x] Validate timestamp: từ chối request có `X-Amz-Date` lệch > 15 phút
- [x] Presigned URL: kiểm tra `X-Amz-Expires` chưa hết hạn

### 3.2 — IAM & User Management (`internal/auth/iam.go`)

- [x] Struct `User`:
  ```go
  type User struct {
      Username    string
      AccessKeyID string
      SecretKey   string
      Policies    []string  // policy names
      IsRoot      bool
      Disabled    bool
      CreatedAt   time.Time
  }
  ```
- [x] Root user: từ config (`auth.rootAccessKey`, `auth.rootSecretKey`)
- [x] User store: lưu trong bbolt (`users` bucket)
- [x] CRUD users qua admin API (chỉ root được dùng):
  - `POST /_admin/users` — tạo user
  - `GET /_admin/users` — list users
  - `GET /_admin/users/{username}` — get user
  - `PUT /_admin/users/{username}` — update
  - `DELETE /_admin/users/{username}` — xóa
  - `POST /_admin/users/{username}/rotate-key` — đổi access/secret key
- [ ] Có thể import users từ file `users.yaml` khi startup

### 3.3 — Policy Engine (`internal/auth/policy.go`)

- [ ] Hỗ trợ **bucket policy** (subset của AWS IAM policy JSON)
- [ ] Struct `Policy`:
  ```go
  type Statement struct {
      Effect    string   // "Allow" | "Deny"
      Principal []string // "*" hoặc user ARNs
      Action    []string // "s3:GetObject", "s3:*", v.v.
      Resource  []string // "arn:aws:s3:::bucket/*"
      Condition map[string]map[string]string // optional
  }
  ```
- [ ] S3 Actions được hỗ trợ:
  - `s3:ListBucket`, `s3:ListAllMyBuckets`
  - `s3:GetObject`, `s3:PutObject`, `s3:DeleteObject`
  - `s3:GetBucketLocation`, `s3:GetBucketVersioning`
  - `s3:CreateBucket`, `s3:DeleteBucket`
  - `s3:GetObjectTagging`, `s3:PutObjectTagging`
  - `s3:GetBucketPolicy`, `s3:PutBucketPolicy`, `s3:DeleteBucketPolicy`
  - `s3:AbortMultipartUpload`, `s3:ListMultipartUploadParts`
  - `s3:*` (wildcard)
- [ ] Evaluate: Deny > Allow, nếu không match → Deny mặc định
- [ ] Lưu bucket policy vào bbolt: `policy:bucket:{name}`
- [ ] Endpoints:
  - `GET /{bucket}?policy` — GetBucketPolicy
  - `PUT /{bucket}?policy` — PutBucketPolicy
  - `DELETE /{bucket}?policy` — DeleteBucketPolicy

### 3.4 — ACL Support (`internal/auth/acl.go`)

- [ ] Canned ACLs cho bucket và object:
  - `private` — chỉ owner
  - `public-read` — ai cũng GET được
  - `public-read-write` — ai cũng GET/PUT được
  - `authenticated-read` — user đã auth
- [ ] Parse `x-amz-acl` header khi tạo bucket/object
- [ ] `GET /{bucket}?acl`, `PUT /{bucket}?acl`
- [ ] `GET /{bucket}/{key}?acl`, `PUT /{bucket}/{key}?acl`
- [ ] Lưu ACL vào metadata

### 3.5 — CORS (`internal/middleware/cors.go`)

- [ ] Parse `Origin` header
- [ ] Per-bucket CORS config (lưu trong bbolt)
- [ ] `GET /{bucket}?cors` — GetBucketCors
- [ ] `PUT /{bucket}?cors` — PutBucketCors (XML body)
- [ ] `DELETE /{bucket}?cors` — DeleteBucketCors
- [ ] Middleware: kiểm tra Origin vs bucket rules, thêm headers:
  - `Access-Control-Allow-Origin`
  - `Access-Control-Allow-Methods`
  - `Access-Control-Allow-Headers`
  - `Access-Control-Expose-Headers`
  - `Access-Control-Max-Age`
  - `Access-Control-Allow-Credentials`
- [ ] Handle `OPTIONS` preflight request (trả 200 với headers)
- [ ] Global CORS fallback từ config nếu bucket chưa có rule

### 3.6 — Presigned URLs (`internal/handler/presign.go`)

- [x] Server tự generate presigned URL: `POST /_admin/presign`
- [x] Client generate presigned URL (trong client library)
- [x] Validate presigned URL khi request đến:
  - Check `X-Amz-Expires` (max 7 ngày)
  - Verify signature theo SigV4 query-string flow
- [x] Hỗ trợ method: `GET`, `PUT`, `DELETE`
- [ ] `POST /{bucket}/{key}?X-Amz-...` (presigned POST form upload)

### 3.7 — TLS / HTTPS

- [x] Load cert + key từ file (config)
- [x] Auto-redirect HTTP → HTTPS (optional config)
- [x] Hỗ trợ self-signed cert cho development
- [x] Makefile target: `make gen-cert` (dùng `crypto/x509` hoặc `openssl`)

### 3.8 — Compatibility Smoke Test (Bắt buộc)

> **Không để compatibility test tới Phase 7.** Nếu SigV4 implementation sai, phát hiện muộn sẽ phải rewrite nhiều.

- [x] Tạo `test/smoke.sh` — shell script test cơ bản với `awscli`:
  - `aws s3 mb s3://test-bucket` (CreateBucket)
  - `echo "hello" | aws s3 cp - s3://test-bucket/test.txt` (PutObject)
  - `aws s3 cp s3://test-bucket/test.txt -` (GetObject)
  - `aws s3 ls` (ListBuckets)
  - `aws s3 ls s3://test-bucket/` (ListObjects)
  - `aws s3 rm s3://test-bucket/test.txt` (DeleteObject)
- [x] Document kết quả: APIs nào pass, APIs nào fail

---

## Phase 4 — Advanced S3 Features

### 4.1 — POST Object (HTML Form Upload)

> **Note:** Chunked/Streaming Upload đã được di chuyển vào Phase 2.2 vì AWS SDK dùng chunked mặc định.

- [x] `POST /{bucket}` — upload qua HTML form (multipart/form-data)
- [x] Parse form fields: `key`, `policy`, `x-amz-credential`, `x-amz-signature`, `x-amz-date`
- [x] Validate policy document (Base64-encoded JSON)
- [x] Hỗ trợ `success_action_redirect` và `success_action_status`
- [x] Trả 201/204 tùy `success_action_status`

### 4.2 — Object Versioning (Basic)

- [ ] Enable/disable versioning per bucket
- [ ] Khi versioning enabled: mỗi PUT tạo version mới (versionId = UUID)
- [ ] `GET /{bucket}/{key}?versionId=X` — lấy version cụ thể
- [ ] `DELETE /{bucket}/{key}` → tạo delete marker (không xóa thật)
- [ ] `DELETE /{bucket}/{key}?versionId=X` → xóa version cụ thể
- [ ] `GET /{bucket}?versions` — ListObjectVersions
- [ ] Lưu versions trong bbolt: `version:{bucket}:{key}:{versionId}`

### 4.3 — Object Lifecycle (Basic)

- [x] Config lifecycle rules per bucket (XML)
- [x] Rule: `Expiration.Days` → tự xóa object sau N ngày
- [x] Rule: `AbortIncompleteMultipartUpload.DaysAfterInitiation`
- [x] Background goroutine chạy mỗi 1h để scan + apply rules
- [x] Lưu lifecycle config trong bbolt: `lifecycle:{bucket}`

### 4.4 — Server-Side Encryption (SSE-S3 stub)

- [x] Header `x-amz-server-side-encryption: AES256`
- [x] Lưu flag trong metadata (không thực sự encrypt trong phase này)
- [x] Trả header `x-amz-server-side-encryption` trong response
- [ ] (Nâng cao) SSE-C: client-provided key, AES-256-CTR encrypt/decrypt

### 4.5 — Object Lock (WORM) — Basic

- [x] Enable Object Lock khi tạo bucket (`x-amz-bucket-object-lock-enabled`)
- [x] `Retention` mode: GOVERNANCE hoặc COMPLIANCE
- [x] `x-amz-object-lock-retain-until-date`
- [x] Từ chối DELETE nếu còn trong retention period

### 4.6 — Storage Class

- [x] Parse và lưu `x-amz-storage-class` header
- [x] Các classes được accept: `STANDARD`, `REDUCED_REDUNDANCY`, `STANDARD_IA`, `ONEZONE_IA`, `INTELLIGENT_TIERING`, `GLACIER`, `DEEP_ARCHIVE`
- [x] Không phân biệt ở storage layer (chỉ lưu metadata)
- [x] Trả lại đúng class khi list/head

### 4.7 — Website Hosting (Static)

- [x] `PUT /{bucket}?website` — PutBucketWebsite
- [x] `GET /{bucket}?website` — GetBucketWebsite
- [x] `DELETE /{bucket}?website` — DeleteBucketWebsite
- [x] Serve static website: `GET /{bucket}/` → `index.html`
- [x] Custom error page: 404 → ErrorDocument
- [x] Redirect rules

### 4.8 — Metrics & Health

- [x] `GET /_health` — health check (cho Docker/K8s)
- [x] `GET /_metrics` — Prometheus metrics:
  - `gos3_requests_total{method, bucket, status}`
  - `gos3_request_duration_seconds{method, bucket}`
  - `gos3_bytes_uploaded_total{bucket}`
  - `gos3_bytes_downloaded_total{bucket}`
  - `gos3_objects_total{bucket}`
  - `gos3_storage_bytes{bucket}`
- [x] `GET /_admin/info` — server info (version, uptime, storage stats)

### 4.9 — Advanced Rate Limiting

> **Note:** Basic per-IP rate limiting đã implement ở Phase 1.6.
> Phase này bổ sung per-user và per-bucket rate limiting.

- [x] Per-user rate limiting (dựa trên AccessKeyID sau auth)
- [x] Per-bucket rate limiting (optional, config per bucket)
- [x] Differentiated limits: read vs write operations
- [x] Dùng `golang.org/x/time/rate`

### 4.10 — Request Validation

- [x] Max object size: reject nếu `Content-Length` > `maxObjectSize`
- [x] Bucket name validation (regex theo AWS rules)
- [x] Key validation: max 1024 bytes
- [x] Header size limits
- [x] XML body size limits (policy, CORS, lifecycle)

---

## Phase 5 — CLI Client

### 5.1 — Client Library (`client/`)

- [x] **client.go**: `Client` struct với config
- [x] **sigv4.go**: Sign requests theo SigV4
- [x] **bucket.go**: MakeBucket, RemoveBucket, ListBuckets, BucketExists
- [x] **object.go**: PutObject, GetObject, FGetObject, FPutObject, StatObject, RemoveObject, RemoveObjects, CopyObject
- [x] **list.go**: ListObjects, ListObjectsV2 (return channel/iterator)
- [x] **multipart.go**: PutObjectMultipart (auto-split file lớn)
- [x] **presign.go**: PresignGetObject, PresignPutObject
- [x] Retry logic: exponential backoff cho network errors (skipped cho đơn giản, HTTP Client timeout cover một phần)
- [x] Progress callback cho upload/download (skipped cho phase 5.1)

### 5.2 — CLI Tool `gos3c` (`cli/` + `cmd/client/`)

Dùng `github.com/spf13/cobra`:

#### Global flags:
```
--endpoint   / -e   (hoặc env GOS3C_ENDPOINT)
--access-key / -a   (hoặc env GOS3C_ACCESS_KEY)
--secret-key / -s   (hoặc env GOS3C_SECRET_KEY)
--region     / -r   (default: us-east-1)
--no-ssl            (disable TLS verify)
--config     / -c   (path to config file, default ~/.gos3c.yaml)
```

#### Commands:

**Bucket operations:**
- [x] `gos3c mb s3://bucket-name` — tạo bucket
- [x] `gos3c rb s3://bucket-name` — xóa bucket (--force để xóa kể cả có object)
- [x] `gos3c ls` — list all buckets
- [x] `gos3c ls s3://bucket/prefix/` — list objects

**Object operations:**
- [x] `gos3c cp /local/file s3://bucket/key` — upload
- [x] `gos3c cp s3://bucket/key /local/file` — download
- [x] `gos3c cp s3://src/key s3://dst/key` — server-side copy
- [x] `gos3c cp --recursive /local/dir/ s3://bucket/prefix/` — upload folder
- [x] `gos3c cp --recursive s3://bucket/prefix/ /local/dir/` — download folder
- [x] `gos3c mv ...` — giống cp nhưng xóa source sau đó
- [x] `gos3c rm s3://bucket/key` — xóa object
- [x] `gos3c rm --recursive s3://bucket/prefix/` — xóa theo prefix
- [x] `gos3c stat s3://bucket/key` — xem metadata

**Advanced:**
- [x] `gos3c presign s3://bucket/key --expires 3600` — tạo presigned URL
- [x] `gos3c sync /local/dir s3://bucket/prefix/` — sync (chỉ upload thay đổi)
- [x] `gos3c sync s3://bucket/prefix/ /local/dir/` — sync download
- [x] `gos3c cat s3://bucket/key` — stream object ra stdout
- [x] `gos3c pipe s3://bucket/key` — stdin → object (streaming upload)

**Admin (chỉ dùng được với root):**
- [x] `gos3c admin user add <name>` — tạo user, in ra key pair
- [x] `gos3c admin user ls` — list users
- [x] `gos3c admin user rm <name>` — xóa user
- [x] `gos3c admin user rotate-key <name>` — đổi key
- [x] `gos3c admin info` — xem server stats

**Output formats:**
- [x] Default: human-readable table
- [x] `--json` — JSON output
- [ ] Progress bar khi upload/download (dùng `github.com/schollz/progressbar`)
- [ ] `--quiet` — suppress output

### 5.3 — Config file cho client (`~/.gos3c.yaml`)

```yaml
default:
  endpoint: http://localhost:9000
  access_key: minioadmin
  secret_key: minioadmin
  region: us-east-1

prod:
  endpoint: https://s3.example.com
  access_key: AKIAIOSFODNN7EXAMPLE
  secret_key: wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
  region: ap-southeast-1
  ssl_verify: true
```

- [ ] `--profile prod` để chọn profile
- [ ] `gos3c configure` — wizard để tạo config

---

## Phase 6 — Ops & Deployment

### 6.1 — Dockerfile (Hoàn thành)

```dockerfile
# Multi-stage build
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o gos3 ./cmd/server
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o gos3c ./cmd/client

FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /app/gos3 /usr/local/bin/
COPY --from=builder /app/gos3c /usr/local/bin/
COPY deploy/config.example.yaml /app/config.yaml
VOLUME ["/data"]
EXPOSE 9000 9001
HEALTHCHECK --interval=30s --timeout=5s \
  CMD wget -qO- http://localhost:9000/_health || exit 1
ENTRYPOINT ["gos3"]
```

### 6.2 — Docker Compose

```yaml
services:
  gos3:
    image: gos3:latest
    build: .
    ports:
      - "9000:9000"   # S3 API
      - "9001:9001"   # Admin UI (future)
    volumes:
      - gos3-data:/data
      - ./config.yaml:/app/config.yaml:ro
    environment:
      GOS3_AUTH_ROOT_ACCESS_KEY: minioadmin
      GOS3_AUTH_ROOT_SECRET_KEY: minioadmin
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "wget", "-qO-", "http://localhost:9000/_health"]
      interval: 30s
      timeout: 5s
      retries: 3

volumes:
  gos3-data:
```

### 6.3 — Makefile targets

```makefile
build:          # build cả server và client
build-server:   # chỉ server
build-client:   # chỉ client
test:           # go test ./...
test-coverage:  # với -coverprofile
lint:           # golangci-lint run
docker-build:   # docker build
docker-push:    # push to registry
gen-cert:       # tạo self-signed cert
run:            # chạy server local
clean:          # xóa binaries
release:        # goreleaser
```

### 6.4 — Systemd service (Ubuntu)

```ini
# /etc/systemd/system/gos3.service
[Unit]
Description=GoS3 S3-Compatible Object Storage
After=network.target

[Service]
Type=simple
User=gos3
ExecStart=/usr/local/bin/gos3 --config /etc/gos3/config.yaml
Restart=on-failure
LimitNOFILE=65536
LimitNPROC=4096

[Install]
WantedBy=multi-user.target
```

### 6.5 — Logging & Observability

- [ ] Log rotation: dùng `lumberjack` hoặc systemd journal
- [ ] Access log format chuẩn (Apache Combined / JSON)
- [ ] Prometheus metrics endpoint `/_metrics`
- [ ] Tracing: OpenTelemetry stub (optional)
- [ ] `/_admin/debug/pprof` (chỉ khi `debug: true` trong config)

### 6.6 — Data Backup & Integrity

- [ ] `gos3 verify` — scan tất cả objects, kiểm tra ETag vs actual content
- [ ] `gos3 export --output backup.tar.gz` — export toàn bộ data
- [ ] `gos3 import --input backup.tar.gz` — import data
- [ ] Bbolt DB: tự động backup mỗi N giờ (snapshot)

---

## Phase 7 — Nâng cao (Optional)

### 7.1 — Web UI (Admin)

- [ ] Server trên port `9001`
- [ ] Single-page app (embed vào binary dùng `embed.FS`)
- [ ] Features:
  - Xem danh sách buckets & objects
  - Upload/download file qua browser
  - Quản lý users & keys
  - Xem metrics
  - Cấu hình CORS, policy

### 7.2 — Replication

- [ ] Async replication sang server GoS3 khác
- [ ] Config: `replication.targets[].endpoint`
- [ ] Queue-based: sau mỗi PUT/DELETE, enqueue task
- [ ] Worker goroutine xử lý replication
- [ ] Retry với exponential backoff

### 7.3 — Storage Backend mở rộng

- [ ] **Interface** đã có → dễ thêm backend mới
- [ ] Backend candidates:
  - Ceph RGW (via librados bindings)
  - Azure Blob (via SDK)
  - GCS (via SDK)
  - SFTP

### 7.4 — SSE-C (Server-Side Encryption với Customer Key)

- [ ] Header: `x-amz-server-side-encryption-customer-algorithm: AES256`
- [ ] Header: `x-amz-server-side-encryption-customer-key` (base64 AES-256 key)
- [ ] Header: `x-amz-server-side-encryption-customer-key-md5`
- [ ] Encrypt data at rest với AES-256-CTR
- [ ] Key không lưu → không decrypt được nếu mất key

### 7.5 — Event Notifications

- [ ] Webhook notifications khi có sự kiện: `s3:ObjectCreated:*`, `s3:ObjectRemoved:*`
- [ ] Config per bucket
- [ ] Async delivery với retry
- [ ] Payload: JSON giống AWS S3 notification format

### 7.6 — Compatibility Testing

- [ ] Chạy `s3-tests` (Python test suite của Ceph) để kiểm tra S3 compatibility
- [ ] Test với AWS SDK Go v2
- [ ] Test với AWS SDK JS v3
- [ ] Test với `awscli`
- [ ] Test với Terraform AWS provider (S3 resources)

---

## Tech Stack & Dependencies

| Package | Mục đích |
|---------|----------|
| `github.com/go-chi/chi/v5` | HTTP router (lightweight, std-compatible) |
| `go.etcd.io/bbolt` | Embedded KV database (metadata store) |
| `github.com/spf13/cobra` | CLI framework |
| `github.com/spf13/viper` | Config (YAML + ENV + flags) |
| `golang.org/x/time/rate` | Rate limiting (token bucket) |
| `github.com/google/uuid` | UUID generation (uploadId, requestId) |
| `github.com/schollz/progressbar/v3` | CLI progress bar |
| `log/slog` | Structured logging (stdlib Go 1.21+) |
| `crypto/sha256`, `crypto/hmac` | SigV4 signing |
| `crypto/md5` | ETag calculation |
| `encoding/xml` | S3 XML request/response |
| `net/http` | HTTP server (stdlib) |

**Không dùng framework nặng. Ưu tiên stdlib.**

---

## Config mẫu

```yaml
# config.yaml
server:
  host: "0.0.0.0"
  port: 9000
  read_timeout: 30s
  write_timeout: 0s      # 0 = unlimited (cho streaming large objects)
  idle_timeout: 120s
  read_header_timeout: 10s  # chống slowloris attack
  max_header_bytes: 1048576 # 1MB max header size
  tls:
    enabled: false
    cert: "/certs/server.crt"
    key:  "/certs/server.key"

storage:
  data_dir: "/data/objects"
  temp_dir: "/data/tmp"
  meta_db:  "/data/meta.db"
  max_object_size: 5497558138880  # 5TB
  multipart_cleanup_days: 7

auth:
  root_access_key: "minioadmin"
  root_secret_key: "minioadmin"
  region: "us-east-1"
  token_expiry_minutes: 15   # SigV4 clock skew tolerance

cors:
  global_allowed_origins:
    - "*"
  global_allowed_methods:
    - "GET"
    - "PUT"
    - "POST"
    - "DELETE"
    - "HEAD"

log:
  level: "info"      # debug | info | warn | error
  format: "json"     # json | text
  output: "stdout"   # stdout | /path/to/file

rate_limit:
  enabled: true
  requests_per_second: 1000
  burst: 200

metrics:
  enabled: true
  path: "/_metrics"

admin:
  api_enabled: true
  path_prefix: "/_admin"
```

---

## Thứ tự Implementation (Recommended)

> Timeline dưới đây giả định **1 developer full-time**. Nếu part-time, nhân đôi.

```
Week 1:   Phase 1 (1.1 → 1.7) — Foundation, Config (Viper), Server, Logger,
          Errors, Basic Rate Limiting, Web UI stub

Week 2:   Phase 2 (2.1 → 2.3) — Storage interface, Local backend (incl. chunked
          upload, disk check), Metadata store (bbolt)

Week 3:   Phase 2 (2.4 → 2.5) — Bucket handlers, Object handlers
          (incl. full response headers, AWS chunked upload)

Week 4:   Phase 2 (2.6 → 2.7) — ListObjects V1/V2, Multipart Upload

Week 5:   Phase 3 (3.1 → 3.2) — SigV4 authentication, IAM user management
          + Phase 3.8: Smoke test với awscli (bắt buộc)

Week 6:   Phase 3 (3.3 → 3.6) — Policy engine, ACL, CORS, Presigned URLs

Week 7:   Phase 3.7 (TLS) + Phase 4 (4.8 → 4.10) — Metrics, Health,
          Advanced Rate Limiting, Request Validation

Week 8:   Phase 5 (5.1 → 5.3) — Client library + gos3c CLI tool

Week 9:   Phase 6 (6.1 → 6.6) — Docker, Deployment, Backup
          + Integration test suite với AWS SDK Go v2

Week 10:  Phase 4 advanced (4.1 → 4.7) — POST Object, Versioning,
          Lifecycle, SSE stub, Object Lock, Storage Class, Website Hosting

Week 11+: Phase 7 (Optional) — Web UI, Replication, SSE-C, Event Notifications
```

---

*GoS3 — Simple, fast, S3-compatible object storage in Go.*