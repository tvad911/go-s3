#!/bin/bash
set -e
# Create bucket
curl -s -X PUT http://localhost:9010/mybucket -H "Authorization: AWS4-HMAC-SHA256 Credential=minioadmin/20260514/us-east-1/s3/aws4_request, SignedHeaders=host, Signature=fake" || true

# We need a valid signature. I will just use the admin API to create the bucket.
COOKIE=$(curl -s -X POST http://localhost:9011/api/v1/login -H 'Content-Type: application/json' -d '{"username":"root","password":"minioadmin"}' -c - | grep gos3_token | awk '{print $7}')

curl -s -X PUT http://localhost:9011/_admin/buckets/mybucket -H "Cookie: gos3_token=$COOKIE"

# Presign PUT object
PUT_URL=$(curl -s -X POST http://localhost:9011/_admin/presign -H "Content-Type: application/json" -H "Cookie: gos3_token=$COOKIE" -d '{"method":"PUT","bucket":"mybucket","key":"test.txt","expires":3600}' | jq -r .url)
# Upload file
curl -s -X PUT "$PUT_URL" -d 'hello world'

# Rename file: Presign PUT target
TARGET_URL=$(curl -s -X POST http://localhost:9011/_admin/presign -H "Content-Type: application/json" -H "Cookie: gos3_token=$COOKIE" -d '{"method":"PUT","bucket":"mybucket","key":"test2.txt","expires":3600}' | jq -r .url)

# Execute copy
curl -v -X PUT "$TARGET_URL" -H "x-amz-copy-source: /mybucket/test.txt"

