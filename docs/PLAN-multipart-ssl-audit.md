# AUDIT: Multipart Upload & SSL/TLS

> **Date:** 2026-05-16
> **Status:** Audit Complete
> **Scope:** Kiểm tra tích hợp upload file lớn (chunked/multipart) và hỗ trợ SSL/TLS

---

## 1. Multipart Upload — ĐÃ TÍCH HỢP ✅

GoS3 đã implement **đầy đủ** S3 Multipart Upload API.

### 1.1 Server-side (S3 API)

| API Endpoint | Status | File |
|-------------|--------|------|
| `POST /{bucket}/{key}?uploads` — CreateMultipartUpload | ✅ Done | `handler/multipart.go:18` |
| `PUT /{bucket}/{key}?partNumber=N&uploadId=X` — UploadPart | ✅ Done | `handler/multipart.go:85` |
| `POST /{bucket}/{key}?uploadId=X` — CompleteMultipartUpload | ✅ Done | `handler/multipart.go:135` |
| `DELETE /{bucket}/{key}?uploadId=X` — AbortMultipartUpload | ✅ Done | `handler/multipart.go:186` |
| `GET /{bucket}/{key}?uploadId=X` — ListParts | ✅ Done | `handler/multipart.go:201` |
| `GET /{bucket}?uploads` — ListMultipartUploads | ✅ Done | `handler/multipart.go:260` |

**Routing:** Tất cả 6 endpoints đều được map trong `server/router.go:226-248`.

### 1.2 Storage Backend

| Feature | Status | Details |
|---------|--------|---------|
| Part lưu trên disk | ✅ | `tempDir/multipart/{uploadID}/parts/{00001..99999}` |
| Atomic write (tmp → rename) | ✅ | `local.go:533-553` |
| Disk space check trước upload | ✅ | `local.go:522-525` |
| MD5 hash tính streaming | ✅ | `local.go:541-555` — `io.MultiWriter(file, hash)` |
| Part bound: 1-10000 | ✅ | `multipart.go:92` — `partNum < 1 || partNum > 10000` |
| Multipart ETag format | ✅ | `MD5(MD5s...)-N` — `local.go:619` |
| Metadata trong BBolt | ✅ | `metadata/bbolt.go:503` (uploads bucket + parts:{uploadID}) |
| Lifecycle cleanup | ✅ | `server/lifecycle.go:118` — `AbortIncompleteMultipartUpload` |

### 1.3 AWS Chunked Upload (Streaming SigV4)

| Feature | Status | Details |
|---------|--------|---------|
| `STREAMING-AWS4-HMAC-SHA256-PAYLOAD` | ✅ | `chunk.go:98-101` |
| AWSChunkedReader | ✅ | `chunk.go:11-94` — decode hex-size;chunk-signature format |
| PutObject chunked | ✅ | `object.go:105` — tự detect và wrap body |
| UploadPart chunked | ✅ | `multipart.go:116-118` — same logic |

### 1.4 Client Library (`client/multipart.go`)

| Feature | Status | Details |
|---------|--------|---------|
| Auto multipart nếu file > 5MB | ✅ | `minPartSize = 5*1024*1024` (line 13) |
| Fallback PutObject nếu < 5MB | ✅ | `multipart.go:49-52` |
| Sequential part upload | ✅ | `multipart.go:93-130` |
| SigV4 signing mỗi part | ✅ | Qua `c.doRequest()` |
| Complete XML construction | ✅ | `multipart.go:132-154` |

### 1.5 CLI (`gos3c`)

| Feature | Status | Details |
|---------|--------|---------|
| `gos3c cp local.file s3://bucket/key` | ✅ | `cli/cp.go:65` — gọi `PutObjectMultipart` |
| Auto chunked cho file lớn | ✅ | Client tự quyết định dựa trên file size |

### 1.6 Đánh giá — Những điểm CÓ THỂ CẢI THIỆN

| # | Issue | Severity | Gợi ý |
|---|-------|----------|-------|
| 1 | Upload part tuần tự (sequential) | LOW | Thêm parallel upload (goroutine pool) cho tốc độ |
| 2 | Không có progress bar trong multipart upload | LOW | Tích hợp `progressbar/v3` vào `PutObjectMultipart` |
| 3 | Không retry nếu 1 part fail | MEDIUM | Thêm retry logic với exponential backoff |
| 4 | Không abort upload khi error giữa chừng | MEDIUM | Client nên gọi `AbortMultipartUpload` khi fail |
| 5 | `config.example.yaml` không có `multipart_cleanup_days` | LOW | Thêm default config |

---

## 2. SSL/TLS — ĐÃ TÍCH HỢP ✅

