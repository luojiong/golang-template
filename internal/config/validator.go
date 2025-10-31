package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// ValidationError 表示配置验证错误
type ValidationError struct {
	Field   string      // 字段名
	Message string      // 错误消息
	Value   interface{} // 字段值
}

// Error 实现error接口
func (v ValidationError) Error() string {
	return fmt.Sprintf("字段 '%s' 验证失败: %s", v.Field, v.Message)
}

// ValidationResult 包含验证结果
type ValidationResult struct {
	Valid  bool               // 是否有效
	Errors []ValidationError // 错误列表
}

// Validator 提供配置验证功能
type Validator struct {
	config *Config // 配置对象
}

// NewValidator 创建新的配置验证器
func NewValidator(config *Config) *Validator {
	return &Validator{
		config: config,
	}
}

// Validate 验证整个配置
func (v *Validator) Validate() *ValidationResult {
	result := &ValidationResult{
		Valid:  true,
		Errors: []ValidationError{},
	}

	// 验证服务器配置
	v.validateServer(result)

	// 验证数据库配置
	v.validateDatabase(result)

	// 验证认证配置
	v.validateAuth(result)

	// 验证JWT配置
	v.validateJWT(result)

	// 验证Redis配置
	v.validateRedis(result)

	// 验证速率限制配置
	v.validateRateLimit(result)

	// 验证日志配置
	v.validateLogging(result)

	// 验证应用模式
	v.validateMode(result)

	// 根据错误设置有效状态
	result.Valid = len(result.Errors) == 0

	return result
}

// validateServer 验证服务器配置
func (v *Validator) validateServer(result *ValidationResult) {
	server := v.config.Server

	// 验证端口号
	if server.Port == "" {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "server.port",
			Message: "服务器端口是必需的",
			Value:   server.Port,
		})
		result.Valid = false
	} else {
		// 验证端口格式（应为数字）
		if port, err := strconv.Atoi(server.Port); err != nil {
			result.Errors = append(result.Errors, ValidationError{
				Field:   "server.port",
				Message: "服务器端口必须是有效的数字",
				Value:   server.Port,
			})
			result.Valid = false
		} else if port < 1 || port > 65535 {
			result.Errors = append(result.Errors, ValidationError{
				Field:   "server.port",
				Message: "服务器端口必须在1到65535之间",
				Value:   server.Port,
			})
			result.Valid = false
		}
	}

	// 验证主机地址
	if server.Host == "" {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "server.host",
			Message: "服务器主机地址是必需的",
			Value:   server.Host,
		})
		result.Valid = false
	}

	// 验证读取超时时间
	if server.ReadTimeout <= 0 {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "server.read_timeout",
			Message: "服务器读取超时时间必须大于0",
			Value:   server.ReadTimeout,
		})
		result.Valid = false
	}

	// 验证写入超时时间
	if server.WriteTimeout <= 0 {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "server.write_timeout",
			Message: "服务器写入超时时间必须大于0",
			Value:   server.WriteTimeout,
		})
		result.Valid = false
	}
}

// validateDatabase 验证数据库配置
func (v *Validator) validateDatabase(result *ValidationResult) {
	db := v.config.Database

	// 验证数据库主机地址
	if db.Host == "" {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "database.host",
			Message: "数据库主机地址是必需的",
			Value:   db.Host,
		})
		result.Valid = false
	}

	// 验证数据库端口
	if db.Port <= 0 || db.Port > 65535 {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "database.port",
			Message: "数据库端口必须在1到65535之间",
			Value:   db.Port,
		})
		result.Valid = false
	}

	// 验证数据库用户名
	if db.User == "" {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "database.user",
			Message: "数据库用户名是必需的",
			Value:   db.User,
		})
		result.Valid = false
	}

	// 验证数据库名称
	if db.DBName == "" {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "database.dbname",
			Message: "数据库名称是必需的",
			Value:   db.DBName,
		})
		result.Valid = false
	}

	// 验证SSL模式
	validSSLModes := []string{"disable", "allow", "prefer", "require", "verify-ca", "verify-full"}
	if db.SSLMode == "" {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "database.sslmode",
			Message: "数据库SSL模式是必需的",
			Value:   db.SSLMode,
		})
		result.Valid = false
	} else {
		isValidSSLMode := false
		for _, mode := range validSSLModes {
			if db.SSLMode == mode {
				isValidSSLMode = true
				break
			}
		}
		if !isValidSSLMode {
			result.Errors = append(result.Errors, ValidationError{
				Field:   "database.sslmode",
				Message: fmt.Sprintf("数据库SSL模式必须是以下之一: %s", strings.Join(validSSLModes, ", ")),
				Value:   db.SSLMode,
			})
			result.Valid = false
		}
	}

	// 在生产模式下，确保设置了密码
	if v.config.Mode == "production" && db.Password == "" {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "database.password",
			Message: "在生产模式下数据库密码是必需的",
			Value:   "[空]",
		})
		result.Valid = false
	}
}

