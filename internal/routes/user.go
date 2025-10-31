package routes

import (
	"go-server/internal/handlers"
	"go-server/internal/middleware"
	"go-server/pkg/auth"

	"github.com/gin-gonic/gin"
)

func SetupUserRoutes(router *gin.Engine, userHandler *handlers.UserHandler, jwtManager *auth.JWTManager) {
	userGroup := router.Group("/api/v1/users")
	userGroup.Use(middleware.AuthMiddleware(jwtManager))
	{
		userGroup.GET("", userHandler.GetUsers)
		userGroup.GET("/:id", userHandler.GetUser)
		userGroup.PUT("/:id", userHandler.UpdateUser)
		userGroup.DELETE("/:id", userHandler.DeleteUser)
	}
}
