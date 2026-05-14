# Phase 7: Advanced Features (SSE-C & Notifications)

## Objective
Implement Phase 7.4 (SSE-C) and Phase 7.5 (Event Notifications) from the main `plan.md`.

## Features
1. **SSE-C (Server-Side Encryption with Customer Key)**
   - Parse `x-amz-server-side-encryption-customer-algorithm`, `x-amz-server-side-encryption-customer-key`, `x-amz-server-side-encryption-customer-key-md5` in `PutObject` and `GetObject`.
   - Encrypt/Decrypt data streams using AES-256-CTR with the provided key.
   - Do not store the customer key, only metadata indicating SSE-C is used.

2. **Event Notifications**
   - Webhook configurations per bucket (`PutBucketNotification`, `GetBucketNotification`).
   - Emit `s3:ObjectCreated:*` and `s3:ObjectRemoved:*` events.
   - Async delivery with retry.

## Technical Tasks
- Update `s3.ObjectMeta` to include SSE-C flags.
- Create `internal/crypto/ssec.go` for AES-256 streaming.
- Create `internal/event/webhook.go` for notification dispatch.
- Update `object.go` handlers to intercept and wrap the `io.Reader` for encryption.
