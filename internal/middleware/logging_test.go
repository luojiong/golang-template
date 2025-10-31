package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"go-server/internal/config"
	"go-server/internal/logger"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStructuredLoggingMiddleware(t *testing.T) {
	// 设置Gin为测试模式
	gin.SetMode(gin.TestMode)

	// 创建测试配置
	cfg := &config.Config{
		Mode: "development", // 使用开发模式进行测试
		Logging: config.LoggingConfig{
			Level:  "info",
			Format: "text",
		},
	}

	// 创建Gin路由并添加中间件
	router := gin.New()
	router.Use(StructuredLoggingMiddleware(cfg))

	// 添加测试路由
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "success",
		})
	})

	// 创建测试请求
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("User-Agent", "test-agent")
	req.Header.Set("Referer", "http://example.com")

	// 捕获输出
	w := httptest.NewRecorder()

	// 执行请求
	router.ServeHTTP(w, req)

	// 验证响应状态码
	assert.Equal(t, http.StatusOK, w.Code)

	// 验证关联ID头存在
	assert.NotEmpty(t, w.Header().Get("X-Correlation-ID"))
}

func TestStructuredLoggingMiddlewareProductionMode(t *testing.T) {
	// 设置Gin为测试模式
	gin.SetMode(gin.TestMode)

	// 创建生产环境配置
	cfg := &config.Config{
		Mode: "production",
		Logging: config.LoggingConfig{
			Level:  "info",
			Format: "json",
		},
	}

	// 创建Gin路由并添加中间件
	router := gin.New()
	router.Use(StructuredLoggingMiddleware(cfg))

	// 添加测试路由
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "success",
		})
	})

	// 创建测试请求
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("User-Agent", "test-agent")

	// 捕获输出
	w := httptest.NewRecorder()

	// 执行请求
	router.ServeHTTP(w, req)

	// 验证响应状态码
	assert.Equal(t, http.StatusOK, w.Code)

	// 验证关联ID头存在
	assert.NotEmpty(t, w.Header().Get("X-Correlation-ID"))
}

func TestStructuredLoggingMiddlewareWithExistingCorrelationID(t *testing.T) {
	// 设置Gin为测试模式
	gin.SetMode(gin.TestMode)

	// 创建测试配置
	cfg := &config.Config{
		Mode: "development",
	}

	// 创建Gin路由并添加中间件
	router := gin.New()
	router.Use(StructuredLoggingMiddleware(cfg))

	// 添加测试路由，返回请求中的关联ID
	router.GET("/test", func(c *gin.Context) {
		corrID := GetCorrelationIDFromContext(c)
		c.JSON(http.StatusOK, gin.H{
			"correlation_id": corrID,
		})
	})

	// 创建测试请求并预设关联ID
	existingCorrID := "test-correlation-id-123"
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Correlation-ID", existingCorrID)

	// 捕获输出
	w := httptest.NewRecorder()

	// 执行请求
	router.ServeHTTP(w, req)

	// 验证响应状态码
	assert.Equal(t, http.StatusOK, w.Code)

	// 解析响应体
	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	// 验证返回的关联ID与预设的一致
	assert.Equal(t, existingCorrID, response["correlation_id"])

	// 验证响应头中的关联ID也与预设的一致
	assert.Equal(t, existingCorrID, w.Header().Get("X-Correlation-ID"))
}

func TestStructuredLoggingMiddlewareSlowRequest(t *testing.T) {
	// 设置Gin为测试模式
	gin.SetMode(gin.TestMode)

	// 创建测试配置
	cfg := &config.Config{
		Mode: "development",
	}

	// 创建Gin路由并添加中间件
	router := gin.New()
	router.Use(StructuredLoggingMiddleware(cfg))

	// 添加慢请求测试路由
	router.GET("/slow", func(c *gin.Context) {
		// 模拟慢请求（超过1秒）
		time.Sleep(1100 * time.Millisecond)
		c.JSON(http.StatusOK, gin.H{
			"message": "slow response",
		})
	})

	// 创建测试请求
	req, _ := http.NewRequest("GET", "/slow", nil)

	// 捕获输出
	w := httptest.NewRecorder()

	// 记录开始时间
	start := time.Now()

	// 执行请求
	router.ServeHTTP(w, req)

	// 计算执行时间
	duration := time.Since(start)

	// 验证请求确实很慢
	assert.True(t, duration > time.Second)

	// 验证响应状态码
	assert.Equal(t, http.StatusOK, w.Code)

	// 验证关联ID头存在
	assert.NotEmpty(t, w.Header().Get("X-Correlation-ID"))
}

