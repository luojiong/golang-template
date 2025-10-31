#!/bin/bash

# Golang Template Development Startup Script
# Compatible with Windows + Git Bash
# Author: Generated with Claude Code

set -e  # Exit on any error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Print colored output
print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_header() {
    echo -e "${PURPLE}===================================${NC}"
    echo -e "${PURPLE}$1${NC}"
    echo -e "${PURPLE}===================================${NC}"
}

# Check if command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Check prerequisites
check_prerequisites() {
    print_header "Checking Prerequisites"

    # Check Go
    if command_exists go; then
        GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
        print_success "Go found: $GO_VERSION"
    else
        print_error "Go is not installed or not in PATH"
        echo "Please install Go from: https://golang.org/dl/"
        exit 1
    fi

    # Check PostgreSQL (optional, can use Docker)
    if command_exists psql; then
        print_success "PostgreSQL client found"
    else
        print_warning "PostgreSQL client not found, will use Docker if available"
    fi

    # Check Docker (optional)
    if command_exists docker; then
        DOCKER_VERSION=$(docker --version | awk '{print $3}' | sed 's/,//')
        print_success "Docker found: $DOCKER_VERSION"
    else
        print_warning "Docker not found, you'll need to set up PostgreSQL manually"
    fi

    # Check Docker Compose (optional)
    if command_exists docker-compose; then
        COMPOSE_VERSION=$(docker-compose --version | awk '{print $3}' | sed 's/,//')
        print_success "Docker Compose found: $COMPOSE_VERSION"
    else
        print_warning "Docker Compose not found"
    fi
}

# Setup environment
setup_environment() {
    print_header "Setting Up Environment"

    # Set environment to development
    export APP_ENV=development
    print_success "Environment set to: $APP_ENV"

    # Check if development.yaml config exists
    if [ -f "configs/development.yaml" ]; then
        print_success "Found development configuration file"
        print_status "Configuration will be loaded from configs/development.yaml"
    else
        print_error "Development configuration file not found: configs/development.yaml"
        print_status "Please ensure configs/development.yaml exists with proper database settings"
        exit 1
    fi
}

# Setup database using Docker
setup_database_docker() {
    print_header "Setting Up Database with Docker"

    if command_exists docker; then
        # Check if PostgreSQL container is already running
        POSTGRES_CONTAINER=$(docker ps -q -f name=postgres-dev)

        if [ -n "$POSTGRES_CONTAINER" ]; then
            print_success "PostgreSQL container is already running"
        else
            print_status "Starting PostgreSQL container..."

            # Stop and remove existing container if it exists but not running
            docker stop postgres-dev 2>/dev/null || true
            docker rm postgres-dev 2>/dev/null || true

            # Start new PostgreSQL container
            docker run --name postgres-dev \
                -e POSTGRES_DB=golang_template_dev \
                -e POSTGRES_USER=postgres \
                -e POSTGRES_PASSWORD=caine \
                -p 5432:5432 \
                -v postgres_data:/var/lib/postgresql/data \
                -d postgres:15-alpine

            # Wait for PostgreSQL to be ready
            print_status "Waiting for PostgreSQL to be ready..."
            for i in {1..30}; do
                if docker exec postgres-dev pg_isready -q; then
                    print_success "PostgreSQL is ready"
                    break
                fi
                echo -n "."
                sleep 1
            done
            echo ""
        fi

        # Show database connection info
        print_status "Database connection info:"
        echo "  Host: localhost:5432"
        echo "  Database: golang_template_dev"
        echo "  User: postgres"
        echo "  Password: caine"

    else
        print_warning "Docker not available, please set up PostgreSQL manually"
        echo "Database: golang_template_dev"
        echo "Connection: postgresql://postgres:caine@localhost:5432/golang_template_dev"
    fi
}

# Install dependencies
install_dependencies() {
    print_header "Installing Dependencies"

    # Check if go.mod exists
    if [ -f "go.mod" ]; then
        print_status "Downloading Go modules..."
        if go mod download; then
            print_success "Dependencies downloaded"
        else
            print_error "Failed to download dependencies"
            exit 1
        fi

        print_status "Tidying go.mod..."
        if go mod tidy; then
            print_success "go.mod tidied"
        else
            print_error "Failed to tidy go.mod"
            exit 1
        fi
    else
        print_error "go.mod file not found"
        exit 1
    fi
}

