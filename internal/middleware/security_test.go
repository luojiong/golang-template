package middleware

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go-server/internal/config"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestSecurityHeadersMiddleware(t *testing.T) {
	// 设置Gin为测试模式
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name            string
		mode            string
		isHTTPS         bool
		path            string
		expectedHeaders map[string]string
	}{
		{
			name:    "Production HTTPS - 增强安全策略",
			mode:    "production",
			isHTTPS: true,
			path:    "/api/v1/users",
			expectedHeaders: map[string]string{
				"X-Content-Type-Options":            "nosniff",
				"X-Frame-Options":                   "DENY",
				"X-XSS-Protection":                  "1; mode=block",
				"X-DNS-Prefetch-Control":            "off",
				"X-Download-Options":                "noopen",
				"X-Permitted-Cross-Domain-Policies": "none",
				"X-Server":                          "go-server",
				"Referrer-Policy":                   "strict-origin-when-cross-origin",
				"Cross-Origin-Embedder-Policy":      "require-corp",
				"Cross-Origin-Resource-Policy":      "same-origin",
				"Cross-Origin-Opener-Policy":        "same-origin",
				"Permissions-Policy":                "geolocation=(), microphone=(), camera=(), payment=(), usb=(), magnetometer=(), gyroscope=(), accelerometer=(), autoplay=(), encrypted-media=(), fullscreen=(), picture-in-picture=(), interest-cohort=()",
				"Strict-Transport-Security":         "max-age=31536000; includeSubDomains; preload",
				"Cache-Control":                     "no-store, no-cache, must-revalidate, proxy-revalidate",
				"Pragma":                            "no-cache",
				"Expires":                           "0",
				"Surrogate-Control":                 "no-store",
			},
		},
		{
			name:    "Staging HTTPS - 中等安全策略",
			mode:    "staging",
			isHTTPS: true,
			path:    "/api/v1/users",
			expectedHeaders: map[string]string{
				"X-Content-Type-Options":            "nosniff",
				"X-Frame-Options":                   "DENY",
				"X-XSS-Protection":                  "1; mode=block",
				"X-DNS-Prefetch-Control":            "off",
				"X-Download-Options":                "noopen",
				"X-Permitted-Cross-Domain-Policies": "none",
				"X-Server":                          "go-server",
				"Referrer-Policy":                   "strict-origin-when-cross-origin",
				"Cross-Origin-Embedder-Policy":      "require-corp",
				"Cross-Origin-Resource-Policy":      "same-origin",
				"Cross-Origin-Opener-Policy":        "same-origin",
				"Permissions-Policy":                "geolocation=(), microphone=(), camera=(), payment=(), usb=(), magnetometer=(), gyroscope=(), accelerometer=(), autoplay=(), encrypted-media=(), fullscreen=(), picture-in-picture=(), interest-cohort=()",
				"Strict-Transport-Security":         "max-age=2592000; includeSubDomains",
				"Cache-Control":                     "no-store, no-cache, must-revalidate, proxy-revalidate",
				"Pragma":                            "no-cache",
				"Expires":                           "0",
				"Surrogate-Control":                 "no-store",
			},
		},
		{
			name:    "Development HTTPS - 宽松安全策略",
			mode:    "development",
			isHTTPS: true,
			path:    "/api/v1/users",
			expectedHeaders: map[string]string{
				"X-Content-Type-Options":            "nosniff",
				"X-Frame-Options":                   "DENY",
				"X-XSS-Protection":                  "1; mode=block",
				"X-DNS-Prefetch-Control":            "off",
				"X-Download-Options":                "noopen",
				"X-Permitted-Cross-Domain-Policies": "none",
				"X-Server":                          "go-server",
				"Referrer-Policy":                   "strict-origin-when-cross-origin",
				"Cross-Origin-Embedder-Policy":      "require-corp",
				"Cross-Origin-Resource-Policy":      "same-origin",
				"Cross-Origin-Opener-Policy":        "same-origin",
				"Permissions-Policy":                "geolocation=(), microphone=(), camera=(), payment=(), usb=(), magnetometer=(), gyroscope=(), accelerometer=(), autoplay=(), encrypted-media=(), fullscreen=(), picture-in-picture=(), interest-cohort=()",
				"Strict-Transport-Security":         "max-age=86400; includeSubDomains",
				"Cache-Control":                     "no-store, no-cache, must-revalidate, proxy-revalidate",
				"Pragma":                            "no-cache",
				"Expires":                           "0",
				"Surrogate-Control":                 "no-store",
			},
		},
		{
			name:    "HTTP - 无HSTS头",
			mode:    "production",
			isHTTPS: false,
			path:    "/api/v1/users",
			expectedHeaders: map[string]string{
				"X-Content-Type-Options":            "nosniff",
				"X-Frame-Options":                   "DENY",
				"X-XSS-Protection":                  "1; mode=block",
				"X-DNS-Prefetch-Control":            "off",
				"X-Download-Options":                "noopen",
				"X-Permitted-Cross-Domain-Policies": "none",
				"X-Server":                          "go-server",
				"Referrer-Policy":                   "strict-origin-when-cross-origin",
				"Cross-Origin-Embedder-Policy":      "require-corp",
				"Cross-Origin-Resource-Policy":      "same-origin",
				"Cross-Origin-Opener-Policy":        "same-origin",
				"Permissions-Policy":                "geolocation=(), microphone=(), camera=(), payment=(), usb=(), magnetometer=(), gyroscope=(), accelerometer=(), autoplay=(), encrypted-media=(), fullscreen=(), picture-in-picture=(), interest-cohort=()",
				"Cache-Control":                     "no-store, no-cache, must-revalidate, proxy-revalidate",
				"Pragma":                            "no-cache",
				"Expires":                           "0",
				"Surrogate-Control":                 "no-store",
			},
		},
		{
			name:    "Swagger文档页面 - 特殊CSP策略和宽松CORS",
			mode:    "production",
			isHTTPS: true,
			path:    "/swagger/index.html",
			expectedHeaders: map[string]string{
				"X-Content-Type-Options":            "nosniff",
				"X-Frame-Options":                   "DENY",
				"X-XSS-Protection":                  "1; mode=block",
				"X-DNS-Prefetch-Control":            "off",
				"X-Download-Options":                "noopen",
				"X-Permitted-Cross-Domain-Policies": "none",
				"X-Server":                          "go-server",
				"Referrer-Policy":                   "strict-origin-when-cross-origin",
				"Permissions-Policy":                "geolocation=(), microphone=(), camera=(), payment=(), usb=(), magnetometer=(), gyroscope=(), accelerometer=(), autoplay=(), encrypted-media=(), fullscreen=(), picture-in-picture=(), interest-cohort=()",
				"Strict-Transport-Security":         "max-age=31536000; includeSubDomains; preload",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建配置
			cfg := &config.Config{
				Mode: tt.mode,
			}

			// 创建Gin路由器
			router := gin.New()
			router.Use(SecurityHeadersMiddleware(cfg))
			router.GET(tt.path, func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"message": "success"})
			})

			// 创建请求
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			if tt.isHTTPS {
				req.TLS = &tls.ConnectionState{} // 模拟HTTPS连接
			}

			// 记录响应
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// 验证响应状态
			assert.Equal(t, http.StatusOK, w.Code)

			// 验证安全头
			for header, expectedValue := range tt.expectedHeaders {
				actualValue := w.Header().Get(header)
				assert.Equal(t, expectedValue, actualValue,
					"Header %s should have value %s", header, expectedValue)
			}

			// 验证CSP头是否存在且格式正确
			cspHeader := w.Header().Get("Content-Security-Policy")
			assert.NotEmpty(t, cspHeader, "Content-Security-Policy header should be set")

			if !strings.Contains(tt.path, "/swagger/") {
				if tt.mode == "production" {
					// API路径使用严格的CSP策略（不包含upgrade-insecure-requests）
					if strings.Contains(tt.path, "/api/") {
						// API路径使用最严格的策略
						assert.Contains(t, cspHeader, "default-src 'none'")
						assert.Contains(t, cspHeader, "script-src 'none'")
					} else {
						// 非API路径的生产环境CSP应该包含upgrade-insecure-requests
						assert.Contains(t, cspHeader, "upgrade-insecure-requests",
							"Production CSP should include upgrade-insecure-requests")
					}
				} else {
					// 开发环境CSP应该允许WebSocket
					assert.Contains(t, cspHeader, "ws: wss:",
						"Development CSP should allow WebSocket connections")
				}
			}
		})
	}
}

