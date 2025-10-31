#!/bin/bash

# Golang Template Health Check Script
# Performs comprehensive health checks for the application and its dependencies
# Compatible with Linux/macOS and Windows + Git Bash
# Author: Generated with Claude Code

set -e  # Exit on any error

# Default configuration
HEALTH_CHECK_URL=${HEALTH_CHECK_URL:-"http://localhost:8080/api/v1/health"}
TIMEOUT=${TIMEOUT:-10}
RETRY_COUNT=${RETRY_COUNT:-3}
RETRY_DELAY=${RETRY_DELAY:-2}
VERBOSE=${VERBOSE:-false}
CHECK_SERVICES=${CHECK_SERVICES:-true}
CHECK_ENDPOINTS=${CHECK_ENDPOINTS:-true}
CHECK_DEPENDENCIES=${CHECK_DEPENDENCIES:-true}
JSON_OUTPUT=${JSON_OUTPUT:-false}

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Health check results
declare -A RESULTS
OVERALL_STATUS="PASSING"

# Print colored output (unless JSON output is enabled)
print_status() {
    if [[ "$JSON_OUTPUT" != "true" ]]; then
        echo -e "${BLUE}[INFO]${NC} $1"
    fi
}

print_success() {
    if [[ "$JSON_OUTPUT" != "true" ]]; then
        echo -e "${GREEN}[SUCCESS]${NC} $1"
    fi
    RESULTS["$1"]="PASSING"
}

print_warning() {
    if [[ "$JSON_OUTPUT" != "true" ]]; then
        echo -e "${YELLOW}[WARNING]${NC} $1"
    fi
    RESULTS["$1"]="WARNING"
    if [[ "$OVERALL_STATUS" == "PASSING" ]]; then
        OVERALL_STATUS="WARNING"
    fi
}

print_error() {
    if [[ "$JSON_OUTPUT" != "true" ]]; then
        echo -e "${RED}[ERROR]${NC} $1"
    fi
    RESULTS["$1"]="FAILING"
    OVERALL_STATUS="FAILING"
}

print_header() {
    if [[ "$JSON_OUTPUT" != "true" ]]; then
        echo -e "${PURPLE}===================================${NC}"
        echo -e "${PURPLE}$1${NC}"
        echo -e "${PURPLE}===================================${NC}"
    fi
}

# Check if command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Log verbose output
log_verbose() {
    if [[ "$VERBOSE" == "true" ]]; then
        echo -e "${CYAN}[VERBOSE]${NC} $1"
    fi
}

# Check HTTP endpoint
check_http_endpoint() {
    local name=$1
    local url=$2
    local timeout=$3
    local expected_status=${4:-200}

    log_verbose "Checking endpoint: $url (expected status: $expected_status)"

    local response_code=$(curl -s -o /dev/null -w "%{http_code}" --connect-timeout "$timeout" "$url" 2>/dev/null || echo "000")

    if [[ "$response_code" == "$expected_status" ]]; then
        print_success "$name"
        log_verbose "Response code: $response_code"
        return 0
    else
        print_error "$name (HTTP $response_code)"
        if [[ "$VERBOSE" == "true" ]]; then
            curl -s --connect-timeout "$timeout" "$url" 2>&1 | head -5 || echo "Connection failed"
        fi
        return 1
    fi
}

