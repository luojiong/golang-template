#!/bin/bash

# Golang Template Production Deployment Script
# Compatible with Linux/macOS and Windows + Git Bash
# Author: Generated with Claude Code

set -e  # Exit on any error

# Default configuration
DEPLOY_ENV=${DEPLOY_ENV:-production}
BACKUP_ENABLED=${BACKUP_ENABLED:-true}
HEALTH_CHECK_TIMEOUT=${HEALTH_CHECK_TIMEOUT:-60}
ROLLBACK_ENABLED=${ROLLBACK_ENABLED:-true}

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

# Log deployment events
log_deployment() {
    local level=$1
    local message=$2
    local timestamp=$(date '+%Y-%m-%d %H:%M:%S')
    echo "[$timestamp] [$level] $message" >> "logs/deployment.log"
    print_status "Logged: $message"
}

# Validate deployment environment
validate_environment() {
    print_header "Validating Deployment Environment"

    # Check required environment variables
    local required_vars=(
        "APP_JWT_SECRET_KEY"
        "APP_DATABASE_HOST"
        "APP_DATABASE_PASSWORD"
        "APP_DATABASE_NAME"
        "APP_REDIS_HOST"
    )

    local missing_vars=()

    for var in "${required_vars[@]}"; do
        if [[ -z "${!var}" ]]; then
            missing_vars+=("$var")
        else
            # Mask sensitive values in logs
            if [[ "$var" == *"PASSWORD"* ]] || [[ "$var" == *"SECRET"* ]]; then
                print_success "$var: [CONFIGURED]"
            else
                print_success "$var: ${!var}"
            fi
        fi
    done

    if [[ ${#missing_vars[@]} -gt 0 ]]; then
        print_error "Missing required environment variables:"
        for var in "${missing_vars[@]}"; do
            echo "  - $var"
        done
        print_error "Please set these variables and try again"
        exit 1
    fi

    # Check configuration files
    if [[ ! -f "configs/production.yaml" ]]; then
        print_error "Production configuration file not found: configs/production.yaml"
        exit 1
    fi

    print_success "Environment validation completed"
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
        exit 1
    fi

    # Check Docker
    if command_exists docker; then
        DOCKER_VERSION=$(docker --version | awk '{print $3}' | sed 's/,//')
        print_success "Docker found: $DOCKER_VERSION"
    else
        print_error "Docker is required for production deployment"
        exit 1
    fi

    # Check Docker Compose
    if command_exists docker-compose; then
        COMPOSE_VERSION=$(docker-compose --version | awk '{print $3}' | sed 's/,//')
        print_success "Docker Compose found: $COMPOSE_VERSION"
    else
        print_error "Docker Compose is required for production deployment"
        exit 1
    fi

    # Check if required directories exist
    local required_dirs=("logs" "configs")
    for dir in "${required_dirs[@]}"; do
        if [[ ! -d "$dir" ]]; then
            print_status "Creating directory: $dir"
            mkdir -p "$dir"
        fi
    done
}

# Create backup if enabled
create_backup() {
    if [[ "$BACKUP_ENABLED" != "true" ]]; then
        print_warning "Backup is disabled"
        return
    fi

    print_header "Creating Backup"

    local backup_dir="backups/$(date '+%Y%m%d_%H%M%S')"
    mkdir -p "$backup_dir"

    # Backup current running container
    if docker ps -q -f name=go-server-app | grep -q .; then
        print_status "Backing up current container..."
        docker commit go-server-app "go-server-backup:$(date +%s)"
        print_success "Container backup created"
    fi

    # Backup configuration
    if [[ -d "configs" ]]; then
        cp -r configs "$backup_dir/"
        print_success "Configuration backup created"
    fi

    # Backup database (if PostgreSQL is accessible)
    if [[ -n "$APP_DATABASE_HOST" ]]; then
        print_status "Attempting database backup..."
        # This would need to be implemented based on your database backup strategy
        print_warning "Database backup should be implemented separately"
    fi

    print_success "Backup completed: $backup_dir"
}

# Build application
build_application() {
    print_header "Building Application"

    # Install dependencies
    print_status "Installing dependencies..."
    go mod download
    go mod tidy

    # Run tests
    print_status "Running tests..."
    if ! go test ./... -v; then
        print_error "Tests failed, aborting deployment"
        exit 1
    fi

    # Build application
    print_status "Building application..."
    if CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o bin/go-server cmd/api/main.go; then
        print_success "Application built successfully"
    else
        print_error "Build failed"
        exit 1
    fi

    # Generate Swagger documentation
    print_status "Generating Swagger documentation..."
    if command_exists swag; then
        swag init -g cmd/api/main.go -o docs
        print_success "Swagger documentation generated"
    else
        print_warning "swag not found, skipping documentation generation"
    fi
}

# Deploy services
deploy_services() {
    print_header "Deploying Services"

    # Set production environment
    export APP_ENV=production
    export COMPOSE_PROJECT_NAME=go-server

    print_status "Deploying with docker-compose..."

    # Stop existing services
    print_status "Stopping existing services..."
    docker-compose -f docker-compose.yml down || true

    # Pull latest images
    print_status "Pulling latest images..."
    docker-compose -f docker-compose.yml pull

    # Build and start services
    print_status "Building and starting services..."
    if docker-compose -f docker-compose.yml up -d --build; then
        print_success "Services deployed successfully"
    else
        print_error "Service deployment failed"
        if [[ "$ROLLBACK_ENABLED" == "true" ]]; then
            print_status "Attempting rollback..."
            rollback_deployment
        fi
        exit 1
    fi

    # Wait for services to be ready
    print_status "Waiting for services to be ready..."
    sleep 10
}

# Health check
perform_health_check() {
    print_header "Performing Health Check"

    local max_attempts=$((HEALTH_CHECK_TIMEOUT / 5))
    local attempt=1

    while [[ $attempt -le $max_attempts ]]; do
        print_status "Health check attempt $attempt/$max_attempts..."

        # Check if application is responding
        if curl -f -s http://localhost:8080/api/v1/health >/dev/null 2>&1; then
            print_success "Application health check passed"
            return 0
        fi

        # Check service status
        local app_status=$(docker-compose -f docker-compose.yml ps -q app)
        if [[ -n "$app_status" ]]; then
            local container_status=$(docker inspect --format='{{.State.Status}}' "$app_status" 2>/dev/null || echo "unknown")
            if [[ "$container_status" != "running" ]]; then
                print_error "Application container is not running (status: $container_status)"
                docker logs "$app_status" --tail 20
                return 1
            fi
        else
            print_error "Application container not found"
            return 1
        fi

        print_warning "Health check failed, retrying in 5 seconds..."
        sleep 5
        ((attempt++))
    done

    print_error "Health check failed after $HEALTH_CHECK_TIMEOUT seconds"

    # Show container logs for debugging
    print_status "Application container logs:"
    docker-compose -f docker-compose.yml logs --tail=50 app

    return 1
}

# Rollback deployment
rollback_deployment() {
    print_header "Rolling Back Deployment"

    print_status "Rolling back to previous version..."

    # Stop current services
    docker-compose -f docker-compose.yml down || true

    # Try to restore from backup
    local latest_backup=$(docker images --format "table {{.Repository}}:{{.Tag}}" | grep "go-server-backup" | head -1)
    if [[ -n "$latest_backup" ]]; then
        print_status "Restoring from backup: $latest_backup"
        # Implement rollback logic based on your backup strategy
        print_warning "Rollback implementation needed"
    else
        print_warning "No backup found for rollback"
    fi

    print_warning "Rollback completed. Please verify the application status."
}

# Post-deployment verification
post_deployment_verification() {
    print_header "Post-Deployment Verification"

    # Verify all required services are running
    local services=("app" "postgres" "redis")
    for service in "${services[@]}"; do
        local status=$(docker-compose -f docker-compose.yml ps -q "$service")
        if [[ -n "$status" ]]; then
            local container_status=$(docker inspect --format='{{.State.Status}}' "$status" 2>/dev/null || echo "unknown")
            if [[ "$container_status" == "running" ]]; then
                print_success "$service service is running"
            else
                print_error "$service service is not running (status: $container_status)"
                return 1
            fi
        else
            print_error "$service service not found"
            return 1
        fi
    done

    # Verify endpoints are accessible
    local endpoints=(
        "http://localhost:8080/api/v1/health"
        "http://localhost:8080/api/v1"
        "http://localhost:8080/swagger/index.html"
    )

    for endpoint in "${endpoints[@]}"; do
        if curl -f -s "$endpoint" >/dev/null 2>&1; then
            print_success "Endpoint accessible: $endpoint"
        else
            print_warning "Endpoint not accessible: $endpoint"
        fi
    done

    print_success "Post-deployment verification completed"
}

# Cleanup function
cleanup() {
    print_header "Cleanup"

    # Remove unused Docker images
    print_status "Cleaning up unused Docker images..."
    docker image prune -f >/dev/null 2>&1 || true

    print_success "Cleanup completed"
}

# Display deployment summary
display_summary() {
    print_header "Deployment Summary"

    echo -e "${GREEN}✓ Environment: $DEPLOY_ENV${NC}"
    echo -e "${GREEN}✓ Application: http://localhost:8080${NC}"
    echo -e "${GREEN}✓ API Docs: http://localhost:8080/swagger/index.html${NC}"
    echo -e "${GREEN}✓ Health Check: http://localhost:8080/api/v1/health${NC}"
    echo ""
    echo -e "${BLUE}Useful commands:${NC}"
    echo -e "${BLUE}  docker-compose logs -f app    # View application logs${NC}"
    echo -e "${BLUE}  docker-compose ps             # Check service status${NC}"
    echo -e "${BLUE}  ./scripts/health-check.sh     # Run health check${NC}"
    echo ""
    echo -e "${YELLOW}Deployment completed successfully!${NC}"
}

# Main deployment function
main() {
    echo -e "${PURPLE}"
    echo "  ____              _   _   _      _   _            "
    echo " |  _ \            | | | | | |    | | | |           "
    echo " | |_) | __ _ _ __ | | | | | |    | | | | ___  _ __ "
    echo " |  _ < / _' | '_ \| | | | | |    | | | |/ _ \| '__|"
    echo " | |_) | (_| | | | | | | | | |____| |_| | (_) | |   "
    echo " |____/ \__,_|_| |_|_|_|_|_|______|\___/ \___/|_|   "
    echo ""
    echo -e "        Production Deployment${NC}"
    echo ""

    print_header "Starting Production Deployment"

    # Log deployment start
    log_deployment "INFO" "Starting deployment for environment: $DEPLOY_ENV"

    # Run deployment steps
    validate_environment
    check_prerequisites
    create_backup
    build_application
    deploy_services

    # Health check
    if perform_health_check; then
        post_deployment_verification
        display_summary
        log_deployment "SUCCESS" "Deployment completed successfully"

        # Cleanup
        cleanup
    else
        log_deployment "ERROR" "Deployment failed health check"
        print_error "Deployment failed health check"

        if [[ "$ROLLBACK_ENABLED" == "true" ]]; then
            rollback_deployment
        fi

        exit 1
    fi
}

# Set up signal handlers
trap cleanup EXIT

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --env)
            DEPLOY_ENV="$2"
            shift 2
            ;;
        --no-backup)
            BACKUP_ENABLED=false
            shift
            ;;
        --no-rollback)
            ROLLBACK_ENABLED=false
            shift
            ;;
        --health-timeout)
            HEALTH_CHECK_TIMEOUT="$2"
            shift 2
            ;;
        --help)
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --env ENV              Set deployment environment (default: production)"
            echo "  --no-backup           Disable backup creation"
            echo "  --no-rollback         Disable automatic rollback"
            echo "  --health-timeout SEC  Set health check timeout in seconds (default: 60)"
            echo "  --help                Show this help message"
            echo ""
            echo "Required Environment Variables:"
            echo "  APP_JWT_SECRET_KEY    JWT secret key"
            echo "  APP_DATABASE_HOST     Database host"
            echo "  APP_DATABASE_PASSWORD Database password"
            echo "  APP_DATABASE_NAME     Database name"
            echo "  APP_REDIS_HOST        Redis host"
            echo ""
            exit 0
            ;;
        *)
            print_error "Unknown option: $1"
            echo "Use --help for usage information"
            exit 1
            ;;
    esac
done

# Run main function
main "$@"