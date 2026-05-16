# Data & Bucket Configuration (Bucket Management)

This document explains how to manage data (Files/Folders) and utilize advanced features for each Bucket in GoS3.

*Read in [English](bucket_management_en.md) | Đọc bằng [Tiếng Việt](bucket_management_vi.md)*

---

## 1. Basic Operations (Object Browser)

- Navigate to **Buckets** in the menu. Click **Create Bucket** to create a new storage space (Note: Bucket names must be lowercase, numbers, hyphens, and contain no spaces).
- Click on the newly created Bucket name to open the **Object Browser** interface:
  - **Upload Files:** Drag and drop files/folders or click the **Upload** button (supports large file uploads via chunking).
  - **Create Folder:** Click **New Folder** to organize your data visually.
  - **Share Files:** Click the 🔗 icon on a file to generate a Presigned URL. This link allows secure file sharing for a fixed duration (e.g., 1 hour) even if the Bucket is Private.
  - **Rename & Delete Multiple Files:** Use the checkboxes for bulk deletion or click the pencil icon to rename a file.

---

## 2. Advanced Features (Bucket Settings)

Each Bucket has advanced configurations. You can access them by clicking the ⚙️ (Settings) icon on the Bucket card or selecting the **Settings** tab from the Object Browser.

### A. Static Website Hosting & Custom Domains
This feature allows you to host a static website (HTML/CSS/JS) or set up a direct image CDN from the Bucket.

1. Switch to the **Website Hosting** tab and enable the feature. Enter the default document name (e.g., `index.html`).
2. Switch to the **Custom Domains** tab and add your domain (e.g., `cdn.mycompany.com`).
3. Point your DNS (CNAME record) for `cdn.mycompany.com` to your GoS3 server address. You can now access your static content via your custom domain.

### B. Lifecycle Management
Lifecycle automatically cleans up old files to save disk space (e.g., delete log files after 30 days).

1. Switch to the **Lifecycle** tab.
2. Enter the JSON configuration to set an auto-delete rule (`Expiration`) for a specific `Prefix`.

### C. CORS (Cross-Origin Resource Sharing)
If you have another website (e.g., `https://your-domain.com`) and want browsers to allow users to upload files directly from the browser to GoS3:
- Switch to the **CORS** tab.
- Set `AllowedOrigins` to your domain and grant permissions in `AllowedMethods` (e.g., GET, PUT).

### D. Webhooks
Want to notify your Backend server whenever a user uploads an image to GoS3?
- Switch to the **Webhooks** tab.
- Register your backend URL and the event to listen for (e.g., `s3:ObjectCreated:*`). GoS3 will automatically send an HTTP POST request to that URL when the event occurs.
