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

**Frontend:**
- [x] Sửa Login form: chỉ 2 trường Username + Password
- [x] Xóa logic `localStorage` lưu raw key
- [x] Admin API calls dùng Cookie thay vì aws4fetch sign
- [x] S3 API calls từ Browser: dùng Presigned URL do server generate (thay vì client-side SigV4)

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
- [ ] Versioning tab: Toggle Enable/Suspend
- [ ] Bucket card: hiển thị thêm số objects, dung lượng

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
- [ ] Custom Domain mapping (Backend middleware + UI)
- [x] Audit Log viewer
- [ ] Bucket Notification/Events

---

## 5. Quy tắc kỹ thuật

- **Frontend stack:** Vanilla JS (đã có), không thêm framework. Dùng ES Modules.
- **Auth flow Web:** JWT trong HttpOnly Cookie. Không lưu bất kỳ secret nào trong `localStorage`.
- **Auth flow S3 API:** SigV4 không đổi, nhưng lookup key từ `service_accounts` thay vì `users`.
- **Password hashing:** `bcrypt` (stdlib `golang.org/x/crypto/bcrypt`).
- **JWT:** Dùng `crypto/hmac` + `crypto/sha256` stdlib, KHÔNG thêm dependency JWT library.
- **Backward compatibility:** Root user startup vẫn dùng `root_access_key`/`root_secret_key` từ config → auto-tạo ServiceAccount tương ứng.