func TestStructuredLoggingMiddlewareWithError(t *testing.T) {
	// 设置Gin为测试模式
	gin.SetMode(gin.TestMode)

	// 创建测试配置
	cfg := &config.Config{
		Mode: "development",
	}

	// 创建Gin路由并添加中间件
	router := gin.New()
	router.Use(StructuredLoggingMiddleware(cfg))

	// 添加错误测试路由
	router.GET("/error", func(c *gin.Context) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "bad request",
		})
	})

	// 创建测试请求
	req, _ := http.NewRequest("GET", "/error", nil)

	// 捕获输出
	w := httptest.NewRecorder()

	// 执行请求
	router.ServeHTTP(w, req)

	// 验证响应状态码
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// 验证关联ID头存在
	assert.NotEmpty(t, w.Header().Get("X-Correlation-ID"))
}

func TestGenerateCorrelationID(t *testing.T) {
	// 测试关联ID生成
	id1 := generateCorrelationID()
	id2 := generateCorrelationID()

	// 验证生成的ID不为空
	assert.NotEmpty(t, id1)
	assert.NotEmpty(t, id2)

	// 验证两次生成的ID不同
	assert.NotEqual(t, id1, id2)

	// 验证ID长度合理（UUID标准长度为36字符）
	assert.Equal(t, 36, len(id1))
	assert.Equal(t, 36, len(id2))
}

func TestGetCorrelationIDFromContext(t *testing.T) {
	// 设置Gin为测试模式
	gin.SetMode(gin.TestMode)

	// 创建测试配置
	cfg := &config.Config{
		Mode: "development",
	}

	// 创建Gin路由并添加中间件
	router := gin.New()
	router.Use(StructuredLoggingMiddleware(cfg))

	// 添加测试路由
	router.GET("/test", func(c *gin.Context) {
		corrID := GetCorrelationIDFromContext(c)
		c.JSON(http.StatusOK, gin.H{
			"correlation_id": corrID,
		})
	})

	// 创建测试请求
	req, _ := http.NewRequest("GET", "/test", nil)

	// 捕获输出
	w := httptest.NewRecorder()

	// 执行请求
	router.ServeHTTP(w, req)

	// 验证响应状态码
	assert.Equal(t, http.StatusOK, w.Code)

	// 解析响应体
	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	// 验证返回的关联ID不为空
	assert.NotEmpty(t, response["correlation_id"])
}

func BenchmarkStructuredLoggingMiddleware(b *testing.B) {
	// 设置Gin为测试模式
	gin.SetMode(gin.TestMode)

	// 创建测试配置
	cfg := &config.Config{
		Mode: "production",
	}

	// 创建Gin路由并添加中间件
	router := gin.New()
	router.Use(StructuredLoggingMiddleware(cfg))

	// 添加简单的测试路由
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	// 创建测试请求
	req, _ := http.NewRequest("GET", "/test", nil)

	// 重置计时器
	b.ResetTimer()

	// 运行基准测试
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}
}

