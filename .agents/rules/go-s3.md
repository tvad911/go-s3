---
trigger: always_on
---

# GoS3 — Project Rules & Conventions

> Đây là file quy tắc dự án dùng cho AI coding assistant.
> Đọc toàn bộ file này trước khi sinh bất kỳ code nào.

---

## 1. Tổng quan dự án

**GoS3** là một S3-compatible object storage server + CLI client viết bằng Go.
Mục tiêu: đơn giản như Garage, client như MinIO Client (`mc`), chạy tốt trên Ubuntu/Docker.

- **Ngôn ngữ:** Go 1.22+
- **Target OS:** Linux (Ubuntu 22.04/24.04), deploy bằng Docker
- **Kiến trúc:** monorepo, hai binary: `gos3` (server) và `gos3c` (client)
- **S3 Compatibility:** AWS S3 API, Signature V4, CORS, ACL, Bucket Policy, Presigned URLs, Multipart Upload, AWS Chunked Upload

---

## 2. Cấu trúc thư mục (bắt buộc tuân theo)

```
gos3/
├── cmd/
│   ├── server/main.go          # entry point server
│   └── client/main.go          # entry point client CLI
├── internal/
│   ├── config/                 # load & validate config (Viper)
│   ├── server/                 # HTTP server + router + embed.go
│   ├── middleware/             # auth, cors, logging, ratelimit, recover
│   ├── handler/                # S3 HTTP handlers (bucket, object, multipart)
│   ├── auth/                   # sigv4, iam, policy, acl
│   ├── storage/
│   │   ├── interface.go        # Backend interface (bắt buộc)
│   │   ├── local/              # local filesystem implementation
│   │   │   ├── local.go        # Backend impl
│   │   │   ├── chunk.go        # streaming write
│   │   │   └── disk.go         # disk space checking
│   │   └── metadata/           # bbolt metadata store
│   ├── s3/                     # S3 types, XML structs, error codes
│   └── util/                   # etag, xml helpers, time helpers
├── client/                     # reusable Go client library
├── cli/                        # cobra CLI commands
├── web/
│   └── dist/                   # Web UI static files (embed.FS)
│       └── index.html          # placeholder for Phase 7
├── test/
│   └── smoke.sh                # awscli compatibility smoke test
├── deploy/
│   ├── Dockerfile
│   ├── docker-compose.yml
│   └── config.example.yaml
├── go.mod
├── go.sum
└── Makefile
```

**Quy tắc:**
- Code trong `internal/` KHÔNG được import từ ngoài module
- `cmd/` chỉ chứa `main.go`, logic thật nằm trong `internal/` hoặc `client/`
- `client/` là public library, có thể được import bởi project khác
- Mỗi package chỉ làm một việc, tên package = tên thư mục
- `web/dist/` chứa static files, embedded vào binary qua `//go:embed`

---

## 3. Ngôn ngữ & Code Style

### 3.1 Quy tắc Go cơ bản

- Tuân theo **Effective Go** và **Go Code Review Comments**
- Format code bằng `gofmt` / `goimports` — không thương lượng
- Lint bằng `golangci-lint` với config chuẩn dự án
- Không dùng `init()` trừ khi thực sự cần thiết
- Không dùng global mutable state, truyền dependency qua constructor
- Không dùng `panic()` trong production code — trả error thay thế
- Không dùng `interface{}` / `any` trừ khi bắt buộc (encoding, generics thay thế)

### 3.2 Error handling

```go
// ✅ ĐÚNG — wrap error với context
if err != nil {
    return fmt.Errorf("putObject %s/%s: %w", bucket, key, err)
}

// ❌ SAI — bỏ qua error
val, _ := doSomething()

// ❌ SAI — error message viết hoa hoặc kết thúc bằng dấu chấm
return fmt.Errorf("Failed to open file.")
```

- Luôn wrap error bằng `fmt.Errorf("context: %w", err)`
- Error message: chữ thường, không có dấu chấm cuối
- Custom error types cho S3 errors (trong `internal/s3/errors.go`)
- Không dùng `errors.New` trực tiếp trong handler — dùng S3 error types

