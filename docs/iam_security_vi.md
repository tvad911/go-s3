# Quản lý Danh Tính & Bảo Mật (IAM & Security)

Để đảm bảo an toàn cho hệ thống, **không bao giờ sử dụng tài khoản Root (`minioadmin`) cho các ứng dụng thực tế**. Thay vào đó, hãy sử dụng tính năng quản lý danh tính (IAM) của GoS3.

*Đọc bằng [Tiếng Việt](iam_security_vi.md) | Read in [English](iam_security_en.md)*

## Kiến trúc Phân quyền

GoS3 áp dụng mô hình phân quyền theo chuẩn AWS IAM:

```
Root Admin (config.yaml / ENV)
  └── Đăng nhập Web Console (cổng 9001)
        ├── Tạo IAM Users
        │     ├── Gắn IAM Policies (N policies / user)
        │     └── Tạo Access Keys (N keys / user)
        │           └── Kế thừa 100% quyền từ User cha
        └── Tạo IAM Policies (JSON documents)
```

**Nguyên tắc cốt lõi:**
- **Root Admin** được cấu hình trong `config.yaml` hoặc biến môi trường. Đây là tài khoản duy nhất có quyền đăng nhập vào Web Console.
- **IAM Users** là các danh tính được Root tạo ra bên trong hệ thống. Chúng **không thể** đăng nhập Web Console — chỉ dùng để gán quyền và cấp Access Key.
- **Access Keys** là cặp khóa (`AccessKeyID` / `SecretKey`) để ứng dụng/SDK/CLI giao tiếp với API S3. Access Key **kế thừa toàn bộ** quyền từ IAM User cha, không có Policy riêng.
- **IAM Policies** là các tài liệu JSON định nghĩa quyền hạn, có thể gắn cho nhiều User.

## 1. Quản lý Người dùng (IAM Users)
IAM User là thực thể chính mang danh tính trong hệ thống. Tất cả quyền hạn được gắn ở cấp User.

1. Truy cập mục **IAM Users** ở Menu bên trái.
2. Bấm **Create User**.
3. Nhập Tên đăng nhập và Mật khẩu.
4. Sau khi tạo xong, bấm **Attach Policies** trên dòng User đó để gán quyền.
5. Bấm **Create Key** trên dòng User để tạo Access Key cho User đó.

> **Lưu ý:** IAM Users không thể đăng nhập vào trang quản trị Web Console. Chỉ tài khoản Root mới có quyền này.

## 2. Quản lý Quyền Truy Cập (IAM Policies)
Policy xác định giới hạn những thao tác nào được phép thực hiện trên hệ thống. GoS3 sử dụng định dạng JSON Policy chuẩn AWS IAM.

- Truy cập mục **IAM Policies**.
- Bạn có thể viết Policy riêng dạng JSON.

**Giải đáp: IAM Policy áp dụng cho toàn bộ hay từng bucket?**
Policy có thể áp dụng cho **toàn bộ hệ thống** hoặc **chỉ một bucket cụ thể**, tùy thuộc vào trường `"Resource"` mà bạn khai báo trong file JSON. Nếu dùng `arn:aws:s3:::*`, nó áp dụng toàn bộ. Nếu dùng `arn:aws:s3:::ten-bucket`, nó chỉ áp dụng cho bucket đó.

Dưới đây là các **Template Mẫu (Copy & Paste)** thường dùng nhất:

### Template 1: Toàn quyền (Full Access - Quản trị viên)
Cấp quyền đọc, ghi, và xóa trên **tất cả** các bucket.
```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": ["s3:*"],
      "Resource": ["arn:aws:s3:::*"]
    }
  ]
}
```

### Template 2: Chỉ đọc (Read-Only) cho MỘT Bucket cụ thể
Thích hợp cho ứng dụng Frontend hoặc chia sẻ public. Thay `my-bucket` bằng tên bucket của bạn.
```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "s3:GetObject",
        "s3:ListBucket"
      ],
      "Resource": [
        "arn:aws:s3:::my-bucket",
        "arn:aws:s3:::my-bucket/*"
      ]
    }
  ]
}
```

### Template 3: Chỉ cho phép Upload (Write-Only) vào MỘT Bucket
Thích hợp cho tính năng User Upload, backup log, không cho phép đọc hay xóa dữ liệu cũ.
```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "s3:PutObject"
      ],
      "Resource": [
        "arn:aws:s3:::my-bucket/*"
      ]
    }
  ]
}
```

### Template 4: Quyền Quản lý toàn diện trên MỘT Bucket
Cho phép ứng dụng backend làm mọi thứ (tạo file, xoá file) nhưng **chỉ trong giới hạn** 1 bucket duy nhất.
```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": ["s3:*"],
      "Resource": [
        "arn:aws:s3:::my-bucket",
        "arn:aws:s3:::my-bucket/*"
      ]
    }
  ]
}
```

## 3. Quản lý Access Keys (Service Accounts)
Access Key là cặp khóa dành cho các ứng dụng (App), Backend, SDK, hoặc CLI kết nối đến GoS3 thông qua API thay vì thông qua Web Console.

**Cách tạo Access Key:**

1. **Từ bảng Users (Nhanh nhất):** Bấm nút **Create Key** trên dòng User mà bạn muốn cấp key.
2. **Từ tab Access Keys:** Bấm **Create Access Key**, chọn **Target User** (bắt buộc), rồi bấm Generate.
3. Hệ thống sẽ sinh ngẫu nhiên `AccessKeyID` và `SecretKey`.
4. **LƯU Ý QUAN TRỌNG:** Copy lại `SecretKey` ngay lập tức, vì hệ thống sẽ không hiển thị lại lần 2 vì lý do bảo mật.

**Về quyền hạn của Access Key:**
Access Key **tự động kế thừa toàn bộ** quyền (Policies) từ IAM User cha. Không cần (và không thể) gán Policy riêng cho Access Key.

Ví dụ: Nếu User `frontend-app` được gắn Policy `ReadOnlyAssets`, thì tất cả Access Keys tạo ra cho `frontend-app` đều chỉ có quyền đọc. Muốn quyền khác → tạo IAM User khác.

## 4. Nhật Ký Truy Cập (Audit Logs)
Để kiểm tra ai đã làm gì trên hệ thống, giúp dễ dàng rà soát bảo mật:
- Truy cập mục **Audit Logs**.
- Tại đây sẽ ghi lại chi tiết: User/Access Key nào, thực hiện hành động gì (vd: `s3:PutObject`), lúc nào và từ địa chỉ IP nào.
