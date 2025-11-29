package database

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go-server/internal/config"
	"go-server/internal/logger"
	"go-server/internal/models"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// Database 数据库连接和健康监控
type Database struct {
	DB           *gorm.DB               // 数据库连接
	config       *config.DatabaseConfig // 数据库配置
	logger       logger.Logger          // 日志记录器
	healthStatus *PoolHealthStatus      // 连接池健康状态
	queryStats   *QueryPerformanceStats // 查询性能统计
	queryMu      sync.RWMutex           // 查询统计读写锁
	mu           sync.RWMutex           // 读写锁
}

// PoolHealthStatus 连接池健康状态指标
type PoolHealthStatus struct {
	OpenConnections    int       `json:"open_connections"`        // 打开的连接数
	InUseConnections   int       `json:"in_use_connections"`      // 正在使用的连接数
	IdleConnections    int       `json:"idle_connections"`        // 空闲连接数
	MaxOpenConnections int       `json:"max_open_connections"`    // 最大打开连接数
	MaxIdleConnections int       `json:"max_idle_connections"`    // 最大空闲连接数
	WaitCount          int64     `json:"wait_count"`              // 等待次数
	WaitDuration       time.Time `json:"wait_duration"`           // 等待持续时间
	MaxLifetimeClosed  int64     `json:"max_lifetime_closed"`     // 因超过最大生存时间关闭的连接数
	MaxIdleTimeClosed  int64     `json:"max_idle_time_closed"`    // 因超过最大空闲时间关闭的连接数
	LastHealthCheck    time.Time `json:"last_health_check"`       // 最后健康检查时间
	IsHealthy          bool      `json:"is_healthy"`              // 是否健康
	ErrorMessage       string    `json:"error_message,omitempty"` // 错误消息
}

// QueryPerformanceStats 查询性能指标
type QueryPerformanceStats struct {
	TotalQueries       int64         `json:"total_queries"`             // 总查询数
	SlowQueries        int64         `json:"slow_queries"`              // 慢查询数
	TotalDurationNanos int64         `json:"total_duration_nanos"`      // 总持续时间（纳秒）
	AverageDuration    time.Duration `json:"average_duration"`          // 平均持续时间
	SlowQueryThreshold time.Duration `json:"slow_query_threshold"`      // 慢查询阈值
	LastSlowQuery      time.Time     `json:"last_slow_query,omitempty"` // 最后慢查询时间
	MaxDuration        time.Duration `json:"max_duration"`              // 最大持续时间
	MinDuration        time.Duration `json:"min_duration"`              // 最小持续时间
}

// QueryLogEntry 慢查询日志条目
type QueryLogEntry struct {
	Timestamp    time.Duration `json:"timestamp"`            // 时间戳
	Duration     time.Duration `json:"duration"`             // 持续时间
	Query        string        `json:"query"`                // 查询语句
	Parameters   interface{}   `json:"parameters,omitempty"` // 查询参数
	RowsAffected int64         `json:"rows_affected"`        // 受影响的行数
	Error        string        `json:"error,omitempty"`      // 错误信息
}

// 查询性能监控常量
const (
	SlowQueryThreshold = 50 * time.Millisecond // 慢查询阈值
)

// updateQueryStats 更新查询性能统计
func (d *Database) updateQueryStats(duration time.Duration, query string, vars []interface{}, err error, rowsAffected int64) {
	d.queryMu.Lock()
	defer d.queryMu.Unlock()

	// 更新原子计数器
	atomic.AddInt64(&d.queryStats.TotalQueries, 1)
	atomic.AddInt64(&d.queryStats.TotalDurationNanos, duration.Nanoseconds())

	// 更新最小/最大持续时间
	if d.queryStats.MaxDuration < duration {
		d.queryStats.MaxDuration = duration
	}
	if d.queryStats.MinDuration == 0 || d.queryStats.MinDuration > duration {
		d.queryStats.MinDuration = duration
	}

	// 计算平均持续时间
	if d.queryStats.TotalQueries > 0 {
		d.queryStats.AverageDuration = time.Duration(d.queryStats.TotalDurationNanos / d.queryStats.TotalQueries)
	}

	// 检查是否为慢查询
	if duration > SlowQueryThreshold {
		atomic.AddInt64(&d.queryStats.SlowQueries, 1)
		d.queryStats.LastSlowQuery = time.Now()

		// 记录慢查询
		d.logSlowQuery(duration, query, vars, err, rowsAffected)
	}
}

