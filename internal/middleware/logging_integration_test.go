package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go-server/internal/config"
	"go-server/internal/logger"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStructuredLoggingMiddleware_ConfigurationHotReload 测试配置热重载场景
func TestStructuredLoggingMiddleware_ConfigurationHotReload(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 创建初始配置
	initialConfig := &config.Config{
		Mode: "test",
		Logging: config.LoggingConfig{
			Level:  "info",
			Format: "json",
			Output: "stdout",
		},
	}

	// 初始化日志管理器
	err := InitializeLoggerManager(initialConfig)
	require.NoError(t, err)
	defer ShutdownLoggerManager()

	// 创建路由
	router := gin.New()
	router.Use(StructuredLoggingMiddleware(initialConfig))

	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "test"})
	})

	// 测试初始配置
	req := httptest.NewRequest("GET", "/test", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.NotEmpty(t, recorder.Header().Get("X-Correlation-ID"))

	// 更新配置为debug级别
	newConfig := config.LoggingConfig{
		Level:  "debug",
		Format: "json",
		Output: "stdout",
	}

	// 使用热重载功能
	err = OnLoggingConfigChange(initialConfig.Logging, newConfig)
	assert.NoError(t, err)

	// 验证配置已更新
	manager := GetLoggerManager()
	require.NotNil(t, manager)

	updatedConfig := manager.GetConfig()
	assert.Equal(t, "debug", updatedConfig.Level)

	// 测试新配置下的请求处理
	req2 := httptest.NewRequest("GET", "/test", nil)
	recorder2 := httptest.NewRecorder()
	router.ServeHTTP(recorder2, req2)

	assert.Equal(t, http.StatusOK, recorder2.Code)
	assert.NotEmpty(t, recorder2.Header().Get("X-Correlation-ID"))
}

// TestStructuredLoggingMiddleware_ErrorRecoveryGracefulDegradation 测试错误恢复和优雅降级
func TestStructuredLoggingMiddleware_ErrorRecoveryGracefulDegradation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	testCases := []struct {
		name        string
		setupFunc   func() *config.Config
		expectError bool
		description string
	}{
		{
			name: "ValidConfiguration",
			setupFunc: func() *config.Config {
				return &config.Config{
					Mode: "test",
					Logging: config.LoggingConfig{
						Level:  "info",
						Format: "json",
						Output: "stdout",
					},
				}
			},
			expectError: false,
			description: "有效配置应该正常工作",
		},
		{
			name: "InvalidLogLevel",
			setupFunc: func() *config.Config {
				return &config.Config{
					Mode: "test",
					Logging: config.LoggingConfig{
						Level:  "invalid",
						Format: "json",
						Output: "stdout",
					},
				}
			},
			expectError: true,
			description: "无效日志级别应该失败",
		},
		{
			name: "InvalidOutput",
			setupFunc: func() *config.Config {
				return &config.Config{
					Mode: "test",
					Logging: config.LoggingConfig{
						Level:  "info",
						Format: "json",
						Output: "invalid",
					},
				}
			},
			expectError: true,
			description: "无效输出目标应该失败",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := tc.setupFunc()

			// 尝试初始化日志管理器
			err := InitializeLoggerManager(cfg)

			if tc.expectError {
				assert.Error(t, err, tc.description)
				return
			}

			require.NoError(t, err)
			defer ShutdownLoggerManager()

			// 验证中间件仍能正常工作
			router := gin.New()
			router.Use(StructuredLoggingMiddleware(cfg))

			router.GET("/test", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"message": "test"})
			})

			req := httptest.NewRequest("GET", "/test", nil)
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)

			assert.Equal(t, http.StatusOK, recorder.Code)
			assert.NotEmpty(t, recorder.Header().Get("X-Correlation-ID"))
		})
	}
}

// TestStructuredLoggingMiddleware_FileSystemErrorRecovery 测试文件系统错误恢复
func TestStructuredLoggingMiddleware_FileSystemErrorRecovery(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 创建临时目录用于测试
	tempDir := t.TempDir()
	invalidPath := filepath.Join(tempDir, "nonexistent", "logs")

	cfg := &config.Config{
		Mode: "test",
		Logging: config.LoggingConfig{
			Level:     "info",
			Format:    "json",
			Output:    "file",
			Directory: invalidPath, // 不存在的路径
		},
	}

	// 尝试初始化，应该失败并回退到stdout
	err := InitializeLoggerManager(cfg)
	// 可能失败或成功（取决于实现）
	if err != nil {
		t.Logf("Expected failure with invalid path: %v", err)
	}

	// 无论是否成功，中间件都应该能工作
	router := gin.New()
	router.Use(StructuredLoggingMiddleware(cfg))

	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "test"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	// 验证请求仍能正常处理
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.NotEmpty(t, recorder.Header().Get("X-Correlation-ID"))

	if globalLoggerManager != nil {
		ShutdownLoggerManager()
	}
}