// validateAuth 验证认证配置
func (v *Validator) validateAuth(result *ValidationResult) {
	auth := v.config.Auth

	// 验证bcrypt成本
	if auth.BcryptCost < 4 || auth.BcryptCost > 31 {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "auth.bcrypt_cost",
			Message: "认证bcrypt成本必须在4到31之间",
			Value:   auth.BcryptCost,
		})
		result.Valid = false
	}
}

// validateJWT 验证JWT配置
func (v *Validator) validateJWT(result *ValidationResult) {
	jwt := v.config.JWT

	// 验证JWT密钥
	if jwt.SecretKey == "" {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "jwt.secret_key",
			Message: "JWT密钥是必需的",
			Value:   "[空]",
		})
		result.Valid = false
	} else {
		// 出于安全考虑，JWT密钥应至少32个字符
		if len(jwt.SecretKey) < 32 {
			result.Errors = append(result.Errors, ValidationError{
				Field:   "jwt.secret_key",
				Message: "出于安全考虑，JWT密钥必须至少32个字符长",
				Value:   fmt.Sprintf("[%d个字符]", len(jwt.SecretKey)),
			})
			result.Valid = false
		}

		// 检查是否为默认开发密钥
		if jwt.SecretKey == "your-secret-key-change-in-production" && v.config.Mode == "production" {
			result.Errors = append(result.Errors, ValidationError{
				Field:   "jwt.secret_key",
				Message: "在生产模式下必须更改默认的JWT密钥",
				Value:   "[默认密钥]",
			})
			result.Valid = false
		}
	}

	// 验证JWT过期时间
	if jwt.ExpiresIn <= 0 {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "jwt.expires_in",
			Message: "JWT过期时间必须大于0",
			Value:   jwt.ExpiresIn,
		})
		result.Valid = false
	} else if jwt.ExpiresIn > 8760 { // 1年（小时）
		result.Errors = append(result.Errors, ValidationError{
			Field:   "jwt.expires_in",
			Message: "出于安全考虑，JWT过期时间不应超过8760小时（1年）",
			Value:   jwt.ExpiresIn,
		})
		result.Valid = false
	}
}

// validateRedis validates Redis configuration
func (v *Validator) validateRedis(result *ValidationResult) {
	redis := v.config.Redis

	// 验证Redis主机地址
	if redis.Host == "" {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "redis.host",
			Message: "Redis主机地址是必需的",
			Value:   redis.Host,
		})
		result.Valid = false
	}

	// 验证Redis端口
	if redis.Port <= 0 || redis.Port > 65535 {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "redis.port",
			Message: "Redis端口必须在1到65535之间",
			Value:   redis.Port,
		})
		result.Valid = false
	}

	// 验证Redis数据库编号
	if redis.DB < 0 || redis.DB > 15 {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "redis.db",
			Message: "Redis数据库编号必须在0到15之间",
			Value:   redis.DB,
		})
		result.Valid = false
	}

	// 验证Redis连接池大小
	if redis.PoolSize <= 0 {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "redis.pool_size",
			Message: "Redis连接池大小必须大于0",
			Value:   redis.PoolSize,
		})
		result.Valid = false
	}
}

