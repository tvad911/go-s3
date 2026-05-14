# PLAN: GoS3 Console Upgrade v1

> Kế hoạch triển khai nâng cấp GoS3 Web Console theo chuẩn MinIO Console.
> Tham chiếu chi tiết: [`update_v1.md`](../update_v1.md)

---

## Phase 1: Backend Auth v2 + Login UI

**Mục tiêu:** Đăng nhập bằng Username/Password, bỏ nhập raw key trên giao diện.

### Task 1.1: Sửa struct User — thêm PasswordHash
- **File:** `internal/auth/iam.go`
- **Nội dung:** Thêm field `PasswordHash string` vào struct `User`. Thêm function `HashPassword(plain string) string` và `CheckPassword(hash, plain string) bool` dùng `bcrypt`.
- **Dependency:** Thêm `golang.org/x/crypto/bcrypt` vào `go.mod`.
- **Verification:** Unit test hash + verify password.

### Task 1.2: Tạo struct ServiceAccount + Store
- **File mới:** `internal/auth/service_account.go`
- **Nội dung:**
  ```go
  type ServiceAccount struct {
      ID          string    // UUID
      AccessKeyID string    // auto-generated (20 chars)
      SecretKey   string    // auto-generated (40 chars), chỉ trả 1 lần khi tạo
      ParentUser  string    // Username owning this key
      Description string
      Policies    []string
      Disabled    bool
      ExpiresAt   *time.Time
      CreatedAt   time.Time
  }
  ```
- **Bbolt bucket:** `service_accounts` (key = AccessKeyID, value = JSON ServiceAccount)
- **Interface:** `ServiceAccountStore` — `Create`, `List(parentUser)`, `GetByAccessKey`, `Delete`, `Disable`
- **Verification:** Unit test CRUD.

### Task 1.3: Implement ServiceAccountStore trong metadata bbolt
- **File:** `internal/storage/metadata/service_account.go`
- **Nội dung:** Implement `ServiceAccountStore` interface dùng bbolt transactions.
- **Verification:** Integration test với bbolt temp file.

### Task 1.4: Tạo JWT Session module
- **File mới:** `internal/auth/session.go`
- **Nội dung:**
  - `GenerateToken(userID string, isRoot bool) (string, error)` — HMAC-SHA256, exp 24h.
  - `ValidateToken(tokenStr string) (*Claims, error)` — Parse + verify.
  - JWT signing key lấy từ config hoặc auto-generate random 32 bytes khi startup.
  - **KHÔNG** dùng thư viện JWT bên ngoài — dùng stdlib `crypto/hmac`, `encoding/base64`, `encoding/json`.
- **Verification:** Unit test generate + validate + expired token.

### Task 1.5: Tạo Auth API handlers
- **File mới:** `internal/handler/auth_handler.go`
- **Endpoints:**
  - `POST /api/v1/login` — nhận `{username, password}`, verify bcrypt, trả JWT trong `Set-Cookie: token=...; HttpOnly; SameSite=Strict; Path=/`
  - `POST /api/v1/logout` — clear cookie.
  - `GET /api/v1/me` — trả thông tin user hiện tại (username, isRoot, policies).
- **Verification:** Integration test với httptest.

### Task 1.6: Tạo Service Account API handlers
- **File:** Bổ sung vào `internal/handler/admin.go` hoặc file mới `service_account_handler.go`
- **Endpoints:**
  - `POST /api/v1/service-accounts` — tạo key mới cho user đang login.
  - `GET /api/v1/service-accounts` — list keys của user đang login.
  - `DELETE /api/v1/service-accounts/{id}` — xóa key.
- **Response tạo key:** Trả cả AccessKey + SecretKey (1 lần duy nhất).
- **Verification:** Integration test.

### Task 1.7: Sửa SigV4 Verifier — lookup ServiceAccount
- **File:** `internal/auth/sigv4.go`
- **Nội dung:** Khi nhận AccessKeyID từ Authorization header:
  1. Lookup trong `service_accounts` table trước.
  2. Nếu tìm thấy → dùng SecretKey của ServiceAccount để verify + resolve ParentUser.
  3. Nếu không tìm thấy → fallback lookup trong `users` table (backward compat).
- **Verification:** Test cả 2 luồng: old-style user key và new-style service account key.

### Task 1.8: Sửa Router — thêm routes mới
- **File:** `internal/server/router.go`
- **Nội dung:**
  - Mount `/api/v1/login`, `/api/v1/logout` (không cần auth).
  - Mount `/api/v1/me`, `/api/v1/service-accounts` (cần JWT cookie middleware).
  - Admin routes (`/_admin/*`) chấp nhận cả JWT cookie và SigV4.
- **Verification:** Smoke test curl.

### Task 1.9: Auto-create root ServiceAccount khi startup
- **File:** `cmd/server/main.go`
- **Nội dung:** Khi tạo root user, đồng thời tạo ServiceAccount với AccessKey/SecretKey từ config (`root_access_key`, `root_secret_key`). Đảm bảo backward compatibility.
- **Verification:** Server start → cũ config vẫn hoạt động với aws-cli.

