# GoS3 Comprehensive User Guide

This document provides a comprehensive guide from basic to advanced features to help you leverage the full power of **GoS3** – an S3-compatible object storage system with a built-in Web Console.

---

## 1. Accessing the Web Console

By default, the GoS3 Web Console is served on port `9001` (the S3 API runs on port `9000`).

1. Open your browser and navigate to: `http://<server-ip>:9001`
2. **First Login:**
   - **Username:** `minioadmin` (or the value of `GOS3_AUTH_ROOT_ACCESS_KEY`)
   - **Password:** `minioadmin` (or the value of `GOS3_AUTH_ROOT_SECRET_KEY`)
   *(Note: You should change these default credentials in the `docker-compose.yml` file before deploying to production).*

Once logged in, you will be directed to the **Dashboard**, which provides a high-level overview of system storage, number of buckets, active users, and server uptime.

---

## 2. Identity and Access Management (IAM & Security)

To enhance security, never use the Root account (`minioadmin`) directly within applications or share it with others. Instead, use Users and Service Accounts.

### A. Managing Users
- Navigate to the **Users** tab on the left sidebar.
- Click **Create User** to add a new account. These users can log into the Web Console.
- Enter a username and password. You can also assign pre-defined IAM Policies to this user.

### B. IAM Policies
- Navigate to the **Policies** tab.
- GoS3 uses standard AWS IAM JSON formatting for policies.
- Example Policy granting Read-Only access to a specific bucket:
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
- Navigate to the **Service Accounts** tab.
- Service Accounts are used to generate Access Keys and Secret Keys for applications, SDKs, or CLI tools.
- Upon creation, the system randomly generates the keys. **Important: Click the eye icon to view and copy the Secret Key, as it cannot be viewed again once you leave the page.**
- You can attach specific IAM Policies to each Service Account to restrict its permissions.

---

## 3. Data Management (Object Browser)

- Go to the **Buckets** menu. Click **Create Bucket** to create a new storage space (bucket names must be lowercase without spaces, matching AWS S3 standards).
- Click on a bucket name to open the **Object Browser**:
  - **Upload:** Drag and drop files or click the Upload button. Supports large streaming uploads and chunked transfers.
  - **Create Folders:** Click "Create Folder". GoS3 supports logical folder structures based on `/` prefixes.
  - **Share (Presigned URLs):** Click the 🔗 icon next to a file, select an expiration time (e.g., 1 Hour), and generate a secure, temporary download link—even if the bucket is Private.
  - **Bulk Deletion:** Select multiple files and click Delete at the top.

---

## 4. Advanced Features (Bucket Settings)

Inside the Object Browser, switch to the **Settings** tab of the bucket to access advanced configurations:

### 1. Static Website Hosting & Custom Domains
This feature turns your bucket into a Web Server to host static HTML/CSS/JS websites or serve as a CDN.
- **Enable Website Hosting:** 
  - Toggle the switch to "Enabled".
  - Specify the **Index Document** (usually `index.html`).
- **Add a Custom Domain:**
  - Enter your desired domain (e.g., `cdn.mycompany.com`).
  - *External DNS Configuration:* Point the DNS CNAME or A Record of your domain to the GoS3 server.
  - Now, navigating to `http://cdn.mycompany.com` will automatically serve the static `index.html` file from the root of this bucket.

### 2. Lifecycle Management
Use this to automate data cleanup, such as deleting old logs or incomplete multi-part uploads to save storage space.
- Navigate to the **Lifecycle** tab.
- You can configure XML rules to automatically expire/delete objects after N days.
- *Example: Delete files in the `logs/` directory after 30 days:*
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
Trigger external APIs (e.g., Slack, Discord, your custom backend) whenever a storage event occurs.
- Navigate to the **Webhooks** tab.
- Register an endpoint URL (e.g., `https://api.myweb.com/webhook/s3`).
- Select the target Event (e.g., `s3:ObjectCreated:*`).
- GoS3 will send an HTTP POST request with a JSON payload to the configured URL every time the event is triggered.

### 4. CORS (Cross-Origin Resource Sharing)
Required if you want browsers running on `yourdomain.com` to upload files directly to GoS3 via JavaScript without being blocked.
- Navigate to the **CORS** tab to allow origins (e.g., `*` or specific domains) and HTTP methods (GET, PUT, POST).

---

## 5. Integrating via AWS CLI & SDKs

GoS3 is 100% compatible with the AWS S3 standard. You can use any existing S3 libraries.

### A. Using AWS CLI
Install `aws-cli`, then configure it:
```bash
aws configure --profile gos3
# AWS Access Key ID: <Enter Service Account Access Key>
# AWS Secret Access Key: <Enter Service Account Secret Key>
# Default region name: us-east-1
```
Commands:
```bash
# Upload a file
aws --profile gos3 --endpoint-url http://localhost:9000 s3 cp image.jpg s3://my-bucket/

# Delete a bucket forcefully
aws --profile gos3 --endpoint-url http://localhost:9000 s3 rb s3://my-bucket --force
```

### B. Using NodeJS (AWS SDK v3)
```javascript
const { S3Client, PutObjectCommand } = require("@aws-sdk/client-s3");
const fs = require("fs");

const s3 = new S3Client({
  endpoint: "http://localhost:9000",
  region: "us-east-1",
  credentials: {
    accessKeyId: "YOUR_ACCESS_KEY",
    secretAccessKey: "YOUR_SECRET_KEY"
  },
  forcePathStyle: true // CRITICAL: Must be true when using self-hosted S3 endpoints
});

async function upload() {
  const fileStream = fs.createReadStream("image.png");
  await s3.send(new PutObjectCommand({
    Bucket: "my-bucket",
    Key: "uploads/image.png",
    Body: fileStream,
  }));
  console.log("Upload successful!");
}
upload();
```

---

## 6. Audit Logs

To trace administrative and S3 API actions:
- From the Web Console, click on **Audit Logs** in the sidebar.
- You will see a chronological history of who (User, Access Key), did what (e.g., `s3:PutObject`, `admin:CreateUser`), at what time, from which IP address.
- This feature is vital for security monitoring and debugging integration issues.

---
*Happy storing!*
