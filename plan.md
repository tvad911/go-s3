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

- [x] Hỗ trợ **bucket policy** (subset của AWS IAM policy JSON)
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
- [x] Lưu bucket policy vào bbolt`
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

- [x] `gos3 verify` — scan tất cả objects, kiểm tra ETag vs actual content
- [x] `gos3 export --output backup.tar.gz` — export toàn bộ data
- [x] `gos3 import --input backup.tar.gz` — import data
- [ ] Bbolt DB: tự động backup mỗi N giờ (snapshot)

---

## Phase 7 — Nâng cao (Optional)

### 7.1 — Web UI (Admin)

- [x] Server trên port `9001`
- [x] Single-page app (embed vào binary dùng `embed.FS`)
- [x] Features:
  - [x] Xem danh sách buckets & objects
  - [x] Upload/download file qua browser
  - [x] Quản lý users & keys
  - [x] Xem metrics
  - [ ] Cấu hình CORS, policy

### 7.2 — Replication

- [x] Async replication sang server GoS3 khác
- [x] Config: `replication.targets[].endpoint`
- [x] Queue-based: sau mỗi PUT/DELETE, enqueue task
- [x] Worker goroutine xử lý replication
- [x] Retry với exponential backoff

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

---

<!-- Merged from test-plan.md -->

# GoS3 Test Plan 🧪

Tài liệu này dùng để theo dõi tiến độ và kịch bản test cho toàn bộ hệ thống GoS3.

## 🟢 Phase 1: Core S3 API & AWS CLI Compatibility
- [x] Khởi động server thành công
- [x] Cấu hình AWS CLI với endpoint `http://localhost:9000`
- [x] Chạy `aws s3 mb s3://test-bucket` (Tạo bucket)
- [x] Chạy `aws s3 ls` (Liệt kê bucket)
- [x] Chạy `aws s3 rb s3://test-bucket` (Xóa bucket)

## 🟡 Phase 2: Local Storage & Multipart Upload
- [x] Upload file nhỏ: `aws s3 cp hello.txt s3://test-bucket/`
- [x] Download file: `aws s3 cp s3://test-bucket/hello.txt .`
- [x] Upload file lớn (> 10MB) để test Multipart Upload: `aws s3 cp video.mp4 s3://test-bucket/`
- [x] Streaming đọc/ghi (không bị crash server khi hết RAM)
- [x] Bbolt DB ghi metadata chính xác (Size, ETag, ContentType)

## 🔵 Phase 3: Advanced S3 Features
- [x] Bật/Tắt Versioning trên bucket
- [x] Ghi đè file nhiều lần và lấy thử version cũ (`aws s3api get-object --version-id ...`)
- [x] Xóa file đã bật Versioning (Sinh DeleteMarker)
- [x] Download một đoạn nội dung (Range Request): `curl -r 0-10 ...`
- [x] Presigned URL: Sinh URL có thời hạn và tải file qua browser không cần auth.

## 🟣 Phase 4: Security, IAM & CORS
- [x] Đăng nhập thất bại khi sai Access/Secret Key
- [x] Phân quyền Bucket Policy (Chỉ đọc, Chỉ ghi)
- [x] Kiểm tra CORS (OPTIONS request) từ frontend khác domain
- [x] Rate Limit: Thử spam request liên tục để kích hoạt HTTP 429 Too Many Requests.

## 🟤 Phase 5: CLI Client (`gos3c`) & Lifecycle
- [x] Cấu hình `gos3c config add local http://localhost:9000 ...`
- [x] Chạy `gos3c ls`, `gos3c mb`, `gos3c cp`
- [x] Gắn Lifecycle rule (ví dụ: Xóa sau 1 ngày) bằng file `test-lifecycle.json`
- [x] Khởi chạy Lifecycle Worker và theo dõi log dọn dẹp.

## 🟠 Phase 6: Web Console (Admin UI)
- [x] Truy cập `http://localhost:9011`
- [x] Giao diện load mượt mà (Glassmorphism), không bị lỗi 301
- [x] Đăng nhập thành công với thông tin admin
- [x] Xem danh sách bucket, kéo thả file upload qua UI
- [x] Quản lý user/metrics trên UI