// logSlowQuery 记录慢查询及其性能指标
func (d *Database) logSlowQuery(duration time.Duration, query string, vars []interface{}, err error, rowsAffected int64) {
	// 清理查询字符串以便记录
	cleanQuery := strings.TrimSpace(query)
	if cleanQuery == "" {
		cleanQuery = "未知查询"
	}

	// 构建日志消息
	logMessage := fmt.Sprintf(
		"[慢查询] 持续时间: %v | 查询: %s | 影响行数: %d",
		duration,
		cleanQuery,
		rowsAffected,
	)

	// 如果有错误信息，添加到日志中
	if err != nil {
		logMessage += fmt.Sprintf(" | 错误: %v", err)
	}

	// 记录慢查询
	d.logger.Warn(context.Background(), "检测到数据库慢查询",
		logger.String("duration", duration.String()),
		logger.String("query", cleanQuery),
		logger.Int64("rows_affected", rowsAffected),
		logger.String("error", func() string {
			if err != nil {
				return err.Error()
			}
			return ""
		}()),
		logger.Any("parameters", func() interface{} {
			if len(vars) > 0 && d.config != nil && config.IsDevelopment("development") {
				return vars
			}
			return nil
		}()))
}

// registerQueryCallbacks 向GORM注册查询监控回调
func (d *Database) registerQueryCallbacks() {
	callback := d.DB.Callback()

	// 为查询操作注册回调
	callback.Query().Before("gorm:query").Register("query_monitor:before", func(db *gorm.DB) {
		db.InstanceSet("query_start_time", time.Now())
	})

	callback.Query().After("gorm:query").Register("query_monitor:after", func(db *gorm.DB) {
		if startTime, ok := db.InstanceGet("query_start_time"); ok {
			if start, ok := startTime.(time.Time); ok {
				duration := time.Since(start)
				d.updateQueryStats(duration, db.Statement.SQL.String(), db.Statement.Vars, db.Error, db.RowsAffected)
			}
		}
	})

	// 为创建操作注册回调
	callback.Create().Before("gorm:create").Register("query_monitor:create_before", func(db *gorm.DB) {
		db.InstanceSet("query_start_time", time.Now())
	})

	callback.Create().After("gorm:create").Register("query_monitor:create_after", func(db *gorm.DB) {
		if startTime, ok := db.InstanceGet("query_start_time"); ok {
			if start, ok := startTime.(time.Time); ok {
				duration := time.Since(start)
				d.updateQueryStats(duration, db.Statement.SQL.String(), db.Statement.Vars, db.Error, db.RowsAffected)
			}
		}
	})

	// 为更新操作注册回调
	callback.Update().Before("gorm:update").Register("query_monitor:update_before", func(db *gorm.DB) {
		db.InstanceSet("query_start_time", time.Now())
	})

	callback.Update().After("gorm:update").Register("query_monitor:update_after", func(db *gorm.DB) {
		if startTime, ok := db.InstanceGet("query_start_time"); ok {
			if start, ok := startTime.(time.Time); ok {
				duration := time.Since(start)
				d.updateQueryStats(duration, db.Statement.SQL.String(), db.Statement.Vars, db.Error, db.RowsAffected)
			}
		}
	})

	// 为删除操作注册回调
	callback.Delete().Before("gorm:delete").Register("query_monitor:delete_before", func(db *gorm.DB) {
		db.InstanceSet("query_start_time", time.Now())
	})

	callback.Delete().After("gorm:delete").Register("query_monitor:delete_after", func(db *gorm.DB) {
		if startTime, ok := db.InstanceGet("query_start_time"); ok {
			if start, ok := startTime.(time.Time); ok {
				duration := time.Since(start)
				d.updateQueryStats(duration, db.Statement.SQL.String(), db.Statement.Vars, db.Error, db.RowsAffected)
			}
		}
	})

	// 为原始操作注册回调
	callback.Raw().Before("gorm:raw").Register("query_monitor:raw_before", func(db *gorm.DB) {
		db.InstanceSet("query_start_time", time.Now())
	})

	callback.Raw().After("gorm:raw").Register("query_monitor:raw_after", func(db *gorm.DB) {
		if startTime, ok := db.InstanceGet("query_start_time"); ok {
			if start, ok := startTime.(time.Time); ok {
				duration := time.Since(start)
				d.updateQueryStats(duration, db.Statement.SQL.String(), db.Statement.Vars, db.Error, db.RowsAffected)
			}
		}
	})

	d.logger.Debug(context.Background(), "数据库查询监控回调已注册")
}