// validateRateLimit 验证速率限制配置
func (v *Validator) validateRateLimit(result *ValidationResult) {
	rateLimit := v.config.RateLimit

	// 验证速率限制请求数
	if rateLimit.Requests <= 0 {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "rate_limit.requests",
			Message: "速率限制请求数必须大于0",
			Value:   rateLimit.Requests,
		})
		result.Valid = false
	}

	// 验证速率限制时间窗口格式
	if rateLimit.Window == "" {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "rate_limit.window",
			Message: "速率限制时间窗口是必需的",
			Value:   rateLimit.Window,
		})
		result.Valid = false
	} else {
		// 简单验证时间窗口格式（例如："1m", "30s", "1h", "1d"）
		windowPattern := regexp.MustCompile(`^\d+[smhd]$`)
		if !windowPattern.MatchString(rateLimit.Window) {
			result.Errors = append(result.Errors, ValidationError{
				Field:   "rate_limit.window",
				Message: "速率限制时间窗口格式必须为'1m', '30s', '1h', '1d'等",
				Value:   rateLimit.Window,
			})
			result.Valid = false
		}
	}

	// 验证速率限制Redis键名
	if rateLimit.RedisKey == "" {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "rate_limit.redis_key",
			Message: "速率限制Redis键名是必需的",
			Value:   rateLimit.RedisKey,
		})
		result.Valid = false
	}
}

// validateLogging 验证日志配置
func (v *Validator) validateLogging(result *ValidationResult) {
	logging := v.config.Logging

	// 验证日志级别
	validLevels := []string{"debug", "info", "warn", "error", "fatal", "panic"}
	if logging.Level == "" {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "logging.level",
			Message: "日志级别是必需的，请设置有效的日志级别",
			Value:   logging.Level,
		})
		result.Valid = false
	} else {
		isValidLevel := false
		for _, level := range validLevels {
			if logging.Level == level {
				isValidLevel = true
				break
			}
		}
		if !isValidLevel {
			result.Errors = append(result.Errors, ValidationError{
				Field:   "logging.level",
				Message: fmt.Sprintf("无效的日志级别 '%s'，必须是以下之一: %s", logging.Level, strings.Join(validLevels, ", ")),
				Value:   logging.Level,
			})
			result.Valid = false
		}
	}

	// 验证日志格式
	validFormats := []string{"json", "text"}
	if logging.Format == "" {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "logging.format",
			Message: "日志格式是必需的，请选择 'json' 或 'text' 格式",
			Value:   logging.Format,
		})
		result.Valid = false
	} else {
		isValidFormat := false
		for _, format := range validFormats {
			if logging.Format == format {
				isValidFormat = true
				break
			}
		}
		if !isValidFormat {
			result.Errors = append(result.Errors, ValidationError{
				Field:   "logging.format",
				Message: fmt.Sprintf("无效的日志格式 '%s'，必须是以下之一: %s", logging.Format, strings.Join(validFormats, ", ")),
				Value:   logging.Format,
			})
			result.Valid = false
		}
	}

	// 验证日志输出位置
	validOutputs := []string{"stdout", "stderr", "file", "both", "console"}
	if logging.Output == "" {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "logging.output",
			Message: "日志输出位置是必需的，请选择有效的输出方式",
			Value:   logging.Output,
		})
		result.Valid = false
	} else {
		isValidOutput := false
		for _, output := range validOutputs {
			if logging.Output == output {
				isValidOutput = true
				break
			}
		}
		if !isValidOutput {
			result.Errors = append(result.Errors, ValidationError{
				Field:   "logging.output",
				Message: fmt.Sprintf("无效的日志输出位置 '%s'，必须是以下之一: %s", logging.Output, strings.Join(validOutputs, ", ")),
				Value:   logging.Output,
			})
			result.Valid = false
		}
	}

	// 如果输出为文件，验证文件日志设置
	if logging.Output == "file" {
		v.validateFileLoggingSettings(logging, result)
	} else {
		// 如果输出不是文件，警告文件相关设置将被忽略
		if logging.Directory != "" || logging.MaxSize > 0 || logging.MaxBackups > 0 || logging.MaxAge > 0 {
			// 这里不设为错误，只作为提示，因为用户可能想要保留配置
		}
	}
}