## 🔴 Phase 7.2: Replication
- [x] Khởi động Server A (Cổng 9000) và Server B (Cổng 9020)
- [x] Cấu hình `replication.targets` của Server A trỏ sang Server B
- [x] Upload file lên Server A
- [x] Chờ 2-3s, kiểm tra Server B xem file đã tự động xuất hiện chưa
- [x] Xóa file ở Server A, kiểm tra Server B có xóa theo chưa


---

<!-- Merged from phase-7-advanced.md -->

# Phase 7: Advanced Features (SSE-C & Notifications)

## Objective
Implement Phase 7.4 (SSE-C) and Phase 7.5 (Event Notifications) from the main `plan.md`.

## Features
1. **SSE-C (Server-Side Encryption with Customer Key)**
   - Parse `x-amz-server-side-encryption-customer-algorithm`, `x-amz-server-side-encryption-customer-key`, `x-amz-server-side-encryption-customer-key-md5` in `PutObject` and `GetObject`.
   - Encrypt/Decrypt data streams using AES-256-CTR with the provided key.
   - Do not store the customer key, only metadata indicating SSE-C is used.

2. **Event Notifications**
   - Webhook configurations per bucket (`PutBucketNotification`, `GetBucketNotification`).
   - Emit `s3:ObjectCreated:*` and `s3:ObjectRemoved:*` events.
   - Async delivery with retry.

## Technical Tasks
- Update `s3.ObjectMeta` to include SSE-C flags.
- Create `internal/crypto/ssec.go` for AES-256 streaming.
- Create `internal/event/webhook.go` for notification dispatch.
- Update `object.go` handlers to intercept and wrap the `io.Reader` for encryption.


---

<!-- Merged from update_v1.md -->

# Kế hoạch Nâng cấp GoS3 Web Console (Update v1)

Tài liệu này chuẩn hóa lại toàn bộ luồng UX/UI và kiến trúc Backend của GoS3 Web Console, lấy **MinIO Console** làm nguyên mẫu. Đã audit toàn bộ codebase hiện tại để xác định chính xác cái gì đã có và cái gì cần làm.

---

## 1. Audit hiện trạng codebase

### 1.1 Những gì ĐÃ CÓ (Backend)
| Module | File | Mô tả |
| :--- | :--- | :--- |
| Auth SigV4 | `internal/auth/sigv4.go` | Xác thực S3 request chuẩn AWS SigV4, hoạt động tốt |
| User struct | `internal/auth/iam.go` | Struct `User` có `Username`, `AccessKeyID`, `SecretKey`, `Policies`, `IsRoot`, `Disabled` |
| UserStore | `internal/auth/iam.go` | Interface CRUD user: `GetByAccessKey`, `GetByUsername`, `List`, `Create`, `Update`, `Delete` |
| Admin Handler | `internal/handler/admin.go` | API: `ListUsers`, `CreateUser`, `GetUser`, `UpdateUser`, `DeleteUser`, `ServerInfo`, `Presign` |
| Admin Router | `internal/server/router.go` | Đường dẫn `/_admin/users`, `/_admin/info`, `/_admin/presign` — **yêu cầu SigV4 + IsRoot** |
| S3 API đầy đủ | `internal/handler/*.go` | ListBuckets, CRUD Object, Multipart, Versioning, Lifecycle, CORS, ACL, Website, Policy |
| Storage Backend | `internal/storage/interface.go` | Interface hoàn chỉnh: Bucket + Object + Multipart + Lifecycle + CORS |
| Replication | `internal/replication/service.go` | Async queue replication sang node khác |

### 1.2 Những gì ĐÃ CÓ (Frontend — `web/dist/`)
| Tính năng | Trạng thái | Vấn đề |
| :--- | :--- | :--- |
| Login form | ✅ Có | ❌ Yêu cầu nhập Endpoint + Access Key + Secret Key (3 trường) |
| Sidebar nav | ✅ Có | Chỉ có 3 mục: Buckets, IAM Users, Server Info |
| Bucket list | ✅ Có | Chỉ hiển thị tên + ngày tạo, không có thống kê dung lượng |
| Object list | ✅ Có | ❌ Hiển thị phẳng (flat list), không hỗ trợ folder navigation |
| Upload (Drag & Drop) | ✅ Có | Chỉ upload file, chưa upload folder |
| Download/Delete object | ✅ Có | Download qua blob (chậm với file lớn) |
| Create/Delete bucket | ✅ Có | Thiếu các settings (Versioning, Policy) |
| Create/Delete user | ✅ Có | Không có field password, tạo user chỉ trả access key |
| Server info | ✅ Có | Hiển thị version, uptime, storage |
| Auth mechanism | ❌ | Frontend lưu raw Secret Key vào LocalStorage, dùng aws4fetch ký mọi request |

