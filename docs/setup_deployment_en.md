# Setup & Deployment Guide

This document guides you on how to deploy GoS3 to your server/VPS using Docker quickly and securely.

*Read in [English](setup_deployment_en.md) | Đọc bằng [Tiếng Việt](setup_deployment_vi.md)*

---

## 1. System Requirements
- Server running Linux OS (Ubuntu/Debian/CentOS).
- **Docker** and **Docker Compose** installed.
- Ensure ports `9000` (S3 API) and `9001` (Web Console) are open on your Firewall.

## 2. Deployment with Docker Compose (Recommended)

### Step 1: Create configuration file
Create a new directory on your server (e.g., `/opt/gos3`) and create a `docker-compose.yml` file inside it:

```yaml
services:
  gos3:
    image: tvad9111/gos3:latest # Pulls the official image directly from Docker Hub
    ports:
      - "9000:9000"   # Port for S3 API
      - "9001:9001"   # Port for Web Console
    volumes:
      - gos3-data:/data
    environment:
      # Set Default Root credentials (MANDATORY TO CHANGE IN PRODUCTION)
      GOS3_AUTH_ROOT_ACCESS_KEY: admin
      GOS3_AUTH_ROOT_SECRET_KEY: SuperSecretPassword123
      
      # Directory Configurations (Inside the container)
      GOS3_STORAGE_DATA_DIR: /data
      GOS3_STORAGE_TEMP_DIR: /data/tmp
      GOS3_STORAGE_META_DB: /data/meta.db
      
      # Optional Rate Limit configurations
      GOS3_RATELIMIT_ENABLED: "true"
      GOS3_RATELIMIT_RPS: 1000
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "wget", "-qO-", "http://localhost:9000/_health"]
      interval: 30s
      timeout: 5s
      retries: 3

volumes:
  gos3-data:
```

### Step 2: Start the Server
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

## 3. Detailed Configuration (config.yaml)

Instead of using Environment Variables, you can use a `config.yaml` file for more flexible configuration.
Mount the `config.yaml` file into the container by adding this line to the `volumes` section in `docker-compose.yml`:
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