# Check Docker services
check_docker_services() {
    print_header "Checking Docker Services"

    if ! command_exists docker; then
        print_error "Docker not available"
        return 1
    fi

    if ! command_exists docker-compose; then
        print_error "Docker Compose not available"
        return 1
    fi

    # Check if docker-compose.yml exists
    if [[ ! -f "docker-compose.yml" ]]; then
        print_warning "docker-compose.yml not found"
        return 1
    fi

    local services=("app" "postgres" "redis")

    for service in "${services[@]}"; do
        log_verbose "Checking service: $service"

        local container_id=$(docker-compose -f docker-compose.yml ps -q "$service" 2>/dev/null)

        if [[ -z "$container_id" ]]; then
            print_error "$service service not found"
            continue
        fi

        local container_status=$(docker inspect --format='{{.State.Status}}' "$container_id" 2>/dev/null || echo "unknown")
        local container_health=$(docker inspect --format='{{.State.Health.Status}}' "$container_id" 2>/dev/null || echo "none")

        log_verbose "$service: status=$container_status, health=$container_health"

        if [[ "$container_status" == "running" ]]; then
            if [[ "$container_health" == "healthy" ]] || [[ "$container_health" == "none" ]]; then
                print_success "$service service"

                # Show resource usage if verbose
                if [[ "$VERBOSE" == "true" ]]; then
                    local stats=$(docker stats --no-stream --format "table {{.CPUPerc}}\t{{.MemUsage}}" "$container_id" 2>/dev/null || echo "Stats unavailable")
                    log_verbose "$service resources: $stats"
                fi
            else
                print_warning "$service service (health: $container_health)"
            fi
        else
            print_error "$service service (status: $container_status)"

            if [[ "$VERBOSE" == "true" ]]; then
                log_verbose "Recent logs for $service:"
                docker-compose -f docker-compose.yml logs --tail=5 "$service" 2>/dev/null || echo "Logs unavailable"
            fi
        fi
    done
}

# Check application endpoints
check_application_endpoints() {
    print_header "Checking Application Endpoints"

    local endpoints=(
        "Health Check:$HEALTH_CHECK_URL"
        "API Info:http://localhost:8080/api/v1"
        "Swagger UI:http://localhost:8080/swagger/index.html"
    )

    for endpoint_info in "${endpoints[@]}"; do
        IFS=':' read -r name url <<< "$endpoint_info"

        log_verbose "Checking endpoint: $name ($url)"

        local retry_count=0
        local endpoint_healthy=false

        while [[ $retry_count -lt $RETRY_COUNT ]]; do
            if check_http_endpoint "$name" "$url" "$TIMEOUT"; then
                endpoint_healthy=true
                break
            fi

            ((retry_count++))
            if [[ $retry_count -lt $RETRY_COUNT ]]; then
                log_verbose "Retrying $name in $RETRY_DELAY seconds... (attempt $retry_count/$RETRY_COUNT)"
                sleep $RETRY_DELAY
            fi
        done

        if [[ "$endpoint_healthy" != "true" ]]; then
            print_error "$name after $RETRY_COUNT attempts"
        fi
    done
}

# Check dependencies (PostgreSQL, Redis)
check_dependencies() {
    print_header "Checking Dependencies"

    # Check PostgreSQL
    check_postgresql() {
        log_verbose "Checking PostgreSQL connection..."

        local postgres_container=$(docker-compose -f docker-compose.yml ps -q postgres 2>/dev/null)

        if [[ -n "$postgres_container" ]]; then
            # Use docker exec to check PostgreSQL health
            if docker exec "$postgres_container" pg_isready -U postgres -d golang_template_dev >/dev/null 2>&1; then
                print_success "PostgreSQL connection"

                if [[ "$VERBOSE" == "true" ]]; then
                    local db_stats=$(docker exec "$postgres_container" psql -U postgres -d golang_template_dev -c "SELECT count(*) FROM information_schema.tables WHERE table_schema='public';" -t 2>/dev/null | tr -d ' ' || echo "0")
                    log_verbose "PostgreSQL tables count: $db_stats"
                fi
            else
                print_error "PostgreSQL connection"
            fi
        else
            print_warning "PostgreSQL container not found"
        fi
    }

    # Check Redis
    check_redis() {
        log_verbose "Checking Redis connection..."

        local redis_container=$(docker-compose -f docker-compose.yml ps -q redis 2>/dev/null)

        if [[ -n "$redis_container" ]]; then
            # Use docker exec to check Redis health
            if docker exec "$redis_container" redis-cli ping 2>/dev/null | grep -q "PONG"; then
                print_success "Redis connection"

                if [[ "$VERBOSE" == "true" ]]; then
                    local redis_info=$(docker exec "$redis_container" redis-cli info server 2>/dev/null | head -5 || echo "Info unavailable")
                    log_verbose "Redis info: $redis_info"
                fi
            else
                print_error "Redis connection"
            fi
        else
            print_warning "Redis container not found"
        fi
    }

    check_postgresql
    check_redis
}

