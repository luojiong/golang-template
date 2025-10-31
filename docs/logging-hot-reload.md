# 日志配置热重载功能使用指南

本文档介绍如何使用日志配置热重载功能，该功能允许在不重启应用程序的情况下动态更新日志配置。

## 功能概述

日志配置热重载功能包含以下特性：

- **动态配置更新**：支持日志级别、格式、输出目标等配置的实时更新
- **文件监控**：自动监控配置文件变更，无需手动触发
- **类型安全**：使用反射确保类型安全的方法调用
- **错误处理**：完善的错误处理和降级机制
- **变更日志**：详细记录配置变更事件和结果

## 集成方法

### 1. 在main.go中集成

```go
package main

import (
    "log"
    "go-server/internal/config"
    "go-server/internal/logger"
)

func main() {
    // 1. 加载初始配置
    cfg, err := config.LoadConfig()
    if err != nil {
        log.Fatalf("加载配置失败: %v", err)
    }

    // 2. 创建日志管理器
    loggerManager, err := logger.NewManager(cfg.Logging)
    if err != nil {
        log.Fatalf("创建日志管理器失败: %v", err)
    }

    // 3. 启动日志管理器
    if err := loggerManager.Start(); err != nil {
        log.Fatalf("启动日志管理器失败: %v", err)
    }
    defer func() {
        if err := loggerManager.Stop(); err != nil {
            log.Printf("关闭日志管理器时出错: %v", err)
        }
    }()

    // 4. 设置配置文件监控，启用日志配置热重载
    watcher, err := config.WatchConfigFileWithLogger(cfg, loggerManager)
    if err != nil {
        log.Printf("启动配置文件监控失败: %v", err)
        log.Printf("日志配置热重载功能不可用")
    } else {
        log.Printf("日志配置热重载功能已启用")
        defer watcher.Stop()
    }

    // 5. 获取应用程序日志记录器
    appLogger := loggerManager.GetLogger("app")

    // 6. 应用程序继续正常运行
    appLogger.Info(nil, "应用程序启动成功")

    // 当配置文件中的日志配置发生变更时，系统会自动热重载
}
```

### 2. 配置文件示例

```yaml
# configs/development.yaml
mode: development
logging:
  level: "info"           # 可以动态更改为 debug, info, warn, error, fatal
  format: "json"          # 可以动态更改为 json, text
  output: "stdout"        # 可以动态更改为 stdout, file, both
  directory: "./logs"     # 日志文件目录
  buffer_size: 65536      # 缓冲区大小
  flush_interval: 5       # 刷新间隔（秒）
  max_size: 100           # 单个文件最大大小（MB）
  max_backups: 3          # 最大备份文件数
  max_age: 28             # 最大保存天数
  compress: true          # 是否压缩旧文件
```

## 支持的配置变更

以下配置项支持热重载：

| 配置项 | 描述 | 示例变更 |
|--------|------|----------|
| `level` | 日志级别 | `"info"` → `"debug"` |
| `format` | 日志格式 | `"json"` → `"text"` |
| `output` | 输出目标 | `"stdout"` → `"file"` |
| `directory` | 日志目录 | `"./logs"` → `"./app-logs"` |
| `buffer_size` | 缓冲区大小 | `65536` → `131072` |
| `flush_interval` | 刷新间隔 | `5` → `10` |
| `max_size` | 最大文件大小 | `100` → `200` |
| `max_backups` | 最大备份数 | `3` → `5` |
| `max_age` | 最大保存天数 | `28` → `60` |
| `compress` | 压缩设置 | `true` → `false` |

## 使用场景

### 1. 调试时动态调整日志级别

在开发或调试过程中，可以临时将日志级别从 `info` 调整为 `debug`，获取更详细的日志信息，而无需重启应用程序。

```bash
# 编辑配置文件
vim configs/development.yaml

# 将 level: "info" 改为 level: "debug"
# 保存文件后，日志级别立即生效
```

### 2. 生产环境问题排查

在生产环境中遇到问题时，可以临时启用调试日志：

```yaml
logging:
  level: "debug"
  format: "json"
  output: "both"          # 同时输出到控制台和文件
  directory: "./debug-logs"
```

### 3. 性能调优

根据系统负载动态调整缓冲区大小和刷新间隔：

```yaml
logging:
  buffer_size: 131072     # 增大缓冲区
  flush_interval: 10       # 降低刷新频率
```

## 监控和日志

### 配置变更事件

系统会记录所有配置变更事件：

```
2025/10/30 13:07:57 检测到日志配置变更: 级别: info -> debug, 格式: json -> text
2025/10/30 13:07:57 日志配置变更事件: map[
    change_result:success
    config:map[level:debug format:text output:stdout ...]
    event_type:logging_config_change
    timestamp:2025-10-30T05:07:57Z
]
2025/10/30 13:07:57 日志配置热重载成功
```

### 错误处理

如果配置更新失败，系统会记录错误信息并继续使用之前的配置：

```
2025/10/30 13:07:57 日志配置热重载失败: 更新日志配置失败: invalid log level 'invalid'
2025/10/30 13:07:57 日志配置变更事件: map[change_result:failed error:...]
```

## 最佳实践

### 1. 配置验证

在修改配置前，建议先验证配置的正确性：

```yaml
logging:
  level: "debug"         # 确保使用有效的日志级别
  format: "json"         # 确保使用有效的格式
  output: "stdout"       # 确保使用有效的输出目标
```

### 2. 渐进式变更

建议进行渐进式的配置变更，一次只修改一个配置项，便于验证效果。

### 3. 监控影响

在大规模生产环境中，建议监控配置变更对系统性能的影响。

### 4. 备份配置

在修改配置前，建议备份原始配置文件，以便快速回滚。

## 故障排除

### 常见问题

1. **配置变更未生效**
   - 检查配置文件语法是否正确
   - 确认应用程序在开发模式下运行（热重载仅在开发模式下启用）
   - 查看应用程序日志中的错误信息

2. **日志管理器未启动错误**
   - 确保日志管理器已正确初始化和启动
   - 检查日志目录权限

3. **反射调用失败**
   - 确保传递给WatchConfigFileWithLogger的是正确的logger.Manager实例
   - 检查logger.Manager是否实现了所需的方法

### 调试步骤

1. 检查应用程序启动日志
2. 验证配置文件路径和权限
3. 确认配置文件监控是否正常启动
4. 测试手动修改配置文件
5. 查看详细的错误日志

## 技术实现

### 核心组件

- **ConfigWatcher**: 配置文件监控器
- **反射调用**: 使用reflect包动态调用logger.Manager方法
- **配置比较**: 比较新旧配置，避免不必要的更新
- **错误处理**: 完善的错误处理和恢复机制

### 安全考虑

- 热重载功能仅在开发模式下启用
- 生产模式下禁用配置文件监控
- 所有配置变更都有详细的日志记录
- 支持配置验证和错误恢复

这个日志配置热重载功能为开发人员提供了强大的动态配置管理能力，特别适用于开发调试和生产环境的问题排查场景。