func TestSecurityHeadersMiddleware_CSPPreventsXSS(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		Mode: "production",
	}

	router := gin.New()
	router.Use(SecurityHeadersMiddleware(cfg))
	router.GET("/test", func(c *gin.Context) {
		// 模拟包含潜在XSS的响应
		c.Header("Content-Type", "text/html")
		c.String(http.StatusOK, "<script>alert('XSS')</script>")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.TLS = &tls.ConnectionState{}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 验证CSP头包含script-src限制
	cspHeader := w.Header().Get("Content-Security-Policy")
	assert.Contains(t, cspHeader, "script-src", "CSP should restrict script sources")
	assert.Contains(t, cspHeader, "'self'", "CSP should allow only self sources")
}

func TestSecurityHeadersMiddleware_HSTSNotSetOnHTTP(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		Mode: "production",
	}

	router := gin.New()
	router.Use(SecurityHeadersMiddleware(cfg))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	// HTTP请求（无TLS）
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 验证HSTS头未设置
	hstsHeader := w.Header().Get("Strict-Transport-Security")
	assert.Empty(t, hstsHeader, "HSTS should not be set on HTTP requests")
}

// TestEnhancedSecurityHeaders 测试新增的安全头
func TestEnhancedSecurityHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		Mode: "production",
	}

	router := gin.New()
	router.Use(SecurityHeadersMiddleware(cfg))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.TLS = &tls.ConnectionState{}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 验证增强的安全头
	expectedEnhancedHeaders := map[string]string{
		"X-DNS-Prefetch-Control":            "off",
		"X-Download-Options":                "noopen",
		"X-Permitted-Cross-Domain-Policies": "none",
		"X-Server":                          "go-server",
		"Cross-Origin-Opener-Policy":        "same-origin",
	}

	for header, expectedValue := range expectedEnhancedHeaders {
		actualValue := w.Header().Get(header)
		assert.Equal(t, expectedValue, actualValue,
			"Enhanced header %s should have value %s", header, expectedValue)
	}

	// 验证增强的权限策略包含新的功能限制
	permissionsPolicy := w.Header().Get("Permissions-Policy")
	assert.Contains(t, permissionsPolicy, "autoplay=()")
	assert.Contains(t, permissionsPolicy, "encrypted-media=()")
	assert.Contains(t, permissionsPolicy, "fullscreen=()")
	assert.Contains(t, permissionsPolicy, "picture-in-picture=()")
	assert.Contains(t, permissionsPolicy, "interest-cohort=()")
}