### Task 1.10: Sửa Frontend Login
- **File:** `web/dist/index.html`, `web/dist/app.js`
- **Nội dung:**
  - Login form: 2 trường Username + Password.
  - Submit → `POST /api/v1/login` → nhận cookie → redirect dashboard.
  - Xóa toàn bộ logic `localStorage` lưu key và `aws4fetch`.
  - Admin API calls: dùng `fetch()` thường (cookie tự gửi kèm).
  - S3 API calls từ browser: gọi qua Admin API proxy hoặc presigned URL.
- **Verification:** Test đăng nhập/đăng xuất trên browser.

---

## Phase 2: Object Browser nâng cấp

**Mục tiêu:** Duyệt file theo thư mục, tìm kiếm, sắp xếp, share link.

### Task 2.1: Folder navigation
- Gọi ListObjectsV2 với `delimiter=/` → tách `CommonPrefixes` (folders) và `Contents` (files).
- Breadcrumb navigation: click segment để quay lại.
- Icon phân biệt 📁 folder vs 📄 file.

### Task 2.2: Create Folder
- Nút "Create Folder" → prompt tên → `PUT /{bucket}/{prefix}{name}/` với body rỗng.

### Task 2.3: Search + Sort
- Ô Search: client-side filter danh sách hiện tại theo tên.
- Sort dropdown: sort array `Contents` theo name/size/lastModified trước khi render.

### Task 2.4: Share Link
- Nút "Share" trên mỗi file → gọi `POST /_admin/presign` → hiển thị URL trong modal + nút Copy.

### Task 2.5: Preview
- Click file ảnh → modal preview (dùng presigned URL làm `img.src`).
- Detect bằng extension: `.jpg`, `.png`, `.gif`, `.webp`, `.svg`.

---

## Phase 3: Bucket Settings + Identity

**Mục tiêu:** Quản trị bucket chuyên sâu, quản lý user + policy.

### Task 3.1: Bucket Details page (tabs)
- Route: click bucket card → trang chi tiết thay vì trực tiếp vào Object Browser.
- Tabs: Overview | Objects | Access | Versioning | CORS | Lifecycle | Website

### Task 3.2: Access tab (Bucket Policy)
- Toggle nhanh: Public Read / Private.
- Advanced: JSON editor cho custom policy.
- API: `GET/PUT/DELETE /{bucket}?policy`

### Task 3.3: Versioning tab
- Toggle: Enable / Suspend.
- API: `GET/PUT /{bucket}?versioning`

### Task 3.4: User management sửa lại
- Form tạo user: username + password.
- Bảng: hiển thị username, policies, status, created.
- Actions: đổi password, gán policy, disable/enable, delete.

### Task 3.5: Policy management (trang mới)
- Preset policies: `ReadOnly`, `ReadWrite`, `WriteOnly`, `Admin`.
- Custom: JSON editor.
- Enforce logic trong middleware (check policy khi truy cập bucket/object).

### Task 3.6: Bucket stats API
- `GET /_admin/buckets/{name}/stats` → trả `{objectCount, totalSize}`.
- Hiển thị trên Bucket card + Overview tab.

---

## Phase 4: Service Accounts UI + Dashboard

### Task 4.1: Service Accounts page
- Danh sách keys của user đang login.
- Nút Create → hiển thị AccessKey + SecretKey (1 lần) + nút Copy.
- Snippet cấu hình mẫu: AWS CLI, boto3, GoS3 Client.

### Task 4.2: Dashboard page
- Cards: Tổng dung lượng, Số buckets, Số users, Server uptime.
- Gợi ý Endpoint API.
- Quick actions: Create bucket, Create user.

### Task 4.3: Sidebar mở rộng
- Thêm mục: Dashboard, Policies, Service Accounts.
- Nhóm: Identity → Users + Policies.

---

## Phase 5: Tính năng nâng cao (P2-P3)

### Task 5.1: Rename/Move object
### Task 5.2: Multi-select + Bulk operations
### Task 5.3: Upload folder (webkitdirectory)
### Task 5.4: Download file lớn qua Presigned URL
### Task 5.5: CORS / Lifecycle / Website editor UI
### Task 5.6: Custom Domain mapping
### Task 5.7: Audit Log viewer

---

## Verification Checklist

- [ ] Login bằng username/password thành công
- [ ] Logout clear session
- [ ] Tạo Service Account → nhận AccessKey/SecretKey
- [ ] AWS CLI dùng Service Account key → hoạt động bình thường
- [ ] Old config (root_access_key) → backward compatible
- [ ] Object Browser hiển thị folders + files
- [ ] Create Folder hoạt động
- [ ] Search + Sort hoạt động
- [ ] Share link (Presigned URL) hoạt động
- [ ] Bucket Settings: toggle Versioning, edit Policy
- [ ] Tạo user mới với password
- [ ] Gán policy cho user → enforce quyền
- [ ] Dashboard hiển thị thống kê đúng