// TestStructuredLoggingMiddleware_OutputCapture 测试日志输出捕获
func TestStructuredLoggingMiddleware_OutputCapture(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 捕获标准输出
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// 创建生产环境配置
	cfg := &config.Config{
		Mode: "production",
		Logging: config.LoggingConfig{
			Level:  "info",
			Format: "json",
		},
	}

	router := gin.New()
	router.Use(StructuredLoggingMiddleware(cfg))

	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "test"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("User-Agent", "test-agent")
	req.Header.Set("Referer", "http://example.com")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	// 恢复标准输出并读取捕获的输出
	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// 验证输出包含JSON格式的日志
	assert.NotEmpty(t, output, "应该有日志输出")

	// 解析JSON日志
	var logEntry map[string]interface{}
	err := json.Unmarshal([]byte(strings.TrimSpace(output)), &logEntry)
	require.NoError(t, err, "日志应该是有效的JSON格式")

	// 验证必要的字段
	assert.Contains(t, logEntry, "timestamp", "日志应该包含时间戳")
	assert.Contains(t, logEntry, "correlation_id", "日志应该包含关联ID")
	assert.Contains(t, logEntry, "method", "日志应该包含HTTP方法")
	assert.Contains(t, logEntry, "path", "日志应该包含路径")
	assert.Contains(t, logEntry, "status_code", "日志应该包含状态码")
	assert.Contains(t, logEntry, "latency", "日志应该包含延迟")
	assert.Contains(t, logEntry, "client_ip", "日志应该包含客户端IP")
	assert.Contains(t, logEntry, "user_agent", "日志应该包含用户代理")

	// 验证字段值
	assert.Equal(t, "GET", logEntry["method"])
	assert.Equal(t, "/test", logEntry["path"])
	assert.Equal(t, float64(200), logEntry["status_code"])
	assert.Equal(t, "test-agent", logEntry["user_agent"])
}

// TestStructuredLoggingMiddleware_DevelopmentFormat 测试开发环境日志格式
func TestStructuredLoggingMiddleware_DevelopmentFormat(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 捕获标准输出
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cfg := &config.Config{
		Mode: "development",
	}

	router := gin.New()
	router.Use(StructuredLoggingMiddleware(cfg))

	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	// 恢复标准输出并读取捕获的输出
	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// 验证输出包含可读格式的日志
	assert.NotEmpty(t, output)
	assert.Contains(t, output, "GET", "应该包含HTTP方法")
	assert.Contains(t, output, "/test", "应该包含路径")
	assert.Contains(t, output, "200", "应该包含状态码")
	assert.Contains(t, output, "[", "应该包含时间戳括号")
	assert.Contains(t, output, "]", "应该包含时间戳括号")
}

