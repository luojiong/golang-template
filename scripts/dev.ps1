# Golang Template Development Startup Script for PowerShell
# Compatible with Windows PowerShell and PowerShell Core

# Enable colorful output
$Host.UI.RawUI.WindowTitle = "Golang Template - Development Environment"

function Write-ColorOutput {
    param(
        [string]$Message,
        [ConsoleColor]$Color = "White"
    )
    Write-Host $Message -ForegroundColor $Color
}

function Write-Header {
    param([string]$Title)
    Write-ColorOutput "===================================" "Magenta"
    Write-ColorOutput $Title "Magenta"
    Write-ColorOutput "===================================" "Magenta"
}

function Write-Status {
    param([string]$Message, [string]$Type = "INFO")

    switch ($Type) {
        "SUCCESS" { Write-ColorOutput "[SUCCESS] $Message" "Green" }
        "WARNING" { Write-ColorOutput "[WARNING] $Message" "Yellow" }
        "ERROR"   { Write-ColorOutput "[ERROR] $Message" "Red" }
        default    { Write-ColorOutput "[INFO] $Message" "Blue" }
    }
}

# Display ASCII art
Write-Host @"
  ____              _   _   _      _   _
 |  _ \            | | | | | |    | | | |
 | |_) | __ _ _ __ | | | | | |    | | | | ___  _ __
 |  _ < / _` | '_ \| | | | | |    | | | |/ _ \| '__|
 | |_) | (_| | | | | | | | | |____| | | | (_) | |
 |____/ \__,_|_| |_|_|_|_|______|\___/ \___/|_|
"@ -ForegroundColor "Magenta"

Write-ColorOutput "          Development Environment for PowerShell" "Cyan"
Write-Host ""

# Check prerequisites
Write-Header "Checking Prerequisites"

# Check Go
$goVersion = & go version 2>$null
if ($LASTEXITCODE -eq 0) {
    $version = ($goVersion -split ' ')[2]
    Write-Status "Go found: $version" "SUCCESS"
} else {
    Write-Status "Go is not installed or not in PATH" "ERROR"
    Write-Host "Please install Go from: https://golang.org/dl/"
    Read-Host "Press Enter to exit"
    exit 1
}

# Check Docker
$dockerAvailable = Get-Command docker -ErrorAction SilentlyContinue
if ($dockerAvailable) {
    $dockerVersion = & docker --version
    $version = ($dockerVersion -split ',')[0] -replace 'Docker version ', ''
    Write-Status "Docker found: $version" "SUCCESS"
} else {
    Write-Status "Docker not found, you'll need to set up PostgreSQL manually" "WARNING"
}

# Check Docker Compose
$composeAvailable = Get-Command docker-compose -ErrorAction SilentlyContinue
if ($composeAvailable) {
    $composeVersion = & docker-compose --version
    $version = ($composeVersion -split ',')[0] -replace 'docker-compose version ', ''
    Write-Status "Docker Compose found: $version" "SUCCESS"
} else {
    Write-Status "Docker Compose not found" "WARNING"
}

# Setup environment
Write-Header "Setting Up Environment"

$env:APP_ENV = "development"
$env:GIN_MODE = "debug"
Write-Status "Environment set to: $($env:APP_ENV)" "SUCCESS"

# Create .env file if it doesn't exist
if (-not (Test-Path ".env")) {
    Write-Status "Creating .env file with defaults" "INFO"
    @"
# Development Environment Variables
APP_ENV=development
APP_SERVER_HOST=localhost
APP_SERVER_PORT=8080
APP_JWT_SECRET_KEY=dev-secret-key-change-in-production
APP_DATABASE_HOST=localhost
APP_DATABASE_PORT=5432
APP_DATABASE_USER=postgres
APP_DATABASE_PASSWORD=password
APP_DATABASE_NAME=golang_template_dev
APP_DATABASE_SSLMODE=disable
"@ | Out-File -FilePath ".env" -Encoding UTF8
    Write-Status ".env file created" "SUCCESS"
} else {
    Write-Status ".env file found" "SUCCESS"
}

# Setup database using Docker
Write-Header "Setting Up Database with Docker"

if ($dockerAvailable) {
    # Check if PostgreSQL container is running
    $postgresContainer = & docker ps -q -f "name=postgres-dev" 2>$null

    if ($postgresContainer) {
        Write-Status "PostgreSQL container is already running" "SUCCESS"
    } else {
        Write-Status "Starting PostgreSQL container..." "INFO"

        # Stop and remove existing container if it exists
        & docker stop postgres-dev 2>$null | Out-Null
        & docker rm postgres-dev 2>$null | Out-Null

        # Start new PostgreSQL container
        & docker run --name postgres-dev `
            -e POSTGRES_DB=golang_template_dev `
            -e POSTGRES_USER=postgres `
            -e POSTGRES_PASSWORD=password `
            -p 5432:5432 `
            -d postgres:15-alpine | Out-Null

        # Wait for PostgreSQL to be ready
        Write-Status "Waiting for PostgreSQL to be ready..." "INFO"
        $ready = $false
        for ($i = 1; $i -le 30; $i++) {
            try {
                & docker exec postgres-dev pg_isready -q 2>$null | Out-Null
                if ($LASTEXITCODE -eq 0) {
                    $ready = $true
                    break
                }
            } catch {
                # Container might not be ready yet
            }
            Write-Host -NoNewline "."
            Start-Sleep -Seconds 1
        }
        Write-Host ""

        if ($ready) {
            Write-Status "PostgreSQL is ready" "SUCCESS"
        } else {
            Write-Status "PostgreSQL failed to start within timeout" "WARNING"
        }
    }

    # Show database connection info
    Write-Status "Database connection info:" "INFO"
    Write-Host "  Host: localhost:5432"
    Write-Host "  Database: golang_template_dev"
    Write-Host "  User: postgres"
    Write-Host "  Password: password"
} else {
    Write-Status "Docker not available, please set up PostgreSQL manually" "WARNING"
    Write-Host "Database: golang_template_dev"
    Write-Host "Connection: postgresql://postgres:password@localhost:5432/golang_template_dev"
}