### 1.3 Những gì THIẾU (Gap Analysis)
| Tính năng | Backend API | Frontend UI | Ghi chú |
| :--- | :--- | :--- | :--- |
| **Login bằng Username/Password** | ❌ Chưa có `/api/v1/login` | ❌ Chưa có form 2 trường | Hiện tại bắt nhập raw key |
| **Session/JWT Token** | ❌ Chưa có middleware session | ❌ Lưu key vào LocalStorage | Không an toàn |
| **Service Accounts (Multi-key)** | ❌ User chỉ có 1 AccessKey | ❌ Chưa có UI | Cần bảng `service_accounts` riêng trong bbolt |
| **Password Hash** | ❌ User không có field password | — | Cần thêm `PasswordHash` vào struct `User` |
| **Policy Editor** | ❌ API có `Policies []string` nhưng không có logic enforce | ❌ Chưa có UI | Chỉ kiểm tra `IsRoot`, không check policy chi tiết |
| **Bucket Settings UI** | ✅ Backend đã có API (Versioning, Policy, CORS, Lifecycle, Website) | ❌ Chưa có trang Settings | API sẵn rồi, chỉ cần làm UI |
| **Folder navigation** | ✅ Backend hỗ trợ `prefix` + `delimiter` | ❌ Frontend gọi ListObjectsV2 không truyền delimiter | Cần sửa frontend |
| **Search object** | ❌ Chưa có admin search API | ❌ Chưa có ô search | S3 API gốc chỉ hỗ trợ prefix match |
| **Sort object** | ❌ S3 API chỉ sort alphabet | ❌ Chưa có nút sort | Cần Admin API riêng hoặc sort client-side |
| **Create Folder** | ✅ Backend chấp nhận PutObject 0-byte key kết thúc `/` | ❌ Chưa có nút Create Folder | Chỉ cần thêm UI |
| **Share link (Presigned)** | ✅ Backend có `/_admin/presign` | ❌ Chưa có nút Share trên mỗi object | API sẵn rồi |
| **Preview ảnh** | ✅ Backend trả đúng Content-Type | ❌ Chưa có preview modal | Cần thêm UI |
| **Rename/Move object** | ✅ Backend có `CopyObject` + `DeleteObject` | ❌ Chưa có | Cần kết hợp Copy → Delete |
| **Bucket info (dung lượng)** | ⚠️ Metrics có `StorageBytes` per bucket | ❌ Chưa hiển thị trên bucket card | Cần expose thêm API |
| **Custom Domain mapping** | ❌ Chưa có | ❌ Chưa có | Tính năng mới hoàn toàn |
| **Notification/Events** | ❌ Chưa có | ❌ Chưa có | MinIO có Bucket Notification (webhook, kafka) |
| **Audit Log viewer** | ❌ Chưa có | ❌ Chưa có | MinIO Console có màn hình xem log |

---

## 2. Kiến trúc Xác thực mới (Auth v2)

Khắc phục điểm yếu hiện tại (đánh đồng User Identity với Access Key), hệ thống mới tách bạch:

```
┌──────────────────────────────────────────────────────────┐
│                    GoS3 Auth v2                          │
├──────────────────────────────────────────────────────────┤
│  User Identity                                           │
│  ├── Username (unique)                                   │
│  ├── PasswordHash (bcrypt)                               │
│  ├── IsRoot (bool)                                       │
│  ├── Policies ([]string)                                 │
│  ├── Disabled (bool)                                     │
│  └── ServiceAccounts []ServiceAccount                    │
│       ├── AccessKeyID (auto-generated)                   │
│       ├── SecretKey (auto-generated, chỉ hiện 1 lần)    │
│       ├── Description (label do user đặt)                │
│       ├── Policies ([]string — scoped riêng)             │
│       ├── Expiry (*time.Time — optional)                 │
│       └── Disabled (bool)                                │
├──────────────────────────────────────────────────────────┤
│  Web Console Auth Flow:                                  │
│  POST /api/v1/login {username, password}                 │
│    → Verify bcrypt hash                                  │
│    → Generate JWT (exp: 24h)                             │
│    → Set HttpOnly Secure Cookie                          │
│    → Frontend KHÔNG lưu bất kỳ key nào                  │
│                                                          │
│  Admin API calls from Web Console:                       │
│    Cookie → JWT middleware → Extract UserID → Proceed    │
│                                                          │
│  S3 API calls from SDK/CLI:                              │
│    Authorization header → SigV4 → Lookup ServiceAccount  │
│    → Resolve parent User → Check Policies → Proceed     │
└──────────────────────────────────────────────────────────┘
```

