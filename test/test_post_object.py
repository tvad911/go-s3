import boto3
import requests
import sys

# Configure boto3 to use local GoS3 server
s3 = boto3.client(
    's3',
    endpoint_url='http://localhost:9099',
    aws_access_key_id='minioadmin',
    aws_secret_access_key='minioadmin',
    region_name='us-east-1'
)

bucket_name = 'test-post-bucket'
object_name = 'hello.txt'

# 1. Create bucket (using standard API)
try:
    s3.create_bucket(Bucket=bucket_name)
    print(f"Bucket {bucket_name} created.")
except Exception as e:
    print(f"Bucket might already exist: {e}")

# 2. Generate Presigned POST Policy
post_data = s3.generate_presigned_post(
    Bucket=bucket_name,
    Key=object_name,
    ExpiresIn=3600
)

# 3. Use requests to upload a file via multipart form-data
print("Generated POST data:", post_data)
files = {'file': ('hello.txt', b'Hello from POST upload!')}

print(f"Sending POST request to {post_data['url']} ...")
response = requests.post(post_data['url'], data=post_data['fields'], files=files)

print("Status Code:", response.status_code)
print("Response Body:", response.text)

if response.status_code in [200, 204]:
    print("POST Upload Successful!")
else:
    print("POST Upload Failed!")
    sys.exit(1)

# Verify by downloading
resp = s3.get_object(Bucket=bucket_name, Key=object_name)
content = resp['Body'].read()
print("Downloaded content:", content)
if content == b'Hello from POST upload!':
    print("Content verified!")
else:
    print("Content mismatch!")

# Cleanup
s3.delete_object(Bucket=bucket_name, Key=object_name)
s3.delete_bucket(Bucket=bucket_name)