# Install dependencies
Write-Header "Installing Dependencies"

if (Test-Path "go.mod") {
    Write-Status "Downloading Go modules..." "INFO"
    & go mod download
    if ($LASTEXITCODE -eq 0) {
        Write-Status "Dependencies downloaded" "SUCCESS"
    } else {
        Write-Status "Failed to download dependencies" "ERROR"
        Read-Host "Press Enter to exit"
        exit 1
    }

    Write-Status "Tidying go.mod..." "INFO"
    & go mod tidy
    if ($LASTEXITCODE -eq 0) {
        Write-Status "go.mod tidied" "SUCCESS"
    } else {
        Write-Status "Failed to tidy go.mod" "ERROR"
        Read-Host "Press Enter to exit"
        exit 1
    }
} else {
    Write-Status "go.mod file not found" "ERROR"
    Read-Host "Press Enter to exit"
    exit 1
}

# Install development tools
Write-Header "Installing Development Tools"

# Install swag
$swagAvailable = Get-Command swag -ErrorAction SilentlyContinue
if (-not $swagAvailable) {
    Write-Status "Installing swag..." "INFO"
    & go install github.com/swaggo/swag/cmd/swag@latest
    Write-Status "swag installed" "SUCCESS"
} else {
    Write-Status "swag already installed" "SUCCESS"
}

# Install golangci-lint
$golangciAvailable = Get-Command golangci-lint -ErrorAction SilentlyContinue
if (-not $golangciAvailable) {
    Write-Status "Installing golangci-lint..." "INFO"
    & go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
    Write-Status "golangci-lint installed" "SUCCESS"
} else {
    Write-Status "golangci-lint already installed" "SUCCESS"
}

# Generate Swagger documentation
Write-Header "Generating Swagger Documentation"

Write-Status "Generating Swagger docs..." "INFO"
& swag init -g cmd/api/main.go -o docs 2>$null
if ($LASTEXITCODE -eq 0) {
    Write-Status "Swagger documentation generated" "SUCCESS"
} else {
    Write-Status "Failed to generate Swagger docs (non-critical)" "WARNING"
}

# Start the development server
Write-Header "Starting Development Server"

Write-Status "Environment variables:" "INFO"
Write-Host "  APP_ENV: $($env:APP_ENV)"
Write-Host "  GIN_MODE: $($env:GIN_MODE)"
Write-Host "  APP_SERVER_HOST: $($env:APP_SERVER_HOST)"
Write-Host "  APP_SERVER_PORT: $($env:APP_SERVER_PORT)"

Write-Host ""
Write-Status "Server will be available at: http://localhost:8080" "SUCCESS"
Write-Status "Swagger UI will be available at: http://localhost:8080/swagger/index.html" "SUCCESS"
Write-Status "Health check: http://localhost:8080/api/v1/health" "SUCCESS"

Write-Host ""
Write-ColorOutput "Press Ctrl+C to stop the server" "Cyan"
Write-Host ""

# Cleanup function
$cleanupScript = {
    Write-Host ""
    Write-Header "Cleanup"

    if ($dockerAvailable) {
        $response = Read-Host "Do you want to stop the PostgreSQL container? (y/N)"
        if ($response -match '^[Yy]') {
            Write-Status "Stopping PostgreSQL container..." "INFO"
            & docker stop postgres-dev 2>$null | Out-Null
            & docker rm postgres-dev 2>$null | Out-Null
            Write-Status "PostgreSQL container stopped and removed" "SUCCESS"
        }
    }

    Write-Status "Cleanup completed" "SUCCESS"
}

# Set up Ctrl+C handler
$originalErrorActionPreference = $ErrorActionPreference
$ErrorActionPreference = "Stop"

try {
    # Start the application
    & go run cmd/api/main.go
} catch [System.Management.Automation.HaltCommandException] {
    # User pressed Ctrl+C
    Write-Host ""
    Write-Status "Server stopped by user" "INFO"
} catch {
    Write-Status "Error occurred: $($_.Exception.Message)" "ERROR"
} finally {
    # Run cleanup
    & $cleanupScript
    $ErrorActionPreference = $originalErrorActionPreference
}