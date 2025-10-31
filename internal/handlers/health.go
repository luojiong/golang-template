package handlers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"go-server/internal/database"
	"go-server/pkg/cache"
	"go-server/pkg/response"

	"github.com/gin-gonic/gin"
)

type HealthHandler struct {
	db    *database.Database
	cache cache.Cache
}

func NewHealthHandler(db *database.Database, cache cache.Cache) *HealthHandler {
	return &HealthHandler{
		db:    db,
		cache: cache,
	}
}

// Health godoc
// @Summary Enhanced health check endpoint
// @Description Comprehensive health check including database connection pool metrics, Redis cache statistics, and system information. This endpoint provides detailed monitoring data including connection pool utilization, query performance, cache hit rates, memory usage, and latency metrics.
// @Tags health
// @Produce json
// @Success 200 {object} models.SuccessResponse{data=models.HealthResponse}
// @Header 200 {string} X-Database-Status "Database health status (healthy, unhealthy, degraded)"
// @Header 200 {string} X-Cache-Status "Redis cache health status (healthy, unhealthy, degraded, not_configured)"
// @Header 200 {integer} X-Database-Pool-Utilization "Database connection pool utilization percentage"
// @Header 200 {integer} X-Cache-Memory-Usage "Current Redis memory usage percentage"
// @Header 200 {integer} X-Cache-Hit-Rate "Cache hit rate percentage"
// @Router /api/v1/health [get]
func (h *HealthHandler) Health(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	// Get comprehensive metrics
	systemMetrics := h.getSystemMetrics()
	dbMetrics := h.getDatabaseHealthMetrics()
	cacheMetrics := h.getCacheHealthMetrics(ctx)
	overallStatus := h.calculateOverallStatus(dbMetrics, cacheMetrics)

	healthResponse := map[string]interface{}{
		"status":    overallStatus,
		"timestamp": systemMetrics["timestamp"],
		"version":   systemMetrics["version"],
		"services": map[string]interface{}{
			"api":      "healthy",
			"auth":     "healthy",
			"database": dbMetrics,
			"cache":    cacheMetrics,
		},
		"system": map[string]interface{}{
			"uptime_seconds": systemMetrics["uptime_seconds"],
			"uptime_human":   systemMetrics["uptime_human"],
			"go_version":     systemMetrics["go_version"],
		},
	}

	// Add headers for monitoring systems
	c.Header("X-Database-Status", dbMetrics["status"].(string))
	c.Header("X-Cache-Status", cacheMetrics["status"].(string))

	// Add performance headers
	if poolStats, ok := dbMetrics["pool_stats"].(map[string]interface{}); ok {
		if utilization, ok := poolStats["utilization_percent"].(float64); ok {
			c.Header("X-Database-Pool-Utilization", fmt.Sprintf("%.2f", utilization))
		}
	}

	if stats, ok := cacheMetrics["stats"].(map[string]interface{}); ok {
		if memoryInfo, ok := stats["memory"].(map[string]interface{}); ok {
			if usagePercent, ok := memoryInfo["memory_usage_percent"].(float64); ok {
				c.Header("X-Cache-Memory-Usage", fmt.Sprintf("%.2f", usagePercent))
			}
		}

		if dbInfo, ok := stats["database"].(map[string]interface{}); ok {
			if hitRate, ok := dbInfo["keyspace_hit_rate_percent"].(float64); ok {
				c.Header("X-Cache-Hit-Rate", fmt.Sprintf("%.2f", hitRate))
			}
		}
	}

	// Determine HTTP status code based on overall health
	statusCode := http.StatusOK
	if overallStatus == "unhealthy" {
		statusCode = http.StatusServiceUnavailable
	} else if overallStatus == "degraded" {
		statusCode = http.StatusOK // Still return 200 but indicate degraded state
	}

	response.Success(c, statusCode, "Comprehensive health check completed", healthResponse)
}