### 3.3 Naming conventions

```go
// Structs: PascalCase
type BucketMeta struct { ... }

// Interfaces: tên mô tả khả năng, thường kết thúc -er
type Backend interface { ... }
type ObjectStorer interface { ... }

// Functions/Methods: PascalCase (exported), camelCase (unexported)
func (s *Server) ListBuckets(...) { ... }
func (s *Server) parseRange(...) { ... }

// Constants: PascalCase nếu exported, camelCase nếu không
const MaxObjectSize = 5 * 1024 * 1024 * 1024 * 1024

// Error variables: ErrXxx
var ErrBucketNotFound = errors.New("bucket not found")

// Acronyms: giữ nguyên hoa toàn bộ
type S3Handler struct { ... }  // không phải S3handler
func parseXML(...) { ... }     // không phải parseXml
```

### 3.4 Comments & Documentation

```go
// ✅ ĐÚNG — exported symbol phải có doc comment
// ListBuckets trả về danh sách tất cả bucket của user hiện tại.
// Kết quả được sắp xếp theo tên bucket tăng dần.
func (h *Handler) ListBuckets(w http.ResponseWriter, r *http.Request) { ... }

// ✅ ĐÚNG — comment giải thích tại sao, không phải cái gì
// Dùng rename thay vì copy+delete để đảm bảo atomic write.
if err := os.Rename(tmpPath, finalPath); err != nil { ... }

// ❌ SAI — comment thừa, chỉ lặp lại code
// Increment i by 1
i++
```

- Mọi exported function, type, const phải có doc comment
- Comment bằng **tiếng Anh** (code comments), **tiếng Việt** cho TODO/NOTE nội bộ
- Dùng `// TODO(username): ...` cho việc còn làm

### 3.5 Imports

```go
import (
    // 1. stdlib
    "context"
    "fmt"
    "net/http"

    // 2. internal packages
    "github.com/yourname/gos3/internal/auth"
    "github.com/yourname/gos3/internal/s3"

    // 3. third-party
    "github.com/go-chi/chi/v5"
    "go.etcd.io/bbolt"
)
```

- Ba nhóm, phân tách bằng dòng trống: stdlib → internal → third-party
- Không dùng dot import (`. "pkg"`)
- Không dùng blank import (`_ "pkg"`) trừ driver registration

---

## 4. Kiến trúc & Ràng buộc kỹ thuật

### 4.1 HTTP Layer

- Router: **`github.com/go-chi/chi/v5`** — không dùng framework khác (Gin, Echo, Fiber, v.v.)
- Handler signature: `func(w http.ResponseWriter, r *http.Request)` — chuẩn stdlib
- Middleware: dùng `chi.Middleware` pattern (func(http.Handler) http.Handler)
- Không dùng `http.DefaultServeMux`
- Mọi handler đều nhận `context.Context` từ `r.Context()`
- Server security: `MaxHeaderBytes` (1MB), `ReadHeaderTimeout` (10s) — chống slowloris
- `WriteTimeout: 0` cho streaming nhưng dùng `ReadHeaderTimeout` để bảo vệ header phase

```go
// ✅ ĐÚNG — handler pattern
func (h *BucketHandler) CreateBucket(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    bucket := chi.URLParam(r, "bucket")
    // ...
}

// ✅ ĐÚNG — server config với security settings
srv := &http.Server{
    ReadTimeout:       30 * time.Second,
    WriteTimeout:      0, // unlimited cho streaming large objects
    IdleTimeout:       120 * time.Second,
    ReadHeaderTimeout: 10 * time.Second,  // chống slowloris
    MaxHeaderBytes:    1 << 20,           // 1MB
}
```

### 4.2 Storage Layer

