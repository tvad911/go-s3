# GoS3

GoS3 is a lightweight, S3-compatible object storage server and CLI client written entirely in Go. Designed as a simpler alternative to Garage and MinIO, GoS3 is highly performant, easy to deploy via Docker, and fully compatible with the AWS S3 API (Signature V4).

## 🚀 Features

### Core S3 Compatibility
- **API Compatibility**: Fully supports AWS S3 SDKs, `aws-cli`, and `mc` (MinIO Client).
- **Authentication**: AWS Signature V4 with robust IAM Policy evaluation.
- **Object Operations**: Streaming large objects, AWS Chunked Uploads, Multipart Uploads, ETag hashing.
- **Bucket Features**: Versioning, Object Lock (stub/base), CORS, and Bucket Policies.

### Advanced Features
- **Static Website Hosting**: Serve static websites directly from your buckets.
- **Custom Domains**: Virtual-hosted style domains (e.g., `cdn.yourdomain.com`).
- **Lifecycle Management**: Automated background worker to expire/delete objects based on configurable rules.
- **Audit Logs**: Comprehensive logging of S3 API access and Admin actions.
- **Webhooks**: Event notifications (e.g., `s3:ObjectCreated:*`) sent to external HTTP endpoints.

### Web Administration Console
GoS3 includes a built-in, lightweight (vanilla JS/HTML/CSS) Web UI served on port 9001.
- **Identity Management**: Manage Users and IAM Policies.
- **Service Accounts**: Create and manage Access/Secret keys for applications.
- **Object Browser**: Navigate, preview, search, upload, and generate presigned URLs for your files.
- **Bucket Settings**: Visual interfaces for configuring Lifecycle rules, CORS, Policies, Webhooks, and Custom Domains.
- **Server Telemetry**: Real-time hardware telemetry dashboard (CPU, RAM, Disk) and configuration metrics.

---

## 🏗 Architecture

GoS3 follows a clean, single-binary architecture (monorepo) without heavy external dependencies:
- **HTTP Layer**: `go-chi/chi` for routing and middleware (Rate Limiting, CORS, SigV4 Auth).
- **Storage Layer**: Local filesystem for object data (atomic rename writes, streaming reads).
- **Metadata Database**: Embedded `bbolt` database for fast, transactional metadata storage (buckets, users, policies, multipart states).
- **Frontend**: ES Modules and Vanilla CSS embedded directly into the binary via `//go:embed`.

---

## 🛠 Getting Started

### 1. Run with Docker Compose (Recommended)

GoS3 comes with a production-ready `docker-compose.yml`.

```bash
# Clone the repository
git clone https://github.com/tvad911/go-s3.git
cd go-s3

# Start the server in the background
docker-compose -f deploy/docker-compose.yml up -d
```

- **S3 API Port**: `9000`
- **Web Console Port**: `9001`
- **Default Credentials**: `minioadmin` / `minioadmin` (Configurable in `docker-compose.yml`)

### 2. Build from Source

Ensure you have Go 1.22+ installed.

```bash
# Build the server and client binaries
make build

# The binaries will be generated in the root directory
./gos3
./gos3c
```

---

## 📚 Documentation & User Guides

GoS3 provides comprehensive documentation in both English and Vietnamese. Please refer to the detailed guides below:

- **[Setup & Deployment Guide](./docs/setup_deployment_en.md)** | **[Cài đặt & Triển khai](./docs/setup_deployment_vi.md)**
  Detailed instructions for Docker deployment, configuration via YAML, and Production security best practices.
- **[Identity & Security Management (IAM)](./docs/iam_security_en.md)** | **[Quản lý Danh Tính & Bảo Mật](./docs/iam_security_vi.md)**
  User management, Service Accounts (Access Keys), IAM JSON Policies, and Audit Logs.
- **[Data & Bucket Configuration](./docs/bucket_management_en.md)** | **[Quản lý Dữ liệu & Bucket](./docs/bucket_management_vi.md)**
  Using the Object Browser, Static Web Hosting, Custom Domains, Lifecycle, CORS, and Webhooks.
- **[API Integration (CLI & SDK)](./docs/api_integration_en.md)** | **[Tích hợp API & Ứng dụng](./docs/api_integration_vi.md)**
  How to connect to GoS3 using the AWS CLI and NodeJS AWS SDK v3.

---

## 💻 Usage

### Accessing the Web Console
Open your browser and navigate to `http://localhost:9001`. Login using your Root Credentials (default: `minioadmin` / `minioadmin`).

### Using AWS CLI
You can interact with GoS3 just like Amazon S3 using the AWS CLI.

```bash
# Configure AWS CLI profile
aws configure --profile gos3
# AWS Access Key ID: minioadmin
# AWS Secret Access Key: minioadmin
# Default region name: us-east-1

# Create a bucket
aws --profile gos3 --endpoint-url http://localhost:9000 s3 mb s3://my-bucket

# Upload a file
aws --profile gos3 --endpoint-url http://localhost:9000 s3 cp ./hello.txt s3://my-bucket/

# List files
aws --profile gos3 --endpoint-url http://localhost:9000 s3 ls s3://my-bucket/
```

### Static Website & Custom Domains
1. Go to the Web Console -> **Bucket Settings**.
2. Enable **Static Website Hosting** and set `index.html` as the Index Document.
3. Add a **Custom Domain** (e.g., `cdn.example.com`).
4. Point your DNS (A Record / CNAME) to the GoS3 server.
5. Visitors can now access the website at `http://cdn.example.com` directly.

---

## 🔒 Security & Best Practices

- **Root Credentials**: Always change the default `GOS3_AUTH_ROOT_ACCESS_KEY` and `GOS3_AUTH_ROOT_SECRET_KEY` in your environment variables before deploying to production.
- **Service Accounts**: Do not use the Root Credentials for your applications. Instead, use the Web Console to create dedicated Service Accounts with restrictive IAM Policies.
- **Reverse Proxy**: It is recommended to deploy GoS3 behind a reverse proxy (like Nginx, Traefik, or Caddy) to handle TLS/SSL certificates and HTTPS termination.

---

## 🧪 CI/CD & Testing

The project uses GitHub Actions for continuous integration. The pipeline automatically runs:
- `golangci-lint`
- Unit and integration tests (`go test -race`)
- Docker build checks

To run tests locally:
```bash
make test
```

---

## 📄 License
Apache License 2.0