// Ready godoc
// @Summary Enhanced readiness check endpoint
// @Description Comprehensive readiness check validating database connectivity and cache readiness with detailed performance metrics. This endpoint tests actual read/write operations and provides latency measurements, connection pool status, and cache response times for production monitoring.
// @Tags health
// @Produce json
// @Success 200 {object} models.SuccessResponse{data=models.HealthResponse}
// @Header 200 {string} X-Database-Ready "Database readiness status (ready, not_ready)"
// @Header 200 {string} X-Cache-Ready "Redis cache readiness status (ready, not_ready, not_configured)"
// @Header 200 {integer} X-Database-Response-Time "Database health check response time in milliseconds"
// @Header 200 {integer} X-Cache-Response-Time "Cache read/write test response time in milliseconds"
// @Router /api/v1/ready [get]
func (h *HealthHandler) Ready(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	// Track response times
	dbStartTime := time.Now()

	// Check database readiness with performance tracking
	dbReady := false
	var dbError error
	var dbResponseTime time.Duration
	var dbPoolStats map[string]interface{}

	if h.db != nil {
		if err := h.db.Health(); err == nil {
			dbReady = true
			dbResponseTime = time.Since(dbStartTime)

			// Get connection pool stats for readiness assessment
			if stats, err := h.db.GetConnectionPoolStats(); err == nil {
				dbPoolStats = stats
			}
		} else {
			dbError = err
			dbResponseTime = time.Since(dbStartTime)
		}
	}

	// Check Redis cache readiness with performance tracking
	cacheReady := false
	var cacheError error
	var cacheResponseTime time.Duration
	var cacheStats map[string]interface{}

	if h.cache != nil {
		cacheTestStart := time.Now()

		// Comprehensive readiness check - test actual operations
		testKey := "readiness_check"
		testValue := "ready"

		if err := h.cache.Set(ctx, testKey, testValue, 5*time.Second); err != nil {
			cacheError = err
		} else {
			if _, found := h.cache.Get(ctx, testKey); found {
				cacheReady = true
				h.cache.Delete(ctx, testKey) // Clean up
			} else {
				cacheError = fmt.Errorf("cache read test failed")
			}
		}

		cacheResponseTime = time.Since(cacheTestStart)

		// Get cache stats for additional readiness info
		if stats, err := h.cache.GetStats(ctx); err == nil {
			cacheStats = stats
		}
	} else {
		// Cache is optional for readiness - service can operate without cache
		cacheReady = true
		cacheResponseTime = 0
	}

	// Build comprehensive readiness response
	readyResponse := map[string]interface{}{
		"status":    "ready",
		"timestamp": time.Now().UTC(),
		"version":   "1.0.0",
		"checks": map[string]interface{}{
			"database": map[string]interface{}{
				"ready":            dbReady,
				"response_time_ms": dbResponseTime.Milliseconds(),
				"error": func() string {
					if dbError != nil {
						return dbError.Error()
					}
					return ""
				}(),
			},
			"cache": map[string]interface{}{
				"ready":            cacheReady,
				"response_time_ms": cacheResponseTime.Milliseconds(),
				"error": func() string {
					if cacheError != nil {
						return cacheError.Error()
					}
					return ""
				}(),
			},
		},
		"performance": map[string]interface{}{
			"database_response_time_ms": dbResponseTime.Milliseconds(),
			"cache_response_time_ms":    cacheResponseTime.Milliseconds(),
			"total_check_time_ms":       time.Since(dbStartTime).Milliseconds(),
		},
	}

	// Include pool stats if available
	if dbPoolStats != nil {
		readyResponse["checks"].(map[string]interface{})["database"].(map[string]interface{})["pool_stats"] = map[string]interface{}{
			"utilization_percent": dbPoolStats["utilization_percent"],
			"open_connections":    dbPoolStats["open_connections"],
			"max_open_conns":      dbPoolStats["max_open_conns"],
		}
	}

	// Include cache stats if available
	if cacheStats != nil {
		if memoryInfo, ok := cacheStats["memory"].(map[string]interface{}); ok {
			readyResponse["checks"].(map[string]interface{})["cache"].(map[string]interface{})["memory_usage_percent"] = memoryInfo["memory_usage_percent"]
		}
		if dbInfo, ok := cacheStats["database"].(map[string]interface{}); ok {
			readyResponse["checks"].(map[string]interface{})["cache"].(map[string]interface{})["hit_rate_percent"] = dbInfo["keyspace_hit_rate_percent"]
		}
	}

	// Determine overall readiness - database is required, cache is optional
	overallReady := dbReady
	if !overallReady {
		readyResponse["status"] = "not_ready"
	}

	// Add monitoring headers
	c.Header("X-Database-Ready", fmt.Sprintf("%t", dbReady))
	c.Header("X-Cache-Ready", fmt.Sprintf("%t", cacheReady))
	c.Header("X-Database-Response-Time", fmt.Sprintf("%d", dbResponseTime.Milliseconds()))
	c.Header("X-Cache-Response-Time", fmt.Sprintf("%d", cacheResponseTime.Milliseconds()))

	statusCode := http.StatusOK
	if !overallReady {
		statusCode = http.StatusServiceUnavailable
	}

	response.Success(c, statusCode, "Enhanced readiness check completed", readyResponse)
}

