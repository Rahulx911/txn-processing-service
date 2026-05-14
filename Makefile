.PHONY: build run test test-coverage test-integration lint clean docker-build docker-run

APP_NAME := txn-processing-service
BUILD_DIR := ./bin
MAIN_PATH := ./cmd/server

# Build the application binary
build:
	@echo "Building $(APP_NAME)..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 go build -ldflags="-s -w" -o $(BUILD_DIR)/$(APP_NAME) $(MAIN_PATH)
	@echo "Build complete: $(BUILD_DIR)/$(APP_NAME)"

# Run the application locally
run:
	@echo "Starting $(APP_NAME)..."
	go run $(MAIN_PATH)/main.go

# Run unit tests
test:
	@echo "Running unit tests..."
	go test ./internal/... ./pkg/... -v -count=1

# Run tests with coverage report
test-coverage:
	@echo "Running tests with coverage..."
	go test ./internal/... ./pkg/... -v -count=1 -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Run integration tests (requires LocalStack)
test-integration:
	@echo "Running integration tests..."
	go test ./test/... -v -count=1 -tags=integration

# Lint the codebase
lint:
	@echo "Linting..."
	golangci-lint run ./...

# Clean build artefacts
clean:
	@echo "Cleaning..."
	rm -rf $(BUILD_DIR) coverage.out coverage.html

# Docker build
docker-build:
	docker build -t $(APP_NAME):latest .

# Docker run with LocalStack
docker-run:
	docker-compose up --build

# Docker stop
docker-stop:
	docker-compose down

# Terraform init
tf-init:
	cd terraform && terraform init

# Terraform plan
tf-plan:
	cd terraform && terraform plan -var="environment=dev"

# Terraform apply
tf-apply:
	cd terraform && terraform apply -var="environment=dev" -auto-approve

# Format Go code
fmt:
	go fmt ./...

# Download dependencies
deps:
	go mod download
	go mod tidy