- **Storage interface** (`internal/storage/interface.go`) là nguồn sự thật duy nhất
- Không gọi thẳng filesystem functions từ handler — phải qua interface
- Mọi write phải là **atomic**: write temp → rename
- Mọi read phải là **streaming**: dùng `io.Reader`/`io.Writer`, không buffer toàn bộ vào RAM
- ETag = MD5 của object content, tính trong quá trình stream (không đọc lại)
- **Disk space check** bắt buộc trước PutObject và UploadPart (`syscall.Statfs`)
- **AWS Chunked Upload** phải được hỗ trợ từ đầu — AWS SDK gửi chunked mặc định
- **Startup cleanup**: scan `tmp/` và `multipart/` directories, xóa orphan files từ lần chạy trước

```go
// ✅ ĐÚNG — streaming write với disk check
func (b *LocalBackend) PutObject(ctx context.Context, bucket, key string,
    r io.Reader, size int64, meta ObjectMeta) (*PutResult, error) {
    // Check disk space trước
    if err := checkDiskSpace(b.dataDir, size); err != nil {
        return nil, err // InsufficientStorage
    }
    tmp, _ := os.CreateTemp(b.tempDir, "upload-*")
    h := md5.New()
    _, err := io.Copy(tmp, io.TeeReader(r, h))
    // ...
    os.Rename(tmp.Name(), finalPath)
}

// ❌ SAI — đọc hết vào memory
body, _ := io.ReadAll(r.Body)

// ❌ SAI — không check disk space
func (b *LocalBackend) PutObject(...) {
    // trực tiếp write mà không kiểm tra dung lượng
}
```

### 4.3 Metadata Database

- Dùng **bbolt** (`go.etcd.io/bbolt`) — embedded, không cần external DB (KHÔNG dùng sqlite)
- Một file DB duy nhất: `{dataDir}/meta.db`
- Bucket naming trong bbolt:
  - `buckets` — danh sách bucket
  - `objects:{bucketName}` — objects trong bucket đó
  - `uploads` — multipart upload metadata
  - `parts:{uploadId}` — parts của multipart
  - `users` — IAM users
  - `policies` — bucket policies
  - `cors` — CORS configs
  - `tags:bucket:{name}` — bucket tags
  - `tags:object:{bucket}:{key}` — object tags
- Mọi write vào bbolt phải trong transaction
- Đọc dùng View transaction, ghi dùng Update transaction
- **Write contention:** bbolt cho phép 1 write transaction tại 1 thời điểm (MVCC single-writer). Chấp nhận limitation này cho small/medium scale. Nếu cần scale → xem xét batch write queue (channel + single writer goroutine)
- **ObjectMeta** phải lưu đầy đủ fields: `Size, ETag, ContentType, ContentEncoding, ContentDisposition, ContentLanguage, CacheControl, Expires, LastModified, UserMeta, Tags, StorageClass`

### 4.4 Authentication & Security

- **Signature V4** là phương thức xác thực chính
- Flow xác thực PHẢI theo thứ tự:
  1. Parse Authorization header hoặc query-string params
  2. Extract AccessKeyID
  3. Lookup secret key từ IAM store
  4. Tính lại chữ ký, so sánh constant-time (`subtle.ConstantTimeCompare`)
  5. Kiểm tra timestamp (lệch không quá 15 phút)
- KHÔNG lưu secret key dưới dạng plaintext trong log
- KHÔNG trả secret key trong bất kỳ response nào
- Rate limiting PHẢI chạy trước auth middleware
- CORS headers PHẢI được thêm kể cả khi request bị từ chối

```go
// Middleware order (bắt buộc)
r.Use(middleware.RealIP)
r.Use(middleware.RequestID)
r.Use(middleware.Logger)
r.Use(middleware.Recoverer)
r.Use(mw.RateLimit)   // per-IP cơ bản từ Phase 1, per-user/bucket ở Phase 4
r.Use(mw.CORS)
r.Use(mw.Auth)        // SigV4 last — sau CORS để OPTIONS pass qua
```

### 4.5 S3 Protocol