### Thay đổi cần làm ở Backend:
1. Thêm field `PasswordHash string` vào struct `User` trong `internal/auth/iam.go`
2. Tạo struct `ServiceAccount` mới (bảng `service_accounts` trong bbolt)
3. Tạo `internal/auth/session.go` — JWT generation/validation, Cookie middleware
4. Tạo handler `POST /api/v1/login`, `POST /api/v1/logout`, `GET /api/v1/me`
5. Sửa SigV4 verifier: lookup `ServiceAccount` thay vì trực tiếp `User`
6. Admin API middleware hỗ trợ cả 2 luồng: JWT Cookie (Web) HOẶC SigV4 (CLI)

---

## 3. Các chức năng Giao diện (GUI) cần bổ sung

### 3.1 Bảng tổng hợp tính năng (đối chiếu MinIO Console)

| Nhóm chức năng | Chức năng chi tiết | Backend | Frontend | Ưu tiên |
| :--- | :--- | :--- | :--- | :--- |
| **Login** | Đăng nhập Username/Password, tự nhận diện Endpoint | ❌ Cần làm | ❌ Cần làm | P0 |
| **Dashboard** | Thống kê tổng: dung lượng, số bucket, số user, uptime | ⚠️ Có một phần | ❌ Cần làm | P1 |
| **Object Browser** | Duyệt file dạng cây thư mục (prefix/delimiter) | ✅ Sẵn | ❌ Cần sửa | P0 |
| **Object Browser** | Tạo thư mục rỗng (PutObject 0-byte key `folder/`) | ✅ Sẵn | ❌ Cần nút | P1 |
| **Object Browser** | Tìm kiếm file theo tên (client-side filter hoặc admin API) | ❌ Cần làm | ❌ Cần làm | P1 |
| **Object Browser** | Sắp xếp theo tên/size/date (client-side sort) | ✅ Data có đủ | ❌ Cần làm | P1 |
| **Object Browser** | Preview ảnh/video trực tiếp | ✅ Sẵn | ❌ Cần modal | P2 |
| **Object Browser** | Share link (Presigned URL) | ✅ API sẵn | ❌ Cần nút | P1 |
| **Object Browser** | Rename/Move (Copy + Delete) | ✅ Sẵn | ❌ Cần làm | P2 |
| **Object Browser** | Multi-select + Bulk delete | ✅ API `DeleteObjects` sẵn | ❌ Cần làm | P2 |
| **Bucket Settings** | Trang chi tiết: Overview, Objects, Settings | ✅ API sẵn | ❌ Cần làm | P1 |
| **Bucket Settings** | Versioning (Enable/Suspend) | ✅ API sẵn | ❌ Cần toggle | P1 |
| **Bucket Settings** | Bucket Policy (Public/Private/Custom JSON) | ✅ API sẵn | ❌ Cần UI | P1 |
| **Bucket Settings** | CORS Configuration | ✅ API sẵn | ❌ Cần UI | P2 |
| **Bucket Settings** | Lifecycle Rules | ✅ API sẵn | ❌ Cần UI | P2 |
| **Bucket Settings** | Static Website Hosting | ✅ API sẵn | ❌ Cần UI | P2 |
| **Bucket Settings** | Custom Domain mapping | ❌ Cần làm | ❌ Cần làm | P3 |
| **Identity (Users)** | Tạo user bằng username + password | ❌ Cần sửa | ❌ Cần sửa | P0 |
| **Identity (Users)** | Đổi mật khẩu, enable/disable user | ⚠️ UpdateUser có nhưng chưa hash | ❌ Cần UI | P1 |
| **Identity (Users)** | Gán Policy cho user | ⚠️ Field có nhưng chưa enforce | ❌ Cần UI | P1 |
| **Identity (Policies)** | Định nghĩa Policy (JSON hoặc preset) | ❌ Cần làm | ❌ Cần làm | P1 |
| **Service Accounts** | Tạo/Xóa/Disable Access Key per user | ❌ Cần làm | ❌ Cần làm | P0 |
| **Service Accounts** | Hiển thị Endpoint + Key để copy vào code | — | ❌ Cần làm | P0 |
| **Monitoring** | Audit log viewer (request history) | ❌ Cần làm | ❌ Cần làm | P3 |
| **Monitoring** | Bucket Notification/Events (webhook) | ❌ Cần làm | ❌ Cần làm | P3 |

