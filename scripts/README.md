# Development Scripts

This directory contains startup scripts for the Golang Template development environment, optimized for different Windows environments.

## Available Scripts

### 1. `dev.sh` - Git Bash / WSL / Linux / macOS (Recommended)
**Best for**: Windows with Git Bash, WSL, Linux, or macOS

**Features**:
- ✅ Full color output with emojis
- ✅ Comprehensive prerequisite checking
- ✅ Automatic Docker setup
- ✅ Development tools installation
- ✅ Swagger documentation generation
- ✅ Database health checks
- ✅ Graceful cleanup on exit
- ✅ Interactive prompts

**Usage**:
```bash
# Make it executable (Linux/macOS/WSL)
chmod +x scripts/dev.sh

# Run the script
./scripts/dev.sh
```

**Windows Git Bash**:
```bash
# Simply run from Git Bash
./scripts/dev.sh
```

### 2. `dev.bat` - Windows Command Prompt
**Best for**: Windows Command Prompt

**Features**:
- ✅ Basic prerequisite checking
- ✅ Docker setup
- ✅ Dependency installation
- ✅ Swagger documentation generation
- ✅ Interactive cleanup
- ✅ Simple batch syntax

**Usage**:
```cmd
# From Command Prompt
scripts\dev.bat

# Or from PowerShell
cmd /c scripts\dev.bat
```

### 3. `dev.ps1` - Windows PowerShell
**Best for**: Windows PowerShell or PowerShell Core

**Features**:
- ✅ Modern PowerShell features
- ✅ Structured error handling
- ✅ Colorized output
- ✅ Docker integration
- ✅ Comprehensive setup
- ✅ Graceful cleanup on Ctrl+C

**Usage**:
```powershell
# From PowerShell
.\scripts\dev.ps1

# Allow script execution if needed
Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser
```

## Script Comparison

| Feature | dev.sh | dev.bat | dev.ps1 |
|---------|--------|---------|---------|
| Color Output | ✅ Full | ⚠️ Limited | ✅ Full |
| Git Bash Support | ✅ | ❌ | ❌ |
| PowerShell Support | ❌ | ❌ | ✅ |
| Command Prompt | ❌ | ✅ | ❌ |
| Docker Auto-setup | ✅ | ✅ | ✅ |
| Interactive Cleanup | ✅ | ✅ | ✅ |
| Error Handling | ✅ | ⚠️ Basic | ✅ Advanced |
| Development Tools | ✅ | ⚠️ Basic | ✅ Full |
| Swagger Generation | ✅ | ✅ | ✅ |
| Health Checks | ✅ | ⚠️ Basic | ✅ Full |
| Prerequisites Check | ✅ | ⚠️ Basic | ✅ Full |

## Recommended Usage

### For Windows + Git Bash (Primary Recommendation)
```bash
./scripts/dev.sh
```

### For Windows PowerShell
```powershell
.\scripts\dev.ps1
```

### For Windows Command Prompt
```cmd
scripts\dev.bat
```

## What the Scripts Do

### 1. Prerequisites Check
- ✅ Go installation and version
- ✅ Docker availability (optional)
- ✅ Docker Compose availability (optional)

### 2. Environment Setup
- ✅ Set development environment variables
- ✅ Create `.env` file with defaults
- ✅ Load existing `.env` file if present

### 3. Database Setup
- ✅ Start PostgreSQL container with Docker
- ✅ Wait for database to be ready
- ✅ Provide connection information

### 4. Dependencies & Tools
- ✅ Download Go modules (`go mod download`)
- ✅ Tidy dependencies (`go mod tidy`)
- ✅ Install development tools:
  - `swag` for Swagger documentation
  - `golangci-lint` for linting

### 5. Documentation Generation
- ✅ Generate Swagger documentation
- ✅ Create API documentation files

### 6. Server Startup
- ✅ Set debug mode
- ✅ Display server URLs
- ✅ Start the development server
- ✅ Handle graceful shutdown