### 2.1 Server TLS Config

```go
// internal/config/config.go:36-42
type TLSConfig struct {
    Enabled      bool   `mapstructure:"enabled"`
    Cert         string `mapstructure:"cert"`
    Key          string `mapstructure:"key"`
    AutoRedirect bool   `mapstructure:"auto_redirect"`
    HTTPPort     int    `mapstructure:"http_port"`
}
```

| Feature | Status | Details |
|---------|--------|---------|
| HTTPS server (ListenAndServeTLS) | ✅ | `server.go:101` |
| TLS on/off toggle | ✅ | `server.tls.enabled` config key |
| Cert + Key file path | ✅ | `server.tls.cert`, `server.tls.key` |
| HTTP → HTTPS auto redirect | ✅ | `server.go:79-98` — chạy redirect server riêng |
| Redirect port configurable | ✅ | `server.tls.http_port` (default 8080) |
| Graceful shutdown redirect server | ✅ | `server.go:148-152` |

### 2.2 Config Defaults

```go
// internal/config/defaults.go:19-21
v.SetDefault("server.tls.enabled", false)
v.SetDefault("server.tls.auto_redirect", false)
v.SetDefault("server.tls.http_port", 8080)
```

### 2.3 Client SSL Support

```go
// client/client.go:18,35
type Config struct {
    UseSSL bool  // ← Toggle SSL cho client
}
// Xây URL: https:// nếu UseSSL = true
```

### 2.4 CLI SSL Flag

```go
// cli/root.go:16,63,86
rootCmd.PersistentFlags().BoolVar(&noSSL, "--no-ssl", false, "Disable TLS")
// Config: UseSSL = !noSSL
```

### 2.5 Cách bật SSL

**YAML config:**
```yaml
server:
  tls:
    enabled: true
    cert: "/path/to/cert.pem"
    key: "/path/to/key.pem"
    auto_redirect: true  # HTTP → HTTPS redirect
    http_port: 8080       # Port cho HTTP redirect server
```

**ENV:**
```bash
GOS3_SERVER_TLS_ENABLED=true
GOS3_SERVER_TLS_CERT=/path/to/cert.pem
GOS3_SERVER_TLS_KEY=/path/to/key.pem
```

**CLI client:**
```bash
gos3c --endpoint https://s3.example.com:9000 ls s3://bucket
# Hoặc tắt SSL verify:
gos3c --no-ssl --endpoint http://localhost:9000 ls s3://bucket
```

### 2.6 Đánh giá — Những điểm CÓ THỂ CẢI THIỆN

| # | Issue | Severity | Gợi ý |
|---|-------|----------|-------|
| 1 | Client không skip TLS verify (InsecureSkipVerify) | LOW | Thêm `--insecure` flag cho self-signed certs |
| 2 | Không hỗ trợ Let's Encrypt auto-cert | LOW | Tích hợp `golang.org/x/crypto/acme/autocert` |
| 3 | Web UI server (port 9001) không chạy TLS riêng | MEDIUM | UIServer cũng cần `ListenAndServeTLS` nếu TLS enabled |
| 4 | `config.example.yaml` có `tls_cert`/`tls_key` nhưng struct dùng `tls.cert`/`tls.key` | LOW | Sync config key names |
| 5 | Không log TLS version/cipher suite khi connection established | LOW | Thêm logging TLS info |

---

## 3. Tổng kết

| Feature | Status | Coverage |
|---------|--------|----------|
| **S3 Multipart Upload API** | ✅ Đầy đủ | 6/6 endpoints |
| **AWS Chunked Upload (Streaming SigV4)** | ✅ Đầy đủ | PutObject + UploadPart |
| **Client auto-multipart (>5MB)** | ✅ Đầy đủ | Auto fallback |
| **CLI multipart upload** | ✅ Đầy đủ | `gos3c cp` |
| **Server TLS/HTTPS** | ✅ Đầy đủ | Config + ListenAndServeTLS |
| **HTTP → HTTPS redirect** | ✅ Đầy đủ | Auto redirect server |
| **Client SSL toggle** | ✅ Đầy đủ | `--no-ssl` flag |
| **Lifecycle cleanup orphan multiparts** | ✅ Đầy đủ | AbortIncompleteMultipartUpload |

### Điểm cần cải thiện (ưu tiên từ cao → thấp)

1. **MEDIUM:** Client multipart retry + abort on error
2. **MEDIUM:** Web UI server cũng cần TLS khi `tls.enabled=true`
3. **LOW:** Parallel part upload cho tốc độ
4. **LOW:** `--insecure` flag cho self-signed certificates
5. **LOW:** Sync `config.example.yaml` TLS key names