### 3.2 Chi tiết từng màn hình

#### Màn hình Login (Sửa lại)
- **Hiện tại:** 3 trường: Endpoint, Access Key, Secret Key → lưu raw key vào `localStorage`.
- **Update v1:** 2 trường: Username, Password → `POST /api/v1/login` → nhận JWT Cookie HttpOnly → frontend không lưu gì nhạy cảm.

#### Dashboard (Bổ sung mới)
- Card thống kê: Tổng dung lượng đã dùng, Số bucket, Số user, Uptime server.
- Gợi ý API Endpoint (tự detect từ `window.location.origin` thay thế port UI bằng port S3).
- Quick actions: Tạo bucket, Tạo user.

#### Sidebar Navigation (Mở rộng)
Hiện tại chỉ có 3 mục, cần mở rộng:
```
├── Dashboard           (mới)
├── Buckets             (có)
├── Object Browser      (tách riêng, không chung Buckets)
├── Identity
│   ├── Users           (có, cần sửa)
│   └── Policies        (mới)
├── Service Accounts    (mới)
└── Settings
    ├── Server Info     (có, đổi tên)
    └── Audit Log       (mới, P3)
```

#### Object Browser (Nâng cấp lớn)
- **Toolbar:** Ô tìm kiếm (Search) + Dropdown sắp xếp (Sort by: Name ↑↓, Size ↑↓, Date ↑↓) + Nút "Create Folder" + Nút "Upload".
- **Breadcrumb điều hướng:** `Bucket / folder1 / folder2 /` — click vào segment bất kỳ để quay lại cấp đó.
- **Bảng danh sách:** Hỗ trợ icon phân biệt Folder vs File. Click Folder → đi sâu vào (cập nhật prefix). Click File → panel chi tiết (metadata, preview, share link).
- **Multi-select:** Checkbox trên mỗi row → Bulk delete, Bulk download.
- **Context menu (chuột phải):** Download, Share, Rename, Delete.

#### Bucket Settings (Trang mới)
Click icon Settings trên Bucket card → Trang chi tiết với các tab:
- **Overview:** Tên, ngày tạo, dung lượng, số objects.
- **Access (Policy):** Toggle nhanh Public/Private + JSON editor cho Custom policy.
- **Versioning:** Toggle Enable/Suspend.
- **CORS:** Danh sách rules, nút thêm/xóa rule.
- **Lifecycle:** Danh sách rules (prefix, expiration days), nút thêm/xóa.
- **Website:** Enable/Disable static hosting, cấu hình index/error document.
- **Domain:** (P3) Cấu hình custom domain mapping.

#### Identity — Users (Sửa lại)
- Form tạo user: Username + Password (thay vì chỉ username).
- Bảng danh sách: hiển thị Username, Policies, Status (Active/Disabled), Created At.
- Action per user: Edit (đổi password, gán policy), Disable, Delete.

#### Identity — Policies (Trang mới + Cấu trúc Backend)
- **Cấu trúc lưu trữ:** Bảng `policies` mới trong bbolt, lưu policy dưới định dạng JSON chuẩn AWS IAM Policy (hỗ trợ `Version`, `Statement` với `Effect`, `Action`, `Resource`, `Condition`).
- **Preset Policies:** Tích hợp sẵn (không thể xóa) các policy cơ bản: `ReadOnly`, `ReadWrite`, `WriteOnly`, `Admin`.
- **Policy Engine (Backend):** 
  - Mặc định là **Deny** (nếu không có policy nào Allow một action cụ thể).
  - Viết middleware/hàm đánh giá quyền truy cập chạy sau xác thực (auth).
  - Khớp các `Action` (hỗ trợ wildcard, ví dụ: `s3:Get*`, `s3:PutObject`) và `Resource` (ví dụ: `arn:aws:s3:::my-bucket/*`).
  - Hợp nhất (Union) quyền hạn: Nếu một thao tác được Allow bởi **User Policy** hoặc **Bucket Policy** (và không bị Explicit Deny), thì thao tác đó được phép.
