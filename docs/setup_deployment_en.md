# Setup & Deployment Guide

This document guides you on how to deploy GoS3 to your server/VPS using Docker quickly and securely.

*Read in [English](setup_deployment_en.md) | Đọc bằng [Tiếng Việt](setup_deployment_vi.md)*

---

## 1. System Requirements
- Server running Linux OS (Ubuntu/Debian/CentOS).
- **Docker** and **Docker Compose** installed.
- Ensure ports `9000` (S3 API) and `9001` (Web Console) are open on your Firewall.

## 2. Deployment with Docker Compose (Recommended)

You have two options to deploy the system depending on your needs:

### Option A: Use Pre-built Image (Fastest)
Create a new directory (e.g., `/opt/gos3`) and create a `docker-compose.yml` file:

```yaml
services:
  gos3:
    image: tvad9111/gos3:latest # Pulls directly from Docker Hub
    ports:
      - "9000:9000"   # Port for S3 API
      - "9001:9001"   # Port for Web Console
    volumes:
      - gos3-data:/data
    environment:
      # Set Default Root credentials (MANDATORY TO CHANGE IN PRODUCTION)
      GOS3_AUTH_ROOT_ACCESS_KEY: admin
      GOS3_AUTH_ROOT_SECRET_KEY: SuperSecretPassword123
      
      # Directory Configurations
      GOS3_STORAGE_DATA_DIR: /data
      GOS3_STORAGE_TEMP_DIR: /data/tmp
      GOS3_STORAGE_META_DB: /data/meta.db
    restart: unless-stopped

volumes:
  gos3-data:
```

### Option B: Build from Source (For Latest Updates)
If the Docker Image on Docker Hub is not yet updated, or if you wish to modify the code locally:
1. Clone the repository: `git clone https://github.com/tvad911/go-s3.git`
2. Open the file `deploy/docker-compose.yml`. You can uncomment the build block to compile it directly:

```yaml
services:
  gos3:
    # Use the local build context instead of the image line:
    build: 
      context: ..
      dockerfile: deploy/Dockerfile
    # ... keep the rest of the configuration
```

### Start the Server
Run the following command in the directory containing `docker-compose.yml`:
```bash
docker-compose up -d
```

To check the operational logs:
```bash
docker-compose logs -f
```

### Step 3: Login
Open your browser, navigate to `http://<Server-IP>:9001` and log in using the `GOS3_AUTH_ROOT_ACCESS_KEY` and `GOS3_AUTH_ROOT_SECRET_KEY` configured in Step 1.

---

## 3. Configuration Methods: ENV vs config.yaml

GoS3 provides two ways to configure your server: **Environment Variables (ENV)** and a **`config.yaml`** file.

### Method 1: Using Environment Variables (Recommended for Docker)
By default, the `docker-compose.yml` template above relies purely on environment variables under the `environment:` section. This is the standard cloud-native approach.
Key variables you can set:
- `GOS3_AUTH_ROOT_ACCESS_KEY` & `GOS3_AUTH_ROOT_SECRET_KEY`: Set your root admin credentials.
- `GOS3_SERVER_PORT`: Change the default S3 API port (9000).
- `GOS3_RATELIMIT_ENABLED` & `GOS3_RATELIMIT_RPS`: Control rate limiting to prevent abuse.

*When using this method, you do not need to mount any configuration file.*

### Method 2: Using a `config.yaml` file (For Advanced Configurations)
Instead of using Environment Variables, you can use a `config.yaml` file for more complex configurations (like multi-node replication or detailed TLS settings).

1. Create a `config.yaml` file in the same directory as your `docker-compose.yml` (You can copy the contents from `deploy/config.example.yaml` in the source code).
2. Mount the `config.yaml` file into the container by adding this line to the `volumes` section in your `docker-compose.yml`:

```yaml
    volumes:
      - gos3-data:/data
      - ./config.yaml:/app/config.yaml:ro
```

**Sample `config.yaml` file:**
```yaml
server:
  port: 9000
  base_domain: "s3.mycompany.com" # Used for Virtual-host style routing (bucket.s3.mycompany.com)
  tls_cert: "" # Configure SSL/TLS here if not using a Reverse Proxy
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

## 4. Production Security Guidelines

1. **Change Root Password:** NEVER keep the default `minioadmin` password. Change it immediately via environment variables or `config.yaml`.
2. **Reverse Proxy (Nginx/Traefik):** It is recommended to place GoS3 behind an Nginx Reverse Proxy to handle SSL/TLS (Let's Encrypt) termination securely.
3. **Firewall:** If you only use GoS3 as private Backend Storage, block port `9000` from the public internet and only allow requests from your Backend server IPs. Restrict port `9001` (Admin) to the IPs of system administrators.