// validateFileLoggingSettings 验证文件日志相关设置
func (v *Validator) validateFileLoggingSettings(logging LoggingConfig, result *ValidationResult) {
	// 验证日志目录
	if logging.Directory == "" {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "logging.directory",
			Message: "当日志输出为文件时，日志目录是必需的，请指定日志文件存储目录",
			Value:   logging.Directory,
		})
		result.Valid = false
	} else {
		// 验证目录路径格式
		if !filepath.IsAbs(logging.Directory) {
			result.Errors = append(result.Errors, ValidationError{
				Field:   "logging.directory",
				Message: fmt.Sprintf("建议使用绝对路径作为日志目录，当前路径 '%s' 是相对路径", logging.Directory),
				Value:   logging.Directory,
			})
			// 这只是一个警告，不阻止应用启动
		}

		// 验证目录访问性和写入权限
		v.validateDirectoryAccess(logging.Directory, result)
	}

	
	// 验证日志轮换设置 - 最大文件大小
	if logging.MaxSize <= 0 {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "logging.max_size",
			Message: "日志文件最大大小必须大于0MB，建议设置为100MB",
			Value:   logging.MaxSize,
		})
		result.Valid = false
	} else if logging.MaxSize < 1 {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "logging.max_size",
			Message: fmt.Sprintf("日志文件最大大小 %dMB 过小，会导致频繁轮换，建议至少设置1MB", logging.MaxSize),
			Value:   logging.MaxSize,
		})
		// 警告但不阻止启动
	} else if logging.MaxSize > 1024 { // 1GB上限
		result.Errors = append(result.Errors, ValidationError{
			Field:   "logging.max_size",
			Message: fmt.Sprintf("日志文件最大大小 %dMB 过大，可能占用过多磁盘空间，建议不超过1024MB（1GB）", logging.MaxSize),
			Value:   logging.MaxSize,
		})
		result.Valid = false
	}

	// 验证最大备份文件数
	if logging.MaxBackups < 0 {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "logging.max_backups",
			Message: "日志最大备份数不能为负数，请设置0或正整数，建议设置为3",
			Value:   logging.MaxBackups,
		})
		result.Valid = false
	} else if logging.MaxBackups > 100 {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "logging.max_backups",
			Message: fmt.Sprintf("日志最大备份数 %d 过多，可能占用过多磁盘空间，建议不超过100个", logging.MaxBackups),
			Value:   logging.MaxBackups,
		})
		// 警告但不阻止启动
	}

	// 验证日志文件最大保存天数
	if logging.MaxAge < 0 {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "logging.max_age",
			Message: "日志最大保存天数不能为负数，请设置0或正整数，建议设置为28天",
			Value:   logging.MaxAge,
		})
		result.Valid = false
	} else if logging.MaxAge > 365 {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "logging.max_age",
			Message: fmt.Sprintf("日志最大保存天数 %d 过长，可能占用过多磁盘空间，建议不超过365天（1年）", logging.MaxAge),
			Value:   logging.MaxAge,
		})
		// 警告但不阻止启动
	}

	// 验证压缩设置的合理性
	if logging.Compress && logging.MaxSize < 10 {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "logging.compress",
			Message: fmt.Sprintf("当启用压缩时，日志文件最大大小 %dMB 可能过小，建议至少设置10MB以获得更好的压缩效果", logging.MaxSize),
			Value:   logging.Compress,
		})
		// 警告但不阻止启动
	}
}

