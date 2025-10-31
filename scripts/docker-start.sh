#!/bin/bash

# Docker-based Development Startup Script
# Simplified Docker configuration for PostgreSQL

set -e

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}===================================${NC}"
echo -e "${BLUE}Golang Template - Docker Start${NC}"
echo -e "${BLUE}===================================${NC}"
echo ""

# Check Docker
if ! command -v docker &> /dev/null; then
    echo -e "${RED}[ERROR] Docker is not installed${NC}"
    echo "Please install Docker from: https://www.docker.com/products/docker-desktop"
    exit 1
fi

echo -e "${GREEN}[SUCCESS] Docker found${NC}"

# Check existing PostgreSQL container
if docker ps -q -f name=postgres-dev | grep -q .; then
    echo -e "${GREEN}[SUCCESS] PostgreSQL container is already running${NC}"
else
    echo -e "${BLUE}[INFO] Starting PostgreSQL container...${NC}"

    # Stop and remove existing container if it exists
    docker stop postgres-dev 2>/dev/null || true
    docker rm postgres-dev 2>/dev/null || true

    # Start new PostgreSQL container (simplified)
    docker run --name postgres-dev \
        -e POSTGRES_DB=golang_template_dev \
        -e POSTGRES_USER=postgres \
        -e POSTGRES_PASSWORD=caine \
        -p 5432:5432 \
        -d postgres:15-alpine

    # Wait for database to be ready
    echo -e "${BLUE}[INFO] Waiting for PostgreSQL to be ready...${NC}"
    for i in {1..15}; do
        if docker exec postgres-dev pg_isready -q 2>/dev/null; then
            echo -e "${GREEN}[SUCCESS] PostgreSQL is ready${NC}"
            break
        fi
        echo -n "."
        sleep 1
    done
    echo ""
fi

# Database connection info
echo -e "${BLUE}[INFO] Database connection:${NC}"
echo "  Host: localhost:5432"
echo "  Database: golang_template_dev"
echo "  User: postgres"
echo "  Password: caine"
echo ""

# Install dependencies
echo -e "${BLUE}[INFO] Installing Go dependencies...${NC}"
go mod download
go mod tidy
echo -e "${GREEN}[SUCCESS] Dependencies installed${NC}"

# Install tools
echo -e "${BLUE}[INFO] Installing development tools...${NC}"
if ! command -v swag &> /dev/null; then
    go install github.com/swaggo/swag/cmd/swag@latest
fi

# Generate docs
echo -e "${BLUE}[INFO] Generating Swagger docs...${NC}"
swag init -g cmd/api/main.go -o docs 2>/dev/null || echo -e "${YELLOW}[WARNING] Swagger generation failed${NC}"

# Set environment (only the mode, other settings come from YAML)
export APP_ENV=development

# Start server
echo ""
echo -e "${GREEN}===================================${NC}"
echo -e "${GREEN}Starting Development Server${NC}"
echo -e "${GREEN}===================================${NC}"
echo ""
echo -e "${BLUE}Server: http://localhost:8080${NC}"
echo -e "${BLUE}Swagger: http://localhost:8080/swagger/index.html${NC}"
echo -e "${BLUE}Health: http://localhost:8080/api/v1/health${NC}"
echo ""
echo -e "${YELLOW}Press Ctrl+C to stop server and cleanup${NC}"
echo ""

# Cleanup function
cleanup() {
    echo ""
    echo -e "${BLUE}[INFO] Cleaning up...${NC}"
    echo -n "${YELLOW}Stop PostgreSQL container? (y/N): ${NC}"
    read -r response
    if [[ "$response" =~ ^([yY])$ ]]; then
        docker stop postgres-dev 2>/dev/null || true
        docker rm postgres-dev 2>/dev/null || true
        echo -e "${GREEN}[SUCCESS] PostgreSQL container stopped${NC}"
    fi
    echo -e "${GREEN}[SUCCESS] Cleanup completed${NC}"
}

trap cleanup EXIT

# Start the application
go run cmd/api/main.go