// NewDatabase 创建新的数据库连接
func NewDatabase(cfg *config.Config, loggerManager *logger.Manager) (*Database, error) {
    dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=%s TimeZone=UTC",
        cfg.Database.Host,
        cfg.Database.User,
        cfg.Database.Password,
        cfg.Database.DBName,
        cfg.Database.Port,
        cfg.Database.SSLMode,
    )

	// 配置GORM日志
	var gormLogLevel gormlogger.LogLevel
	if config.IsDevelopment(cfg.Mode) {
		gormLogLevel = gormlogger.Info
	} else {
		gormLogLevel = gormlogger.Error
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormLogLevel),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != nil {
		return nil, fmt.Errorf("连接数据库失败: %w", err)
	}

	// 获取底层SQL数据库对象以配置连接池
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("获取底层sql.DB失败: %w", err)
	}

	// 使用配置中的设置配置连接池
	maxIdleConns := cfg.Database.MaxIdleConns
	if maxIdleConns <= 0 {
		maxIdleConns = 10 // 默认回退值
	}

	maxOpenConns := cfg.Database.MaxOpenConns
	if maxOpenConns <= 0 {
		maxOpenConns = 100 // 默认回退值
	}

	connMaxLifetime := time.Duration(cfg.Database.ConnMaxLifetime) * time.Second
	if connMaxLifetime <= 0 {
		connMaxLifetime = time.Hour // 默认回退值
	}

	sqlDB.SetMaxIdleConns(maxIdleConns)
	sqlDB.SetMaxOpenConns(maxOpenConns)
	sqlDB.SetConnMaxLifetime(connMaxLifetime)

	// 获取数据库日志记录器
	dbLogger := loggerManager.GetLogger("database")

	dbLogger.Info(context.Background(), "数据库连接池配置完成",
		logger.Int("max_idle_conns", maxIdleConns),
		logger.Int("max_open_conns", maxOpenConns),
		logger.String("conn_max_lifetime", connMaxLifetime.String()))

	// Test the connection
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	dbLogger.Info(context.Background(), "数据库连接建立成功")

	// Initialize database with health monitoring and query performance tracking
	database := &Database{
		DB:     db,
		config: &cfg.Database,
		logger: dbLogger,
		healthStatus: &PoolHealthStatus{
			MaxOpenConnections: maxOpenConns,
			MaxIdleConnections: maxIdleConns,
			LastHealthCheck:    time.Now(),
			IsHealthy:          true,
		},
		queryStats: &QueryPerformanceStats{
			SlowQueryThreshold: SlowQueryThreshold,
			MinDuration:        0, // Will be set on first query
		},
	}

	// Perform initial health check
	if err := database.updateHealthStatus(); err != nil {
		dbLogger.Warn(context.Background(), "初始健康检查失败", logger.Error(err))
	}

	// Register query monitoring callbacks
	database.registerQueryCallbacks()

	dbLogger.Info(context.Background(), "数据库查询监控已启用",
		logger.String("slow_query_threshold", SlowQueryThreshold.String()))

	return database, nil
}

// AutoMigrate 运行数据库迁移
func (d *Database) AutoMigrate() error {
	d.logger.Info(context.Background(), "正在运行数据库迁移")

	err := d.DB.AutoMigrate(
		&models.User{},
	)
	if err != nil {
		return fmt.Errorf("运行迁移失败: %w", err)
	}

	d.logger.Info(context.Background(), "数据库迁移成功完成")

	// 创建索引
	if err := d.createIndexes(); err != nil {
		return fmt.Errorf("创建索引失败: %w", err)
	}

	// 如果在开发模式下，植入初始数据
	if err := d.seedData(); err != nil {
		d.logger.Warn(context.Background(), "植入初始数据失败", logger.Error(err))
	}

	return nil
}

