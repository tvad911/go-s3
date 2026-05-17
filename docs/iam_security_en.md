# Identity & Security Management (IAM & Security)

To ensure system security, **never use the Root account (`minioadmin`) for production applications**. Instead, utilize GoS3's Identity and Access Management (IAM) features.

*Read in [English](iam_security_en.md) | Đọc bằng [Tiếng Việt](iam_security_vi.md)*

## Permission Architecture

GoS3 implements a permission model based on AWS IAM standards:

```
Root Admin (config.yaml / ENV)
  └── Logs into Web Console (port 9001)
        ├── Creates IAM Users
        │     ├── Attaches IAM Policies (N policies / user)
        │     └── Creates Access Keys (N keys / user)
        │           └── Inherits 100% permissions from parent User
        └── Creates IAM Policies (JSON documents)
```

**Core principles:**
- **Root Admin** is configured in `config.yaml` or environment variables. This is the only account that can log in to the Web Console.
- **IAM Users** are identities created by Root within the system. They **cannot** log in to the Web Console — they exist only to carry policies and own Access Keys.
- **Access Keys** are credential pairs (`AccessKeyID` / `SecretKey`) for applications/SDKs/CLIs to communicate with the S3 API. Access Keys **inherit all** permissions from their parent IAM User and have no policies of their own.
- **IAM Policies** are JSON documents defining permissions, reusable across multiple Users.

## 1. User Management (IAM Users)
IAM Users are the core identity entities in the system. All permissions are attached at the User level.

1. Navigate to **IAM Users** on the left menu.
2. Fill out the "Create User" form directly.
3. Enter a Username, Password, and select one or more **IAM Policies** from the dropdown to assign permissions immediately.
4. After creation, you can manage permissions via the **Attach Policies** button.
5. Click **Create Key** on the User row to generate an Access Key for that User.

> **Note:** IAM Users cannot log in to the Web Console. Only the Root account has this privilege.

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

## 3. Access Keys (Service Accounts)
Access Keys are credential pairs for Applications, Backends, SDKs, or CLI tools connecting to GoS3 via APIs instead of the Web Console.

**How to create an Access Key:**

1. **From the Users table (Fastest):** Click the **Create Key** button on the User row you want to provision.
2. **From the Access Keys tab:** Click **Create Access Key**, select a **Target User** (required). You can also optionally specify an **Expires In** duration (e.g. 7 days, 30 days, 1 year).
3. The system will randomly generate an `AccessKeyID` and `SecretKey`.
4. **IMPORTANT:** Copy the `SecretKey` immediately. For security reasons, the system will not display it a second time.
5. **Revocation:** You can click the **Disable/Enable** button on the Access Keys table to temporarily revoke a key without deleting it.

**About Access Key permissions:**
Access Keys **automatically inherit all** permissions (Policies) from their parent IAM User. You cannot (and do not need to) assign policies directly to an Access Key.

Example: If User `frontend-app` has the `ReadOnlyAssets` policy, then all Access Keys created for `frontend-app` will only have read permissions. For different permissions → create a different IAM User.

## 4. Audit Logs
To track who did what on the system and easily review security:
- Navigate to **Audit Logs**.
- Here you'll find detailed records: Which User/Access Key performed what action (e.g., `s3:PutObject`), when, and from what IP address.
