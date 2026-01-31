.PHONY: build clean deploy test

BINARY_NAME=bootstrap
BUILD_DIR=build

build:
	@echo "Building Lambda binary..."
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -tags lambda.norpc -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/lambda/...
	@echo "Creating deployment package..."
	cd $(BUILD_DIR) && zip -j lambda.zip $(BINARY_NAME)

clean:
	rm -rf $(BUILD_DIR)

test:
	go test -v ./...

tidy:
	go mod tidy

terraform-init:
	cd terraform && terraform init

terraform-plan:
	cd terraform && terraform plan

terraform-apply:
	cd terraform && terraform apply

terraform-destroy:
	cd terraform && terraform destroy

upload-config:
	@echo "Uploading config to S3..."
	aws s3 cp config/config.json s3://cheap-uptime-monitor-config-bucket/config.json