// createIndexes 创建额外的数据库索引
func (d *Database) createIndexes() error {
	d.logger.Info(context.Background(), "正在创建数据库索引")

	// 用户索引
	if err := d.DB.Exec("CREATE INDEX IF NOT EXISTS idx_users_email_active ON users(email, is_active) WHERE deleted_at IS NULL").Error; err != nil {
		return fmt.Errorf("创建email_active索引失败: %w", err)
	}

	if err := d.DB.Exec("CREATE INDEX IF NOT EXISTS idx_users_username_active ON users(username, is_active) WHERE deleted_at IS NULL").Error; err != nil {
		return fmt.Errorf("创建username_active索引失败: %w", err)
	}

	if err := d.DB.Exec("CREATE INDEX IF NOT EXISTS idx_users_created_at ON users(created_at) WHERE deleted_at IS NULL").Error; err != nil {
		return fmt.Errorf("创建created_at索引失败: %w", err)
	}

	// 使用CONCURRENTLY创建性能索引以避免阻塞
	// 为last_login DESC排序创建索引（对最近用户活动查询有用）
	if err := d.DB.Exec("CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_users_last_login_desc ON users(last_login DESC) WHERE deleted_at IS NULL AND last_login IS NOT NULL").Error; err != nil {
		return fmt.Errorf("failed to create last_login_desc index: %w", err)
	}

	// Composite index for created_at DESC with is_active (useful for active user queries ordered by creation time)
	if err := d.DB.Exec("CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_users_created_at_active_desc ON users(created_at DESC, is_active) WHERE deleted_at IS NULL").Error; err != nil {
		return fmt.Errorf("failed to create created_at_active_desc index: %w", err)
	}

	d.logger.Info(context.Background(), "数据库索引创建成功")
	return nil
}

// seedData seeds initial data for development
func (d *Database) seedData() error {
	d.logger.Info(context.Background(), "正在植入初始数据")

	// Check if admin user already exists
	var adminCount int64
	d.DB.Model(&models.User{}).Where("email = ?", "admin@example.com").Count(&adminCount)
	if adminCount > 0 {
		d.logger.Info(context.Background(), "管理员用户已存在，跳过植入")
		return nil
	}

	// Create admin user
	adminUser := models.User{
		ID:        uuid.New().String(),
		Username:  "admin",
		Email:     "admin@example.com",
		Password:  "$2a$10$92IXUNpkjO0rOQ5byMi.Ye4oKoEa3Ro9llC/.og/at2.uheWG/igi", // password: "password"
		FirstName: "Admin",
		LastName:  "User",
		IsActive:  true,
		IsAdmin:   true,
	}

	if err := d.DB.Create(&adminUser).Error; err != nil {
		return fmt.Errorf("failed to create admin user: %w", err)
	}

	// Create test user
	testUser := models.User{
		ID:        uuid.New().String(),
		Username:  "testuser",
		Email:     "user@example.com",
		Password:  "$2a$10$92IXUNpkjO0rOQ5byMi.Ye4oKoEa3Ro9llC/.og/at2.uheWG/igi", // password: "password"
		FirstName: "Test",
		LastName:  "User",
		IsActive:  true,
		IsAdmin:   false,
	}

	if err := d.DB.Create(&testUser).Error; err != nil {
		return fmt.Errorf("failed to create test user: %w", err)
	}

	d.logger.Info(context.Background(), "初始数据植入成功")
	return nil
}

// Close 关闭数据库连接
func (d *Database) Close() error {
	sqlDB, err := d.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// updateHealthStatus 更新连接池健康状态
func (d *Database) updateHealthStatus() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	sqlDB, err := d.DB.DB()
	if err != nil {
		d.healthStatus.IsHealthy = false
		d.healthStatus.ErrorMessage = fmt.Sprintf("获取sql.DB失败: %v", err)
		d.healthStatus.LastHealthCheck = time.Now()
		return err
	}

	// 获取连接池统计信息
	stats := sqlDB.Stats()

	// 更新健康状态
	d.healthStatus.OpenConnections = stats.OpenConnections
	d.healthStatus.InUseConnections = stats.InUse
	d.healthStatus.IdleConnections = stats.Idle
	d.healthStatus.WaitCount = stats.WaitCount
	d.healthStatus.MaxLifetimeClosed = stats.MaxLifetimeClosed
	d.healthStatus.MaxIdleTimeClosed = stats.MaxIdleTimeClosed
	d.healthStatus.LastHealthCheck = time.Now()

	// 使用ping执行健康检查
	if err := sqlDB.Ping(); err != nil {
		d.healthStatus.IsHealthy = false
		d.healthStatus.ErrorMessage = fmt.Sprintf("Ping失败: %v", err)
		return err
	}

	// 检查潜在问题
	if stats.WaitCount > 0 && stats.WaitDuration > 5*time.Second {
		d.healthStatus.IsHealthy = false
		d.healthStatus.ErrorMessage = fmt.Sprintf("连接池经历高等待时间: %v", stats.WaitDuration)
		return fmt.Errorf("连接池经历高等待时间: %v", stats.WaitDuration)
	}

	if stats.OpenConnections >= d.healthStatus.MaxOpenConnections*95/100 {
		d.healthStatus.IsHealthy = false
		d.healthStatus.ErrorMessage = "Connection pool near capacity (95%+ utilization)"
		return fmt.Errorf("connection pool near capacity: %d/%d connections open", stats.OpenConnections, d.healthStatus.MaxOpenConnections)
	}

	d.healthStatus.IsHealthy = true
	d.healthStatus.ErrorMessage = ""
	return nil
}