# Check system resources
check_system_resources() {
    print_header "Checking System Resources"

    # Check disk space
    local disk_usage=$(df -h . 2>/dev/null | awk 'NR==2 {print $5}' | sed 's/%//' || echo "0")
    if [[ $disk_usage -lt 90 ]]; then
        print_success "Disk space (${disk_usage}% used)"
    else
        print_warning "Disk space (${disk_usage}% used)"
    fi

    # Check memory (Linux only)
    if [[ -f "/proc/meminfo" ]]; then
        local mem_available=$(grep MemAvailable /proc/meminfo | awk '{print $2}')
        local mem_total=$(grep MemTotal /proc/meminfo | awk '{print $2}')
        local mem_usage=$((100 - (mem_available * 100 / mem_total)))

        if [[ $mem_usage -lt 90 ]]; then
            print_success "Memory usage (${mem_usage}%)"
        else
            print_warning "Memory usage (${mem_usage}%)"
        fi
    fi

    # Check Docker resources
    if command_exists docker; then
        local docker_images=$(docker images --format "table {{.Repository}}:{{.Tag}}" | wc -l)
        local docker_containers=$(docker ps -a --format "table {{.Names}}" | wc -l)
        log_verbose "Docker resources: $docker_images images, $docker_containers containers"
    fi
}

# Check configuration files
check_configuration() {
    print_header "Checking Configuration"

    local config_files=(
        "configs/production.yaml"
        "docker-compose.yml"
        "go.mod"
    )

    for config_file in "${config_files[@]}"; do
        if [[ -f "$config_file" ]]; then
            print_success "$config_file exists"

            if [[ "$VERBOSE" == "true" ]]; then
                local file_size=$(stat -f%z "$config_file" 2>/dev/null || stat -c%s "$config_file" 2>/dev/null || echo "0")
                local file_mtime=$(stat -f%Sm -t%Y-%m-%d\ %H:%M:%S "$config_file" 2>/dev/null || stat -c%y "$config_file" 2>/dev/null | cut -d. -f1 || echo "unknown")
                log_verbose "$config_file: ${file_size} bytes, modified: $file_mtime"
            fi
        else
            print_error "$config_file missing"
        fi
    done
}

# Generate JSON report
generate_json_report() {
    local timestamp=$(date -u +"%Y-%m-%dT%H:%M:%S.%3NZ")

    cat << EOF
{
  "timestamp": "$timestamp",
  "overall_status": "$OVERALL_STATUS",
  "health_check_url": "$HEALTH_CHECK_URL",
  "timeout": $TIMEOUT,
  "retry_count": $RETRY_COUNT,
  "results": {
EOF

    local first=true
    for key in "${!RESULTS[@]}"; do
        if [[ "$first" == "true" ]]; then
            first=false
        else
            echo ","
        fi
        echo "    \"$key\": \"${RESULTS[$key]}\""
    done

    cat << EOF
  },
  "configuration": {
    "check_services": $CHECK_SERVICES,
    "check_endpoints": $CHECK_ENDPOINTS,
    "check_dependencies": $CHECK_DEPENDENCIES,
    "verbose": $VERBOSE
  }
}
EOF
}