- **Giao diện:** Cho phép tạo Custom Policy bằng JSON editor (kiểm tra cú pháp hợp lệ trước khi lưu).
- **Gán quyền:** Cho phép gắn nhiều Policy vào một User hoặc Service Account. Khi đánh giá, các quyền sẽ được gộp lại.

#### Service Accounts (Trang mới)
- Mỗi user (sau khi login) thấy danh sách Access Keys của mình.
- Nút "Create Access Key" → sinh cặp key → hiển thị Secret Key **1 lần duy nhất** (có nút Copy).
- Hiển thị kèm snippet cấu hình mẫu cho AWS CLI, GoS3 Client, Python boto3.
- Admin (Root) có thể xem Service Accounts của tất cả users.

---

## 4. Roadmap triển khai

### Phase 1: Backend Auth v2 + Login UI (P0 — Ưu tiên cao nhất)
**Mục tiêu:** Người dùng có thể đăng nhập bằng Username/Password, không cần nhập key.

**Backend:**
- [x] Thêm `PasswordHash` vào struct `User`, migrate dữ liệu cũ (root user auto-hash password)
- [x] Tạo `internal/auth/session.go`: JWT generate/validate, Cookie middleware
- [x] Tạo handler: `POST /api/v1/login`, `POST /api/v1/logout`, `GET /api/v1/me`
- [x] Sửa Admin API middleware: chấp nhận cả JWT Cookie (Web) và SigV4 (CLI)
- [x] Struct + Store cho `ServiceAccount` (bảng bbolt `service_accounts`)
- [x] Sửa SigV4 verifier: lookup key từ bảng `service_accounts` trước, fallback `users`
- [x] API: `POST /api/v1/service-accounts`, `GET /api/v1/service-accounts`, `DELETE /api/v1/service-accounts/{id}`
- [x] Auto-create Service Account cho root user khi startup (tương thích ngược với config `root_access_key/root_secret_key`)
- [x] Tạo `SettingStore` dùng `bbolt` để lưu cấu hình.

**Frontend:**
- [x] Sửa Login form: chỉ 2 trường Username + Password
- [x] Xóa logic `localStorage` lưu raw key
- [x] Admin API calls dùng Cookie thay vì aws4fetch sign
- [x] S3 API calls từ Browser: dùng Presigned URL do server generate (thay vì client-side SigV4)
- [x] Thêm màn hình `Global Settings` (Pagination size, session expiry, CORS).

### Phase 2: Object Browser nâng cấp (P0-P1)
**Mục tiêu:** Duyệt file theo thư mục, tìm kiếm, sắp xếp, share link.

**Frontend (không cần sửa Backend vì API đã có):**
- [x] Gọi ListObjectsV2 với `delimiter=/` để tách folder vs file
- [x] Breadcrumb navigation multi-level
- [x] Icon phân biệt folder/file
- [x] Nút "Create Folder" → PutObject 0-byte key `prefix/foldername/`
- [x] Ô Search (client-side filter theo key name)
- [x] Dropdown Sort (client-side sort theo name/size/date)
- [x] Nút Share trên mỗi file → gọi `/_admin/presign` → hiển thị URL + copy
- [x] Preview modal cho ảnh (detect Content-Type từ extension)

### Phase 3: Bucket Settings + Identity nâng cấp (P1)
**Mục tiêu:** Quản trị Bucket chuyên sâu, quản lý user + policy đầy đủ.

**Frontend (Backend API đã có sẵn):**
- [x] Trang Bucket Details với tabs: Overview, Access, Versioning, CORS, Lifecycle, Website
- [x] Access tab: Toggle Public/Private + JSON editor cho custom policy
- [x] Versioning tab: Toggle Enable/Suspend
- [x] Bucket card: hiển thị thêm số objects, dung lượng
- [x] UI ghép Bucket Policy với Service Account (Grant Access directly to SA)

**Backend + Frontend:**
- [x] API expose bucket stats (`/_admin/buckets/{name}/stats`)
- [x] Trang Users: form tạo user với username + password
- [x] Trang Users: action đổi password, gán policy, disable
- [x] Trang Policies: danh sách preset + custom JSON editor
- [x] Xây dựng `Policy Engine` hỗ trợ matching `Action`, `Resource`, `Wildcard` theo chuẩn AWS.
- [x] Tích hợp Policy Engine vào logic xử lý S3 API (chặn request nếu không được Allow).
- [x] Enforce phân quyền trong middleware cho Admin API (không chỉ check `IsRoot` mà còn check các action như `admin:ListUsers`).