// TestCacheControlHeaders 测试缓存控制头
func TestCacheControlHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		Mode: "production",
	}

	router := gin.New()
	router.Use(SecurityHeadersMiddleware(cfg))

	tests := []struct {
		name               string
		path               string
		expectCacheHeaders bool
	}{
		{
			name:               "API端点应有缓存控制头",
			path:               "/api/v1/users",
			expectCacheHeaders: true,
		},
		{
			name:               "非API端点不应有缓存控制头",
			path:               "/static/style.css",
			expectCacheHeaders: false,
		},
		{
			name:               "Swagger页面不应有缓存控制头",
			path:               "/swagger/index.html",
			expectCacheHeaders: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router.GET(tt.path, func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"message": "success"})
			})

			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			req.TLS = &tls.ConnectionState{}
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if tt.expectCacheHeaders {
				assert.Equal(t, "no-store, no-cache, must-revalidate, proxy-revalidate",
					w.Header().Get("Cache-Control"))
				assert.Equal(t, "no-cache", w.Header().Get("Pragma"))
				assert.Equal(t, "0", w.Header().Get("Expires"))
				assert.Equal(t, "no-store", w.Header().Get("Surrogate-Control"))
			} else {
				// 非API路径不应该设置这些缓存控制头
				cacheControl := w.Header().Get("Cache-Control")
				if cacheControl != "" {
					assert.NotEqual(t, "no-store, no-cache, must-revalidate, proxy-revalidate", cacheControl)
				}
			}
		})
	}
}