# Display summary
display_summary() {
    if [[ "$JSON_OUTPUT" == "true" ]]; then
        generate_json_report
        return
    fi

    print_header "Health Check Summary"

    echo -e "Overall Status: ${BLUE}$OVERALL_STATUS${NC}"
    echo ""

    echo -e "${BLUE}Detailed Results:${NC}"
    for key in "${!RESULTS[@]}"; do
        local status="${RESULTS[$key]}"
        local color=""

        case "$status" in
            "PASSING")
                color="$GREEN"
                ;;
            "WARNING")
                color="$YELLOW"
                ;;
            "FAILING")
                color="$RED"
                ;;
        esac

        echo -e "  ${color}$status${NC} - $key"
    done

    echo ""

    if [[ "$OVERALL_STATUS" == "PASSING" ]]; then
        echo -e "${GREEN}✓ All health checks passed!${NC}"
        exit 0
    elif [[ "$OVERALL_STATUS" == "WARNING" ]]; then
        echo -e "${YELLOW}⚠ Health checks completed with warnings${NC}"
        exit 1
    else
        echo -e "${RED}✗ Health checks failed!${NC}"
        exit 2
    fi
}

# Main health check function
main() {
    if [[ "$JSON_OUTPUT" != "true" ]]; then
        echo -e "${PURPLE}"
        echo "  ____              _   _   _      _   _            "
        echo " |  _ \            | | | | | |    | | | |           "
        echo " | |_) | __ _ _ __ | | | | | |    | | | | ___  _ __ "
        echo " |  _ < / _' | '_ \| | | | | |    | | | |/ _ \| '__|"
        echo " | |_) | (_| | | | | | | | | |____| |_| | (_) | |   "
        echo " |____/ \__,_|_| |_|_|_|_|_|______|\___/ \___/|_|   "
        echo ""
        echo -e "          Health Check${NC}"
        echo ""
    fi

    # Run health checks
    if [[ "$CHECK_SERVICES" == "true" ]]; then
        check_docker_services
    fi

    if [[ "$CHECK_ENDPOINTS" == "true" ]]; then
        check_application_endpoints
    fi

    if [[ "$CHECK_DEPENDENCIES" == "true" ]]; then
        check_dependencies
    fi

    # Always run these checks
    check_system_resources
    check_configuration

    # Display results
    display_summary
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --url)
            HEALTH_CHECK_URL="$2"
            shift 2
            ;;
        --timeout)
            TIMEOUT="$2"
            shift 2
            ;;
        --retry-count)
            RETRY_COUNT="$2"
            shift 2
            ;;
        --retry-delay)
            RETRY_DELAY="$2"
            shift 2
            ;;
        --verbose|-v)
            VERBOSE=true
            shift
            ;;
        --json)
            JSON_OUTPUT=true
            shift
            ;;
        --no-services)
            CHECK_SERVICES=false
            shift
            ;;
        --no-endpoints)
            CHECK_ENDPOINTS=false
            shift
            ;;
        --no-dependencies)
            CHECK_DEPENDENCIES=false
            shift
            ;;
        --help)
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --url URL              Health check URL (default: http://localhost:8080/api/v1/health)"
            echo "  --timeout SEC          Request timeout in seconds (default: 10)"
            echo "  --retry-count NUM      Number of retries (default: 3)"
            echo "  --retry-delay SEC      Delay between retries in seconds (default: 2)"
            echo "  --verbose, -v          Enable verbose output"
            echo "  --json                 Output results in JSON format"
            echo "  --no-services          Skip Docker services check"
            echo "  --no-endpoints         Skip application endpoints check"
            echo "  --no-dependencies      Skip dependencies check"
            echo "  --help                 Show this help message"
            echo ""
            echo "Exit codes:"
            echo "  0  All checks passed"
            echo "  1  Some checks passed with warnings"
            echo "  2  Some checks failed"
            echo ""
            echo "Examples:"
            echo "  $0                     # Run all health checks"
            echo "  $0 --verbose           # Run with verbose output"
            echo "  $0 --json              # Output results in JSON"
            echo "  $0 --no-dependencies   # Skip dependency checks"
            echo ""
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            echo "Use --help for usage information"
            exit 1
            ;;
    esac
done

# Run main function
main "$@"