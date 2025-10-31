package middleware

import (
	"context"
	"runtime/debug"
	"time"

	"go-server/internal/errors"
	"go-server/internal/logger"
	"go-server/internal/models"

	"github.com/gin-gonic/gin"
)

// ErrorHandlerMiddleware provides centralized error handling for Gin
func ErrorHandlerMiddleware(logger logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// Handle panic
				logError(logger, c.Request.Context(), "Panic recovered", map[string]interface{}{
					"error":       toString(err),
					"stack_trace": string(debug.Stack()),
					"method":      c.Request.Method,
					"path":        c.Request.URL.Path,
					"client_ip":   c.ClientIP(),
				})

				// Create a standardized error response
				appErr := errors.NewInternalError("Internal server error", nil)
				if correlationID := GetCorrelationIDFromContext(c); correlationID != "" {
					appErr = appErr.WithCorrelationID(correlationID)
				}

				// Send error response
				sendErrorResponse(c, appErr)
				c.Abort()
			}
		}()

		// Process request
		c.Next()

		// Handle any errors that occurred during request processing
		if len(c.Errors) > 0 {
			// Get the last error (most recent)
			err := c.Errors.Last().Err

			// Convert to AppError if needed
			var appErr *errors.AppError
			if customErr, ok := err.(*errors.AppError); ok {
				appErr = customErr
			} else {
				appErr = errors.NewInternalError("Internal server error", err)
			}

			// Add correlation ID if available
			if correlationID := GetCorrelationIDFromContext(c); correlationID != "" {
				appErr = appErr.WithCorrelationID(correlationID)
			}

			// Send error response
			sendErrorResponse(c, appErr)
		}
	}
}

// sendErrorResponse sends a standardized error response
func sendErrorResponse(c *gin.Context, appErr *errors.AppError) {
	// Determine if this is a client error (4xx) or server error (5xx)
	statusCode := errors.GetHTTPStatusCode(appErr)
	isClientError := statusCode >= 400 && statusCode < 500

	// Prepare error response
	errorResponse := models.ErrorResponse{
		Success: false,
		Message: getUserFriendlyMessage(appErr, isClientError),
		Error: &models.EnhancedErrorResponse{
			Code:        models.ErrorCode(string(appErr.Code)),
			Message:     appErr.Message,
			UserMessage: getUserFriendlyMessage(appErr, true),
			Details:     appErr.Details,
		},
		CorrelationID: appErr.CorrelationID,
		Timestamp:     time.Now().UTC(),
	}

	// Add internal error details for development or client errors
	if isClientError || isDevelopmentEnvironment() {
		if appErr.Cause != nil {
			errorResponse.Error.InternalError = appErr.Cause.Error()
		}
	}

	c.JSON(statusCode, errorResponse)
}

// getUserFriendlyMessage returns an appropriate user-friendly message
func getUserFriendlyMessage(appErr *errors.AppError, preferUserMessage bool) string {
	if preferUserMessage && appErr.UserMessage != "" {
		return appErr.UserMessage
	}

	if appErr.UserMessage != "" {
		return appErr.UserMessage
	}

	return errors.GetUserMessage(appErr)
}

// isDevelopmentEnvironment checks if the application is running in development mode
func isDevelopmentEnvironment() bool {
	return gin.Mode() == gin.DebugMode
}

// RecoveryMiddleware provides a recovery mechanism that works with the error handler
func RecoveryMiddleware(logger logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// Log the panic with stack trace
				logError(logger, c.Request.Context(), "Panic recovered", map[string]interface{}{
					"error":       toString(err),
					"stack_trace": string(debug.Stack()),
					"method":      c.Request.Method,
					"path":        c.Request.URL.Path,
					"client_ip":   c.ClientIP(),
				})

				// Create an appropriate error response
				appErr := errors.NewInternalError("Internal server error", nil).
					WithDetail("panic", true).
					WithCorrelationID(GetCorrelationIDFromContext(c))

				// Send error response
				sendErrorResponse(c, appErr)
				c.Abort()
			}
		}()

		c.Next()
	}
}

// toString converts any value to string
func toString(v any) string {
	if v == nil {
		return "nil"
	}
	if s, ok := v.(string); ok {
		return s
	}
	return "unknown"
}

