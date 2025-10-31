package routes

import (
	"go-server/internal/handlers"

	"github.com/gin-gonic/gin"
)

func SetupHealthRoutes(router *gin.Engine, healthHandler *handlers.HealthHandler) {
	router.GET("/api/v1/health", healthHandler.Health)
	router.GET("/api/v1/ready", healthHandler.Ready)
	router.GET("/api/v1/live", healthHandler.Live)
}
