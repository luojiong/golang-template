@echo off
setlocal enabledelayedexpansion

REM Golang Template Development Startup Script for Windows
REM Compatible with Windows Command Prompt and PowerShell

echo.
echo  ____              _   _   _      _   _
echo ^|  _ \            ^| ^| ^| ^| ^| ^|    ^| ^| ^| ^|
echo ^| ^|_^) ^| __ _ _ __ ^| ^| ^| ^| ^| ^|    ^| ^| ^| ^| ___  _ __
echo ^|  _ ^< ^/ _' ^| '_ \^| ^| ^| ^| ^| ^|    ^| ^| ^| ^|/ _ \^| '__^|
echo ^| ^|_^) ^| (^|_^| ^| ^| ^| ^| ^| ^| ^| ^|____^| ^| ^| ^| ^| (_) ^| ^|
echo ^|____/ \__,_^|_^|_^|_^|_^|_^|_^|_^|______^|_^|_^|_^| \___/^|_^|
echo.
echo          Development Environment for Windows
echo.

REM Check if Go is installed
where go >nul 2>nul
if %errorlevel% neq 0 (
    echo [ERROR] Go is not installed or not in PATH
    echo Please install Go from: https://golang.org/dl/
    pause
    exit /b 1
)

for /f "tokens=3" %%i in ('go version') do set GO_VERSION=%%i
echo [SUCCESS] Go found: !GO_VERSION!

REM Set environment variables
set APP_ENV=development
set GIN_MODE=debug
echo [INFO] Environment set to development mode

REM Check if .env file exists
if not exist .env (
    echo [INFO] Creating .env file with defaults
    (
        echo # Development Environment Variables
        echo APP_ENV=development
        echo APP_SERVER_HOST=localhost
        echo APP_SERVER_PORT=8080
        echo APP_JWT_SECRET_KEY=dev-secret-key-change-in-production
        echo APP_DATABASE_HOST=localhost
        echo APP_DATABASE_PORT=5432
        echo APP_DATABASE_USER=postgres
        echo APP_DATABASE_PASSWORD=password
        echo APP_DATABASE_NAME=golang_template_dev
        echo APP_DATABASE_SSLMODE=disable
    ) > .env
    echo [SUCCESS] .env file created
) else (
    echo [SUCCESS] .env file found
)

REM Check if Docker is available
where docker >nul 2>nul
if %errorlevel% equ 0 (
    echo [SUCCESS] Docker found
    for /f "tokens=3" %%i in ('docker --version') do set DOCKER_VERSION=%%i
    echo [INFO] Docker version: !DOCKER_VERSION!

    REM Check if PostgreSQL container is running
    docker ps -q -f name=postgres-dev >nul 2>nul
    if %errorlevel% equ 0 (
        echo [SUCCESS] PostgreSQL container is already running
    ) else (
        echo [INFO] Starting PostgreSQL container...
        docker stop postgres-dev >nul 2>nul
        docker rm postgres-dev >nul 2>nul

        docker run --name postgres-dev ^
            -e POSTGRES_DB=golang_template_dev ^
            -e POSTGRES_USER=postgres ^
            -e POSTGRES_PASSWORD=password ^
            -p 5432:5432 ^
            -d postgres:15-alpine

        echo [INFO] Waiting for PostgreSQL to be ready...
        timeout /t 10 /nobreak >nul
        echo [SUCCESS] PostgreSQL is ready
    )
) else (
    echo [WARNING] Docker not found, please set up PostgreSQL manually
    echo Database: golang_template_dev
    echo Connection: postgresql://postgres:password@localhost:5432/golang_template_dev
)

REM Install dependencies
echo [INFO] Downloading Go modules...
go mod download
if %errorlevel% neq 0 (
    echo [ERROR] Failed to download dependencies
    pause
    exit /b 1
)
echo [SUCCESS] Dependencies downloaded

go mod tidy
if %errorlevel% neq 0 (
    echo [ERROR] Failed to tidy go.mod
    pause
    exit /b 1
)
echo [SUCCESS] go.mod tidied

REM Install development tools
where swag >nul 2>nul
if %errorlevel% neq 0 (
    echo [INFO] Installing swag...
    go install github.com/swaggo/swag/cmd/swag@latest
    echo [SUCCESS] swag installed
) else (
    echo [SUCCESS] swag already installed
)

REM Generate Swagger documentation
echo [INFO] Generating Swagger documentation...
swag init -g cmd/api/main.go -o docs >nul 2>nul
if %errorlevel% equ 0 (
    echo [SUCCESS] Swagger documentation generated
) else (
    echo [WARNING] Failed to generate Swagger docs ^(non-critical^)
)

REM Display startup information
echo.
echo =====================================
echo Starting Development Server
echo =====================================
echo.
echo Environment variables:
echo   APP_ENV: %APP_ENV%
echo   GIN_MODE: %GIN_MODE%
echo   APP_SERVER_HOST: %APP_SERVER_HOST%
echo   APP_SERVER_PORT: %APP_SERVER_PORT%
echo.
echo Server will be available at: http://localhost:8080
echo Swagger UI will be available at: http://localhost:8080/swagger/index.html
echo Health check: http://localhost:8080/api/v1/health
echo.
echo Press Ctrl+C to stop the server
echo.

REM Start the server
go run cmd/api/main.go


pause