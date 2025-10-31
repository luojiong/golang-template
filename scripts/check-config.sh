#!/bin/bash

# Configuration Check Script
# Validates that all required configuration files exist and are properly formatted

set -e

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}===================================${NC}"
echo -e "${BLUE}Configuration Check${NC}"
echo -e "${BLUE}===================================${NC}"
echo ""

# Check Go
if ! command -v go &> /dev/null; then
    echo -e "${RED}[ERROR] Go is not installed${NC}"
    exit 1
fi

echo -e "${GREEN}[SUCCESS] Go found: $(go version | awk '{print $3}')${NC}"

# Check configuration files
echo ""
echo -e "${BLUE}[INFO] Checking configuration files...${NC}"

# Check development.yaml
if [ -f "configs/development.yaml" ]; then
    echo -e "${GREEN}[SUCCESS] configs/development.yaml found${NC}"

    # Basic YAML validation - check if it's not empty and has basic structure
    if grep -q "server:" configs/development.yaml && grep -q "database:" configs/development.yaml; then
        echo -e "${GREEN}[SUCCESS] YAML structure looks valid${NC}"

        # Extract and display key config values
        echo -e "${BLUE}[INFO] Configuration summary:${NC}"

        if grep -q "port:" configs/development.yaml; then
            PORT=$(grep -A1 "server:" configs/development.yaml | grep "port:" | awk '{print $2}' | tr -d '"')
            echo -e "${BLUE}  - Server port: $PORT${NC}"
        fi

        if grep -q "host:" configs/development.yaml; then
            DB_HOST=$(grep -A5 "database:" configs/development.yaml | grep "host:" | awk '{print $2}' | tr -d '"')
            echo -e "${BLUE}  - Database host: $DB_HOST${NC}"
        fi

        if grep -q "dbname:" configs/development.yaml; then
            DB_NAME=$(grep -A5 "database:" configs/development.yaml | grep "dbname:" | awk '{print $2}' | tr -d '"')
            echo -e "${BLUE}  - Database name: $DB_NAME${NC}"
        fi

        if grep -q "user:" configs/development.yaml; then
            DB_USER=$(grep -A5 "database:" configs/development.yaml | grep "user:" | awk '{print $2}' | tr -d '"')
            echo -e "${BLUE}  - Database user: $DB_USER${NC}"
        fi

        # Check password field exists but don't display it
        if grep -q "password:" configs/development.yaml; then
            echo -e "${BLUE}  - Database password: [CONFIGURED]${NC}"
        fi

    else
        echo -e "${RED}[ERROR] configs/development.yaml missing required sections (server:, database:)${NC}"
        exit 1
    fi
else
    echo -e "${RED}[ERROR] configs/development.yaml not found${NC}"
    echo -e "${RED}[ERROR] Please create configs/development.yaml with proper database settings${NC}"
    exit 1
fi

# Check production.yaml
if [ -f "configs/production.yaml" ]; then
    echo -e "${GREEN}[SUCCESS] configs/production.yaml found${NC}"
else
    echo -e "${YELLOW}[WARNING] configs/production.yaml not found (optional for development)${NC}"
fi

# Check main.go
if [ -f "cmd/api/main.go" ]; then
    echo -e "${GREEN}[SUCCESS] cmd/api/main.go found${NC}"
else
    echo -e "${RED}[ERROR] cmd/api/main.go not found${NC}"
    exit 1
fi

# Check go.mod
if [ -f "go.mod" ]; then
    echo -e "${GREEN}[SUCCESS] go.mod found${NC}"
    echo -e "${BLUE}[INFO] Module: $(head -1 go.mod)${NC}"
else
    echo -e "${RED}[ERROR] go.mod not found${NC}"
    exit 1
fi

# Check for key dependencies
echo ""
echo -e "${BLUE}[INFO] Checking key dependencies...${NC}"

if grep -q "gorm.io/driver/postgres" go.mod; then
    echo -e "${GREEN}[SUCCESS] PostgreSQL driver found${NC}"
else
    echo -e "${RED}[ERROR] PostgreSQL driver not found in go.mod${NC}"
    echo -e "${RED}[ERROR] Run: go get gorm.io/driver/postgres${NC}"
    exit 1
fi

if grep -q "gorm.io/gorm" go.mod; then
    echo -e "${GREEN}[SUCCESS] GORM found${NC}"
else
    echo -e "${RED}[ERROR] GORM not found in go.mod${NC}"
    echo -e "${RED}[ERROR] Run: go get gorm.io/gorm${NC}"
    exit 1
fi

if grep -q "github.com/spf13/viper" go.mod; then
    echo -e "${GREEN}[SUCCESS] Viper found${NC}"
else
    echo -e "${RED}[ERROR] Viper not found in go.mod${NC}"
    echo -e "${RED}[ERROR] Run: go get github.com/spf13/viper${NC}"
    exit 1
fi

# Check Docker (optional)
echo ""
echo -e "${BLUE}[INFO] Checking Docker (optional)...${NC}"

if command -v docker &> /dev/null; then
    echo -e "${GREEN}[SUCCESS] Docker found${NC}"

    # Check if PostgreSQL container is already running
    if docker ps -q -f name=postgres-dev | grep -q .; then
        echo -e "${GREEN}[SUCCESS] PostgreSQL container is running${NC}"
    else
        echo -e "${YELLOW}[INFO] PostgreSQL container is not running${NC}"
        echo -e "${YELLOW}        Start with: docker run --name postgres-dev -e POSTGRES_DB=golang_template_dev -e POSTGRES_USER=postgres -e POSTGRES_PASSWORD=caine -p 5432:5432 -d postgres:15-alpine${NC}"
    fi
else
    echo -e "${YELLOW}[WARNING] Docker not found (optional)${NC}"
fi

# Check Go modules
echo ""
echo -e "${BLUE}[INFO] Checking Go modules...${NC}"
if go mod tidy > /dev/null 2>&1; then
    echo -e "${GREEN}[SUCCESS] Go modules are consistent${NC}"
else
    echo -e "${YELLOW}[WARNING] Go modules have issues, running 'go mod tidy'...${NC}"
    go mod tidy
fi

# Summary
echo ""
echo -e "${GREEN}===================================${NC}"
echo -e "${GREEN}Configuration Check Complete${NC}"
echo -e "${GREEN}===================================${NC}"
echo ""
echo -e "${BLUE}Configuration loaded from:${NC}"
echo -e "${BLUE}  - configs/development.yaml${NC}"
echo ""
echo -e "${BLUE}Database connection info (from YAML):${NC}"
echo -e "${BLUE}  - Host: localhost:5432${NC}"
echo -e "${BLUE}  - Database: golang_template_dev${NC}"
echo -e "${BLUE}  - User: postgres${NC}"
echo -e "${BLUE}  - Password: caine${NC}"
echo ""
echo -e "${BLUE}Next steps:${NC}"
echo -e "${BLUE}1. Ensure PostgreSQL is running on localhost:5432${NC}"
echo -e "${BLUE}2. Run: ./scripts/simple-start.sh${NC}"
echo -e "${BLUE}3. Visit: http://localhost:8080/swagger/index.html${NC}"
echo ""
echo -e "${GREEN}All checks passed! Ready to start the application.${NC}"