// validateDirectoryAccess 验证目录的访问性和写入权限
func (v *Validator) validateDirectoryAccess(directory string, result *ValidationResult) {
	// 尝试获取目录信息
	info, err := os.Stat(directory)
	if err != nil {
		if os.IsNotExist(err) {
			// 目录不存在，尝试创建
			if err := os.MkdirAll(directory, 0755); err != nil {
				result.Errors = append(result.Errors, ValidationError{
					Field:   "logging.directory",
					Message: fmt.Sprintf("无法创建日志目录 '%s': %v，请检查路径和权限", directory, err),
					Value:   directory,
				})
				result.Valid = false
				return
			}
			// 创建成功，记录信息但不阻止启动
		} else {
			result.Errors = append(result.Errors, ValidationError{
				Field:   "logging.directory",
				Message: fmt.Sprintf("无法访问日志目录 '%s': %v，请检查路径和权限", directory, err),
				Value:   directory,
			})
			result.Valid = false
			return
		}
	} else {
		// 验证是否为目录
		if !info.IsDir() {
			result.Errors = append(result.Errors, ValidationError{
				Field:   "logging.directory",
				Message: fmt.Sprintf("指定的日志路径 '%s' 不是目录，请指定有效的目录路径", directory),
				Value:   directory,
			})
			result.Valid = false
			return
		}

		// 验证目录权限
		// 检查写入权限：尝试创建临时文件
		testFile := filepath.Join(directory, ".log_access_test")
		file, err := os.OpenFile(testFile, os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			result.Errors = append(result.Errors, ValidationError{
				Field:   "logging.directory",
				Message: fmt.Sprintf("无法在日志目录 '%s' 中写入文件: %v，请检查目录权限", directory, err),
				Value:   directory,
			})
			result.Valid = false
			return
		}
		file.Close()

		// 清理测试文件
		if err := os.Remove(testFile); err != nil {
			// 删除失败不是致命错误，记录警告
		}

		// 检查磁盘空间（警告级别）
		if stat, err := getDiskUsage(directory); err == nil {
			availableMB := stat.Available / (1024 * 1024)
			if availableMB < 100 { // 少于100MB
				result.Errors = append(result.Errors, ValidationError{
					Field:   "logging.directory",
					Message: fmt.Sprintf("日志目录 '%s' 可用磁盘空间仅 %dMB，空间可能不足，建议清理磁盘或选择其他目录", directory, availableMB),
					Value:   directory,
				})
				// 这只是警告，不阻止启动
			}
		}
	}
}

// DiskUsage 磁盘使用情况信息
type DiskUsage struct {
	Total     uint64 // 总空间
	Free      uint64 // 空闲空间
	Available uint64 // 可用空间
}

// getDiskUsage 获取目录所在磁盘的使用情况
func getDiskUsage(path string) (DiskUsage, error) {
	var usage DiskUsage

	// 在Windows上，我们简化处理，只检查目录是否存在
	// 在生产环境中，可以使用更复杂的系统调用来获取磁盘信息
	if _, err := os.Stat(path); err != nil {
		return usage, err
	}

	// 返回默认值，表示无法获取具体磁盘信息
	// 在实际实现中，可以使用 syscall 包或第三方库来获取真实的磁盘使用情况
	return usage, nil
}

// validateMode 验证应用模式
func (v *Validator) validateMode(result *ValidationResult) {
	mode := v.config.Mode

	// 验证应用模式
	validModes := []string{"development", "production", "test"}
	if mode == "" {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "mode",
			Message: "应用模式是必需的",
			Value:   mode,
		})
		result.Valid = false
	} else {
		isValidMode := false
		for _, validMode := range validModes {
			if mode == validMode {
				isValidMode = true
				break
			}
		}
		if !isValidMode {
			result.Errors = append(result.Errors, ValidationError{
				Field:   "mode",
				Message: fmt.Sprintf("应用模式必须是以下之一: %s", strings.Join(validModes, ", ")),
				Value:   mode,
			})
			result.Valid = false
		}
	}
}

// FormatErrors 将验证错误格式化为可读的字符串
func (vr *ValidationResult) FormatErrors() string {
	if vr.Valid {
		return "配置验证通过"
	}

	var messages []string
	for _, err := range vr.Errors {
		messages = append(messages, err.Error())
	}

	return "配置验证失败:\n" + strings.Join(messages, "\n")
}