### 7. Cleanup
- ✅ Optional PostgreSQL container cleanup
- ✅ Environment cleanup

## Environment Variables

The scripts automatically set these environment variables:

```bash
APP_ENV=development
GIN_MODE=debug
APP_SERVER_HOST=localhost
APP_SERVER_PORT=8080
APP_JWT_SECRET_KEY=dev-secret-key-change-in-production
APP_DATABASE_HOST=localhost
APP_DATABASE_PORT=5432
APP_DATABASE_USER=postgres
APP_DATABASE_PASSWORD=password
APP_DATABASE_NAME=golang_template_dev
APP_DATABASE_SSLMODE=disable
```

## Database Configuration

### Docker Container Configuration
- **Container Name**: `postgres-dev`
- **Database**: `golang_template_dev`
- **User**: `postgres`
- **Password**: `password`
- **Port**: `5432`
- **Image**: `postgres:15-alpine`

### Connection String
```
postgresql://postgres:password@localhost:5432/golang_template_dev
```

## Default Test Users

After setup, these users are automatically created:

### Admin User
- **Email**: `admin@example.com`
- **Password**: `password`
- **Role**: Administrator

### Regular User
- **Email**: `user@example.com`
- **Password**: `password`
- **Role**: User

## Access Points

Once the server starts:

- **API Server**: http://localhost:8080
- **Swagger UI**: http://localhost:8080/swagger/index.html
- **Health Check**: http://localhost:8080/api/v1/health
- **API Info**: http://localhost:8080/api/v1

## Troubleshooting

### Port Already in Use
If port 8080 is already in use:
1. Stop the other service using the port
2. Or change the port in `.env` file:
   ```bash
   APP_SERVER_PORT=8081
   ```

### Docker Issues
If Docker commands fail:
1. Make sure Docker Desktop is running
2. Check Docker permissions
3. Try running Docker as administrator

### Go Module Issues
If Go commands fail:
1. Check Go installation
2. Verify `GOPATH` and `GOROOT` environment variables
3. Try `go clean -modcache` and retry

### PostgreSQL Connection Issues
If database connection fails:
1. Wait a few more seconds for PostgreSQL to start
2. Check if port 5432 is available
3. Verify Docker container is running: `docker ps`

### Permission Issues (PowerShell)
If PowerShell script execution is blocked:
```powershell
Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser
```

### Git Bash Path Issues
If Git Bash can't find commands:
1. Make sure Git Bash is added to PATH
2. Or use full path to Git Bash
3. Verify Go is accessible from Git Bash

## Customization

You can customize the scripts by:

1. **Modifying Environment Variables**: Edit the `.env` file
2. **Changing Database Settings**: Modify database configuration in the script
3. **Adding New Tools**: Update the development tools installation section
4. **Customizing Ports**: Change default ports in the script
5. **Adding Health Checks**: Add additional service health checks

## Manual Setup (Alternative)

If you prefer not to use the scripts, you can manually set up:

1. **Install Dependencies**:
   ```bash
   go mod download
   go mod tidy
   ```

2. **Install Tools**:
   ```bash
   go install github.com/swaggo/swag/cmd/swag@latest
   go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
   ```

3. **Start Database**:
   ```bash
   docker run --name postgres-dev -e POSTGRES_DB=golang_template_dev -e POSTGRES_USER=postgres -e POSTGRES_PASSWORD=password -p 5432:5432 -d postgres:15-alpine
   ```

4. **Generate Documentation**:
   ```bash
   swag init -g cmd/api/main.go -o docs
   ```

5. **Start Server**:
   ```bash
   APP_ENV=development go run cmd/api/main.go
   ```

## Support

If you encounter issues with the scripts:

1. Check the prerequisites section above
2. Try running with elevated/administrator privileges
3. Verify all dependencies are properly installed
4. Check for port conflicts
5. Review Docker container status
6. Open an issue in the repository with details about your environment and the error message