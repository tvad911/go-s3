# Cài đặt & Triển khai Hệ thống (Setup & Deployment)

Tài liệu này hướng dẫn bạn cách triển khai GoS3 lên máy chủ (Server/VPS) thông qua Docker một cách nhanh chóng và an toàn.

*Đọc bằng [Tiếng Việt](setup_deployment_vi.md) | Read in [English](setup_deployment_en.md)*

---

## 1. Yêu cầu Hệ thống
- Máy chủ sử dụng hệ điều hành Linux (Ubuntu/Debian/CentOS).
- Đã cài đặt **Docker** và **Docker Compose**.
- Đảm bảo đã mở các port `9000` (S3 API) và `9001` (Web Console) trên Firewall.

## 2. Triển khai bằng Docker Compose (Khuyên dùng)

Bạn có thể lựa chọn 2 cách triển khai dưới đây tuỳ thuộc vào nhu cầu:

### Lựa chọn A: Dùng bản Build sẵn (Nhanh nhất)
Tạo thư mục mới (Ví dụ: `/opt/gos3`) và tạo file `docker-compose.yml`:

```yaml
services:
  gos3:
    image: tvad9111/gos3:latest # Kéo trực tiếp Image từ Docker Hub
    ports:
      - "9000:9000"   # Port dành cho S3 API
      - "9001:9001"   # Port dành cho Web Console
    volumes:
      - gos3-data:/data
    environment:
      # Thiết lập tài khoản Root mặc định (BẮT BUỘC ĐỔI TRÊN PRODUCTION)
      GOS3_AUTH_ROOT_ACCESS_KEY: admin
      GOS3_AUTH_ROOT_SECRET_KEY: SuperSecretPassword123
      
      # Cấu hình thư mục lưu trữ
      GOS3_STORAGE_DATA_DIR: /data
      GOS3_STORAGE_TEMP_DIR: /data/tmp
      GOS3_STORAGE_META_DB: /data/meta.db
    restart: unless-stopped

volumes:
  gos3-data:
```

### Lựa chọn B: Tự Build từ Mã Nguồn (Lấy Code mới nhất)
Nếu bạn muốn sử dụng các tính năng mới nhất chưa được update lên Docker Hub, hoặc muốn chỉnh sửa lại mã nguồn:
1. Tải toàn bộ source code từ Github: `git clone https://github.com/tvad911/go-s3.git`
2. Mở file `deploy/docker-compose.yml`. Mặc định file này đã được cấu hình sẵn để tự Build từ source:

```yaml
services:
  gos3:
    # Bỏ dòng image đi và sử dụng cấu trúc build:
    build: 
      context: ..
      dockerfile: deploy/Dockerfile
    # ... Các cấu hình còn lại giữ nguyên
```

### Khởi chạy Server
Chạy lệnh sau tại thư mục chứa file `docker-compose.yml`:
```bash
docker-compose up -d
```

Để kiểm tra trạng thái hoạt động:
```bash
docker-compose logs -f
```

### Bước 3: Đăng nhập
Mở trình duyệt, truy cập `http://<IP-May-Chu>:9001` và đăng nhập bằng thông tin `GOS3_AUTH_ROOT_ACCESS_KEY` và `GOS3_AUTH_ROOT_SECRET_KEY` bạn đã thiết lập ở Bước 1.

---

## 3. Cấu hình chi tiết (Config yaml)

Thay vì dùng Environment Variables (như `GOS3_AUTH_ROOT_ACCESS_KEY`), bạn có thể dùng file `config.yaml` để có cấu hình linh hoạt hơn.
Mount file `config.yaml` vào container bằng cách thêm dòng sau vào mục `volumes` trong `docker-compose.yml`:
```yaml
    volumes:
      - gos3-data:/data
      - ./config.yaml:/app/config.yaml:ro
```

**Mẫu file `config.yaml`:**
```yaml
server:
  port: 9000
  base_domain: "s3.mycompany.com" # Dùng cho Virtual-host style (bucket.s3.mycompany.com)
  tls_cert: "" # Cấu hình SSL/TLS nếu không dùng Reverse Proxy
  tls_key: ""
  max_header_bytes: 1048576

storage:
  type: "local"
  data_dir: "/data"
  temp_dir: "/data/tmp"
  meta_db: "/data/meta.db"

auth:
  root_access_key: "admin"
  root_secret_key: "SuperSecretPassword123"

admin:
  ui_enabled: true
  ui_port: 9001
```

---

## 4. Bảo mật khi chạy Production

1. **Đổi PassRoot:** KHÔNG BAO GIỜ để pass mặc định `minioadmin`. Đổi ngay trong biến môi trường hoặc file `config.yaml`.
2. **Reverse Proxy (Nginx/Traefik):** Bạn nên đặt GoS3 phía sau một Nginx Reverse Proxy để cấu hình SSL/TLS (Let's Encrypt) an toàn hơn so với việc gắn SSL trực tiếp vào GoS3.
3. **Firewall:** Nếu bạn chỉ dùng GoS3 làm Backend Storage riêng tư, hãy chặn port `9000` từ ngoài Internet và chỉ cho phép dải IP của server Backend truy cập. Chỉ mở port `9001` cho IP của quản trị viên.
