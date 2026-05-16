# Quản lý Danh Tính & Bảo Mật (IAM & Security)

Để đảm bảo an toàn cho hệ thống, **không bao giờ sử dụng tài khoản Root (`minioadmin`) cho các ứng dụng thực tế**. Thay vào đó, hãy sử dụng tính năng quản lý danh tính (IAM) của GoS3.

## 1. Quản lý Người dùng (Users)
Người dùng (User) có thể sử dụng Web Console để trực tiếp quản trị dữ liệu.

1. Truy cập mục **IAM Users** ở Menu bên trái.
2. Bấm **Create User**.
3. Nhập Tên đăng nhập và Mật khẩu. Bạn có thể cấp sẵn các quyền (Policy) cơ bản cho User ngay lúc này.
4. User này giờ đây có thể dùng thông tin vừa tạo để đăng nhập vào Web Console.

## 2. Quản lý Quyền Truy Cập (IAM Policies)
Policy xác định giới hạn những thao tác nào được phép thực hiện trên hệ thống. GoS3 sử dụng định dạng JSON Policy chuẩn AWS IAM.

- Truy cập mục **IAM Policies**.
- Bạn có thể viết Policy riêng dạng JSON.

**Ví dụ:** Cấp quyền chỉ-đọc (Read-Only) cho một bucket tên `my-bucket`:
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

## 3. Quản lý Service Accounts (Access / Secret Keys)
Service Account là tài khoản dành cho các ứng dụng (App), Backend, SDK, hoặc CLI kết nối đến GoS3 thông qua API thay vì thông qua Web Console.

1. Truy cập mục **Access Keys** (Service Accounts).
2. Bấm **Create Access Key**.
3. Hệ thống sẽ sinh ngẫu nhiên `AccessKey` và `SecretKey`. 
4. **LƯU Ý QUAN TRỌNG:** Copy lại `SecretKey` ngay lập tức, vì hệ thống sẽ không hiển thị lại lần 2 vì lý do bảo mật.
5. Bạn có thể gán Policy cho Service Account này để giới hạn phạm vi truy cập (Ví dụ: Ứng dụng A chỉ được quyền ghi vào Bucket A).

## 4. Nhật Ký Truy Cập (Audit Logs)
Để kiểm tra ai đã làm gì trên hệ thống, giúp dễ dàng rà soát bảo mật:
- Truy cập mục **Audit Logs**.
- Tại đây sẽ ghi lại chi tiết: User/Access Key nào, thực hiện hành động gì (vd: `s3:PutObject`), lúc nào và từ địa chỉ IP nào.
