package handlers

import (
	"net/http"
	"strings"

	"go-server/internal/models"
	"go-server/internal/services"
	"go-server/pkg/auth"
	"go-server/pkg/cache"
	"go-server/pkg/errors"
	"go-server/pkg/response"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	jwtManager       *auth.JWTManager
	userService      services.UserService
	blacklistService *cache.BlacklistService
}

func NewAuthHandler(jwtManager *auth.JWTManager, userService services.UserService, blacklistService *cache.BlacklistService) *AuthHandler {
	return &AuthHandler{
		jwtManager:       jwtManager,
		userService:      userService,
		blacklistService: blacklistService,
	}
}

// Login godoc
// @Summary Login user
// @Description Authenticate a user and return a JWT token
// @Tags auth
// @Accept json
// @Produce json
// @Param loginRequest body models.LoginRequest true "Login credentials"
// @Success 200 {object} models.SuccessResponse{data=models.LoginResponse} "Login successful"
// @Failure 400 {object} models.ErrorResponse{error=models.EnhancedErrorResponse} "Validation error - Invalid input data"
// @Failure 401 {object} models.ErrorResponse{error=models.EnhancedErrorResponse} "Authentication error - Invalid credentials"
// @Header 200 {string} X-Correlation-ID "Unique identifier for request tracing"
// @Header 400 {string} X-Correlation-ID "Unique identifier for request tracing"
// @Header 401 {string} X-Correlation-ID "Unique identifier for request tracing"
// @Router /api/v1/auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req models.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, "Invalid request format: "+err.Error())
		return
	}

	// Validate credentials using user service
	user, err := h.userService.Login(&req)
	if err != nil {
		response.UnauthorizedError(c, "Invalid credentials")
		return
	}

	// Generate JWT token
	token, err := h.jwtManager.GenerateToken(user.ID, user.Username, user.Email)
	if err != nil {
		response.InternalServerErrorWithCause(c, "Failed to generate token", err)
		return
	}

	response.Success(c, http.StatusOK, "Login successful", models.LoginResponse{
		Token: token,
		User:  user.ToSafeUser(),
	})
}

// Register godoc
// @Summary Register new user
// @Description Register a new user account
// @Tags auth
// @Accept json
// @Produce json
// @Param registerRequest body models.RegisterRequest true "User registration data"
// @Success 201 {object} models.SuccessResponse{data=models.SafeUser} "Registration successful"
// @Failure 400 {object} models.ErrorResponse{error=models.EnhancedErrorResponse} "Validation error - Invalid input data with field-level details"
// @Failure 409 {object} models.ErrorResponse{error=models.EnhancedErrorResponse} "Conflict error - User already exists"
// @Header 201 {string} X-Correlation-ID "Unique identifier for request tracing"
// @Header 400 {string} X-Correlation-ID "Unique identifier for request tracing"
// @Header 409 {string} X-Correlation-ID "Unique identifier for request tracing"
// @Router /api/v1/auth/register [post]
func (h *AuthHandler) Register(c *gin.Context) {
	var req models.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, "Invalid request format: "+err.Error())
		return
	}

	// Create user using user service
	user, err := h.userService.Register(&req)
	if err != nil {
		if err.Error() == "user with this email already exists" {
			response.ConflictError(c, "Email already registered", map[string]interface{}{
				"field": "email",
				"value": req.Email,
			})
			return
		}
		if err.Error() == "username already taken" {
			response.ConflictError(c, "Username already taken", map[string]interface{}{
				"field": "username",
				"value": req.Username,
			})
			return
		}
		response.InternalServerErrorWithCause(c, "Failed to register user", err)
		return
	}

	response.Created(c, "User registered successfully", user.ToSafeUser())
}

// Me godoc
// @Summary Get current user profile
// @Description Get the profile of the currently authenticated user. This endpoint serves frequently accessed user profile data from Redis cache with 5-minute TTL. If Redis is unavailable, data is served directly from PostgreSQL database. Cache status is provided in response headers.
// @Tags auth
// @Produce json
// @Security BearerAuth
// @Success 200 {object} models.SuccessResponse{data=models.SafeUser}
// @Header 200 {string} X-Cache "Cache status (HIT, MISS, BYPASS, STALE)"
// @Header 200 {integer} X-Cache-TTL "Time to live in seconds for cached data"
// @Header 200 {string} X-Cache-Backend "Cache backend used (redis, database)"
// @Failure 401 {object} models.ErrorResponse
// @Router /api/v1/auth/me [get]
func (h *AuthHandler) Me(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		response.UnauthorizedError(c, "User not authenticated")
		return
	}

	// Get user from database using user service
	user, err := h.userService.GetByID(userID.(string))
	if err != nil {
		response.NotFoundError(c, "User", userID.(string))
		return
	}

	response.Success(c, http.StatusOK, "User profile retrieved successfully", user.ToSafeUser())
}

// ChangePassword godoc
// @Summary Change user password
// @Description Change the password of the currently authenticated user
// @Tags auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param changePasswordRequest body models.ChangePasswordRequest true "Password change data"
// @Success 200 {object} models.SuccessResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.ErrorResponse
// @Router /api/v1/auth/change-password [post]
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	var req models.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, "Invalid request format: "+err.Error())
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		response.UnauthorizedError(c, "User not authenticated")
		return
	}

	// Change password using user service
	if err := h.userService.ChangePassword(userID.(string), &req); err != nil {
		if err.Error() == "old password is incorrect" {
			response.ValidationError(c, "Old password is incorrect",
				errors.ErrorDetails{Field: "old_password", Message: "Old password is incorrect"})
			return
		}
		response.InternalServerErrorWithCause(c, "Failed to change password", err)
		return
	}

	response.Success(c, http.StatusOK, "Password changed successfully", nil)
}

// Logout godoc
// @Summary Logout user
// @Description Logout the current user by blacklisting their JWT token
// @Tags auth
// @Produce json
// @Security BearerAuth
// @Success 200 {object} models.SuccessResponse
// @Failure 401 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Router /api/v1/auth/logout [post]
func (h *AuthHandler) Logout(c *gin.Context) {
	// Extract the token from the Authorization header
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		response.UnauthorizedError(c, "Authorization header is required")
		return
	}

	// Remove "Bearer " prefix
	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	if tokenString == authHeader {
		// No "Bearer " prefix found
		response.InvalidTokenError(c, "Invalid authorization header format, expected 'Bearer <token>'")
		return
	}

	// Validate the token before blacklisting to ensure it's a valid token
	claims, err := h.jwtManager.ValidateToken(tokenString)
	if err != nil {
		response.InvalidTokenError(c, "Invalid or expired token: "+err.Error())
		return
	}

	// Add the token to the blacklist
	if h.blacklistService != nil {
		if err := h.blacklistService.AddToBlacklist(c.Request.Context(), tokenString); err != nil {
			// Log the cache error but still return success to the user
			// The token will naturally expire, so this is not a critical failure
			response.CacheError(c, "Failed to blacklist token", err)
		}
	}

	// Return success response
	response.Success(c, http.StatusOK, "Logout successful", map[string]interface{}{
		"user_id":  claims.UserID,
		"username": claims.Username,
		"email":    claims.Email,
	})
}
