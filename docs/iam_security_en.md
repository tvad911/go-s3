# Identity & Security Management (IAM & Security)

To ensure system security, **never use the Root account (`minioadmin`) for production applications**. Instead, utilize GoS3's Identity and Access Management (IAM) features.

*Read in [English](iam_security_en.md) | Đọc bằng [Tiếng Việt](iam_security_vi.md)*

## 1. User Management
Users can utilize the Web Console to directly manage data.

1. Navigate to **IAM Users** on the left menu.
2. Click **Create User**.
3. Enter a Username and Password. You can grant basic Policies to the User immediately.
4. This User can now use these credentials to log in to the Web Console.

## 2. Access Management (IAM Policies)
Policies define exactly which actions are permitted on the system. GoS3 uses the standard AWS IAM JSON Policy format.

- Navigate to **IAM Policies**.
- You can write custom policies in JSON format.

**Q: Does an IAM Policy apply globally or per bucket?**
Policies can apply **globally to all resources** or be scoped to **a specific bucket**, depending on the `"Resource"` field in your JSON. Using `arn:aws:s3:::*` applies globally. Using `arn:aws:s3:::bucket-name` scopes it to that specific bucket.

Below are the most common **Ready-to-Use Templates (Copy & Paste)**:

### Template 1: Full Access (Administrator)
Grants read, write, and delete permissions across **all** buckets.
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

### Template 2: Read-Only for ONE Specific Bucket
Ideal for Frontend applications or public sharing. Replace `my-bucket` with your actual bucket name.
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

### Template 3: Upload-Only (Write-Only) to ONE Bucket
Ideal for User Upload features or log backups. It strictly prevents reading or deleting existing files.
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

### Template 4: Full Management of ONE Bucket
Allows a backend application to do anything (create, list, delete), but **strictly limited** to a single bucket.
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

## 3. Service Accounts (Access / Secret Keys)
Service Accounts are intended for Applications, Backends, SDKs, or CLI tools connecting to GoS3 via APIs instead of the Web Console.

1. Navigate to **Access Keys** (Service Accounts).
2. Click **Create Access Key**.
3. The system will randomly generate an `AccessKey` and `SecretKey`.
4. **IMPORTANT NOTE:** Copy the `SecretKey` immediately. For security reasons, the system will not display it a second time.
5. You can assign a Policy to this Service Account to limit its scope (e.g., Application A can only write to Bucket A).

## 4. Audit Logs
To track who did what on the system and easily review security:
- Navigate to **Audit Logs**.
- Here you'll find detailed records: Which User/Access Key performed what action (e.g., `s3:PutObject`), when, and from what IP address.
