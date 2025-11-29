package routes

import (
	"go-server/internal/handlers"
	"go-server/internal/repositories"
	"go-server/pkg/auth"

	"github.com/gin-gonic/gin"
)

type Router struct {
	engine         *gin.Engine
	authHandler    *handlers.AuthHandler
	userHandler    *handlers.UserHandler
	healthHandler  *handlers.HealthHandler
	jwtManager     *auth.JWTManager
	userRepository repositories.UserRepository
}

func NewRouter(
	authHandler *handlers.AuthHandler,
	userHandler *handlers.UserHandler,
	healthHandler *handlers.HealthHandler,
	jwtManager *auth.JWTManager,
	userRepository repositories.UserRepository,
	middlewares []gin.HandlerFunc,
) *Router {
	// Note: Gin mode is already set in SetupMiddlewares, but we ensure consistent mode here
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()

	// Apply all configured middlewares in the correct order:
	// 1. Structured logging (REQ-MW-003) - logs all requests with correlation IDs, method, path, status, duration
	// 2. Recovery - handles panics with proper error responses and stack traces
	// 3. CORS - handles cross-origin requests with environment-specific origins
	// 4. Security headers - applies security policies including HSTS and CSP
	// 5. Rate limiting (REQ-MW-001) - applies distributed rate limiting with Redis fallback (100/min per IP)
	// 6. Compression (REQ-MW-002) - compresses responses > 1KB when supported by client
	// 7. Request size limiting - prevents oversized requests (10MB limit)
	engine.Use(middlewares...)

	return &Router{
		engine:         engine,
		authHandler:    authHandler,
		userHandler:    userHandler,
		healthHandler:  healthHandler,
		jwtManager:     jwtManager,
		userRepository: userRepository,
	}
}

func (r *Router) SetupRoutes() {
	// Health routes (no auth required)
	SetupHealthRoutes(r.engine, r.healthHandler)

	// Auth routes
	SetupAuthRoutes(r.engine, r.authHandler, r.jwtManager)

	// User routes
	r.SetupUserRoutes()

	// Welcome route with enhanced middleware integration
	r.engine.GET("/", func(c *gin.Context) {
		// Demonstrate correlation ID from structured logging middleware (REQ-MW-003)
		correlationID := c.GetHeader("X-Correlation-ID")
		if correlationID == "" {
			correlationID = "N/A"
		}

		c.JSON(200, gin.H{
			"message":        "Welcome to Golang Template API",
			"version":        "1.0.0",
			"docs":           "/swagger/index.html",
			"correlation_id": correlationID,
			"features": gin.H{
				"structured_logging":        "Enabled with correlation IDs (REQ-MW-003)",
				"distributed_rate_limiting": "Enabled with Redis support (REQ-MW-001)",
				"compression":               "Enabled for responses > 1KB (REQ-MW-002)",
				"security_headers":          "Enhanced with HSTS and CSP",
			},
		})
	})

	// API info route with middleware details
	r.engine.GET("/api/v1", func(c *gin.Context) {
		// Get correlation ID from the structured logging middleware
		correlationID := c.GetHeader("X-Correlation-ID")
		if correlationID == "" {
			correlationID = "N/A"
		}

		// Check if rate limit headers are present (REQ-MW-001)
		rateLimitInfo := gin.H{
			"enabled":             true,
			"anonymous_limit":     "100 requests/minute",
			"authenticated_limit": "200 requests/minute",
			"per_ip":              true,
		}

		c.JSON(200, gin.H{
			"name":           "Golang Template API",
			"version":        "1.0.0",
			"correlation_id": correlationID,
			"endpoints": gin.H{
				"health": "/api/v1/health",
				"auth":   "/api/v1/auth",
				"users":  "/api/v1/users",
				"docs":   "/swagger/index.html",
			},
			"middleware_features": gin.H{
				"structured_logging": gin.H{
					"enabled":        true,
					"format":         "JSON",
					"correlation_id": correlationID,
					"requirement":    "REQ-MW-003",
					"description":    "Structured JSON logging with correlation IDs, method, path, status, and duration",
				},
				"rate_limiting": gin.H{
					"enabled":     true,
					"type":        "Distributed with Redis fallback",
					"requirement": "REQ-MW-001",
					"description": "Rate limiting across multiple instances (100 requests/minute per IP)",
					"limits":      rateLimitInfo,
				},
				"compression": gin.H{
					"enabled":     true,
					"type":        "gzip",
					"threshold":   "1KB",
					"requirement": "REQ-MW-002",
					"description": "Compresses responses larger than 1KB when client supports it",
				},
				"security": gin.H{
					"enabled":     true,
					"features":    "HSTS, CSP, XSS Protection, CORS",
					"description": "Enhanced security headers and policies",
				},
			},
		})
	})
}

func (r *Router) GetEngine() *gin.Engine {
	return r.engine
}