// GetHealthStatus returns the current connection pool health status
func (d *Database) GetHealthStatus() *PoolHealthStatus {
	d.mu.RLock()
	defer d.mu.RUnlock()

	// Return a copy to avoid concurrent access issues
	status := *d.healthStatus
	return &status
}

// Health checks the database health and updates status
func (d *Database) Health() error {
	if err := d.updateHealthStatus(); err != nil {
		return err
	}

	d.mu.RLock()
	defer d.mu.RUnlock()

	if !d.healthStatus.IsHealthy {
		return fmt.Errorf("database unhealthy: %s", d.healthStatus.ErrorMessage)
	}

	return nil
}

// GetConnectionPoolStats returns detailed connection pool statistics
func (d *Database) GetConnectionPoolStats() (map[string]interface{}, error) {
	sqlDB, err := d.DB.DB()
	if err != nil {
		return nil, err
	}

	stats := sqlDB.Stats()

	return map[string]interface{}{
		"open_connections":    stats.OpenConnections,
		"in_use":              stats.InUse,
		"idle":                stats.Idle,
		"wait_count":          stats.WaitCount,
		"wait_duration":       stats.WaitDuration.String(),
		"max_idle_closed":     stats.MaxIdleTimeClosed,
		"max_lifetime_closed": stats.MaxLifetimeClosed,
		"max_open_conns":      d.config.MaxOpenConns,
		"max_idle_conns":      d.config.MaxIdleConns,
		"conn_max_lifetime":   time.Duration(d.config.ConnMaxLifetime) * time.Second,
		"utilization_percent": float64(stats.OpenConnections) / float64(d.config.MaxOpenConns) * 100,
	}, nil
}

// GetQueryPerformanceStats returns the current query performance statistics
func (d *Database) GetQueryPerformanceStats() *QueryPerformanceStats {
	d.queryMu.RLock()
	defer d.queryMu.RUnlock()

	// Return a copy to avoid concurrent access issues
	stats := *d.queryStats
	return &stats
}

// GetQueryPerformanceStatsJSON returns query performance stats as a map for JSON serialization
func (d *Database) GetQueryPerformanceStatsJSON() map[string]interface{} {
	d.queryMu.RLock()
	defer d.queryMu.RUnlock()

	slowQueryPercentage := float64(0)
	if d.queryStats.TotalQueries > 0 {
		slowQueryPercentage = float64(d.queryStats.SlowQueries) / float64(d.queryStats.TotalQueries) * 100
	}

	totalDuration := time.Duration(d.queryStats.TotalDurationNanos)

	return map[string]interface{}{
		"total_queries":         d.queryStats.TotalQueries,
		"slow_queries":          d.queryStats.SlowQueries,
		"total_duration":        totalDuration.String(),
		"average_duration":      d.queryStats.AverageDuration.String(),
		"slow_query_threshold":  d.queryStats.SlowQueryThreshold.String(),
		"last_slow_query":       d.queryStats.LastSlowQuery,
		"max_duration":          d.queryStats.MaxDuration.String(),
		"min_duration":          d.queryStats.MinDuration.String(),
		"slow_query_percentage": fmt.Sprintf("%.2f%%", slowQueryPercentage),
	}
}

// ResetQueryPerformanceStats resets the query performance statistics
func (d *Database) ResetQueryPerformanceStats() {
	d.queryMu.Lock()
	defer d.queryMu.Unlock()

	d.queryStats.TotalQueries = 0
	d.queryStats.SlowQueries = 0
	d.queryStats.TotalDurationNanos = 0
	d.queryStats.AverageDuration = 0
	d.queryStats.LastSlowQuery = time.Time{}
	d.queryStats.MaxDuration = 0
	d.queryStats.MinDuration = 0

	d.logger.Info(context.Background(), "查询性能统计已重置")
}