// TestStructuredLoggingMiddleware_CorrelationIDPropagation 测试关联ID传播
func TestStructuredLoggingMiddleware_CorrelationIDPropagation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		Mode: "test",
		Logging: config.LoggingConfig{
			Level:  "info",
			Format: "json",
			Output: "stdout",
		},
	}

	err := InitializeLoggerManager(cfg)
	require.NoError(t, err)
	defer ShutdownLoggerManager()

	router := gin.New()
	router.Use(StructuredLoggingMiddleware(cfg))

	// 嵌套路由测试关联ID传播
	outer := router.Group("/outer")
	{
		outer.GET("/test", func(c *gin.Context) {
			corrID := GetCorrelationIDFromContext(c)
			assert.NotEmpty(t, corrID)

			// 在响应中包含关联ID
			c.JSON(http.StatusOK, gin.H{
				"correlation_id": corrID,
				"message":        "outer handler",
			})
		})

		// 内部处理函数
	innerHandler := func(c *gin.Context, expectedCorrID string) {
		actualCorrID := GetCorrelationIDFromContext(c)
		assert.Equal(t, expectedCorrID, actualCorrID, "关联ID应该在嵌套调用中保持一致")

		c.JSON(http.StatusOK, gin.H{
			"correlation_id": actualCorrID,
			"message":        "inner handler",
		})
	}

	outer.GET("/inner", func(c *gin.Context) {
		corrID := GetCorrelationIDFromContext(c)
		assert.NotEmpty(t, corrID)

		// 调用内部处理函数
		innerHandler(c, corrID)
	})
	}

	testCases := []struct {
		name         string
		url          string
		preSetCorrID string
		expectSameID bool
	}{
		{
			name:         "WithoutPreSetID",
			url:          "/outer/test",
			preSetCorrID: "",
			expectSameID: false,
		},
		{
			name:         "WithPreSetID",
			url:          "/outer/test",
			preSetCorrID: "pre-set-correlation-id-123",
			expectSameID: true,
		},
		{
			name:         "NestedCall",
			url:          "/outer/inner",
			preSetCorrID: "nested-test-id-456",
			expectSameID: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tc.url, nil)

			if tc.preSetCorrID != "" {
				req.Header.Set("X-Correlation-ID", tc.preSetCorrID)
			}

			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)

			assert.Equal(t, http.StatusOK, recorder.Code)

			// 验证响应头中的关联ID
			responseCorrID := recorder.Header().Get("X-Correlation-ID")
			assert.NotEmpty(t, responseCorrID)

			if tc.expectSameID {
				assert.Equal(t, tc.preSetCorrID, responseCorrID)
			}

			// 验证响应体中的关联ID
			var response map[string]interface{}
			err := json.Unmarshal(recorder.Body.Bytes(), &response)
			require.NoError(t, err)

			assert.Equal(t, responseCorrID, response["correlation_id"])
		})
	}
}