### Phase 4: Service Accounts UI + Dashboard (P1)
**Mục tiêu:** User tự quản lý API keys, Dashboard tổng quan.

- [x] Trang Service Accounts: tạo/xóa/disable Access Key
- [x] Hiển thị Secret Key 1 lần + nút Copy + snippet cấu hình mẫu
- [x] Dashboard: cards thống kê (dung lượng, buckets, users, uptime)
- [x] Dashboard: gợi ý Endpoint API (auto-detect từ `window.location`)

### Phase 5: Tính năng nâng cao (P2-P3)
- [x] Rename/Move object (Copy + Delete)
- [x] Multi-select + Bulk delete/download
- [x] Upload folder (recursive via webkitdirectory)
- [x] Download file lớn qua Presigned URL (thay vì blob)
- [x] Preview video/audio
- [x] CORS editor UI
- [x] Lifecycle editor UI
- [x] Website hosting config UI
- [x] Custom Domain mapping (Backend middleware + UI)
- [x] Audit Log viewer
- [x] Bucket Notification/Events (Webhooks)

### Phase 7: Phân quyền & Access Control (Authorization)
- [x] Bổ sung trường `Owner` cho Bucket trong metadata.
- [x] Sửa `S3Handler` để xác thực quyền Owner (User == Owner).
- [x] Hoàn thiện hàm parse Bucket Policy JSON (đặc biệt hỗ trợ `Principal: "*"`).
- [x] Web UI: Cung cấp preset "Make Public" / "Make Private" cho Bucket Policy.

---

## 5. Quy tắc kỹ thuật

- **Frontend stack:** Vanilla JS (đã có), không thêm framework. Dùng ES Modules.
- **Auth flow Web:** JWT trong HttpOnly Cookie. Không lưu bất kỳ secret nào trong `localStorage`.
- **Auth flow S3 API:** SigV4 không đổi, nhưng lookup key từ `service_accounts` thay vì `users`.
- **Password hashing:** `bcrypt` (stdlib `golang.org/x/crypto/bcrypt`).
- **JWT:** Dùng `crypto/hmac` + `crypto/sha256` stdlib, KHÔNG thêm dependency JWT library.
- **Backward compatibility:** Root user startup vẫn dùng `root_access_key`/`root_secret_key` từ config → auto-tạo ServiceAccount tương ứng.

---

## 6. Cập nhật Thiết kế (Từ Brainstorm)

### 6.1. Lifecycle (Chuẩn AWS S3 API)
- **Backend:** Xử lý XML cấu hình chuẩn AWS thông qua các endpoint `GET/PUT/DELETE ?lifecycle`. Worker quét DB định kỳ để xóa object quá hạn.
- **Web UI:** Form đơn giản để nhập cấu hình (Prefix, số ngày). JavaScript tự động sinh XML và gọi API.

### 6.2. Custom Domain Options
- **Virtual-Hosted Style (Chuẩn AWS):** Hỗ trợ dạng `bucket.s3.domain.com`. Router phân giải dựa trên biến môi trường `GOS3_BASE_DOMAIN`.
- **Cname Mapping:** Ánh xạ một custom domain bất kỳ (vd: `cdn.myweb.com`) thẳng vào một bucket cụ thể thông qua bảng `custom_domains` trong bbolt.

### 6.3. IAM Mental Model (Resource-Based + Service Account)
- **User:** Dành riêng cho Admin login WebUI.
- **Service Account (Access Key):** Dùng cho API, không liên quan đến User login.
- **Bucket Policy:** Phân quyền truy cập tài nguyên. VD: Cấp quyền cho Service Account X truy cập Bucket A. Giao diện trực quan, rõ ràng ai đang truy cập bucket nào.

### 6.4. Global Settings (Env vs DB)
- **Cấu hình tĩnh (ENV/YAML):** Port, đường dẫn Data, Admin gốc, Base Domain, TLS certs. Thay đổi cần khởi động lại.
- **Cấu hình động (Database/WebUI):** Rate Limit, Max Object Size, Pagination size mặc định, Global CORS. Đổi trực tiếp trên UI có tác dụng ngay.
