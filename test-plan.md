# GoS3 Test Plan 🧪

Tài liệu này dùng để theo dõi tiến độ và kịch bản test cho toàn bộ hệ thống GoS3.

## 🟢 Phase 1: Core S3 API & AWS CLI Compatibility
- [x] Khởi động server thành công
- [x] Cấu hình AWS CLI với endpoint `http://localhost:9010`
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
