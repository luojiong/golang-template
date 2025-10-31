.PHONY: build run clean test dev fmt lint deps swag install docker-build docker-run db-migrate db-seed db-reset scripts scripts-bash scripts-bat scripts-ps1 check-config

# Application name
APP_NAME := go-server
VERSION := 1.0.0

# Go parameters
GOCMD := go
GOBUILD := $(GOCMD) build
GOCLEAN := $(GOCMD) clean
GOTEST := $(GOCMD) test
GOGET := $(GOCMD) get
GOMOD := $(GOCMD) mod
GOFMT := gofmt

# Build directory
BUILD_DIR := build
BIN_DIR := bin

# Main package
MAIN_PACKAGE := ./cmd/api

# Build info
BUILD_TIME := $(shell date +%Y-%m-%d_%H:%M:%S)
GIT_COMMIT := $(shell git rev-parse --short HEAD)
GO_VERSION := $(shell go version | awk '{print $$3}')

# LDFLAGS
LDFLAGS := -ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -X main.GitCommit=$(GIT_COMMIT) -X main.GoVersion=$(GO_VERSION)"

# Default target
all: clean deps fmt lint test build

# Install dependencies
deps:
	$(GOMOD) download
	$(GOMOD) tidy

# Install tools
install:
	$(GOGET) -u github.com/swaggo/swag/cmd/swag
	$(GOGET) -u github.com/golangci/golangci-lint/cmd/golangci-lint

# Format code
fmt:
	$(GOFMT) -s -w .

# Run linter
lint:
	golangci-lint run

# Run tests
test:
	$(GOTEST) -v ./...

# Run tests with coverage
test-coverage:
	$(GOTEST) -v -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

# Build application
build:
	@mkdir -p $(BUILD_DIR)/$(BIN_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BIN_DIR)/$(APP_NAME)-linux-amd64 $(MAIN_PACKAGE)
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BIN_DIR)/$(APP_NAME)-darwin-amd64 $(MAIN_PACKAGE)
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BIN_DIR)/$(APP_NAME)-windows-amd64.exe $(MAIN_PACKAGE)

# Build for current platform
build-local:
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(APP_NAME) $(MAIN_PACKAGE)

# Run application
run: build-local
	./$(BUILD_DIR)/$(APP_NAME)

# Run in development mode
dev:
	APP_ENV=development $(GOCMD) run $(MAIN_PACKAGE)

# Generate Swagger documentation
swag:
	swag init -g cmd/api/main.go -o docs

# Clean build artifacts
clean:
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html

# Run application in production mode
prod:
	APP_ENV=production $(GOCMD) run $(MAIN_PACKAGE)

# Docker build
docker-build:
	docker build -t $(APP_NAME):$(VERSION) .

# Docker run
docker-run:
	docker run -p 8080:8080 $(APP_NAME):$(VERSION)

# Docker run in development
docker-dev:
	docker-compose up -d

# Docker stop
docker-stop:
	docker-compose down

# Check for Go vulnerabilities
security:
	$(GOCMD) list -json -m all | nancy sleuth

# Update dependencies
update:
	$(GOMOD) get -u ./...
	$(GOMOD) tidy

# Check for outdated dependencies
outdated:
	$(GOGET) -u github.com/psampaz/go-mod-outdated
	$(GOMOD) list -u -m -json all | go-mod-outdated -update -direct

# Database commands
db-migrate:
	@echo "Running database migrations..."
	$(GOCMD) run $(MAIN_PACKAGE) -migrate-only

db-seed:
	@echo "Seeding database with initial data..."
	$(GOCMD) run $(MAIN_PACKAGE) -seed-only

db-reset:
	@echo "Resetting database (migrate + seed)..."
	$(GOCMD) run $(MAIN_PACKAGE) -reset-db

db-status:
	@echo "Checking database status..."
	@echo "Checking database connection..."
	curl -s http://localhost:8080/api/v1/health | jq .

# Development scripts
scripts:
	@echo "Available development scripts:"
	@echo "  scripts/dev.sh    - Git Bash / Linux / macOS (Recommended)"
	@echo "  scripts/dev.bat   - Windows Command Prompt"
	@echo "  scripts/dev.ps1   - Windows PowerShell"
	@echo ""
	@echo "Usage:"
	@echo "  Git Bash:     ./scripts/dev.sh"
	@echo "  PowerShell:   ./scripts/dev.ps1"
	@echo "  CMD:          scripts\\dev.bat"
	@echo ""
	@echo "See scripts/README.md for detailed instructions"

# Run bash script (Git Bash)
scripts-bash:
	@echo "Running development script for Git Bash..."
	./scripts/dev.sh

# Run batch script (Windows CMD)
scripts-bat:
	@echo "Running development script for Windows Command Prompt..."
	cmd /c scripts\\dev.bat

# Run PowerShell script
scripts-ps1:
	@echo "Running development script for PowerShell..."
	powershell -ExecutionPolicy Bypass -File scripts\\dev.ps1

# Check configuration
check-config:
	@echo "Checking configuration files and dependencies..."
	./scripts/check-config.sh

# Help
help:
	@echo "Available targets:"
	@echo "  all          - Clean, deps, fmt, lint, test, build"
	@echo "  deps         - Install dependencies"
	@echo "  install      - Install development tools"
	@echo "  fmt          - Format code"
	@echo "  lint         - Run linter"
	@echo "  test         - Run tests"
	@echo "  test-coverage- Run tests with coverage report"
	@echo "  build        - Build for all platforms"
	@echo "  build-local  - Build for current platform"
	@echo "  run          - Build and run application"
	@echo "  dev          - Run in development mode"
	@echo "  prod         - Run in production mode"
	@echo "  swag         - Generate Swagger documentation"
	@echo "  clean        - Clean build artifacts"
	@echo "  docker-build - Build Docker image"
	@echo "  docker-run   - Run Docker container"
	@echo "  docker-dev   - Run with Docker Compose"
	@echo "  docker-stop  - Stop Docker Compose"
	@echo "  security     - Check for vulnerabilities"
	@echo "  update       - Update dependencies"
	@echo "  outdated     - Check for outdated dependencies"
	@echo "  db-migrate   - Run database migrations"
	@echo "  db-seed      - Seed database with initial data"
	@echo "  db-reset     - Reset database (migrate + seed)"
	@echo "  db-status    - Check database status"
	@echo "  scripts      - Show available development scripts"
	@echo "  scripts-bash - Run Git Bash development script"
	@echo "  scripts-bat  - Run Windows CMD development script"
	@echo "  scripts-ps1  - Run PowerShell development script"
	@echo "  check-config- Check configuration files and dependencies"
	@echo "  help         - Show this help message"