- Mọi response lỗi PHẢI là XML theo format AWS S3:
```xml
<?xml version="1.0" encoding="UTF-8"?>
<Error>
    <Code>NoSuchBucket</Code>
    <Message>The specified bucket does not exist</Message>
    <BucketName>my-bucket</BucketName>
    <RequestId>...</RequestId>
</Error>
```
- XML encoding/decoding dùng `encoding/xml` stdlib
- Content-Type cho XML responses: `application/xml`
- Mọi response phải có header `x-amz-request-id` (UUID)
- Date format: RFC1123 (`Mon, 02 Jan 2006 15:04:05 GMT`) cho HTTP headers
- Date format: ISO8601 (`2006-01-02T15:04:05.000Z`) cho S3 XML bodies
- Bucket name validation: 3–63 ký tự, lowercase + số + dấu gạch ngang, không bắt đầu/kết thúc bằng gạch ngang, không có hai dấu chấm liền nhau
- S3 Error codes bắt buộc hỗ trợ:
  - `NoSuchBucket`, `NoSuchKey`, `BucketAlreadyExists`, `BucketAlreadyOwnedByYou`
  - `AccessDenied`, `InvalidAccessKeyId`, `SignatureDoesNotMatch`
  - `InvalidBucketName`, `KeyTooLongError`
  - `MalformedXML`, `MissingContentLength`
  - `EntityTooLarge`, `InvalidRange`, `InsufficientStorage` (disk full)
  - `NoSuchUpload`, `InvalidPart`, `InvalidPartOrder`
  - `NotImplemented` (cho S3 Select và các API chưa hỗ trợ)
  - `SlowDown` (429 rate limited)

### 4.6 S3 Response Headers (bắt buộc)

GetObject/HeadObject **PHẢI** trả đầy đủ các headers sau nếu có trong metadata:

| Header | Mô tả |
|--------|--------|
| `ETag` | MD5 hash (PutObject) hoặc `MD5(MD5s...)-N` (Multipart) |
| `Content-Type` | MIME type |
| `Content-Length` | Object size in bytes |
| `Last-Modified` | RFC1123 format |
| `x-amz-meta-*` | User-defined metadata |
| `Content-Encoding` | gzip, br nếu object có encoding |
| `Content-Disposition` | attachment; filename=... |
| `Content-Language` | Ngôn ngữ nội dung |
| `Cache-Control` | Cache directives |
| `Expires` | HTTP expiry |
| `x-amz-storage-class` | Storage class (STANDARD, etc.) |
| `x-amz-server-side-encryption` | SSE info nếu có |
| `Accept-Ranges: bytes` | Bắt buộc — cho Range request support |
| `x-amz-request-id` | UUID per request |

### 4.7 Request-ID Propagation

- Request-ID phải được lưu vào `context.Context` qua custom context key
- Propagate xuyên suốt toàn bộ call chain: middleware → handler → storage → metadata
- Mọi log entry phải kèm request-id
- Mọi S3 response phải có header `x-amz-request-id`

```go
type ctxKey string
const requestIDKey ctxKey = "request_id"

// RequestIDFromContext trích xuất request-id từ context.
func RequestIDFromContext(ctx context.Context) string {
    if id, ok := ctx.Value(requestIDKey).(string); ok {
        return id
    }
    return ""
}
```

### 4.8 Concurrency

- Dùng `context.Context` để cancel operations, truyền qua toàn bộ call chain
- Không dùng goroutine mà không có mechanism để stop nó
- Background workers (lifecycle cleanup, multipart cleanup) phải listen `ctx.Done()`
- Dùng `sync.RWMutex` cho in-memory cache nếu có
- Bbolt đã handle concurrent access — không cần thêm lock ở trên (nhưng nhớ single-writer limitation)
- **Graceful shutdown cho multipart**: nếu server nhận SIGTERM giữa CompleteMultipartUpload:
  - `context.Context` cancellation propagate tới storage layer
  - Transaction-safe complete: write final object → update metadata → delete parts (ordered)
  - Cleanup orphan temp files khi startup

