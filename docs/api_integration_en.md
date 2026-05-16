# Application & API Integration (CLI & SDK)

GoS3 is fully compatible with the AWS S3 standard. Any toolkit or programming library (CLI, NodeJS, Python, Go, PHP) that supports S3 can seamlessly communicate with GoS3.

> **Note:** You must create an **Access Key** and **Secret Key** (via the Service Accounts tab in the Web Console) to use the API.

*Read in [English](api_integration_en.md) | Đọc bằng [Tiếng Việt](api_integration_vi.md)*

---

## 1. Using AWS CLI

This is the fastest way to manage data via the Command Line. You just need to install `aws-cli` on your machine.

### A. Initial Configuration
Run the following command and enter the credentials provided from your Service Account:
```bash
aws configure --profile gos3
```
- **AWS Access Key ID:** `<Your Service Account Access Key>`
- **AWS Secret Access Key:** `<Your Service Account Secret Key>`
- **Default region name:** `us-east-1` (or your configured region)

### B. Execute S3 Commands
All commands require the `--endpoint-url` parameter pointing to the GoS3 server (by default, the S3 API runs on port `9000`):

```bash
# Upload 1 image file to the bucket
aws --profile gos3 --endpoint-url http://localhost:9000 s3 cp image.jpg s3://my-bucket/

# Delete the entire bucket (and its contents)
aws --profile gos3 --endpoint-url http://localhost:9000 s3 rb s3://my-bucket --force
```

---

## 2. NodeJS Integration (AWS SDK v3)

Here is a typical example of how to upload a file from a Backend (NodeJS) to the GoS3 server.

### Installation
```bash
npm install @aws-sdk/client-s3
```

### Sample Source Code (File Upload)
```javascript
const { S3Client, PutObjectCommand } = require("@aws-sdk/client-s3");
const fs = require("fs");

// Configure the Client to connect to GoS3 instead of AWS
const s3 = new S3Client({
  endpoint: "http://localhost:9000",
  region: "us-east-1",
  credentials: {
    accessKeyId: "YOUR_ACCESS_KEY",
    secretAccessKey: "YOUR_SECRET_KEY"
  },
  forcePathStyle: true // MANDATORY: Enable this option when using a self-hosted server like GoS3
});

async function upload() {
  const fileStream = fs.createReadStream("image.png");
  
  await s3.send(new PutObjectCommand({
    Bucket: "my-bucket",
    Key: "uploads/image.png",
    Body: fileStream,
  }));
  
  console.log("Successfully uploaded to GoS3!");
}

upload();
```
*(If you are using other programming languages like Python Boto3 or PHP AWS SDK, always remember to similarly enable the `forcePathStyle: true` configuration).*
