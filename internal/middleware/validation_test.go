package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestValidationMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		method         string
		body           interface{}
		config         ValidationConfig
		expectedStatus int
		expectedError  string
	}{
		{
			name:   "Valid login request",
			method: "POST",
			body: map[string]interface{}{
				"email":    "test@example.com",
				"password": "password123",
			},
			config:         LoginValidation(),
			expectedStatus: http.StatusOK,
		},
		{
			name:   "Invalid login request - missing email",
			method: "POST",
			body: map[string]interface{}{
				"password": "password123",
			},
			config:         LoginValidation(),
			expectedStatus: http.StatusBadRequest,
			expectedError:  "email is required",
		},
		{
			name:   "Invalid login request - invalid email format",
			method: "POST",
			body: map[string]interface{}{
				"email":    "invalid-email",
				"password": "password123",
			},
			config:         LoginValidation(),
			expectedStatus: http.StatusBadRequest,
			expectedError:  "email must be a valid email address",
		},
		{
			name:   "Invalid login request - password too short",
			method: "POST",
			body: map[string]interface{}{
				"email":    "test@example.com",
				"password": "123",
			},
			config:         LoginValidation(),
			expectedStatus: http.StatusBadRequest,
			expectedError:  "password must be at least 6 characters long",
		},
		{
			name:   "Valid register request",
			method: "POST",
			body: map[string]interface{}{
				"username":  "johndoe",
				"email":     "john@example.com",
				"password":  "password123",
				"first_name": "John",
				"last_name":  "Doe",
			},
			config:         RegisterValidation(),
			expectedStatus: http.StatusOK,
		},
		{
			name:   "Valid pagination request",
			method: "GET",
			body:   nil,
			config: PaginationValidation(),
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()

			// 添加验证中间件
			router.Use(ValidationMiddleware(tt.config))

			// 添加测试路由
			router.POST("/test", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"success": true})
			})

			router.GET("/test", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"success": true})
			})

			var req *http.Request
			if tt.method == "GET" {
				req = httptest.NewRequest(tt.method, "/test?page=1&limit=10", nil)
			} else {
				body, _ := json.Marshal(tt.body)
				req = httptest.NewRequest(tt.method, "/test", bytes.NewBuffer(body))
				req.Header.Set("Content-Type", "application/json")
			}

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedError != "" {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)

				if success, ok := response["success"].(bool); ok {
					assert.False(t, success)
				}

				if errorData, ok := response["error"].(map[string]interface{}); ok {
					if details, ok := errorData["details"].(map[string]interface{}); ok {
						if validationErrors, ok := details["validation_errors"].([]interface{}); ok && len(validationErrors) > 0 {
							if validationError, ok := validationErrors[0].(map[string]interface{}); ok {
								if message, ok := validationError["message"].(string); ok {
									assert.Contains(t, message, tt.expectedError)
								}
							}
						}
					}
				}
			}
		})
	}
}

func TestPasswordStrengthRule(t *testing.T) {
	tests := []struct {
		name           string
		password       string
		rule           *PasswordStrengthRule
		expectedError  string
		expectValid    bool
	}{
		{
			name:     "Valid strong password",
			password: "StrongPass123!",
			rule: &PasswordStrengthRule{
				RequireUppercase: true,
				RequireLowercase: true,
				RequireNumber:    true,
				RequireSymbol:    true,
				MinLength:        8,
			},
			expectValid: true,
		},
		{
			name:     "Password too short",
			password: "Short1!",
			rule: &PasswordStrengthRule{
				RequireUppercase: true,
				RequireLowercase: true,
				RequireNumber:    true,
				RequireSymbol:    true,
				MinLength:        8,
			},
			expectedError: "password must contain at least 8 characters",
		},
		{
			name:     "Password missing uppercase",
			password: "weakpass123!",
			rule: &PasswordStrengthRule{
				RequireUppercase: true,
				RequireLowercase: true,
				RequireNumber:    true,
				RequireSymbol:    true,
				MinLength:        8,
			},
			expectedError: "password must contain uppercase letter",
		},
		{
			name:     "Password missing lowercase",
			password: "STRONGPASS123!",
			rule: &PasswordStrengthRule{
				RequireUppercase: true,
				RequireLowercase: true,
				RequireNumber:    true,
				RequireSymbol:    true,
				MinLength:        8,
			},
			expectedError: "password must contain lowercase letter",
		},
		{
			name:     "Password missing number",
			password: "StrongPassword!",
			rule: &PasswordStrengthRule{
				RequireUppercase: true,
				RequireLowercase: true,
				RequireNumber:    true,
				RequireSymbol:    true,
				MinLength:        8,
			},
			expectedError: "password must contain number",
		},
		{
			name:     "Password missing symbol",
			password: "StrongPassword123",
			rule: &PasswordStrengthRule{
				RequireUppercase: true,
				RequireLowercase: true,
				RequireNumber:    true,
				RequireSymbol:    true,
				MinLength:        8,
			},
			expectedError: "password must contain special symbol",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.rule.Validate(tt.password, "password")

			if tt.expectValid {
				assert.Nil(t, err)
			} else {
				assert.NotNil(t, err)
				assert.Contains(t, err.Message, tt.expectedError)
			}
		})
	}
}

func TestSanitizeValue(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected interface{}
	}{
		{
			name:     "String with whitespace",
			input:    "  hello world  ",
			expected: "hello world",
		},
		{
			name:     "String without whitespace",
			input:    "hello",
			expected: "hello",
		},
		{
			name:     "Non-string value",
			input:    123,
			expected: 123,
		},
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeValue(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSanitizeMap(t *testing.T) {
	data := map[string]interface{}{
		"name":  "  John Doe  ",
		"email": " john@example.com ",
		"age":   30,
	}

	sanitizeMap(data)

	expected := map[string]interface{}{
		"name":  "John Doe",
		"email": "john@example.com",
		"age":   30,
	}

	assert.Equal(t, expected, data)
}

func TestGetValidatedData(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(ValidationMiddleware(LoginValidation()))
	router.POST("/test", func(c *gin.Context) {
		data, exists := GetValidatedData(c)
		if exists {
			c.JSON(http.StatusOK, data)
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": "no validated data"})
		}
	})

	body, _ := json.Marshal(map[string]interface{}{
		"email":    "test@example.com",
		"password": "password123",
	})

	req := httptest.NewRequest("POST", "/test", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	// 验证数据被清理了（虽然在这个例子中没有空格需要清理）
	assert.Equal(t, "test@example.com", response["email"])
	assert.Equal(t, "password123", response["password"])
}

func TestInvalidContentType(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(ValidationMiddleware(LoginValidation()))
	router.POST("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	body, _ := json.Marshal(map[string]interface{}{
		"email":    "test@example.com",
		"password": "password123",
	})

	req := httptest.NewRequest("POST", "/test", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "text/plain") // 错误的Content-Type
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.False(t, response["success"].(bool))
	assert.Contains(t, response["message"].(string), "Content-Type must be application/json")
}

func TestInvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(ValidationMiddleware(LoginValidation()))
	router.POST("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	// 无效的JSON
	req := httptest.NewRequest("POST", "/test", bytes.NewBufferString("{invalid json}"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.False(t, response["success"].(bool))
	assert.Contains(t, response["message"].(string), "Invalid JSON format")
}

// 基准测试
func BenchmarkValidationMiddleware(b *testing.B) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(ValidationMiddleware(LoginValidation()))
	router.POST("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	body, _ := json.Marshal(map[string]interface{}{
		"email":    "test@example.com",
		"password": "password123",
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("POST", "/test", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}
}