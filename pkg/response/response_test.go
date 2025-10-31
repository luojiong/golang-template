package response

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	apperrors "go-server/pkg/errors"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestSuccessResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	// 初始化HTTP请求以避免空指针异常
	c.Request, _ = http.NewRequest("GET", "/", nil)

	Success(c, http.StatusOK, "操作成功", map[string]string{"key": "value"})

	assert.Equal(t, http.StatusOK, w.Code)

	var response Response
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.True(t, response.Success)
	assert.Equal(t, "操作成功", response.Message)
	assert.NotEmpty(t, response.CorrelationID)
	assert.NotZero(t, response.Timestamp)
}

func TestValidationError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/", nil)

	ValidationError(c, "输入验证失败",
		apperrors.ErrorDetails{
			Field:      "email",
			Message:    "邮箱格式不正确",
			Value:      "invalid-email",
			Constraint: "必须是有效的邮箱格式",
		},
	)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response Response
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.False(t, response.Success)
	assert.NotNil(t, response.Error)
	assert.Equal(t, apperrors.ErrCodeValidation, response.Error.Code)
	assert.Equal(t, "输入验证失败", response.Error.Message)
	assert.NotEmpty(t, response.CorrelationID)
}

func TestNotFoundError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/", nil)

	NotFoundError(c, "用户", "123")

	assert.Equal(t, http.StatusNotFound, w.Code)

	var response Response
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.False(t, response.Success)
	assert.NotNil(t, response.Error)
	assert.Equal(t, apperrors.ErrCodeNotFound, response.Error.Code)
	assert.Contains(t, response.Error.Message, "用户")
	assert.Contains(t, response.Error.Message, "123")
}

func TestErrorWithAppError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/", nil)

	appError := apperrors.NewUnauthorizedError("未授权访问").
		WithCorrelationID("test-correlation-id").
		WithUserMessage("请先登录")

	ErrorWithAppError(c, appError)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var response Response
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.False(t, response.Success)
	assert.NotNil(t, response.Error)
	assert.Equal(t, apperrors.ErrCodeUnauthorized, response.Error.Code)
	assert.Equal(t, "test-correlation-id", response.CorrelationID)
	assert.Equal(t, "请先登录", response.Error.UserMessage)
}

func TestWithCorrelationIDMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(WithCorrelationID())

	router.GET("/test", func(c *gin.Context) {
		correlationID, exists := c.Get("correlation_id")
		assert.True(t, exists)
		assert.NotEmpty(t, correlationID)

		c.JSON(http.StatusOK, gin.H{"correlation_id": correlationID})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotEmpty(t, w.Header().Get("X-Correlation-ID"))
}

func TestWithCorrelationIDMiddlewareWithExistingHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(WithCorrelationID())

	existingCorrelationID := "existing-correlation-id"

	router.GET("/test", func(c *gin.Context) {
		correlationID, exists := c.Get("correlation_id")
		assert.True(t, exists)
		assert.Equal(t, existingCorrelationID, correlationID)

		c.JSON(http.StatusOK, gin.H{"correlation_id": correlationID})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Correlation-ID", existingCorrelationID)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, existingCorrelationID, w.Header().Get("X-Correlation-ID"))
}

func TestWrapError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/", nil)

	originalErr := apperrors.NewValidationError("密码格式错误")
	WrapError(c, originalErr, apperrors.ErrCodeValidation, "用户输入验证失败")

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response Response
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.False(t, response.Success)
	assert.NotNil(t, response.Error)
	assert.Equal(t, apperrors.ErrCodeValidation, response.Error.Code)
}

func TestRateLimitError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/", nil)

	RateLimitError(c, 100, 3600)

	assert.Equal(t, http.StatusTooManyRequests, w.Code)

	var response Response
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.False(t, response.Success)
	assert.NotNil(t, response.Error)
	assert.Equal(t, apperrors.ErrCodeRateLimitExceeded, response.Error.Code)
	assert.NotNil(t, response.Error.Details)
	assert.Contains(t, response.Error.Details, "limit")
	assert.Contains(t, response.Error.Details, "window_seconds")
	assert.Contains(t, response.Error.Details, "retry_after")
}

