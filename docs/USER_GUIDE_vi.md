# Hướng Dẫn Sử Dụng GoS3 Chi Tiết

Chào mừng bạn đến với GoS3! Tài liệu này sẽ hướng dẫn bạn các bước khai thác tối đa sức mạnh của hệ thống lưu trữ S3 tương thích, được trang bị kèm Web Console quản trị trực quan.

## Bắt Đầu Nhanh (Quick Start)

Giao diện quản trị của GoS3 chạy độc lập với API S3:
- **Giao diện Web Console:** Chạy trên cổng `9001` (VD: `http://localhost:9001`)
- **API S3 dành cho SDK/CLI:** Chạy trên cổng `9000` (VD: `http://localhost:9000`)

### Hướng dẫn đăng nhập lần đầu:
1. Mở trình duyệt và vào địa chỉ: `http://<IP-server>:9001`
2. Đăng nhập bằng thông tin quản trị Root (Hoặc thông tin bạn đã tuỳ chỉnh trong Docker/ENV):
   - **Tài khoản:** `minioadmin`
   - **Mật khẩu:** `minioadmin`

Sau khi đăng nhập, hệ thống hiển thị **Dashboard** theo dõi các chỉ số quan trọng (dung lượng, số lượng file, số bucket đang hoạt động).

---

## Tính Năng & Mục Lục Tài Liệu

Tài liệu đã được chia nhỏ thành các mục chi tiết để bạn dễ dàng tra cứu. Vui lòng bấm vào các liên kết bên dưới để xem hướng dẫn cụ thể:

### 1. [Quản lý Danh Tính & Bảo Mật (IAM & Security)](./iam_security_vi.md)
Hệ thống cấp quyền an toàn:
- Quản lý **Người dùng (Users)** cho Web Console.
- Tạo **Access Keys (Service Accounts)** cho phần mềm.
- Kiểm soát chính xác quyền bằng **IAM Policies** chuẩn JSON.
- Theo dõi lịch sử hệ thống qua **Audit Logs**.

### 2. [Quản lý Dữ liệu & Cấu hình Bucket](./bucket_management_vi.md)
Thao tác với dữ liệu trực tiếp:
- Giao diện **Object Browser**: Upload, tạo thư mục, Share file an toàn (Presign URL).
- Cấu hình chuyên sâu cho Bucket:
  - Host **Website tĩnh (Static Web Hosting)** và **Custom Domains**.
  - Thiết lập vòng đời tự động xoá file cũ (**Lifecycle**).
  - Tích hợp liên nguồn (**CORS**) và sự kiện (**Webhooks**).

### 3. [Tích hợp Ứng dụng & API (CLI & SDK)](./api_integration_vi.md)
GoS3 có chuẩn tương thích AWS S3 100%:
- Cách kết nối và gõ lệnh với **AWS CLI**.
- Tích hợp nhanh với phần mềm của bạn thông qua **AWS SDK (VD: NodeJS)**.

---

> **Tip:** Nếu bạn chỉ muốn thao tác lưu trữ file thủ công cho cá nhân, bạn chỉ cần dùng **Web Console** là đủ. Hãy cấp quyền Service Account nếu bạn định tích hợp GoS3 làm bộ lưu trữ Backend cho phần mềm khác.
