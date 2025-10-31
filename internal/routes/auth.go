package routes

import (
	"go-server/internal/handlers"
	"go-server/internal/middleware"
	"go-server/pkg/auth"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func SetupAuthRoutes(router *gin.Engine, authHandler *handlers.AuthHandler, jwtManager *auth.JWTManager) {
	// Swagger documentation
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	authGroup := router.Group("/api/v1/auth")
	{
		authGroup.POST("/login", authHandler.Login)
		authGroup.POST("/register", authHandler.Register)
		authGroup.POST("/change-password", authHandler.ChangePassword)

		// Protected routes
		protected := authGroup.Group("")
		protected.Use(middleware.AuthMiddleware(jwtManager))
		{
			protected.GET("/me", authHandler.Me)
			protected.POST("/logout", authHandler.Logout)
		}
	}
}
