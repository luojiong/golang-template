package config

import (
	"encoding/json"
)

// deepCopyConfig 使用JSON序列化进行深拷贝
// 这种方法比手动复制每个字段更简洁且不易出错
func deepCopyConfig(cfg *Config) *Config {
	if cfg == nil {
		return nil
	}

	// 序列化为JSON
	data, err := json.Marshal(cfg)
	if err != nil {
		// 如果序列化失败，使用手动拷贝作为后备方案
		return manualDeepCopy(cfg)
	}

	// 反序列化为新对象
	var newCfg Config
	if err := json.Unmarshal(data, &newCfg); err != nil {
		// 如果反序列化失败，使用手动拷贝作为后备方案
		return manualDeepCopy(cfg)
	}

	return &newCfg
}

// manualDeepCopy 手动深拷贝（作为后备方案）
// 当JSON序列化/反序列化失败时使用
func manualDeepCopy(cfg *Config) *Config {
	return &Config{
		Server: ServerConfig{
			Port:         cfg.Server.Port,
			Host:         cfg.Server.Host,
			ReadTimeout:  cfg.Server.ReadTimeout,
			WriteTimeout: cfg.Server.WriteTimeout,
		},
		Database: DatabaseConfig{
			Host:            cfg.Database.Host,
			Port:            cfg.Database.Port,
			User:            cfg.Database.User,
			Password:        cfg.Database.Password,
			DBName:          cfg.Database.DBName,
			SSLMode:         cfg.Database.SSLMode,
			MaxOpenConns:    cfg.Database.MaxOpenConns,
			MaxIdleConns:    cfg.Database.MaxIdleConns,
			ConnMaxLifetime: cfg.Database.ConnMaxLifetime,
		},
		Auth: AuthConfig{
			BcryptCost: cfg.Auth.BcryptCost,
		},
		JWT: JWTConfig{
			SecretKey: cfg.JWT.SecretKey,
			ExpiresIn: cfg.JWT.ExpiresIn,
		},
		Redis: RedisConfig{
			Host:     cfg.Redis.Host,
			Port:     cfg.Redis.Port,
			Password: cfg.Redis.Password,
			DB:       cfg.Redis.DB,
			PoolSize: cfg.Redis.PoolSize,
		},
		RateLimit: RateLimitConfig{
			Enabled:  cfg.RateLimit.Enabled,
			Requests: cfg.RateLimit.Requests,
			Window:   cfg.RateLimit.Window,
			RedisKey: cfg.RateLimit.RedisKey,
		},
		Compression: CompressionConfig{
			Enabled:   cfg.Compression.Enabled,
			Threshold: cfg.Compression.Threshold,
		},
		Logging: LoggingConfig{
			Level:      cfg.Logging.Level,
			Format:     cfg.Logging.Format,
			Output:     cfg.Logging.Output,
			Directory:  cfg.Logging.Directory,
			MaxSize:    cfg.Logging.MaxSize,
			MaxBackups: cfg.Logging.MaxBackups,
			MaxAge:     cfg.Logging.MaxAge,
			Compress:   cfg.Logging.Compress,
		},
		Mode: cfg.Mode,
	}
}