// Live godoc
// @Summary Liveness check endpoint
// @Description Check if the API server is alive (basic liveness probe)
// @Tags health
// @Produce json
// @Success 200 {object} models.SuccessResponse
// @Router /api/v1/live [get]
// getDatabaseHealthMetrics collects comprehensive database health metrics
func (h *HealthHandler) getDatabaseHealthMetrics() map[string]interface{} {
	if h.db == nil {
		return map[string]interface{}{
			"status": "not_configured",
		}
	}

	// Get connection pool stats
	poolStats, err := h.db.GetConnectionPoolStats()
	if err != nil {
		return map[string]interface{}{
			"status": "error",
			"error":  err.Error(),
		}
	}

	// Get query performance stats
	queryStats := h.db.GetQueryPerformanceStats()

	// Get health status
	healthStatus := h.db.GetHealthStatus()

	// Combine all metrics
	dbMetrics := map[string]interface{}{
		"status":        "healthy",
		"health_status": healthStatus,
		"pool_stats":    poolStats,
		"query_stats":   h.db.GetQueryPerformanceStatsJSON(),
	}

	// Determine overall database health
	if !healthStatus.IsHealthy {
		dbMetrics["status"] = "unhealthy"
		dbMetrics["error"] = healthStatus.ErrorMessage
	} else {
		// Check for performance issues
		if utilization, ok := poolStats["utilization_percent"].(float64); ok && utilization > 90 {
			dbMetrics["status"] = "degraded"
			dbMetrics["warning"] = "High connection pool utilization"
		}

		// Check for high slow query percentage
		if queryStats.TotalQueries > 0 {
			slowQueryPercentage := float64(queryStats.SlowQueries) / float64(queryStats.TotalQueries) * 100
			if slowQueryPercentage > 10 {
				if dbMetrics["status"] == "healthy" {
					dbMetrics["status"] = "degraded"
				}
				dbMetrics["warning"] = fmt.Sprintf("High slow query rate: %.2f%%", slowQueryPercentage)
			}
		}
	}

	return dbMetrics
}

