# Hướng Dẫn Sử Dụng GoS3 Chi Tiết (User Guide)

Tài liệu này cung cấp hướng dẫn toàn diện từ cơ bản đến nâng cao để khai thác tối đa sức mạnh của **GoS3** – Hệ thống lưu trữ tương thích S3 tích hợp sẵn Web Console.

---

## 1. Truy cập Web Console

Giao diện quản trị của GoS3 mặc định được chạy trên cổng `9001` (S3 API chạy trên cổng `9000`).

1. Mở trình duyệt và truy cập: `http://<IP-server>:9001`
2. **Đăng nhập lần đầu:**
   - **Tài khoản:** `minioadmin` (hoặc giá trị `GOS3_AUTH_ROOT_ACCESS_KEY`)
   - **Mật khẩu:** `minioadmin` (hoặc giá trị `GOS3_AUTH_ROOT_SECRET_KEY`)
   *(Lưu ý: Bạn nên thay đổi thông tin này trong file cấu hình/docker-compose khi deploy ra môi trường thực tế).*

Sau khi đăng nhập, bạn sẽ được chuyển tới **Dashboard**, nơi hiển thị tổng quan về dung lượng hệ thống, số lượng Buckets, số lượng tài khoản (Users), và Uptime của server.

---

## 2. Quản lý Danh Tính và Phân Quyền (IAM & Security)

Để tăng cường bảo mật, không bao giờ dùng tài khoản Root (`minioadmin`) cho các ứng dụng (App) hoặc cấp cho người dùng khác. Hãy tạo các User và Service Account.

### A. Quản lý Users (Người dùng)
- Chuyển sang mục **Users** ở Menu trái.
- Bấm **Create User** để tạo tài khoản mới. Các user này có thể dùng để đăng nhập vào Web Console.
- Nhập tên đăng nhập và Mật khẩu. Bạn cũng có thể chọn cấp các quyền (Policy) có sẵn cho User này.

### B. IAM Policies (Quyền truy cập)
- Chuyển sang mục **Policies**.
- GoS3 sử dụng định dạng JSON Policy chuẩn AWS IAM.
- Ví dụ Policy cấp quyền chỉ-đọc (Read-Only) cho một bucket cụ thể:
  ```json
  {
    "Version": "2012-10-17",
    "Statement": [
      {
        "Effect": "Allow",
        "Action": ["s3:GetObject", "s3:ListBucket"],
        "Resource": [
          "arn:aws:s3:::my-bucket",
          "arn:aws:s3:::my-bucket/*"
        ]
      }
    ]
  }
  ```

### C. Service Accounts (Access/Secret Keys)
- Chuyển sang mục **Service Accounts**.
- Service Account được dùng để cấp phát Access Key và Secret Key cho các SDK, Ứng dụng, hoặc CLI.
- Khi tạo, hệ thống sẽ tự động sinh ngẫu nhiên `AccessKey` và `SecretKey`. **Lưu ý: Bấm biểu tượng con mắt để xem Secret Key và copy lại vì nó sẽ không hiện lại sau đó.**
- Bạn có thể đính kèm Policy giới hạn quyền cho từng Service Account (vd: App A chỉ được ghi vào Bucket A).

---

## 3. Quản lý Dữ Liệu (Object Browser)

- Truy cập **Buckets** từ Menu. Bấm **Create Bucket** để tạo không gian lưu trữ mới (tên bucket phải viết thường, không có khoảng trắng, giống AWS S3).
- Bấm vào tên Bucket, bạn sẽ được đưa tới giao diện **Object Browser**:
  - **Upload:** Kéo thả file hoặc bấm nút Upload để tải dữ liệu lên. Hỗ trợ tải file lớn (streaming & chunked).
  - **Tạo thư mục:** Bấm "Create Folder". GoS3 hỗ trợ hệ thống folder logic (Dựa vào dấu `/` prefix).
  - **Share (Presigned URL):** Bấm biểu tượng 🔗 trên một file, chọn thời gian hết hạn (vd: 1 Giờ) để tạo một đường link tải xuống an toàn có thể gửi cho người khác, kể cả khi bucket đó là bucket bảo mật (Private).
  - **Xoá nhiều file:** Tích chọn nhiều file và bấm Delete phía trên cùng.

---

## 4. Các tính năng Nâng Cao (Bucket Settings)

Trong màn hình Object Browser, hãy chọn tab **Settings** của Bucket để truy cập các tính năng nâng cao:

### 1. Static Website Hosting & Custom Domains
Chức năng này biến Bucket của bạn thành một Web Server để host web tĩnh (HTML/CSS/JS) hoặc làm CDN ảnh.
- **Bật Website Hosting:** 
  - Kéo thanh gạt sang "Enabled".
  - Nhập **Index Document** (thường là `index.html`).
