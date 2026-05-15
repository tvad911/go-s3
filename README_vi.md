# GoS3

GoS3 là một hệ thống lưu trữ đối tượng (object storage) gọn nhẹ, tương thích với chuẩn S3 và một CLI client được viết hoàn toàn bằng Go. Được thiết kế như một giải pháp thay thế đơn giản hơn cho Garage và MinIO, GoS3 có hiệu năng cao, dễ dàng triển khai qua Docker và tương thích hoàn toàn với AWS S3 API (Signature V4).

## 🚀 Các Tính Năng

### Tương thích S3 Cốt lõi
- **Tương thích API**: Hỗ trợ đầy đủ các AWS S3 SDK, `aws-cli` và `mc` (MinIO Client).
- **Xác thực**: Hỗ trợ AWS Signature V4 với hệ thống đánh giá IAM Policy mạnh mẽ.
- **Thao tác Object**: Hỗ trợ đọc/ghi luồng (streaming) cho các file lớn, AWS Chunked Uploads, Multipart Uploads, tạo mã băm ETag.
- **Tính năng Bucket**: Versioning (Lưu trữ phiên bản), Object Lock, CORS, và Bucket Policies (Phân quyền).

### Tính năng Nâng cao
- **Static Website Hosting**: Chạy các trang web tĩnh (HTML/CSS/JS) trực tiếp từ bucket của bạn.
- **Tên miền Tùy chỉnh (Custom Domains)**: Hỗ trợ tên miền ảo (Virtual-hosted style) (vd: `cdn.yourdomain.com`).
- **Quản lý Vòng đời (Lifecycle Management)**: Các tiến trình chạy ngầm tự động xoá hoặc hết hạn file dựa trên các bộ quy tắc linh hoạt.
- **Audit Logs**: Ghi nhật ký toàn diện các lượt truy cập qua S3 API và các thao tác của Quản trị viên.
- **Webhooks**: Bắn thông báo sự kiện (vd: `s3:ObjectCreated:*`) tới các hệ thống bên ngoài qua HTTP.

### Web Console Quản trị
GoS3 được tích hợp sẵn một giao diện người dùng Web UI gọn nhẹ (viết bằng JS/HTML/CSS thuần) chạy trên cổng 9001.
- **Quản lý Định danh**: Quản lý Users (người dùng) và các IAM Policies.
- **Service Accounts**: Tạo và quản lý Access Key / Secret Key cấp cho các ứng dụng.
- **Trình duyệt Dữ liệu (Object Browser)**: Duyệt cây thư mục, xem trước file, tìm kiếm, tải lên và tạo đường dẫn chia sẻ (presigned URLs).
- **Cấu hình Bucket**: Giao diện trực quan để thiết lập Lifecycle, CORS, Policies, Webhooks, và Custom Domains.

---

## 🏗 Kiến trúc

GoS3 tuân theo kiến trúc đơn-binary gọn gàng (monorepo), không phụ thuộc vào các thư viện ngoài cồng kềnh:
- **Tầng HTTP**: Sử dụng `go-chi/chi` để xử lý routing và middleware (Rate Limiting, CORS, SigV4 Auth).
- **Tầng Lưu trữ**: Lưu trữ dữ liệu object trên hệ thống tệp tin cục bộ (local filesystem) (hỗ trợ rename nguyên tử, streaming).
- **Cơ sở dữ liệu Metadata**: Tích hợp sẵn CSDL `bbolt` chạy nội bộ để lưu trữ siêu dữ liệu cực nhanh (buckets, users, policies, trạng thái multipart).
- **Frontend**: ES Modules và CSS thuần được nhúng trực tiếp vào file thực thi bằng chỉ thị `//go:embed`.

---

## 🛠 Hướng dẫn Khởi chạy

### 1. Chạy với Docker Compose (Khuyên dùng)

GoS3 đi kèm với file `docker-compose.yml` sẵn sàng cho môi trường production.

```bash
# Clone mã nguồn
git clone https://github.com/tvad911/go-s3.git
cd go-s3

# Khởi động server chạy nền
docker-compose -f deploy/docker-compose.yml up -d
```

- **S3 API Port**: `9000`
- **Web Console Port**: `9001`
- **Tài khoản mặc định**: `minioadmin` / `minioadmin` (Có thể đổi trong `docker-compose.yml`)

### 2. Tự biên dịch từ Mã Nguồn

Đảm bảo bạn đã cài đặt Go phiên bản 1.22 trở lên.

```bash
# Biên dịch server và client
make build

# Các file thực thi (binary) sẽ được tạo ở thư mục gốc
./gos3
./gos3c
```

---

## 💻 Cách sử dụng

### Truy cập Web Console
Mở trình duyệt web của bạn và truy cập `http://localhost:9001`. Đăng nhập bằng Tài khoản Root (mặc định: `minioadmin` / `minioadmin`).

### Sử dụng qua AWS CLI
Bạn có thể thao tác với GoS3 y hệt như thao tác với Amazon S3 thông qua AWS CLI.

```bash
# Cấu hình AWS CLI profile
aws configure --profile gos3
# AWS Access Key ID: minioadmin
# AWS Secret Access Key: minioadmin
# Default region name: us-east-1

# Tạo một bucket mới
aws --profile gos3 --endpoint-url http://localhost:9000 s3 mb s3://my-bucket

# Tải 1 file lên
aws --profile gos3 --endpoint-url http://localhost:9000 s3 cp ./hello.txt s3://my-bucket/

# Liệt kê danh sách file
aws --profile gos3 --endpoint-url http://localhost:9000 s3 ls s3://my-bucket/
```

### Static Website & Custom Domains (Web tĩnh & Tên miền riêng)
1. Mở Web Console -> Vào **Bucket Settings**.
2. Kích hoạt tính năng **Static Website Hosting** và gõ `index.html` vào ô Index Document.
3. Bấm thêm **Custom Domain** (vd: `cdn.example.com`).
4. Tại trình quản lý DNS của bạn (A Record / CNAME), trỏ tên miền về địa chỉ server chạy GoS3.
5. Giờ đây người dùng đã có thể truy cập thẳng vào trang web tĩnh qua `http://cdn.example.com`.

---

## 🔒 Bảo Mật & Best Practices

- **Tài Khoản Root**: Luôn luôn thay đổi giá trị mặc định của `GOS3_AUTH_ROOT_ACCESS_KEY` và `GOS3_AUTH_ROOT_SECRET_KEY` trong biến môi trường trước khi đưa dự án ra public (production).
- **Service Accounts**: Không sử dụng thông tin tài khoản Root để gắn vào mã nguồn các ứng dụng (Apps). Thay vào đó, hãy lên Web Console tạo riêng các **Service Accounts** với IAM Policies bị giới hạn chặt chẽ.
- **Reverse Proxy**: Khuyến khích bạn đặt GoS3 đứng sau một Reverse Proxy (như Nginx, Traefik, hay Caddy) để nó giúp bạn quản lý các chứng chỉ bảo mật TLS/SSL và giao thức HTTPS.

---

## 🧪 CI/CD & Kiểm Thử (Testing)

Dự án này sử dụng GitHub Actions cho CI/CD tự động. Pipeline sẽ tự động chạy:
- Công cụ kiểm tra code `golangci-lint`
- Unit và integration tests (`go test -race`)
- Biên dịch thử Docker Image

Để tự chạy quy trình kiểm thử này ở dưới máy của bạn:
```bash
make test
```

---

## 📄 License (Giấy Phép)
Apache License 2.0
