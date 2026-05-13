#!/bin/bash
set -e

./gos3 &
PID=$!
sleep 2

export AWS_ACCESS_KEY_ID="minioadmin"
export AWS_SECRET_ACCESS_KEY="minioadmin"
export AWS_REGION="us-east-1"
ENDPOINT="http://localhost:9099"

echo "1. Creating bucket with Object Lock enabled..."
aws s3api create-bucket --bucket lock-bucket --object-lock-enabled-for-bucket --endpoint-url $ENDPOINT

echo "2. Putting object with GOVERNANCE mode..."
FUTURE_DATE=$(date -u -d "+1 day" +"%Y-%m-%dT%H:%M:%SZ")
aws s3api put-object --bucket lock-bucket --key locked.txt --body test.txt \
  --object-lock-mode GOVERNANCE \
  --object-lock-retain-until-date "$FUTURE_DATE" \
  --endpoint-url $ENDPOINT > put_out.json

VERSION_ID=$(cat put_out.json | grep VersionId | awk -F '"' '{print $4}')
echo "Version ID: $VERSION_ID"

echo "3. Heading object to verify headers..."
aws s3api head-object --bucket lock-bucket --key locked.txt --version-id "$VERSION_ID" --endpoint-url $ENDPOINT

echo "4. Attempting to delete locked version..."
if aws s3api delete-object --bucket lock-bucket --key locked.txt --version-id "$VERSION_ID" --endpoint-url $ENDPOINT 2>&1 | grep "AccessDenied"; then
    echo "SUCCESS: Delete was denied by Object Lock!"
else
    echo "ERROR: Delete was NOT denied!"
    exit 1
fi

kill $PID