- **Thêm Custom Domain (Tên miền riêng):**
  - Nhập tên miền của bạn (Vd: `cdn.mycompany.com`).
  - *Cấu hình bên ngoài:* Trỏ bản ghi DNS CNAME của `cdn.mycompany.com` về địa chỉ server GoS3 của bạn. 
  - Sau khi gắn, người dùng gõ `http://cdn.mycompany.com` trên trình duyệt thì GoS3 sẽ tự động trả về nội dung tĩnh `index.html` của bucket này.

### 2. Lifecycle Management (Quản lý Vòng đời)
Dùng để tự động dọn rác, xoá file lưu tạm (log, tmp) sau 1 khoảng thời gian để tiết kiệm dung lượng.
- Vào tab **Lifecycle**. 
- Bạn có thể cấu hình XML để: Tự động xoá file sau N ngày (Expiration), hoặc tự dọn dẹp các tiến trình Upload bị lỗi (AbortIncompleteMultipartUpload).
- *Ví dụ: Xoá thư mục `logs/` sau 30 ngày:*
  ```xml
  <LifecycleConfiguration>
      <Rule>
          <ID>Delete Old Logs</ID>
          <Filter><Prefix>logs/</Prefix></Filter>
          <Status>Enabled</Status>
          <Expiration><Days>30</Days></Expiration>
      </Rule>
  </LifecycleConfiguration>
  ```

### 3. Webhooks & Notifications
Gọi API đến hệ thống khác (Slack, Discord, Backend của bạn) mỗi khi có sự kiện (như Upload file mới).
- Vào tab **Webhooks**.
- Đăng ký một URL (Vd: `https://api.myweb.com/webhook/s3`).
- Chọn Event (Vd: `s3:ObjectCreated:*`).
- Mỗi khi có ảnh/file tải lên, GoS3 sẽ POST một gói tin JSON báo hiệu cho URL bạn cấu hình.

### 4. CORS (Cross-Origin Resource Sharing)
Bắt buộc nếu bạn muốn trình duyệt của người dùng (từ `yourdomain.com`) upload thẳng file lên GoS3 qua Javascript mà không bị lỗi block.
- Vào tab **CORS**, cấp phép các thông số như `AllowedOrigins` (thành `*` hoặc domain của bạn) và `AllowedMethods` (GET, PUT, POST).

---

## 5. Kết nối Hệ thống bằng AWS CLI & SDK

GoS3 tương thích 100% với chuẩn S3 của AWS. Bạn có thể dùng bất cứ thư viện nào hỗ trợ S3 để kết nối (bỏ qua bước này nếu bạn chỉ dùng Web UI).

### A. Dùng AWS CLI
Cài đặt `aws-cli`, sau đó cấu hình:
```bash
aws configure --profile gos3
# AWS Access Key ID: <Nhập Access Key của Service Account>
# AWS Secret Access Key: <Nhập Secret Key của Service Account>
# Default region name: us-east-1
```
Gọi lệnh:
```bash
# Upload 1 file
aws --profile gos3 --endpoint-url http://localhost:9000 s3 cp image.jpg s3://my-bucket/

# Xoá bucket
aws --profile gos3 --endpoint-url http://localhost:9000 s3 rb s3://my-bucket --force
```

### B. Dùng NodeJS (AWS SDK v3)
```javascript
const { S3Client, PutObjectCommand } = require("@aws-sdk/client-s3");
const fs = require("fs");

const s3 = new S3Client({
  endpoint: "http://localhost:9000",
  region: "us-east-1",
  credentials: {
    accessKeyId: "ACCESS_KEY_CUA_BAN",
    secretAccessKey: "SECRET_KEY_CUA_BAN"
  },
  forcePathStyle: true // BẮT BUỘC bật dòng này khi dùng server tự host
});

async function upload() {
  const fileStream = fs.createReadStream("image.png");
  await s3.send(new PutObjectCommand({
    Bucket: "my-bucket",
    Key: "uploads/image.png",
    Body: fileStream,
  }));
  console.log("Upload thành công!");
}
upload();
```

---

## 6. Audit Logs (Nhật ký Hệ thống)

Để kiểm tra ai đã làm gì (Audit) trên hệ thống:
- Từ Web Console, bấm vào **Audit Logs** trên Menu.
- Bạn sẽ thấy toàn bộ lịch sử: ai (User nào, Access Key nào), làm gì (vd: `s3:PutObject`, `admin:CreateUser`), vào lúc nào, và IP kết nối là gì.
- Tính năng này vô cùng hữu ích để debug hoặc theo dõi bảo mật.

---
*Chúc bạn có trải nghiệm lưu trữ mượt mà và an toàn với GoS3!*