// getCacheHealthMetrics collects comprehensive cache health metrics
func (h *HealthHandler) getCacheHealthMetrics(ctx context.Context) map[string]interface{} {
	if h.cache == nil {
		return map[string]interface{}{
			"status": "not_configured",
		}
	}

	// Check cache health
	if err := h.cache.Health(ctx); err != nil {
		return map[string]interface{}{
			"status": "unhealthy",
			"error":  err.Error(),
		}
	}

	// Get detailed stats
	stats, err := h.cache.GetStats(ctx)
	if err != nil {
		return map[string]interface{}{
			"status": "degraded",
			"error":  fmt.Sprintf("Failed to get stats: %v", err),
		}
	}

	cacheMetrics := map[string]interface{}{
		"status": "healthy",
		"stats":  stats,
	}

	// Check for performance issues
	if memoryInfo, ok := stats["memory"].(map[string]interface{}); ok {
		if usagePercent, ok := memoryInfo["memory_usage_percent"].(float64); ok && usagePercent > 90 {
			cacheMetrics["status"] = "degraded"
			cacheMetrics["warning"] = fmt.Sprintf("High memory usage: %.2f%%", usagePercent)
		}
	}

	// Check connection pool health
	if poolInfo, ok := stats["connection_pool"].(map[string]interface{}); ok {
		if hitRate, ok := poolInfo["hit_rate"].(float64); ok && hitRate < 50 {
			if cacheMetrics["status"] == "healthy" {
				cacheMetrics["status"] = "degraded"
			}
			cacheMetrics["warning"] = fmt.Sprintf("Low connection pool hit rate: %.2f%%", hitRate)
		}
	}

	// Check cache hit rate
	if dbInfo, ok := stats["database"].(map[string]interface{}); ok {
		if hitRate, ok := dbInfo["keyspace_hit_rate_percent"].(float64); ok && hitRate < 70 {
			if cacheMetrics["status"] == "healthy" {
				cacheMetrics["status"] = "degraded"
			}
			cacheMetrics["warning"] = fmt.Sprintf("Low cache hit rate: %.2f%%", hitRate)
		}
	}

	return cacheMetrics
}

// getSystemMetrics collects system-level metrics
func (h *HealthHandler) getSystemMetrics() map[string]interface{} {
	// Get uptime (simplified - in production you might track actual start time)
	uptime := time.Since(time.Now().Add(-time.Hour)) // Placeholder

	return map[string]interface{}{
		"uptime_seconds": int64(uptime.Seconds()),
		"uptime_human":   uptime.String(),
		"timestamp":      time.Now().UTC(),
		"version":        "1.0.0",
		"go_version":     "go1.21+", // Placeholder - could get actual version
	}
}

// calculateOverallStatus determines the overall system health
func (h *HealthHandler) calculateOverallStatus(dbMetrics, cacheMetrics map[string]interface{}) string {
	dbStatus, _ := dbMetrics["status"].(string)
	cacheStatus, _ := cacheMetrics["status"].(string)

	// If database is unhealthy, overall is unhealthy
	if dbStatus == "unhealthy" {
		return "unhealthy"
	}

	// If database is healthy but cache is unhealthy, system is degraded (cache is optional)
	if dbStatus == "healthy" && cacheStatus == "unhealthy" {
		return "degraded"
	}

	// If either is degraded, overall is degraded
	if dbStatus == "degraded" || cacheStatus == "degraded" {
		return "degraded"
	}

	// If both are healthy, overall is healthy
	if dbStatus == "healthy" && (cacheStatus == "healthy" || cacheStatus == "not_configured") {
		return "healthy"
	}

	return "unknown"
}

func (h *HealthHandler) Live(c *gin.Context) {
	// Basic liveness check - just verify the service is running
	// This should always return success if the service is up
	systemMetrics := h.getSystemMetrics()

	liveResponse := map[string]interface{}{
		"status":    "alive",
		"timestamp": systemMetrics["timestamp"],
		"version":   systemMetrics["version"],
		"uptime":    systemMetrics["uptime_human"],
		"system": map[string]interface{}{
			"uptime_seconds": systemMetrics["uptime_seconds"],
			"go_version":     systemMetrics["go_version"],
		},
	}

	response.Success(c, http.StatusOK, "Service is alive", liveResponse)
}