func TestDatabaseError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/", nil)

	DatabaseError(c, "连接数据库失败", apperrors.NewAppError(apperrors.ErrCodeDatabase, "timeout"))

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response Response
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.False(t, response.Success)
	assert.NotNil(t, response.Error)
	assert.Equal(t, apperrors.ErrCodeDatabase, response.Error.Code)
	assert.Equal(t, "连接数据库失败", response.Error.Message)
}

// Additional comprehensive error handling tests for TASK-TEST-003

func TestConflictErrorFunction(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("POST", "/", nil)

	details := map[string]interface{}{
		"existing_resource": "user@example.com",
		"conflict_type":     "duplicate_email",
	}

	ConflictError(c, "User with this email already exists", details)

	assert.Equal(t, http.StatusConflict, w.Code)

	var response Response
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.False(t, response.Success)
	assert.NotNil(t, response.Error)
	assert.Equal(t, apperrors.ErrCodeConflict, response.Error.Code)
	assert.Equal(t, "User with this email already exists", response.Error.Message)
	assert.NotNil(t, response.Error.Details)
	assert.Equal(t, "user@example.com", response.Error.Details["existing_resource"])
	assert.Equal(t, "duplicate_email", response.Error.Details["conflict_type"])
	assert.NotEmpty(t, response.CorrelationID)
}

func TestTimeoutErrorFunction(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/", nil)

	timeout := 5 * time.Second
	TimeoutError(c, "database query", timeout)

	assert.Equal(t, http.StatusRequestTimeout, w.Code)

	var response Response
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.False(t, response.Success)
	assert.NotNil(t, response.Error)
	assert.Equal(t, apperrors.ErrCodeTimeout, response.Error.Code)
	assert.Contains(t, response.Error.Message, "database query")
	assert.Contains(t, response.Error.Message, "5s")
	assert.NotNil(t, response.Error.Details)
	assert.Equal(t, "database query", response.Error.Details["operation"])
	assert.Equal(t, 5.0, response.Error.Details["timeout_seconds"])
}

func TestInvalidTokenErrorFunction(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/", nil)

	InvalidTokenError(c, "token has expired")

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var response Response
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.False(t, response.Success)
	assert.NotNil(t, response.Error)
	assert.Equal(t, apperrors.ErrCodeInvalidToken, response.Error.Code)
	assert.Equal(t, "Invalid token: token has expired", response.Error.Message)
	assert.NotNil(t, response.Error.Details)
	assert.Equal(t, "token has expired", response.Error.Details["reason"])
}