// Helper function to log errors using the logger interface
func logError(log logger.Logger, ctx context.Context, message string, fields map[string]interface{}) {
	// Convert fields to logger.Field format
	var loggerFields []logger.Field
	for k, v := range fields {
		loggerFields = append(loggerFields, logger.Any(k, v))
	}
	log.Error(ctx, message, loggerFields...)
}

// ErrorHandler provides HTTP error handling utilities
type ErrorHandler struct {
	logger logger.Logger
}

// NewErrorHandler creates a new HTTP error handler
func NewErrorHandler(logger logger.Logger) *ErrorHandler {
	return &ErrorHandler{
		logger: logger,
	}
}

// HandleError handles an HTTP error and sends an appropriate response
func (h *ErrorHandler) HandleError(c *gin.Context, err error) {
	ctx := c.Request.Context()

	// Convert to AppError if needed
	var appErr *errors.AppError
	if customErr, ok := err.(*errors.AppError); ok {
		appErr = customErr
	} else {
		appErr = errors.NewInternalError("Internal server error", err)
	}

	// Add correlation ID if available
	if correlationID := GetCorrelationIDFromContext(c); correlationID != "" {
		appErr = appErr.WithCorrelationID(correlationID)
	}

	// Log the error
	logError(h.logger, ctx, "HTTP request error", map[string]interface{}{
		"error_code":     string(appErr.Code),
		"error_message":  appErr.Message,
		"method":         c.Request.Method,
		"path":           c.Request.URL.Path,
		"client_ip":      c.ClientIP(),
		"correlation_id":  appErr.CorrelationID,
	})

	// Send error response
	sendErrorResponse(c, appErr)
}

// HandleValidationError handles validation errors with field details
func (h *ErrorHandler) HandleValidationError(c *gin.Context, message string, fieldDetails ...errors.ErrorDetails) {
	appErr := errors.NewValidationError(message, fieldDetails...)
	h.HandleError(c, appErr)
}

// HandleNotFoundError handles not found errors
func (h *ErrorHandler) HandleNotFoundError(c *gin.Context, resourceType, identifier string) {
	appErr := errors.NewNotFoundError(resourceType, identifier)
	h.HandleError(c, appErr)
}

// HandleUnauthorizedError handles unauthorized errors
func (h *ErrorHandler) HandleUnauthorizedError(c *gin.Context, message string) {
	appErr := errors.NewUnauthorizedError(message)
	h.HandleError(c, appErr)
}

// HandleForbiddenError handles forbidden errors
func (h *ErrorHandler) HandleForbiddenError(c *gin.Context, message string) {
	appErr := errors.NewForbiddenError(message)
	h.HandleError(c, appErr)
}

// HandleConflictError handles conflict errors
func (h *ErrorHandler) HandleConflictError(c *gin.Context, message string, details map[string]any) {
	appErr := errors.NewConflictError(message, details)
	h.HandleError(c, appErr)
}

// HandleRateLimitError handles rate limit errors
func (h *ErrorHandler) HandleRateLimitError(c *gin.Context, limit int, windowSeconds int) {
	appErr := errors.NewRateLimitError(limit, windowSeconds)
	h.HandleError(c, appErr)
}

// HandleTimeoutError handles timeout errors
func (h *ErrorHandler) HandleTimeoutError(c *gin.Context, operation string, timeout any) {
	appErr := errors.NewTimeoutError(operation, timeout)
	h.HandleError(c, appErr)
}

// HandleServiceUnavailableError handles service unavailable errors
func (h *ErrorHandler) HandleServiceUnavailableError(c *gin.Context, serviceName, message string) {
	appErr := errors.NewServiceUnavailableError(serviceName, message)
	h.HandleError(c, appErr)
}

// HandleDatabaseError handles database errors
func (h *ErrorHandler) HandleDatabaseError(c *gin.Context, message string, cause error) {
	appErr := errors.NewDatabaseError(message, cause)
	h.HandleError(c, appErr)
}

// HandleCacheError handles cache errors
func (h *ErrorHandler) HandleCacheError(c *gin.Context, message string, cause error) {
	appErr := errors.NewCacheError(message, cause)
	h.HandleError(c, appErr)
}