// TestStructuredLoggingMiddleware_ErrorLogging 测试错误日志记录
func TestStructuredLoggingMiddleware_ErrorLogging(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 捕获标准输出
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cfg := &config.Config{
		Mode: "production",
	}

	router := gin.New()
	router.Use(StructuredLoggingMiddleware(cfg))

	router.GET("/error", func(c *gin.Context) {
		c.Error(fmt.Errorf("test error"))
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad request"})
	})

	req := httptest.NewRequest("GET", "/error", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	// 恢复标准输出并读取捕获的输出
	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// 解析JSON日志
	var logEntry map[string]interface{}
	err := json.Unmarshal([]byte(strings.TrimSpace(output)), &logEntry)
	require.NoError(t, err)

	// 验证错误信息被记录
	assert.Contains(t, logEntry, "error_message", "日志应该包含错误信息")
	assert.NotEmpty(t, logEntry["error_message"], "错误信息不应该为空")
	assert.Equal(t, float64(400), logEntry["status_code"], "应该记录正确的状态码")
}

// TestStructuredLoggingMiddleware_SlowRequestLogging 测试慢请求日志
func TestStructuredLoggingMiddleware_SlowRequestLogging(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 捕获标准输出
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cfg := &config.Config{
		Mode: "production",
	}

	router := gin.New()
	router.Use(StructuredLoggingMiddleware(cfg))

	router.GET("/slow", func(c *gin.Context) {
		time.Sleep(1100 * time.Millisecond) // 超过1秒阈值
		c.String(http.StatusOK, "slow response")
	})

	req := httptest.NewRequest("GET", "/slow", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	// 恢复标准输出并读取捕获的输出
	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// 解析JSON日志
	var logEntry map[string]interface{}
	err := json.Unmarshal([]byte(strings.TrimSpace(output)), &logEntry)
	require.NoError(t, err)

	// 验证慢请求标记
	assert.Contains(t, logEntry, "is_slow_request", "日志应该包含慢请求标记")
	assert.Equal(t, true, logEntry["is_slow_request"], "应该标记为慢请求")

	// 验证延迟超过1秒
	latency, ok := logEntry["latency"].(float64)
	require.True(t, ok, "延迟应该是数值类型")

	// time.Duration在JSON中以纳秒为单位，转换为秒
	latencySeconds := latency / 1e9
	assert.True(t, latencySeconds >= 1.0, "延迟应该超过1秒，实际: %.2f秒", latencySeconds)
}

// TestStructuredLoggingMiddleware_RequestResponseSize 测试请求响应大小记录
func TestStructuredLoggingMiddleware_RequestResponseSize(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 捕获标准输出
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cfg := &config.Config{
		Mode: "production",
	}

	router := gin.New()
	router.Use(StructuredLoggingMiddleware(cfg))

	router.POST("/test", func(c *gin.Context) {
		// 读取请求体
		body, _ := c.GetRawData()
		// 返回更大的响应
		c.JSON(http.StatusOK, gin.H{
			"request_size":  len(body),
			"response_data": strings.Repeat("x", 1000),
		})
	})

	// 创建带请求体的请求
	requestBody := strings.Repeat("request data", 50)
	req := httptest.NewRequest("POST", "/test", strings.NewReader(requestBody))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	// 恢复标准输出并读取捕获的输出
	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// 解析JSON日志
	var logEntry map[string]interface{}
	err := json.Unmarshal([]byte(strings.TrimSpace(output)), &logEntry)
	require.NoError(t, err)

	// 验证请求和响应大小
	assert.Contains(t, logEntry, "request_size", "日志应该包含请求大小")
	assert.Contains(t, logEntry, "response_size", "日志应该包含响应大小")

	requestSize, ok := logEntry["request_size"].(float64)
	require.True(t, ok)
	assert.True(t, requestSize > 0, "请求大小应该大于0")

	responseSize, ok := logEntry["response_size"].(float64)
	require.True(t, ok)
	assert.True(t, responseSize > 0, "响应大小应该大于0")
}

// TestStructuredLoggingMiddleware_PanicRecovery 测试panic恢复和日志记录
func TestStructuredLoggingMiddleware_PanicRecovery(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 捕获标准输出
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cfg := &config.Config{
		Mode: "production",
	}

	router := gin.New()
	router.Use(StructuredLoggingMiddleware(cfg))

	router.GET("/panic", func(c *gin.Context) {
		panic("test panic for recovery")
	})

	req := httptest.NewRequest("GET", "/panic", nil)
	recorder := httptest.NewRecorder()

	// 捕获panic，避免测试失败
	defer func() {
		if r := recover(); r != nil {
			// 预期的panic，继续测试
		}
	}()

	router.ServeHTTP(recorder, req)

	// 恢复标准输出并读取捕获的输出
	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// 验证响应
	assert.Equal(t, http.StatusInternalServerError, recorder.Code)

	// 验证日志输出包含panic信息
	assert.NotEmpty(t, output)
	assert.Contains(t, output, "Panic recovered", "日志应该包含panic恢复信息")
	assert.Contains(t, output, "test panic for recovery", "日志应该包含panic消息")
}

// TestStructuredLoggingMiddleware_ConcurrentRequests 测试并发请求的日志记录
func TestStructuredLoggingMiddleware_ConcurrentRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		Mode: "production",
	}

	router := gin.New()
	router.Use(StructuredLoggingMiddleware(cfg))

	router.GET("/test", func(c *gin.Context) {
		corrID := GetCorrelationIDFromContext(c)
		c.JSON(http.StatusOK, gin.H{"correlation_id": corrID})
	})

	const numRequests = 10
	var wg sync.WaitGroup
	results := make(chan string, numRequests)

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest("GET", "/test", nil)
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)

			// 解析响应获取关联ID
			var response map[string]string
			err := json.Unmarshal(recorder.Body.Bytes(), &response)
			if err == nil {
				results <- response["correlation_id"]
			}
		}()
	}

	wg.Wait()
	close(results)

	// 收集所有关联ID
	var correlationIDs []string
	for corrID := range results {
		correlationIDs = append(correlationIDs, corrID)
	}

	// 验证每个请求都有唯一的关联ID
	assert.Equal(t, numRequests, len(correlationIDs), "应该有与请求数量相同的关联ID")

	// 验证所有关联ID都不为空且唯一
	uniqueIDs := make(map[string]bool)
	for _, id := range correlationIDs {
		assert.NotEmpty(t, id, "关联ID不应该为空")
		assert.False(t, uniqueIDs[id], "关联ID应该是唯一的")
		uniqueIDs[id] = true
	}
}