func TestTokenBlacklistedErrorFunction(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/", nil)

	TokenBlacklistedError(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var response Response
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.False(t, response.Success)
	assert.NotNil(t, response.Error)
	assert.Equal(t, apperrors.ErrCodeTokenBlacklisted, response.Error.Code)
	assert.Equal(t, "Token has been blacklisted", response.Error.Message)
}

func TestServiceUnavailableErrorFunction(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/", nil)

	ServiceUnavailableError(c, "payment-service", "Payment gateway maintenance")

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	var response Response
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.False(t, response.Success)
	assert.NotNil(t, response.Error)
	assert.Equal(t, apperrors.ErrCodeServiceUnavailable, response.Error.Code)
	assert.Equal(t, "Payment gateway maintenance", response.Error.Message)
	assert.NotNil(t, response.Error.Details)
	assert.Equal(t, "payment-service", response.Error.Details["service_name"])
}

func TestCacheErrorFunction(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/", nil)

	originalErr := errors.New("Redis connection failed")
	CacheError(c, "Failed to cache user data", originalErr)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response Response
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.False(t, response.Success)
	assert.NotNil(t, response.Error)
	assert.Equal(t, apperrors.ErrCodeCache, response.Error.Code)
	assert.Equal(t, "Failed to cache user data", response.Error.Message)
}

func TestInternalServerErrorWithCauseFunction(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("POST", "/", nil)

	originalErr := errors.New("null pointer exception")
	InternalServerErrorWithCause(c, "Failed to process user request", originalErr)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response Response
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.False(t, response.Success)
	assert.NotNil(t, response.Error)
	assert.Equal(t, apperrors.ErrCodeInternal, response.Error.Code)
	assert.Equal(t, "Failed to process user request", response.Error.Message)
	// In development mode, internal error should be populated
	assert.NotEmpty(t, response.Error.InternalError)
}

func TestComprehensiveErrorScenarios(t *testing.T) {
	tests := []struct {
		name            string
		testFunc        func(*gin.Context)
		expectedStatus  int
		expectedCode    apperrors.ErrorCode
		expectedMessage string
		expectedDetails map[string]interface{}
	}{
		{
			name: "complex validation with multiple fields",
			testFunc: func(c *gin.Context) {
				ValidationError(c, "User registration failed",
					apperrors.ErrorDetails{
						Field:      "email",
						Message:    "Invalid email format",
						Value:      "invalid-email",
						Constraint: "valid email required",
					},
					apperrors.ErrorDetails{
						Field:      "password",
						Message:    "Password too short",
						Value:      "123",
						Constraint: "minimum 8 characters",
					},
				)
			},
			expectedStatus:  http.StatusBadRequest,
			expectedCode:    apperrors.ErrCodeValidation,
			expectedMessage: "User registration failed",
			expectedDetails: map[string]interface{}{
				"validation_errors": []interface{}{
					map[string]interface{}{
						"field":      "email",
						"message":    "Invalid email format",
						"value":      "invalid-email",
						"constraint": "valid email required",
					},
					map[string]interface{}{
						"field":      "password",
						"message":    "Password too short",
						"value":      "123",
						"constraint": "minimum 8 characters",
					},
				},
			},
		},
		{
			name: "database error with correlation ID",
			testFunc: func(c *gin.Context) {
				appErr := apperrors.NewDatabaseError("Connection failed", errors.New("timeout")).
					WithCorrelationID("db-123").
					WithDetail("query", "SELECT * FROM users").
					WithDetail("duration_ms", 5000)
				ErrorWithAppError(c, appErr)
			},
			expectedStatus:  http.StatusInternalServerError,
			expectedCode:    apperrors.ErrCodeDatabase,
			expectedMessage: "Connection failed",
			expectedDetails: map[string]interface{}{
				"query":       "SELECT * FROM users",
				"duration_ms": 5000.0,
			},
		},
		{
			name: "rate limit with retry information",
			testFunc: func(c *gin.Context) {
				RateLimitError(c, 100, 3600)
			},
			expectedStatus:  http.StatusTooManyRequests,
			expectedCode:    apperrors.ErrCodeRateLimitExceeded,
			expectedMessage: "Rate limit exceeded",
			expectedDetails: map[string]interface{}{
				"limit":          100.0,
				"window_seconds": 3600.0,
				"retry_after":    3600.0,
			},
		},
		{
			name: "forbidden error with user-friendly message",
			testFunc: func(c *gin.Context) {
				appErr := apperrors.NewForbiddenError("Admin access required").
					WithUserMessage("You need administrator privileges to access this resource").
					WithDetail("required_role", "admin").
					WithDetail("current_role", "user")
				ErrorWithAppError(c, appErr)
			},
			expectedStatus:  http.StatusForbidden,
			expectedCode:    apperrors.ErrCodeForbidden,
			expectedMessage: "Admin access required",
			expectedDetails: map[string]interface{}{
				"required_role": "admin",
				"current_role":  "user",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request, _ = http.NewRequest("GET", "/", nil)

			tt.testFunc(c)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var response Response
			err := json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(t, err)

			assert.False(t, response.Success)
			assert.NotNil(t, response.Error)
			assert.Equal(t, tt.expectedCode, response.Error.Code)
			assert.Equal(t, tt.expectedMessage, response.Error.Message)
			assert.NotEmpty(t, response.CorrelationID)
			assert.NotZero(t, response.Timestamp)

			if tt.expectedDetails != nil {
				assert.NotNil(t, response.Error.Details)
				for key, expectedValue := range tt.expectedDetails {
					assert.Contains(t, response.Error.Details, key)
					assert.Equal(t, expectedValue, response.Error.Details[key])
				}
			}
		})
	}
}

func TestErrorResponseConsistency(t *testing.T) {
	// Test that all error responses follow the same format structure
	gin.SetMode(gin.TestMode)

	errorFunctions := []struct {
		name       string
		testFunc   func(*gin.Context)
		expectCode apperrors.ErrorCode
	}{
		{"ValidationError", func(c *gin.Context) { ValidationError(c, "test") }, apperrors.ErrCodeValidation},
		{"NotFoundError", func(c *gin.Context) { NotFoundError(c, "user", "123") }, apperrors.ErrCodeNotFound},
		{"UnauthorizedError", func(c *gin.Context) { UnauthorizedError(c, "test") }, apperrors.ErrCodeUnauthorized},
		{"ForbiddenError", func(c *gin.Context) { ForbiddenError(c, "test") }, apperrors.ErrCodeForbidden},
		{"ConflictError", func(c *gin.Context) { ConflictError(c, "test", nil) }, apperrors.ErrCodeConflict},
		{"RateLimitError", func(c *gin.Context) { RateLimitError(c, 100, 3600) }, apperrors.ErrCodeRateLimitExceeded},
		{"InternalServerError", func(c *gin.Context) { InternalServerError(c, "test") }, apperrors.ErrCodeInternal},
		{"DatabaseError", func(c *gin.Context) { DatabaseError(c, "test", nil) }, apperrors.ErrCodeDatabase},
		{"CacheError", func(c *gin.Context) { CacheError(c, "test", nil) }, apperrors.ErrCodeCache},
		{"ServiceUnavailableError", func(c *gin.Context) { ServiceUnavailableError(c, "test", "test") }, apperrors.ErrCodeServiceUnavailable},
		{"TimeoutError", func(c *gin.Context) { TimeoutError(c, "test", time.Second) }, apperrors.ErrCodeTimeout},
		{"InvalidTokenError", func(c *gin.Context) { InvalidTokenError(c, "test") }, apperrors.ErrCodeInvalidToken},
		{"TokenBlacklistedError", func(c *gin.Context) { TokenBlacklistedError(c) }, apperrors.ErrCodeTokenBlacklisted},
	}

	for _, ef := range errorFunctions {
		t.Run(ef.name+" response format", func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request, _ = http.NewRequest("GET", "/", nil)

			ef.testFunc(c)

			var response Response
			err := json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(t, err)

			// All error responses should have these fields
			assert.False(t, response.Success, "Success should be false")
			assert.NotNil(t, response.Error, "Error should not be nil")
			assert.NotEmpty(t, response.CorrelationID, "CorrelationID should not be empty")
			assert.NotZero(t, response.Timestamp, "Timestamp should not be zero")

			// Error structure should be consistent
			assert.NotEmpty(t, response.Error.Code, "Error code should not be empty")
			assert.NotEmpty(t, response.Error.Message, "Error message should not be empty")
			assert.Equal(t, ef.expectCode, response.Error.Code, "Error code should match expected")
		})
	}
}

func TestResponseWithHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(WithCorrelationID())

	router.GET("/test-error", func(c *gin.Context) {
		ValidationError(c, "Test error with correlation")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test-error", nil)
	req.Header.Set("X-Correlation-ID", "test-header-correlation")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "test-header-correlation", w.Header().Get("X-Correlation-ID"))

	var response Response
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "test-header-correlation", response.CorrelationID)
}

func TestErrorWithComplexDataStructures(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("POST", "/", nil)

	// Test with complex nested data structures
	complexDetails := map[string]interface{}{
		"validation_summary": map[string]interface{}{
			"total_errors": 3,
			"field_errors": []string{"email", "password", "username"},
		},
		"request_context": map[string]interface{}{
			"method":     "POST",
			"path":       "/api/v1/users",
			"user_agent": "test-agent",
		},
		"nested_object": map[string]interface{}{
			"level1": map[string]interface{}{
				"level2": map[string]interface{}{
					"value": "deeply nested value",
				},
			},
		},
	}

	appError := apperrors.NewValidationError("Complex validation failed").
		WithDetails(complexDetails).
		WithCorrelationID("complex-123").
		WithUserMessage("Please fix the validation errors below")

	ErrorWithAppError(c, appError)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response Response
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.False(t, response.Success)
	assert.NotNil(t, response.Error)
	assert.Equal(t, apperrors.ErrCodeValidation, response.Error.Code)
	assert.Equal(t, "Complex validation failed", response.Error.Message)
	assert.Equal(t, "Please fix the validation errors below", response.Error.UserMessage)
	assert.Equal(t, "complex-123", response.CorrelationID)
	assert.NotNil(t, response.Error.Details)

	// Verify complex data structure serialization
	assert.Contains(t, response.Error.Details, "validation_summary")
	assert.Contains(t, response.Error.Details, "request_context")
	assert.Contains(t, response.Error.Details, "nested_object")
}
