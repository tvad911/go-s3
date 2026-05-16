# GoS3 Comprehensive User Guide

Welcome to GoS3! This guide will help you leverage the full power of our S3-compatible storage system, equipped with an intuitive Web Administration Console.

*Read in [English](USER_GUIDE_en.md) | Đọc bằng [Tiếng Việt](USER_GUIDE_vi.md)*

## Quick Start

The GoS3 administration interface runs independently of the S3 API:
- **Web Console:** Runs on port `9001` (e.g., `http://localhost:9001`)
- **S3 API for SDK/CLI:** Runs on port `9000` (e.g., `http://localhost:9000`)

### First-time Login:
1. Open your browser and go to: `http://<server-IP>:9001`
2. Log in using the Root admin credentials (or custom credentials if you changed them in Docker/ENV):
   - **Username:** `minioadmin`
   - **Password:** `minioadmin`

Once logged in, the system displays the **Dashboard** monitoring vital metrics (storage capacity, number of objects, active buckets).

---

## Features & Documentation Index

The documentation has been divided into detailed sections for easy reference. Please click the links below to view specific guides:

### 1. [Setup & Deployment (Docker)](./setup_deployment_en.md)
Quickly initialize the system:
- System requirements.
- Deploy via Docker & Docker Compose.
- Yaml file configurations and Production security notes.

### 2. [Identity & Security Management (IAM)](./iam_security_en.md)
Secure access control system:
- Manage **Users** for the Web Console.
- Create **Access Keys (Service Accounts)** for software integration.
- Precisely control permissions using standard JSON **IAM Policies**.
- Track system history via **Audit Logs**.

### 2. [Data & Bucket Management](./bucket_management_en.md)
Direct data operations:
- **Object Browser** interface: Upload, create folders, securely share files (Presigned URLs).
- Advanced Bucket configurations:
  - **Static Web Hosting** and **Custom Domains**.
  - Setup automated cleanup rules (**Lifecycle**).
  - Cross-origin integration (**CORS**) and event notifications (**Webhooks**).

### 3. [Application & API Integration (CLI & SDK)](./api_integration_en.md)
GoS3 is 100% compatible with AWS S3 standard:
- How to connect and execute commands with **AWS CLI**.
- Quick integration with your software using **AWS SDK (e.g., NodeJS)**.

---

> **Tip:** If you only want to manually manage files for personal use, the **Web Console** is all you need. Grant Service Account permissions only if you intend to use GoS3 as backend storage for other applications.
