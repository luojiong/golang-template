# 故障排除指南

## 常见问题及解决方案

### 1. PostgreSQL Docker 容器启动失败

**错误信息：**
```
unknown flag: --lc-collate
```

**解决方案：**
这个错误是因为 PostgreSQL Docker 镜像不支持某些 locale 参数。我们已经在脚本中移除了这些参数。

**手动解决：**
```bash
# 停止并删除现有容器
docker stop postgres-dev
docker rm postgres-dev

# 使用简化命令重新启动
docker run --name postgres-dev \
  -e POSTGRES_DB=golang_template_dev \
  -e POSTGRES_USER=postgres \
  -e POSTGRES_PASSWORD=caine \
  -p 5432:5432 \
  -d postgres:15-alpine
```

### 2. 端口被占用

**错误信息：**
```
bind: address already in use
```

**解决方案：**

1. **查找占用端口的进程：**
   ```bash
   # Windows
   netstat -ano | findstr :8080

   # Git Bash/WSL
   lsof -i :8080
   ```

2. **停止占用端口的进程**，或者更改端口：
   ```bash
   # 编辑 .env 文件
   APP_SERVER_PORT=8081
   ```

### 3. 数据库连接失败

**错误信息：**
```
failed to connect to database
```

**解决方案：**

1. **检查 PostgreSQL 是否正在运行：**
   ```bash
   docker ps | grep postgres-dev
   ```

2. **手动测试数据库连接：**
   ```bash
   docker exec postgres-dev psql -U postgres -d golang_template_dev -c "SELECT 1;"
   ```

3. **检查数据库配置：**
   ```bash
   # 确认配置文件中的密码
   cat configs/development.yaml | grep password
   ```

### 4. Go 模块下载失败

**错误信息：**
```
go: module download failed
```

**解决方案：**

1. **设置 Go 代理（中国用户）：**
   ```bash
   go env -w GOPROXY=https://goproxy.cn,direct
   go env -w GOSUMDB=sum.golang.google.cn
   ```

2. **清理模块缓存：**
   ```bash
   go clean -modcache
   go mod download
   ```

### 5. Git Bash 颜色显示问题

**错误信息：**
```
\033[1;33mDo you want to stop...
```

**解决方案：**

1. **更新 Git Bash** 到最新版本
2. **使用 Windows Terminal** 替代默认终端
3. **或者使用简化脚本**：
   ```bash
   ./scripts/simple-start.sh
   ```

### 6. PowerShell 执行策略错误

**错误信息：**
```
cannot be loaded because running scripts is disabled
```

**解决方案：**
```powershell
Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser
```

### 7. Docker 权限问题

**错误信息：**
```
permission denied while trying to connect to the Docker daemon socket
```

**解决方案：**

1. **确保 Docker Desktop 正在运行**
2. **以管理员身份运行终端**
3. **重启 Docker Desktop**

## 推荐启动顺序

### 方案 1：使用简化脚本（推荐）
```bash
# 1. 手动启动 PostgreSQL（如果需要）
docker run --name postgres-dev \
  -e POSTGRES_DB=golang_template_dev \
  -e POSTGRES_USER=postgres \
  -e POSTGRES_PASSWORD=caine \
  -p 5432:5432 \
  -d postgres:15-alpine

# 2. 运行简化启动脚本
./scripts/simple-start.sh
```

### 方案 2：使用 Docker 脚本
```bash
./scripts/docker-start.sh
```

### 方案 3：完全手动
```bash
# 1. 安装依赖
go mod download
go mod tidy

# 2. 安装工具
go install github.com/swaggo/swag/cmd/swag@latest

# 3. 生成文档
swag init -g cmd/api/main.go -o docs

# 4. 设置环境
export APP_ENV=development
export GIN_MODE=debug

# 5. 启动服务器
go run cmd/api/main.go
```

## 环境验证

### 检查所有依赖
```bash
# 检查 Go
go version

# 检查 Docker
docker --version

# 检查 Git
git --version

# 检查 PostgreSQL 连接
docker exec postgres-dev pg_isready
```

### 测试 API
```bash
# 健康检查
curl http://localhost:8080/api/v1/health

# API 信息
curl http://localhost:8080/api/v1
```

## 寻求帮助

如果问题仍然存在：

1. **检查日志：**
   ```bash
   # 查看应用日志
   go run cmd/api/main.go

   # 查看 PostgreSQL 日志
   docker logs postgres-dev
   ```

2. **提供详细信息：**
   - 操作系统版本
   - Go 版本
   - Docker 版本
   - 完整的错误信息
   - 使用的脚本名称

3. **尝试不同的启动方式**，如果一种方法失败，尝试另一种。

## 配置文件位置

- **主配置**: `configs/development.yaml`
- **环境变量**: `.env`
- **数据库配置**: `configs/development.yaml` 中的 `database` 部分
- **服务器配置**: `configs/development.yaml` 中的 `server` 部分

## 默认账户

启动后，可以使用以下账户测试：

- **管理员**: `admin@example.com` / `password`
- **普通用户**: `user@example.com` / `password`