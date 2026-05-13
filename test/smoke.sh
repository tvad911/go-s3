#!/bin/bash
set -e

# Configuration
ENDPOINT="http://localhost:9099"
BUCKET="test-bucket-$(date +%s)"
FILE="test-file.txt"
DOWNLOADED_FILE="downloaded-file.txt"

# Ensure AWS CLI is configured with correct credentials
# We use the default root credentials from config.example.yaml
export AWS_ACCESS_KEY_ID="minioadmin"
export AWS_SECRET_ACCESS_KEY="minioadmin"
export AWS_DEFAULT_REGION="us-east-1"

echo "======================================"
echo "GoS3 Smoke Test"
echo "Endpoint: $ENDPOINT"
echo "Bucket: $BUCKET"
echo "======================================"

# Create a test file
echo "Hello, GoS3!" > $FILE

# 1. Create Bucket
echo "[1] Creating bucket..."
aws --endpoint-url $ENDPOINT s3 mb s3://$BUCKET

# 2. List Buckets
echo "[2] Listing buckets..."
aws --endpoint-url $ENDPOINT s3 ls

# 3. Upload Object
echo "[3] Uploading object..."
aws --endpoint-url $ENDPOINT s3 cp $FILE s3://$BUCKET/$FILE

# 4. List Objects in Bucket
echo "[4] Listing objects in bucket..."
aws --endpoint-url $ENDPOINT s3 ls s3://$BUCKET

# 5. Download Object
echo "[5] Downloading object..."
aws --endpoint-url $ENDPOINT s3 cp s3://$BUCKET/$FILE $DOWNLOADED_FILE
cat $DOWNLOADED_FILE

# 6. Delete Object
echo "[6] Deleting object..."
aws --endpoint-url $ENDPOINT s3 rm s3://$BUCKET/$FILE

# 7. Delete Bucket
echo "[7] Deleting bucket..."
aws --endpoint-url $ENDPOINT s3 rb s3://$BUCKET

# Cleanup
rm -f $FILE $DOWNLOADED_FILE

echo "======================================"
echo "Smoke Test Completed Successfully!"
echo "======================================"