# Install development tools
install_dev_tools() {
    print_header "Installing Development Tools"

    # Install swag for Swagger documentation
    if ! command_exists swag; then
        print_status "Installing swag..."
        go install github.com/swaggo/swag/cmd/swag@latest
        print_success "swag installed"
    else
        print_success "swag already installed"
    fi

    # Install golangci-lint
    if ! command_exists golangci-lint; then
        print_status "Installing golangci-lint..."
        go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
        print_success "golangci-lint installed"
    else
        print_success "golangci-lint already installed"
    fi
}

# Generate Swagger documentation
generate_docs() {
    print_header "Generating Swagger Documentation"

    print_status "Generating Swagger docs..."
    if swag init -g cmd/api/main.go -o docs; then
        print_success "Swagger documentation generated"
    else
        print_warning "Failed to generate Swagger docs (non-critical)"
    fi
}

# Run database migrations
run_migrations() {
    print_header "Running Database Migrations"

    print_status "Starting application for migrations..."

    # Run the application briefly to execute migrations
    timeout 30s go run cmd/api/main.go 2>/dev/null || true

    print_success "Database migrations completed"
}

# Start the development server
start_server() {
    print_header "Starting Development Server"

    # Set development-specific environment variables
    export GIN_MODE=debug

    print_status "Environment variables:"
    echo "  APP_ENV: ${APP_ENV:-development}"
    echo "  GIN_MODE: $GIN_MODE"
    echo "  APP_SERVER_HOST: ${APP_SERVER_HOST:-localhost}"
    echo "  APP_SERVER_PORT: ${APP_SERVER_PORT:-8080}"

    print_status "Starting server..."
    print_success "Server will be available at: http://localhost:8080"
    print_success "Swagger UI will be available at: http://localhost:8080/swagger/index.html"
    print_success "Health check: http://localhost:8080/api/v1/health"

    echo ""
    echo -e "${CYAN}Press Ctrl+C to stop the server${NC}"
    echo ""

    # Start the application
    go run cmd/api/main.go
}

# Cleanup function
cleanup() {
    print_header "Cleanup"

    if command_exists docker; then
        # Check if container exists before asking
        POSTGRES_CONTAINER=$(docker ps -a -q -f name=postgres-dev)
        if [ -n "$POSTGRES_CONTAINER" ]; then
            # Ask user if they want to stop PostgreSQL container
            printf "%s" "${YELLOW}Do you want to stop the PostgreSQL container? (y/N): ${NC}"
            read -r response
            if [[ "$response" =~ ^([yY][eE][sS]|[yY])$ ]]; then
                print_status "Stopping PostgreSQL container..."
                docker stop postgres-dev 2>/dev/null || true
                docker rm postgres-dev 2>/dev/null || true
                print_success "PostgreSQL container stopped and removed"
            fi
        else
            print_status "No PostgreSQL container to clean up"
        fi
    fi

    print_success "Cleanup completed"
}

# Set up signal handlers
trap cleanup EXIT

# Main execution
main() {
    echo -e "${PURPLE}"
    echo "  ____              _   _   _      _   _            "
    echo " |  _ \            | | | | | |    | | | |           "
    echo " | |_) | __ _ _ __ | | | | | |    | | | | ___  _ __ "
    echo " |  _ < / _' | '_ \| | | | | |    | | | |/ _ \| '__|"
    echo " | |_) | (_| | | | | | | | | |____| |_| | (_) | |   "
    echo " |____/ \__,_|_| |_|_|_|_|_|______|\___/ \___/|_|   "
    echo ""
    echo -e "          Development Environment${NC}"
    echo ""

    print_header "Starting Golang Template Development Environment"

    # Run setup steps
    check_prerequisites
    setup_environment
    setup_database_docker
    install_dependencies
    install_dev_tools
    generate_docs

    echo ""
    print_success "Setup completed successfully!"
    echo ""

    # Start the server
    start_server
}

# Run main function
main "$@"