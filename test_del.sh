#!/bin/bash
set -e
COOKIE=$(curl -s -X POST http://localhost:9011/api/v1/login -H 'Content-Type: application/json' -d '{"username":"root","password":"minioadmin"}' -c - | grep gos3_token | awk '{print $7}')
DEL_URL=$(curl -s -X POST http://localhost:9011/_admin/presign -H "Content-Type: application/json" -H "Cookie: gos3_token=$COOKIE" -d '{"method":"DELETE","bucket":"mybucket","key":"test.txt","expires":3600}' | jq -r .url)
curl -v -X DELETE "$DEL_URL"
