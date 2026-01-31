#!/bin/bash
#
# Bootstrap script for Terraform state bucket
# Run this ONCE before terraform init
#
# Usage: ./scripts/bootstrap-state-bucket.sh
#

set -euo pipefail

BUCKET_NAME="cheap-uptime-monitor-tf-state"
REGION="${AWS_REGION:-eu-west-1}"

echo "Creating Terraform state bucket: ${BUCKET_NAME} in ${REGION}"

# Create bucket
aws s3api create-bucket \
  --bucket "${BUCKET_NAME}" \
  --region "${REGION}" \
  --create-bucket-configuration LocationConstraint="${REGION}"

echo "Enabling versioning..."
aws s3api put-bucket-versioning \
  --bucket "${BUCKET_NAME}" \
  --versioning-configuration Status=Enabled

echo "Blocking all public access..."
aws s3api put-public-access-block \
  --bucket "${BUCKET_NAME}" \
  --public-access-block-configuration \
    "BlockPublicAcls=true,IgnorePublicAcls=true,BlockPublicPolicy=true,RestrictPublicBuckets=true"

echo "Enabling server-side encryption (AES256)..."
aws s3api put-bucket-encryption \
  --bucket "${BUCKET_NAME}" \
  --server-side-encryption-configuration \
    '{"Rules":[{"ApplyServerSideEncryptionByDefault":{"SSEAlgorithm":"AES256"},"BucketKeyEnabled":true}]}'

echo "Adding bucket policy to enforce SSL/TLS..."
cat <<EOF | aws s3api put-bucket-policy --bucket "${BUCKET_NAME}" --policy file:///dev/stdin
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "EnforceTLS",
      "Effect": "Deny",
      "Principal": "*",
      "Action": "s3:*",
      "Resource": [
        "arn:aws:s3:::${BUCKET_NAME}",
        "arn:aws:s3:::${BUCKET_NAME}/*"
      ],
      "Condition": {
        "Bool": {
          "aws:SecureTransport": "false"
        }
      }
    }
  ]
}
EOF

echo ""
echo "State bucket created successfully!"
echo ""
echo "Security features enabled:"
echo "  - Versioning (recover previous state files)"
echo "  - Public access blocked"
echo "  - Server-side encryption (AES256)"
echo "  - TLS required for all requests"
echo ""
echo "Next steps:"
echo "  1. cd terraform"
echo "  2. terraform init"
echo "  3. terraform plan"
echo "  4. terraform apply"