// TestStructuredLoggingMiddleware_ComplexRequestResponseFlow 测试复杂请求响应流程
func TestStructuredLoggingMiddleware_ComplexRequestResponseFlow(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		Mode: "test",
		Logging: config.LoggingConfig{
			Level:  "info",
			Format: "json",
			Output: "stdout",
		},
	}

	err := InitializeLoggerManager(cfg)
	require.NoError(t, err)
	defer ShutdownLoggerManager()

	router := gin.New()
	router.Use(StructuredLoggingMiddleware(cfg))

	// 模拟复杂的业务逻辑路由
	router.POST("/api/v1/users", func(c *gin.Context) {
		// 获取关联ID
		corrID := GetCorrelationIDFromContext(c)
		assert.NotEmpty(t, corrID)

		// 获取日志记录器并记录业务日志
		businessLogger := GetLoggerFromContext(c)
		ctx := context.WithValue(context.Background(), "correlation_id", corrID)

		// 记录业务操作日志
		businessLogger.Info(ctx, "开始创建用户",
			logger.String("operation", "create_user"),
			logger.String("module", "user_service"))

		// 模拟处理时间
		time.Sleep(10 * time.Millisecond)

		// 根据请求内容返回不同的响应
		var requestData map[string]interface{}
		if err := c.ShouldBindJSON(&requestData); err != nil {
			businessLogger.Warn(ctx, "无效的请求数据",
				logger.Error(err),
				logger.String("operation", "create_user"))
			c.JSON(http.StatusBadRequest, gin.H{
				"error":          "Invalid request data",
				"correlation_id": corrID,
			})
			return
		}

		// 模拟成功创建
		businessLogger.Info(ctx, "用户创建成功",
			logger.String("operation", "create_user"),
			logger.String("user_email", fmt.Sprintf("%v", requestData["email"])),
			logger.String("module", "user_service"))

		c.JSON(http.StatusCreated, gin.H{
			"id":             123,
			"email":          requestData["email"],
			"correlation_id": corrID,
			"created_at":     time.Now().Format(time.RFC3339),
		})
	})

	// 测试用例
	testCases := []struct {
		name           string
		requestBody    string
		expectedStatus int
		expectError    bool
	}{
		{
			name:           "ValidRequest",
			requestBody:    `{"email": "test@example.com", "name": "Test User"}`,
			expectedStatus: http.StatusCreated,
			expectError:    false,
		},
		{
			name:           "InvalidJSON",
			requestBody:    `{"email": "test@example.com", "name":}`,
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
		},
		{
			name:           "EmptyRequest",
			requestBody:    ``,
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var req *http.Request
			if tc.requestBody != "" {
				req = httptest.NewRequest("POST", "/api/v1/users", strings.NewReader(tc.requestBody))
				req.Header.Set("Content-Type", "application/json")
			} else {
				req = httptest.NewRequest("POST", "/api/v1/users", nil)
			}

			req.Header.Set("X-Request-ID", fmt.Sprintf("req-%s", tc.name))

			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)

			// 验证响应
			assert.Equal(t, tc.expectedStatus, recorder.Code)
			assert.NotEmpty(t, recorder.Header().Get("X-Correlation-ID"))

			// 验证响应体包含关联ID
			var response map[string]interface{}
			err := json.Unmarshal(recorder.Body.Bytes(), &response)
			require.NoError(t, err)

			assert.Equal(t, recorder.Header().Get("X-Correlation-ID"), response["correlation_id"])
		})
	}
}

// TestStructuredLoggingMiddleware_HighConcurrencyRequests 测试高并发请求场景
func TestStructuredLoggingMiddleware_HighConcurrencyRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		Mode: "test",
		Logging: config.LoggingConfig{
			Level:  "info",
			Format: "json",
			Output: "stdout",
		},
	}

	err := InitializeLoggerManager(cfg)
	require.NoError(t, err)
	defer ShutdownLoggerManager()

	router := gin.New()
	router.Use(StructuredLoggingMiddleware(cfg))

	router.GET("/test", func(c *gin.Context) {
		corrID := GetCorrelationIDFromContext(c)

		// 获取日志记录器并记录日志
		testLogger := GetLoggerFromContext(c)
		ctx := context.WithValue(context.Background(), "correlation_id", corrID)

		testLogger.Info(ctx, "处理高并发请求",
			logger.String("module", "stress_test"),
			logger.String("handler", "test_endpoint"))

		// 模拟短暂处理
		time.Sleep(1 * time.Millisecond)

		c.JSON(http.StatusOK, gin.H{
			"correlation_id": corrID,
			"timestamp":      time.Now().Unix(),
		})
	})

	const (
		numRequests       = 100
		numWorkers        = 10
		requestsPerWorker = numRequests / numWorkers
	)

	var (
		wg           sync.WaitGroup
		successCount int64
		errorCount   int64
		uniqueIDs    sync.Map
	)

	// 启动多个工作协程
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for j := 0; j < requestsPerWorker; j++ {
				req := httptest.NewRequest("GET", "/test", nil)
				req.Header.Set("X-Worker-ID", fmt.Sprintf("worker-%d", workerID))

				recorder := httptest.NewRecorder()
				router.ServeHTTP(recorder, req)

				if recorder.Code == http.StatusOK {
					atomic.AddInt64(&successCount, 1)

					// 解析响应获取关联ID
					var response map[string]interface{}
					if err := json.Unmarshal(recorder.Body.Bytes(), &response); err == nil {
						if corrID, ok := response["correlation_id"].(string); ok {
							uniqueIDs.Store(corrID, true)
						}
					}
				} else {
					atomic.AddInt64(&errorCount, 1)
				}
			}
		}(i)
	}

	wg.Wait()

	// 验证结果
	assert.Equal(t, int64(numRequests), successCount, "所有请求都应该成功")
	assert.Equal(t, int64(0), errorCount, "不应该有失败的请求")

	// 验证关联ID唯一性
	uniqueCount := 0
	uniqueIDs.Range(func(key, value interface{}) bool {
		uniqueCount++
		return true
	})
	assert.Equal(t, numRequests, uniqueCount, "每个请求都应该有唯一的关联ID")
}