```go
// ✅ ĐÚNG — goroutine với lifecycle
func (s *Server) startCleanupWorker(ctx context.Context) {
    go func() {
        ticker := time.NewTicker(time.Hour)
        defer ticker.Stop()
        for {
            select {
            case <-ticker.C:
                s.runCleanup(ctx)
            case <-ctx.Done():
                return
            }
        }
    }()
}
```

### 4.9 Configuration

- Config source theo thứ tự ưu tiên (cao → thấp): **CLI flags > ENV > file YAML > defaults**
- Dùng **Viper** (`github.com/spf13/viper`) để merge các source — KHÔNG load YAML thủ công bằng `gopkg.in/yaml.v3`
- Cobra + Viper binding cho CLI flags
- ENV prefix: `GOS3_`, dùng `_` thay `.` (ví dụ: `GOS3_SERVER_PORT=9000`)
- Không hardcode bất kỳ giá trị nào — phải có config key tương ứng
- Validate config khi startup, fail fast nếu thiếu required field
- Secret values (secret key) KHÔNG được log ra kể cả ở debug level
- Server config bắt buộc có: `ReadHeaderTimeout`, `MaxHeaderBytes` (chống slowloris)

---

## 5. Dependencies (được phép dùng)

| Package | Phiên bản | Mục đích |
|---------|-----------|----------|
| `github.com/go-chi/chi/v5` | v5.x | HTTP router |
| `go.etcd.io/bbolt` | v1.3.x | Embedded metadata DB |
| `github.com/spf13/cobra` | v1.x | CLI framework |
| `github.com/spf13/viper` | v1.x | Config management (YAML + ENV + flags) |
| `golang.org/x/time/rate` | latest | Rate limiting |
| `github.com/google/uuid` | v1.x | UUID generation |
| `github.com/schollz/progressbar/v3` | v3.x | CLI progress bar |

**Stdlib được ưu tiên tối đa:**
- `log/slog` — logging (KHÔNG dùng zap, logrus, zerolog)
- `crypto/sha256`, `crypto/hmac`, `crypto/md5` — hashing
- `encoding/xml` — XML (KHÔNG dùng external XML lib)
- `net/http` — HTTP server core
- `crypto/tls` — TLS
- `testing` — unit tests
- `syscall` — disk space check (`Statfs`)

**Tuyệt đối KHÔNG dùng:**
- Web framework: Gin, Echo, Fiber, Beego, Buffalo
- ORM: GORM, Ent, sqlx (không dùng SQL DB)
- Logging: zap, logrus, zerolog (dùng `slog`)
- `github.com/pkg/errors` (dùng stdlib `errors` + `fmt.Errorf %w`)
- `gopkg.in/yaml.v3` trực tiếp (dùng Viper thay thế)
- Bất kỳ package nào không có trong danh sách trên nếu stdlib có thể thay thế

---

## 6. Testing

### 6.1 Quy tắc test

- Mỗi package phải có file `*_test.go` tương ứng
- Test file dùng package `xxx_test` (black-box) trừ khi cần test internal
- Không dùng testing framework (testify là ngoại lệ chấp nhận được cho assertions)
- Table-driven tests là mặc định cho các hàm có nhiều case

```go
// ✅ ĐÚNG — table-driven test
func TestValidateBucketName(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        wantErr bool
    }{
        {"valid simple", "my-bucket", false},
        {"too short", "ab", true},
        {"uppercase", "My-Bucket", true},
        {"consecutive dots", "my..bucket", true},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := validateBucketName(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("validateBucketName(%q) error = %v, wantErr %v",
                    tt.input, err, tt.wantErr)
            }
        })
    }
}
```

### 6.2 Các loại test cần có

- **Unit tests:** mọi function public trong `internal/`
- **Integration tests:** handler tests dùng `httptest.NewRecorder()`
- **Smoke tests:** `test/smoke.sh` — test cơ bản với `awscli` (bắt buộc chạy sau khi SigV4 hoạt động)
  - `aws s3 mb`, `aws s3 cp`, `aws s3 ls`, `aws s3 rm`, `aws s3 rb`
  - Phải pass trước khi coi SigV4 implementation là hoàn thành