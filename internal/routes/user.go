package routes

import (
	"go-server/internal/middleware"
)

func (r *Router) SetupUserRoutes() {
	userGroup := r.engine.Group("/api/v1/users")
	userGroup.Use(middleware.AuthMiddleware(r.jwtManager))
	{
		// Routes available to any authenticated user
		userGroup.GET("/:id", r.userHandler.GetUser)

		// Routes available only to admins
		adminGroup := userGroup.Group("")
		adminGroup.Use(middleware.AdminOnlyMiddleware(r.userRepository))
		{
			adminGroup.GET("", r.userHandler.GetUsers)
			adminGroup.PUT("/:id", r.userHandler.UpdateUser)
			adminGroup.DELETE("/:id", r.userHandler.DeleteUser)
		}
	}
}
