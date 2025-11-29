package middleware

import (
	"go-server/internal/domain/user"
	"go-server/internal/repositories"
	"net/http"
	"strings"

	"go-server/pkg/auth"
	"go-server/pkg/response"

	"github.com/gin-gonic/gin"
)

func AuthMiddleware(jwtManager *auth.JWTManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			response.Error(c, http.StatusUnauthorized, "Authorization header is required")
			c.Abort()
			return
		}

		// Extract token from "Bearer <token>"
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			response.Error(c, http.StatusUnauthorized, "Bearer token is required")
			c.Abort()
			return
		}

		claims, err := jwtManager.ValidateToken(tokenString)
		if err != nil {
			response.Error(c, http.StatusUnauthorized, "Invalid token")
			c.Abort()
			return
		}

		// Set user context
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("email", claims.Email)
		c.Next()
	}
}

func OptionalAuthMiddleware(jwtManager *auth.JWTManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader != "" {
			tokenString := strings.TrimPrefix(authHeader, "Bearer ")
			if tokenString != authHeader {
				if claims, err := jwtManager.ValidateToken(tokenString); err == nil {
					c.Set("user_id", claims.UserID)
					c.Set("username", claims.Username)
					c.Set("email", claims.Email)
				}
			}
		}
		c.Next()
	}
}

func AdminOnlyMiddleware(userRepo repositories.UserRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDVal, exists := c.Get("user_id")
		if !exists {
			response.Error(c, http.StatusUnauthorized, "User not authenticated")
			c.Abort()
			return
		}

		userID, ok := userIDVal.(string)
		if !ok {
			response.Error(c, http.StatusUnauthorized, "Invalid user ID format in context")
			c.Abort()
			return
		}

		// Fetch user model from the database using the correct repository method
		userModel, err := userRepo.GetByID(userID)
		if err != nil {
			response.Error(c, http.StatusForbidden, "User not found or repository error")
			c.Abort()
			return
		}

		// Map the data model to the domain model to check for admin role
		mapper := user.NewMapper()
		userDomainModel, err := mapper.ToDomainModel(userModel)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, "Could not process user data")
			c.Abort()
			return
		}

		// Check if the user has an admin role
		if !userDomainModel.IsAdmin() {
			response.Error(c, http.StatusForbidden, "Admin access required")
			c.Abort()
			return
		}

		c.Next()
	}
}