// TestCSPProductionVsDevelopment 测试生产环境和开发环境的CSP差异
func TestCSPProductionVsDevelopment(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name               string
		mode               string
		expectedCSPContent []string
		notExpectedCSP     []string
	}{
		{
			name: "生产环境CSP应严格限制",
			mode: "production",
			expectedCSPContent: []string{
				"upgrade-insecure-requests",
				"script-src 'self'",
				"connect-src 'self'",
			},
			notExpectedCSP: []string{
				"ws:",
				"wss:",
				"blob:",
			},
		},
		{
			name: "开发环境CSP应允许更多资源",
			mode: "development",
			expectedCSPContent: []string{
				"ws:",
				"wss:",
				"blob:",
				"unsafe-eval",
			},
			notExpectedCSP: []string{
				"upgrade-insecure-requests",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Mode: tt.mode,
			}

			router := gin.New()
			router.Use(SecurityHeadersMiddleware(cfg))
			router.GET("/test", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"message": "success"})
			})

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.TLS = &tls.ConnectionState{}
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			cspHeader := w.Header().Get("Content-Security-Policy")
			assert.NotEmpty(t, cspHeader, "CSP header should be set")

			// 检查期望的CSP内容
			for _, expectedContent := range tt.expectedCSPContent {
				assert.Contains(t, cspHeader, expectedContent,
					"CSP should contain %s for %s mode", expectedContent, tt.mode)
			}

			// 检查不应该出现在CSP中的内容
			for _, notExpected := range tt.notExpectedCSP {
				assert.NotContains(t, cspHeader, notExpected,
					"CSP should not contain %s for %s mode", notExpected, tt.mode)
			}
		})
	}
}

// TestAPIStrictCSP 测试API路径的严格CSP策略
func TestAPIStrictCSP(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		Mode: "production",
	}

	router := gin.New()
	router.Use(SecurityHeadersMiddleware(cfg))
	router.GET("/api/v1/users", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	req.TLS = &tls.ConnectionState{}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	cspHeader := w.Header().Get("Content-Security-Policy")
	assert.NotEmpty(t, cspHeader, "CSP header should be set")

	// API路径应该使用严格的CSP策略
	assert.Contains(t, cspHeader, "default-src 'none'")
	assert.Contains(t, cspHeader, "script-src 'none'")
	assert.Contains(t, cspHeader, "style-src 'none'")
	assert.Contains(t, cspHeader, "img-src 'none'")
	assert.Contains(t, cspHeader, "font-src 'none'")
	assert.Contains(t, cspHeader, "base-uri 'none'")
	assert.Contains(t, cspHeader, "form-action 'none'")
	assert.Contains(t, cspHeader, "connect-src 'self'") // API需要允许连接
}

// buildProductionCSP 构建生产环境CSP字符串
func buildProductionCSP(path string) string {
	return "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; font-src 'self'; connect-src 'self'; frame-ancestors 'none';"
}

// buildDevelopmentCSP 构建开发环境CSP字符串
func buildDevelopmentCSP() string {
	return "default-src 'self' 'unsafe-inline' 'unsafe-eval' *; img-src 'self' data: *;"
}

// TestBuildProductionCSP 测试生产环境CSP构建函数
func TestBuildProductionCSP(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "API路径使用严格CSP",
			path:     "/api/v1/users",
			expected: "default-src 'none'; script-src 'none'; style-src 'none'; img-src 'none'; connect-src 'self'; font-src 'none'; object-src 'none'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'",
		},
		{
			name:     "非API路径使用标准生产CSP",
			path:     "/health",
			expected: "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; font-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'; upgrade-insecure-requests",
		},
		{
			name:     "根路径使用标准生产CSP",
			path:     "/",
			expected: "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; font-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'; upgrade-insecure-requests",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildProductionCSP(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestBuildDevelopmentCSP 测试开发环境CSP构建函数
func TestBuildDevelopmentCSP(t *testing.T) {
	result := buildDevelopmentCSP()

	expected := "default-src 'self' 'unsafe-inline' 'unsafe-eval'; " +
		"script-src 'self' 'unsafe-inline' 'unsafe-eval' blob:; " +
		"style-src 'self' 'unsafe-inline'; " +
		"img-src 'self' data: blob: https: http:; " +
		"font-src 'self' data:; " +
		"connect-src 'self' ws: wss: http: https:; " +
		"frame-ancestors 'none'; " +
		"base-uri 'self'; " +
		"form-action 'self'"

	assert.Equal(t, expected, result)
}
