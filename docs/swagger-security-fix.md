# Swagger UI 安全策略修复

## 问题描述

访问 `http://localhost:12400/swagger/index.html` 时遇到两个问题：

1. **Permissions-Policy 警告**：
   - `Error with Permissions-Policy header: Unrecognized feature: 'ambient-light-sensor'`
   - `Error with Permissions-Policy header: Unrecognized feature: 'speaker-selection'`
   - `Error with Permissions-Policy header: Unrecognized feature: 'vr'`

2. **Swagger UI 空白页**：由于过于严格的安全策略导致 Swagger UI 无法正常加载。

## 解决方案

### 1. 移除已废弃的 Permissions-Policy 特性

从 `Permissions-Policy` 头部中移除了以下已废弃或不被现代浏览器识别的特性：
- `ambient-light-sensor`
- `speaker-selection`
- `vr`

**修改位置**：`server/internal/middleware/middleware.go` 第 64 行

**修改前**：
```go
c.Header("Permissions-Policy", "geolocation=(), microphone=(), camera=(), payment=(), usb=(), magnetometer=(), gyroscope=(), accelerometer=(), ambient-light-sensor=(), autoplay=(), encrypted-media=(), fullscreen=(), picture-in-picture=(), speaker-selection=(), vr=(), interest-cohort=()")
```

**修改后**：
```go
c.Header("Permissions-Policy", "geolocation=(), microphone=(), camera=(), payment=(), usb=(), magnetometer=(), gyroscope=(), accelerometer=(), autoplay=(), encrypted-media=(), fullscreen=(), picture-in-picture=(), interest-cohort=()")
```

### 2. 为 Swagger 路径放宽安全策略

Swagger UI 需要加载各种资源（脚本、样式、图片等），过于严格的 CORS 策略会导致页面无法正常显示。

**修改内容**：

#### a. 条件性设置 Cross-Origin 头部
对于 `/swagger/*` 路径，不设置以下严格的头部：
- `Cross-Origin-Embedder-Policy: require-corp`
- `Cross-Origin-Resource-Policy: same-origin`
- `Cross-Origin-Opener-Policy: same-origin`

#### b. 优化 Swagger CSP 策略

**修改前**：
```go
return "default-src 'self' 'unsafe-inline'; script-src 'self' 'unsafe-inline' 'unsafe-eval'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; font-src 'self'; connect-src 'self'; frame-ancestors 'none'"
```

**修改后**：
```go
return "default-src 'self'; script-src 'self' 'unsafe-inline' 'unsafe-eval'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; font-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'self'; frame-ancestors 'none'"
```

**改进点**：
- 移除了 `default-src 'unsafe-inline'`（更安全）
- 添加了 `font-src 'self' data:`（支持内嵌字体）
- 添加了 `object-src 'none'`（防止插件加载）
- 添加了 `base-uri 'self'`（防止 base 标签劫持）

### 3. 更新测试用例

更新了所有相关的测试用例以反映安全策略的变化：
- 移除了对已废弃 Permissions-Policy 特性的测试
- 更新了 Swagger 路径的测试预期（不再期望 Cross-Origin 头部）

## 测试结果

所有安全中间件测试通过：
```bash
cd server/internal/middleware
go test -v -run TestSecurityHeadersMiddleware
# PASS: TestSecurityHeadersMiddleware
# PASS: TestSecurityHeadersMiddleware_CSPPreventsXSS
# PASS: TestSecurityHeadersMiddleware_HSTSNotSetOnHTTP
# PASS: TestEnhancedSecurityHeaders
```

## 安全影响

### 保持的安全措施
✅ API 端点仍然使用严格的安全策略
✅ HSTS、XSS 保护、内容类型嗅探防护等基本安全头仍然启用
✅ 生产环境的 CSP 策略仍然严格
✅ Permissions-Policy 仍然限制敏感功能访问

### 针对 Swagger 的放宽
⚠️ `/swagger/*` 路径不设置 Cross-Origin 限制头部
⚠️ Swagger CSP 允许 `unsafe-inline` 和 `unsafe-eval`

**注意**：这些放宽仅针对 Swagger 文档页面，不影响 API 端点的安全性。在生产环境中，建议：
1. 将 Swagger UI 部署在独立的子域名
2. 或者在生产环境完全禁用 Swagger UI
3. 使用反向代理限制对 `/swagger/*` 的访问

## 使用方法

修改完成后，重启服务器并访问：
```
http://localhost:12400/swagger/index.html
```

现在应该能够正常看到 Swagger UI 界面，并且浏览器控制台不会再显示 Permissions-Policy 警告。

