#!/usr/bin/env python3

import boto3
from botocore.config import Config

# Configure boto3 to use the local endpoint
s3_client = boto3.client(
    "s3",
    endpoint_url="http://localhost:8080",
    # Use a dummy region and credentials for local testing
    region_name="us-east-1",
    aws_access_key_id="test_keyid",
    aws_secret_access_key="test_secret",
)

# Parameters for the GetObject request
bucket_name = "test-bucket"
object_key = "test-object.txt"

try:
    # Execute the GetObject request
    response = s3_client.get_object(Bucket=bucket_name, Key=object_key)

    # Read and print the object content
    object_content = response["Body"].read().decode("utf-8")
    print(f"Successfully retrieved object: {object_key}")
    print(f"Content: {object_content}")

    # Print response metadata to inspect headers
    print("\nResponse Metadata:")
    for key, value in response["ResponseMetadata"].items():
        print(f"{key}: {value}")

except Exception as e:
    print(f"Error occurred: {e}")
