# Tích hợp Ứng dụng & API (CLI & SDK)

GoS3 hoàn toàn tương thích với chuẩn AWS S3. Bất cứ bộ công cụ hay thư viện lập trình nào (CLI, NodeJS, Python, Go, PHP) hỗ trợ S3 đều có thể dùng để giao tiếp với GoS3 một cách liền mạch.

> **Lưu ý:** Bạn cần phải tạo **Access Key** và **Secret Key** (qua tab Service Accounts trên Web Console) để sử dụng với API.

---

## 1. Dùng công cụ AWS CLI

Đây là cách nhanh nhất để quản lý dữ liệu qua Command Line. Bạn chỉ cần cài đặt `aws-cli` trên máy.

### A. Cấu hình ban đầu
Chạy lệnh sau và nhập các thông số được cung cấp từ Service Account:
```bash
aws configure --profile gos3
```
- **AWS Access Key ID:** `<Access Key của Service Account>`
- **AWS Secret Access Key:** `<Secret Key của Service Account>`
- **Default region name:** `us-east-1` (hoặc vùng mà bạn đã thiết lập)

### B. Thực hiện các câu lệnh S3
Mọi lệnh đều cần thêm tham số `--endpoint-url` trỏ đến server GoS3 (mặc định S3 API chạy ở port `9000`):

```bash
# Upload 1 file ảnh vào bucket
aws --profile gos3 --endpoint-url http://localhost:9000 s3 cp image.jpg s3://my-bucket/

# Xoá toàn bộ bucket (và dữ liệu bên trong)
aws --profile gos3 --endpoint-url http://localhost:9000 s3 rb s3://my-bucket --force
```

---

## 2. Tích hợp bằng NodeJS (AWS SDK v3)

Đây là ví dụ điển hình về cách tải một tệp tin từ Backend (NodeJS) lên server GoS3.

### Cài đặt
```bash
npm install @aws-sdk/client-s3
```

### Mã nguồn mẫu (Upload File)
```javascript
const { S3Client, PutObjectCommand } = require("@aws-sdk/client-s3");
const fs = require("fs");

// Cấu hình Client kết nối tới GoS3 thay vì AWS
const s3 = new S3Client({
  endpoint: "http://localhost:9000",
  region: "us-east-1",
  credentials: {
    accessKeyId: "ACCESS_KEY_CUA_BAN",
    secretAccessKey: "SECRET_KEY_CUA_BAN"
  },
  forcePathStyle: true // BẮT BUỘC: bật tuỳ chọn này khi dùng server tự host như GoS3
});

async function upload() {
  const fileStream = fs.createReadStream("image.png");
  
  await s3.send(new PutObjectCommand({
    Bucket: "my-bucket",
    Key: "uploads/image.png",
    Body: fileStream,
  }));
  
  console.log("Upload lên GoS3 thành công!");
}

upload();
```
*(Nếu bạn dùng các ngôn ngữ lập trình khác như Python Boto3 hay PHP AWS SDK, hãy luôn nhớ bật cấu hình `forcePathStyle: true` tương tự).*