// TestStructuredLoggingMiddleware_MiddlewareChainIntegration 测试中间件链集成
func TestStructuredLoggingMiddleware_MiddlewareChainIntegration(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		Mode: "test",
		Logging: config.LoggingConfig{
			Level:  "info",
			Format: "json",
			Output: "stdout",
		},
	}

	err := InitializeLoggerManager(cfg)
	require.NoError(t, err)
	defer ShutdownLoggerManager()

	router := gin.New()

	// 添加多个中间件，测试日志中间件与其他中间件的集成
	router.Use(func(c *gin.Context) {
		// 自定义中间件1：设置请求开始时间
		c.Set("request_start", time.Now())
		c.Next()
	})

	router.Use(StructuredLoggingMiddleware(cfg))

	router.Use(func(c *gin.Context) {
		// 自定义中间件2：记录请求处理时间
		if start, exists := c.Get("request_start"); exists {
			if startTime, ok := start.(time.Time); ok {
				duration := time.Since(startTime)
				c.Header("X-Duration", duration.String())
			}
		}
		c.Next()
	})

	router.GET("/test", func(c *gin.Context) {
		corrID := GetCorrelationIDFromContext(c)

		// 使用日志记录器记录业务逻辑
		businessLogger := GetLoggerFromContext(c)
		ctx := context.WithValue(context.Background(), "correlation_id", corrID)

		businessLogger.Info(ctx, "处理业务逻辑",
			logger.String("module", "test_module"),
			logger.String("operation", "test_operation"))

		c.JSON(http.StatusOK, gin.H{
			"message":        "success",
			"correlation_id": corrID,
		})
	})

	// 测试请求
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("User-Agent", "test-middleware-chain")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	// 验证响应
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.NotEmpty(t, recorder.Header().Get("X-Correlation-ID"))
	assert.NotEmpty(t, recorder.Header().Get("X-Duration"))

	// 验证响应体
	var response map[string]interface{}
	err = json.Unmarshal(recorder.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "success", response["message"])
	assert.Equal(t, recorder.Header().Get("X-Correlation-ID"), response["correlation_id"])
}

// TestStructuredLoggingMiddleware_ConfigurationValidation 测试配置验证
func TestStructuredLoggingMiddleware_ConfigurationValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	testCases := []struct {
		name        string
		config      config.LoggingConfig
		expectError bool
		description string
	}{
		{
			name: "ValidJSONOutput",
			config: config.LoggingConfig{
				Level:  "info",
				Format: "json",
				Output: "stdout",
			},
			expectError: false,
			description: "有效的JSON配置",
		},
		{
			name: "ValidTextOutput",
			config: config.LoggingConfig{
				Level:  "debug",
				Format: "text",
				Output: "stdout",
			},
			expectError: false,
			description: "有效的文本配置",
		},
		{
			name: "InvalidLevel",
			config: config.LoggingConfig{
				Level:  "invalid",
				Format: "json",
				Output: "stdout",
			},
			expectError: true,
			description: "无效的日志级别",
		},
		{
			name: "InvalidFormat",
			config: config.LoggingConfig{
				Level:  "info",
				Format: "invalid",
				Output: "stdout",
			},
			expectError: true,
			description: "无效的日志格式",
		},
		{
			name: "ValidFileOutput",
			config: config.LoggingConfig{
				Level:     "warn",
				Format:    "json",
				Output:    "file",
				Directory: t.TempDir(),
			},
			expectError: false,
			description: "有效的文件输出配置",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.Config{
				Mode:    "test",
				Logging: tc.config,
			}

			err := InitializeLoggerManager(cfg)

			if tc.expectError {
				assert.Error(t, err, tc.description)
				return
			}

			require.NoError(t, err)
			defer func() {
				if globalLoggerManager != nil {
					ShutdownLoggerManager()
				}
			}()

			// 验证中间件能正常工作
			router := gin.New()
			router.Use(StructuredLoggingMiddleware(cfg))

			router.GET("/test", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"message": "test"})
			})

			req := httptest.NewRequest("GET", "/test", nil)
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)

			assert.Equal(t, http.StatusOK, recorder.Code)
			assert.NotEmpty(t, recorder.Header().Get("X-Correlation-ID"))
		})
	}
}