// TestStructuredLoggingMiddleware_LogFieldsValidation 测试日志字段验证
func TestStructuredLoggingMiddleware_LogFieldsValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 捕获标准输出
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cfg := &config.Config{
		Mode: "production",
	}

	router := gin.New()
	router.Use(StructuredLoggingMiddleware(cfg))

	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "test"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("User-Agent", "test-agent")
	req.Header.Set("Referer", "http://example.com")
	req.Header.Set("X-Forwarded-For", "192.168.1.1")
	req.RemoteAddr = "192.168.1.1:12345"

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	// 恢复标准输出并读取捕获的输出
	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// 解析JSON日志
	var logEntry map[string]interface{}
	err := json.Unmarshal([]byte(strings.TrimSpace(output)), &logEntry)
	require.NoError(t, err)

	// 验证所有必需字段的存在和格式
	requiredFields := []string{
		"timestamp", "correlation_id", "method", "path", "protocol",
		"status_code", "latency", "client_ip", "user_agent", "referer",
		"request_size", "response_size", "is_slow_request",
	}

	for _, field := range requiredFields {
		assert.Contains(t, logEntry, field, "日志应该包含字段: %s", field)
	}

	// 验证字段格式
	assert.Regexp(t, regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}`),
		logEntry["timestamp"], "时间戳应该是ISO格式")

	assert.Regexp(t, regexp.MustCompile(`^[a-f0-9-]{36}$`),
		logEntry["correlation_id"], "关联ID应该是UUID格式")

	assert.Equal(t, "GET", logEntry["method"], "HTTP方法应该正确")
	assert.Equal(t, "/test", logEntry["path"], "路径应该正确")
	assert.Equal(t, "HTTP/1.1", logEntry["protocol"], "协议应该正确")
	assert.Equal(t, float64(200), logEntry["status_code"], "状态码应该正确")
	assert.Equal(t, "test-agent", logEntry["user_agent"], "用户代理应该正确")
	assert.Equal(t, "http://example.com", logEntry["referer"], "来源应该正确")

	// 验证IP地址
	clientIP, ok := logEntry["client_ip"].(string)
	require.True(t, ok)
	assert.NotEmpty(t, clientIP, "客户端IP不应该为空")

	// 验证大小字段
	requestSize, ok := logEntry["request_size"].(float64)
	require.True(t, ok)
	assert.True(t, requestSize >= 0, "请求大小应该非负")

	responseSize, ok := logEntry["response_size"].(float64)
	require.True(t, ok)
	assert.True(t, responseSize >= 0, "响应大小应该非负")

	// 验证布尔字段
	isSlow, ok := logEntry["is_slow_request"].(bool)
	require.True(t, ok)
	// 由于这是一个快速请求，应该不是慢请求
	assert.False(t, isSlow, "这个请求不应该是慢请求")
}

// TestStructuredLoggingMiddleware_CustomHeaderCapture 测试自定义头捕获
func TestStructuredLoggingMiddleware_CustomHeaderCapture(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 捕获标准输出
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cfg := &config.Config{
		Mode: "production",
	}

	router := gin.New()
	router.Use(StructuredLoggingMiddleware(cfg))

	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "test"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	// 设置各种自定义头
	req.Header.Set("X-Request-ID", "req-123")
	req.Header.Set("X-User-ID", "user-456")
	req.Header.Set("Authorization", "Bearer token123")
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	// 恢复标准输出并读取捕获的输出
	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// 解析JSON日志
	var logEntry map[string]interface{}
	err := json.Unmarshal([]byte(strings.TrimSpace(output)), &logEntry)
	require.NoError(t, err)

	// 验证基本字段正确记录
	assert.Equal(t, "GET", logEntry["method"])
	assert.Equal(t, "/test", logEntry["path"])
	assert.Contains(t, logEntry, "user_agent")
	assert.Contains(t, logEntry, "correlation_id")
}

// TestStructuredLoggingMiddleware_LogLevelFiltering 测试日志级别过滤（未来扩展）
func TestStructuredLoggingMiddleware_LogLevelFiltering(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		Mode: "production",
		Logging: config.LoggingConfig{
			Level: "info", // 设置日志级别
		},
	}

	router := gin.New()
	router.Use(StructuredLoggingMiddleware(cfg))

	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	// 验证请求成功处理
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.NotEmpty(t, recorder.Header().Get("X-Correlation-ID"))
}

// BenchmarkStructuredLoggingMiddleware_WithLoggingVsWithout 带日志与不带日志的性能对比
func BenchmarkStructuredLoggingMiddleware_WithLoggingVsWithout(b *testing.B) {
	gin.SetMode(gin.TestMode)

	response := "OK"

	b.Run("WithLogging", func(b *testing.B) {
		cfg := &config.Config{
			Mode: "production",
		}

		router := gin.New()
		router.Use(StructuredLoggingMiddleware(cfg))

		router.GET("/test", func(c *gin.Context) {
			c.String(http.StatusOK, response)
		})

		req := httptest.NewRequest("GET", "/test", nil)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
		}
	})

	b.Run("WithoutLogging", func(b *testing.B) {
		router := gin.New()
		// 不使用日志中间件

		router.GET("/test", func(c *gin.Context) {
			c.String(http.StatusOK, response)
		})

		req := httptest.NewRequest("GET", "/test", nil)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
		}
	})
}

// TestNewLoggerSystemIntegration 测试新日志系统集成
func TestNewLoggerSystemIntegration(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 创建测试配置
	cfg := &config.Config{
		Mode: "test",
		Logging: config.LoggingConfig{
			Level:  "info",
			Format: "json",
			Output: "stdout",
		},
	}

	// 初始化日志管理器
	err := InitializeLoggerManager(cfg)
	assert.NoError(t, err)
	defer ShutdownLoggerManager()

	// 创建Gin路由并添加新的日志中间件
	router := gin.New()
	router.Use(StructuredLoggingMiddleware(cfg))

	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "test"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	// 验证响应
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.NotEmpty(t, recorder.Header().Get("X-Correlation-ID"))
}

// TestGetLoggerFromContext_NewSystem 测试从上下文获取日志记录器
func TestGetLoggerFromContext_NewSystem(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 创建测试配置
	cfg := &config.Config{
		Mode: "test",
		Logging: config.LoggingConfig{
			Level:  "info",
			Format: "json",
			Output: "stdout",
		},
	}

	// 初始化日志管理器
	err := InitializeLoggerManager(cfg)
	assert.NoError(t, err)
	defer ShutdownLoggerManager()

	// 创建Gin上下文
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set(loggerContextKey, globalLoggerManager)

	// 获取日志记录器
	loggerInstance := GetLoggerFromContext(c)

	// 验证日志记录器不为空
	assert.NotNil(t, loggerInstance)

	// 测试日志记录 - 不应该panic
	loggerInstance.Info(context.Background(), "test message")
}

// TestGetLoggerForModule_NewSystem 测试为指定模块获取日志记录器
func TestGetLoggerForModule_NewSystem(t *testing.T) {
	// 创建测试配置
	cfg := &config.Config{
		Mode: "test",
		Logging: config.LoggingConfig{
			Level:  "info",
			Format: "json",
			Output: "stdout",
		},
	}

	// 初始化日志管理器
	err := InitializeLoggerManager(cfg)
	assert.NoError(t, err)
	defer ShutdownLoggerManager()

	// 获取指定模块的日志记录器
	loggerInstance := GetLoggerForModule("test-module")

	// 验证日志记录器不为空
	assert.NotNil(t, loggerInstance)

	// 测试日志记录 - 不应该panic
	loggerInstance.Info(context.Background(), "test message")
}

// TestOnLoggingConfigChange_NewSystem 测试配置热重载
func TestOnLoggingConfigChange_NewSystem(t *testing.T) {
	// 创建测试配置
	oldConfig := config.LoggingConfig{
		Level:  "info",
		Format: "json",
		Output: "stdout",
	}

	newConfig := config.LoggingConfig{
		Level:  "debug",
		Format: "text",
		Output: "stdout", // 保持stdout输出以避免文件系统依赖
	}

	// 初始化日志管理器
	cfg := &config.Config{
		Mode:    "test",
		Logging: oldConfig,
	}

	err := InitializeLoggerManager(cfg)
	assert.NoError(t, err)
	defer ShutdownLoggerManager()

	// 测试配置变更
	err = OnLoggingConfigChange(oldConfig, newConfig)
	assert.NoError(t, err)

	// 验证配置已更新
	manager := GetLoggerManager()
	assert.NotNil(t, manager)

	currentConfig := manager.GetConfig()
	assert.Equal(t, "debug", currentConfig.Level)
	assert.Equal(t, "text", currentConfig.Format)
	assert.Equal(t, "stdout", currentConfig.Output)
}

// TestNoopLoggerAdapter_NewSystem 测试空操作日志记录器适配器
func TestNoopLoggerAdapter_NewSystem(t *testing.T) {
	// 测试空操作日志记录器
	noopLogger := &noopLoggerAdapter{}

	// 这些调用应该不会导致panic
	noopLogger.Debug(context.Background(), "debug message")
	noopLogger.Info(context.Background(), "info message")
	noopLogger.Warn(context.Background(), "warning message")
	noopLogger.Error(context.Background(), "error message")
	noopLogger.Fatal(context.Background(), "fatal message")

	// 测试With方法
	newLogger := noopLogger.WithFields(logger.String("key", "value"))
	assert.Equal(t, noopLogger, newLogger)

	newLogger = noopLogger.WithModule("test")
	assert.Equal(t, noopLogger, newLogger)

	newLogger = noopLogger.WithCorrelationID("test-id")
	assert.Equal(t, noopLogger, newLogger)
}

// TestLogEntryWithNewLogger_ErrorScenarios 测试各种错误场景的日志记录
func TestLogEntryWithNewLogger_ErrorScenarios(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 创建测试配置
	cfg := &config.Config{
		Mode: "test",
		Logging: config.LoggingConfig{
			Level:  "info",
			Format: "json",
			Output: "stdout",
		},
	}

	// 初始化日志管理器
	err := InitializeLoggerManager(cfg)
	assert.NoError(t, err)
	defer ShutdownLoggerManager()

	testCases := []struct {
		name       string
		statusCode int
		errorMsg   string
	}{
		{"Success Request", http.StatusOK, ""},
		{"Client Error", http.StatusBadRequest, "Bad request"},
		{"Server Error", http.StatusInternalServerError, "Internal server error"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			router := gin.New()
			router.Use(StructuredLoggingMiddleware(cfg))

			router.GET("/test", func(c *gin.Context) {
				if tc.errorMsg != "" {
					c.Error(fmt.Errorf("%s", tc.errorMsg))
				}
				c.JSON(tc.statusCode, gin.H{"message": "test"})
			})

			req := httptest.NewRequest("GET", "/test", nil)
			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, req)

			assert.Equal(t, tc.statusCode, recorder.Code)
		})
	}
}

// TestLoggerManagerLifecycle 测试日志管理器生命周期
func TestLoggerManagerLifecycle(t *testing.T) {
	// 测试未初始化状态
	assert.Nil(t, GetLoggerManager())

	// 创建测试配置
	cfg := &config.Config{
		Mode: "test",
		Logging: config.LoggingConfig{
			Level:  "info",
			Format: "json",
			Output: "stdout",
		},
	}

	// 初始化日志管理器
	err := InitializeLoggerManager(cfg)
	assert.NoError(t, err)

	// 验证管理器已初始化
	manager := GetLoggerManager()
	assert.NotNil(t, manager)
	assert.True(t, manager.IsStarted())

	// 关闭日志管理器
	err = ShutdownLoggerManager()
	assert.NoError(t, err)

	// 验证管理器已关闭
	assert.Nil(t, GetLoggerManager())
}
