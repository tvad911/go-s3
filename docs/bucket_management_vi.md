# Quản lý Dữ liệu & Cấu hình Bucket (Bucket Management)

Tài liệu này hướng dẫn cách quản lý dữ liệu (Files/Folders) và sử dụng các tính năng nâng cao cho từng Bucket trên GoS3.

---

## 1. Thao tác Cơ bản (Object Browser)

- Truy cập mục **Buckets** từ menu. Bấm **Create Bucket** để tạo một không gian lưu trữ mới (Lưu ý: Tên bucket viết chữ thường, số, dấu gạch ngang, không có khoảng trắng).
- Bấm vào tên Bucket vừa tạo, giao diện **Object Browser** sẽ xuất hiện:
  - **Tải file lên (Upload):** Kéo thả file/thư mục hoặc bấm nút **Upload** (hỗ trợ upload file lớn bằng cơ chế chia nhỏ).
  - **Tạo thư mục (Create Folder):** Nhấn **New Folder** để tổ chức dữ liệu trực quan.
  - **Chia sẻ tệp tin (Share):** Nhấn biểu tượng 🔗 trên một file để tạo liên kết Presigned URL. Link này giúp chia sẻ file một cách an toàn trong một thời gian cố định (vd: 1 tiếng) ngay cả khi Bucket đó đang khoá ở chế độ bảo mật (Private).
  - **Đổi tên & Xoá nhiều file:** Sử dụng checkbox để xoá hàng loạt hoặc nhấn icon bút chì để đổi tên tệp.

---

## 2. Tính năng Nâng cao (Bucket Settings)

Mỗi Bucket đều có cấu hình nâng cao. Bạn có thể truy cập bằng cách bấm vào biểu tượng ⚙️ (Settings) trên card của Bucket hoặc chọn tab **Settings** từ Object Browser.

### A. Static Website Hosting & Custom Domains
Chức năng này giúp bạn host một trang web tĩnh (HTML/CSS/JS) hoặc thiết lập CDN ảnh trực tiếp từ Bucket.

1. Chuyển sang tab **Website Hosting** và bật tính năng này. Nhập tên file mặc định (VD: `index.html`).
2. Chuyển sang tab **Custom Domains**, thêm tên miền của bạn (VD: `cdn.mycompany.com`).
3. Trỏ DNS (CNAME) của `cdn.mycompany.com` về địa chỉ server GoS3. Bây giờ bạn có thể truy cập nội dung website tĩnh qua tên miền riêng.

### B. Lifecycle Management (Vòng Đời Dữ Liệu)
Lifecycle giúp tự động dọn dẹp các tệp cũ nhằm tiết kiệm không gian đĩa cứng (Ví dụ: Xoá tệp log sau 30 ngày).

1. Chuyển sang tab **Lifecycle**.
2. Nhập cấu hình JSON để thiết lập quy tắc tự động xoá (`Expiration`) cho một `Prefix` nhất định.

### C. CORS (Cross-Origin Resource Sharing)
Nếu bạn có một trang web khác (VD: `https://your-domain.com`) và muốn trình duyệt cho phép người dùng upload thẳng file từ trình duyệt lên GoS3:
- Chuyển sang tab **CORS**.
- Thiết lập `AllowedOrigins` bằng domain của bạn và cấp quyền `AllowedMethods` (VD: GET, PUT).

### D. Webhooks
Bạn muốn báo cho server Backend của mình biết mỗi khi có người dùng upload ảnh lên GoS3?
- Chuyển sang tab **Webhooks**.
- Đăng ký URL backend của bạn và sự kiện cần lắng nghe (VD: `s3:ObjectCreated:*`). GoS3 sẽ tự động gọi HTTP POST đến URL đó khi sự kiện xảy ra.