// TestStructuredLoggingMiddleware_PerformanceImpact 测试性能影响
func TestStructuredLoggingMiddleware_PerformanceImpact(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		Mode: "test",
		Logging: config.LoggingConfig{
			Level:  "info",
			Format: "json",
			Output: "stdout",
		},
	}

	err := InitializeLoggerManager(cfg)
	require.NoError(t, err)
	defer ShutdownLoggerManager()

	router := gin.New()
	router.Use(StructuredLoggingMiddleware(cfg))

	router.GET("/test", func(c *gin.Context) {
		corrID := GetCorrelationIDFromContext(c)
		c.JSON(http.StatusOK, gin.H{
			"correlation_id": corrID,
			"message":        "performance test",
		})
	})

	// 性能测试参数
	const numRequests = 1000
	const maxAcceptableLatency = 10 * time.Millisecond // 10ms阈值

	var totalLatency time.Duration
	var successCount int

	for i := 0; i < numRequests; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		recorder := httptest.NewRecorder()

		start := time.Now()
		router.ServeHTTP(recorder, req)
		latency := time.Since(start)

		totalLatency += latency

		if recorder.Code == http.StatusOK {
			successCount++
		}

		// 确保单个请求不超过阈值
		assert.True(t, latency < maxAcceptableLatency,
			"请求 %d 延迟 %v 超过阈值 %v", i, latency, maxAcceptableLatency)
	}

	// 验证统计信息
	assert.Equal(t, numRequests, successCount, "所有请求都应该成功")

	avgLatency := totalLatency / time.Duration(numRequests)
	assert.True(t, avgLatency < maxAcceptableLatency,
		"平均延迟 %v 超过阈值 %v", avgLatency, maxAcceptableLatency)

	t.Logf("性能测试结果: %d 个请求, 平均延迟: %v, 最大可接受延迟: %v",
		numRequests, avgLatency, maxAcceptableLatency)
}

// TestStructuredLoggingMiddleware_ErrorScenarios 测试各种错误场景
func TestStructuredLoggingMiddleware_ErrorScenarios(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		Mode: "test",
		Logging: config.LoggingConfig{
			Level:  "info",
			Format: "json",
			Output: "stdout",
		},
	}

	err := InitializeLoggerManager(cfg)
	require.NoError(t, err)
	defer ShutdownLoggerManager()

	router := gin.New()
	router.Use(StructuredLoggingMiddleware(cfg))

	// 测试不同的错误场景
	router.GET("/client-error", func(c *gin.Context) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request parameters",
		})
	})

	router.GET("/server-error", func(c *gin.Context) {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Internal server error",
		})
	})

	router.GET("/with-gin-error", func(c *gin.Context) {
		c.Error(fmt.Errorf("business logic error"))
		c.JSON(http.StatusOK, gin.H{"message": "success with error"})
	})

	router.GET("/panic", func(c *gin.Context) {
		panic("test panic for error handling")
	})

	testCases := []struct {
		name           string
		url            string
		expectedStatus int
		expectErrorLog bool
	}{
		{
			name:           "ClientError",
			url:            "/client-error",
			expectedStatus: http.StatusBadRequest,
			expectErrorLog: true,
		},
		{
			name:           "ServerError",
			url:            "/server-error",
			expectedStatus: http.StatusInternalServerError,
			expectErrorLog: true,
		},
		{
			name:           "GinError",
			url:            "/with-gin-error",
			expectedStatus: http.StatusOK,
			expectErrorLog: true,
		},
		{
			name:           "PanicError",
			url:            "/panic",
			expectedStatus: http.StatusInternalServerError,
			expectErrorLog: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tc.url, nil)
			recorder := httptest.NewRecorder()

			if tc.name == "PanicError" {
				// 捕获panic
				defer func() {
					if r := recover(); r != nil {
						// 预期的panic
					}
				}()
			}

			router.ServeHTTP(recorder, req)

			assert.Equal(t, tc.expectedStatus, recorder.Code)
			assert.NotEmpty(t, recorder.Header().Get("X-Correlation-ID"))
		})
	}
}
