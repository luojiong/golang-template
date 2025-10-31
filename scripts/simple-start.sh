#!/bin/bash

# Simple Development Startup Script
# Focused on getting the application running without complex Docker setup

set -e

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}===================================${NC}"
echo -e "${BLUE}Golang Template - Simple Start${NC}"
echo -e "${BLUE}===================================${NC}"
echo ""

# Check Go
if ! command -v go &> /dev/null; then
    echo -e "${RED}[ERROR] Go is not installed${NC}"
    echo "Please install Go from: https://golang.org/dl/"
    exit 1
fi

GO_VERSION=$(go version | awk '{print $3}')
echo -e "${GREEN}[SUCCESS] Go found: $GO_VERSION${NC}"

# Set environment (only the mode, other settings come from YAML)
export APP_ENV=development
echo -e "${GREEN}[INFO] Environment set to development mode (reading from configs/development.yaml)${NC}"

# Install dependencies
echo -e "${BLUE}[INFO] Installing dependencies...${NC}"
go mod download
go mod tidy
echo -e "${GREEN}[SUCCESS] Dependencies installed${NC}"

# Install development tools
echo -e "${BLUE}[INFO] Installing development tools...${NC}"

if ! command -v swag &> /dev/null; then
    go install github.com/swaggo/swag/cmd/swag@latest
    echo -e "${GREEN}[SUCCESS] swag installed${NC}"
fi

# Generate documentation
echo -e "${BLUE}[INFO] Generating Swagger docs...${NC}"
swag init -g cmd/api/main.go -o docs 2>/dev/null || echo -e "${YELLOW}[WARNING] Swagger generation failed (non-critical)${NC}"

# Start server
echo ""
echo -e "${GREEN}===================================${NC}"
echo -e "${GREEN}Starting Development Server${NC}"
echo -e "${GREEN}===================================${NC}"
echo ""
echo -e "${BLUE}Server will be available at: http://localhost:8080${NC}"
echo -e "${BLUE}Swagger UI: http://localhost:8080/swagger/index.html${NC}"
echo -e "${BLUE}Health check: http://localhost:8080/api/v1/health${NC}"
echo ""
echo -e "${YELLOW}Note: Make sure PostgreSQL is running on localhost:5432${NC}"
echo -e "${YELLOW}Database configuration is read from configs/development.yaml:${NC}"
echo -e "${YELLOW}  - Database: golang_template_dev${NC}"
echo -e "${YELLOW}  - User: postgres${NC}"
echo -e "${YELLOW}  - Password: caine${NC}"
echo ""
echo -e "${BLUE}Press Ctrl+C to stop the server${NC}"
echo ""

# Start the application
go run cmd/api/main.go