package monitoring

import (
	"bytes"
	"io"
	"time"

	"github.com/gin-gonic/gin"
)

// MetricsMiddleware 指标收集中间件
func MetricsMiddleware(metrics MetricsCollector) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// 记录请求开始
		metrics.RecordRequestCount("pending", c.Request.URL.Path, "started")

		// 读取请求体用于记录（可选）
		var requestBody []byte
		if c.Request.Body != nil {
			requestBody, _ = io.ReadAll(c.Request.Body)
			c.Request.Body = io.NopCloser(bytes.NewBuffer(requestBody))
		}

		// 处理请求
		c.Next()

		// 计算请求持续时间
		duration := time.Since(start)

		// 记录请求指标
		metrics.RecordRequestDuration(c.Request.Method, c.Request.URL.Path, duration)
		metrics.RecordRequestCount(c.Request.Method, c.Request.URL.Path, string(rune(c.Writer.Status())))

		// 记录用户操作（如果已认证）
		if _, exists := c.Get("user_id"); exists {
			action := c.Request.Method + " " + c.Request.URL.Path
			metrics.RecordUserAction(action)

			// 根据路径记录特定业务指标
			switch {
			case c.Request.URL.Path == "/api/v1/auth/login" && c.Writer.Status() == 200:
				metrics.RecordUserLogin(true)
			case c.Request.URL.Path == "/api/v1/auth/login" && c.Writer.Status() != 200:
				metrics.RecordUserLogin(false)
			case c.Request.URL.Path == "/api/v1/auth/register" && c.Writer.Status() == 201:
				metrics.RecordUserRegistration(true)
			case c.Request.URL.Path == "/api/v1/auth/register" && c.Writer.Status() != 201:
				metrics.RecordUserRegistration(false)
			}
		}
	}
}

// DatabaseMetricsMiddleware 数据库指标中间件（如果使用GORM）
// func DatabaseMetricsMiddleware(metrics MetricsCollector) gorm.Plugin {
// 	return &databaseMetricsPlugin{metrics: metrics}
// }

// type databaseMetricsPlugin struct {
// 	metrics MetricsCollector
// }

// func (p *databaseMetricsPlugin) Name() string {
// 	return "database_metrics"
// }

// func (p *databaseMetricsPlugin) Initialize(db *gorm.DB) error {
// 	callback := db.Callback()

// 	// 查询指标
// 	callback.Query().Before("gorm:query").Register("metrics:before_query", func(db *gorm.DB) {
// 		db.InstanceSet("metrics:start_time", time.Now())
// 	})

// 	callback.Query().After("gorm:query").Register("metrics:after_query", func(db *gorm.DB) {
// 		if startTime, ok := db.InstanceGet("metrics:start_time"); ok {
// 			if start, ok := startTime.(time.Time); ok {
// 				duration := time.Since(start)
// 				table := db.Statement.Table
// 				operation := "select"
// 				p.metrics.RecordDatabaseQuery(table, operation, duration)
// 			}
// 		}
// 	})

// 	// 插入指标
// 	callback.Create().Before("gorm:create").Register("metrics:before_create", func(db *gorm.DB) {
// 		db.InstanceSet("metrics:start_time", time.Now())
// 	})

// 	callback.Create().After("gorm:create").Register("metrics:after_create", func(db *gorm.DB) {
// 		if startTime, ok := db.InstanceGet("metrics:start_time"); ok {
// 			if start, ok := startTime.(time.Time); ok {
// 				duration := time.Since(start)
// 				table := db.Statement.Table
// 				operation := "insert"
// 				p.metrics.RecordDatabaseQuery(table, operation, duration)
// 			}
// 		}
// 	})

// 	// 更新指标
// 	callback.Update().Before("gorm:update").Register("metrics:before_update", func(db *gorm.DB) {
// 		db.InstanceSet("metrics:start_time", time.Now())
// 	})

// 	callback.Update().After("gorm:update").Register("metrics:after_update", func(db *gorm.DB) {
// 		if startTime, ok := db.InstanceGet("metrics:start_time"); ok {
// 			if start, ok := startTime.(time.Time); ok {
// 				duration := time.Since(start)
// 				table := db.Statement.Table
// 				operation := "update"
// 				p.metrics.RecordDatabaseQuery(table, operation, duration)
// 			}
// 		}
// 	})

// 	// 删除指标
// 	callback.Delete().Before("gorm:delete").Register("metrics:before_delete", func(db *gorm.DB) {
// 		db.InstanceSet("metrics:start_time", time.Now())
// 	})

// 	callback.Delete().After("gorm:delete").Register("metrics:after_delete", func(db *gorm.DB) {
// 		if startTime, ok := db.InstanceGet("metrics:start_time"); ok {
// 			if start, ok := startTime.(time.Time); ok {
// 				duration := time.Since(start)
// 				table := db.Statement.Table
// 				operation := "delete"
// 				p.metrics.RecordDatabaseQuery(table, operation, duration)
// 			}
// 		}
// 	})

// 	return nil
// }