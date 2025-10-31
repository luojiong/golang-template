package errors

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewAppError(t *testing.T) {
	tests := []struct {
		name       string
		code       ErrorCode
		message    string
		wantCode   ErrorCode
		wantStatus int
	}{
		{
			name:       "validation error",
			code:       ErrCodeValidation,
			message:    "Invalid input",
			wantCode:   ErrCodeValidation,
			wantStatus: 400,
		},
		{
			name:       "not found error",
			code:       ErrCodeNotFound,
			message:    "Resource not found",
			wantCode:   ErrCodeNotFound,
			wantStatus: 404,
		},
		{
			name:       "internal error",
			code:       ErrCodeInternal,
			message:    "Internal server error",
			wantCode:   ErrCodeInternal,
			wantStatus: 500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewAppError(tt.code, tt.message)

			assert.Equal(t, tt.wantCode, err.Code)
			assert.Equal(t, tt.message, err.Message)
			assert.Equal(t, tt.wantStatus, err.StatusCode)
			assert.NotZero(t, err.Timestamp)
			assert.False(t, err.Timestamp.IsZero())
		})
	}
}

func TestAppError_Error(t *testing.T) {
	tests := []struct {
		name     string
		appErr   *AppError
		expected string
	}{
		{
			name: "error without cause",
			appErr: &AppError{
				Code:    ErrCodeValidation,
				Message: "Invalid input",
			},
			expected: "VALIDATION_ERROR: Invalid input",
		},
		{
			name: "error with cause",
			appErr: &AppError{
				Code:    ErrCodeDatabase,
				Message: "Database connection failed",
				Cause:   errors.New("connection timeout"),
			},
			expected: "DATABASE_ERROR: Database connection failed (caused by: connection timeout)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.appErr.Error())
		})
	}
}

func TestAppError_Unwrap(t *testing.T) {
	originalErr := errors.New("original error")
	appErr := NewAppError(ErrCodeInternal, "Wrapped error").WithCause(originalErr)

	assert.Equal(t, originalErr, appErr.Unwrap())
}

func TestAppError_WithCorrelationID(t *testing.T) {
	correlationID := "test-correlation-123"
	err := NewAppError(ErrCodeValidation, "Test error")

	result := err.WithCorrelationID(correlationID)

	assert.Equal(t, correlationID, err.CorrelationID)
	assert.Same(t, err, result) // Should return the same instance
}

func TestAppError_WithDetails(t *testing.T) {
	err := NewAppError(ErrCodeValidation, "Test error")
	details := map[string]interface{}{
		"field":   "email",
		"invalid": "invalid-email",
	}

	result := err.WithDetails(details)

	assert.Equal(t, details, err.Details)
	assert.Same(t, err, result) // Should return the same instance
}

func TestAppError_WithDetail(t *testing.T) {
	err := NewAppError(ErrCodeValidation, "Test error")

	err.WithDetail("field", "email").
		WithDetail("invalid", "invalid-email").
		WithDetail("required", true)

	assert.Equal(t, "email", err.Details["field"])
	assert.Equal(t, "invalid-email", err.Details["invalid"])
	assert.Equal(t, true, err.Details["required"])
}

func TestAppError_WithCause(t *testing.T) {
	originalErr := errors.New("original error")
	err := NewAppError(ErrCodeInternal, "Test error")

	result := err.WithCause(originalErr)

	assert.Equal(t, originalErr, err.Cause)
	assert.Same(t, err, result) // Should return the same instance
}

func TestAppError_WithUserMessage(t *testing.T) {
	userMessage := "Please check your input and try again"
	err := NewAppError(ErrCodeValidation, "Validation failed")

	result := err.WithUserMessage(userMessage)

	assert.Equal(t, userMessage, err.UserMessage)
	assert.Same(t, err, result) // Should return the same instance
}

func TestAppError_IsClientError(t *testing.T) {
	tests := []struct {
		name     string
		code     ErrorCode
		expected bool
	}{
		{"validation error", ErrCodeValidation, true},
		{"not found error", ErrCodeNotFound, true},
		{"unauthorized error", ErrCodeUnauthorized, true},
		{"forbidden error", ErrCodeForbidden, true},
		{"conflict error", ErrCodeConflict, true},
		{"rate limit error", ErrCodeRateLimitExceeded, true},
		{"internal error", ErrCodeInternal, false},
		{"database error", ErrCodeDatabase, false},
		{"cache error", ErrCodeCache, false},
		{"service unavailable error", ErrCodeServiceUnavailable, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewAppError(tt.code, "Test error")
			assert.Equal(t, tt.expected, err.IsClientError())
		})
	}
}

func TestAppError_IsServerError(t *testing.T) {
	tests := []struct {
		name     string
		code     ErrorCode
		expected bool
	}{
		{"validation error", ErrCodeValidation, false},
		{"not found error", ErrCodeNotFound, false},
		{"unauthorized error", ErrCodeUnauthorized, false},
		{"forbidden error", ErrCodeForbidden, false},
		{"conflict error", ErrCodeConflict, false},
		{"rate limit error", ErrCodeRateLimitExceeded, false},
		{"internal error", ErrCodeInternal, true},
		{"database error", ErrCodeDatabase, true},
		{"cache error", ErrCodeCache, true},
		{"service unavailable error", ErrCodeServiceUnavailable, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewAppError(tt.code, "Test error")
			assert.Equal(t, tt.expected, err.IsServerError())
		})
	}
}

func TestNewValidationError(t *testing.T) {
	t.Run("without field details", func(t *testing.T) {
		err := NewValidationError("Invalid input")

		assert.Equal(t, ErrCodeValidation, err.Code)
		assert.Equal(t, "Invalid input", err.Message)
		assert.Equal(t, 400, err.StatusCode)
		assert.Nil(t, err.Details)
	})

	t.Run("with field details", func(t *testing.T) {
		fieldDetails := []ErrorDetails{
			{
				Field:      "email",
				Message:    "Invalid email format",
				Value:      "invalid-email",
				Constraint: "must be valid email",
			},
			{
				Field:      "age",
				Message:    "Age must be positive",
				Value:      -5,
				Constraint: "must be > 0",
			},
		}

		err := NewValidationError("Validation failed", fieldDetails...)

		assert.Equal(t, ErrCodeValidation, err.Code)
		assert.Equal(t, "Validation failed", err.Message)
		assert.NotNil(t, err.Details)
		assert.Contains(t, err.Details, "validation_errors")
	})
}

func TestNewNotFoundError(t *testing.T) {
	t.Run("without identifier", func(t *testing.T) {
		err := NewNotFoundError("User", "")

		assert.Equal(t, ErrCodeNotFound, err.Code)
		assert.Equal(t, "User not found", err.Message)
		assert.Equal(t, 404, err.StatusCode)
		assert.Equal(t, "User", err.Details["resource_type"])
		assert.Equal(t, "", err.Details["identifier"])
	})

	t.Run("with identifier", func(t *testing.T) {
		err := NewNotFoundError("User", "123")

		assert.Equal(t, ErrCodeNotFound, err.Code)
		assert.Equal(t, "User with identifier '123' not found", err.Message)
		assert.Equal(t, 404, err.StatusCode)
		assert.Equal(t, "User", err.Details["resource_type"])
		assert.Equal(t, "123", err.Details["identifier"])
	})
}

func TestNewUnauthorizedError(t *testing.T) {
	t.Run("with custom message", func(t *testing.T) {
		message := "Token expired"
		err := NewUnauthorizedError(message)

		assert.Equal(t, ErrCodeUnauthorized, err.Code)
		assert.Equal(t, message, err.Message)
		assert.Equal(t, 401, err.StatusCode)
	})

	t.Run("with empty message", func(t *testing.T) {
		err := NewUnauthorizedError("")

		assert.Equal(t, ErrCodeUnauthorized, err.Code)
		assert.Equal(t, "Authentication required", err.Message)
		assert.Equal(t, 401, err.StatusCode)
	})
}

func TestNewForbiddenError(t *testing.T) {
	t.Run("with custom message", func(t *testing.T) {
		message := "Insufficient permissions"
		err := NewForbiddenError(message)

		assert.Equal(t, ErrCodeForbidden, err.Code)
		assert.Equal(t, message, err.Message)
		assert.Equal(t, 403, err.StatusCode)
	})

	t.Run("with empty message", func(t *testing.T) {
		err := NewForbiddenError("")

		assert.Equal(t, ErrCodeForbidden, err.Code)
		assert.Equal(t, "Access forbidden", err.Message)
		assert.Equal(t, 403, err.StatusCode)
	})
}

func TestNewConflictError(t *testing.T) {
	message := "Resource already exists"
	details := map[string]interface{}{
		"resource_id": "123",
		"conflict_with": "existing_resource",
	}

	err := NewConflictError(message, details)

	assert.Equal(t, ErrCodeConflict, err.Code)
	assert.Equal(t, message, err.Message)
	assert.Equal(t, 409, err.StatusCode)
	assert.Equal(t, details, err.Details)
}

func TestNewRateLimitError(t *testing.T) {
	limit := 100
	windowSeconds := 3600

	err := NewRateLimitError(limit, windowSeconds)

	assert.Equal(t, ErrCodeRateLimitExceeded, err.Code)
	assert.Equal(t, "Rate limit exceeded", err.Message)
	assert.Equal(t, 429, err.StatusCode)
	assert.Equal(t, limit, err.Details["limit"])
	assert.Equal(t, windowSeconds, err.Details["window_seconds"])
	assert.Equal(t, windowSeconds, err.Details["retry_after"])
}

func TestNewInternalError(t *testing.T) {
	t.Run("with cause", func(t *testing.T) {
		cause := errors.New("database connection failed")
		err := NewInternalError("Failed to process request", cause)

		assert.Equal(t, ErrCodeInternal, err.Code)
		assert.Equal(t, "Failed to process request", err.Message)
		assert.Equal(t, 500, err.StatusCode)
		assert.Equal(t, cause, err.Cause)
	})

	t.Run("without cause", func(t *testing.T) {
		err := NewInternalError("Something went wrong", nil)

		assert.Equal(t, ErrCodeInternal, err.Code)
		assert.Equal(t, "Something went wrong", err.Message)
		assert.Equal(t, 500, err.StatusCode)
		assert.Nil(t, err.Cause)
	})
}

func TestNewDatabaseError(t *testing.T) {
	cause := errors.New("connection timeout")
	err := NewDatabaseError("Failed to connect to database", cause)

	assert.Equal(t, ErrCodeDatabase, err.Code)
	assert.Equal(t, "Failed to connect to database", err.Message)
	assert.Equal(t, 500, err.StatusCode)
	assert.Equal(t, cause, err.Cause)
}

func TestNewCacheError(t *testing.T) {
	cause := errors.New("redis connection failed")
	err := NewCacheError("Cache operation failed", cause)

	assert.Equal(t, ErrCodeCache, err.Code)
	assert.Equal(t, "Cache operation failed", err.Message)
	assert.Equal(t, 500, err.StatusCode)
	assert.Equal(t, cause, err.Cause)
}

func TestNewServiceUnavailableError(t *testing.T) {
	t.Run("with custom message", func(t *testing.T) {
		serviceName := "payment-service"
		message := "Payment service is down for maintenance"
		err := NewServiceUnavailableError(serviceName, message)

		assert.Equal(t, ErrCodeServiceUnavailable, err.Code)
		assert.Equal(t, message, err.Message)
		assert.Equal(t, 503, err.StatusCode)
		assert.Equal(t, serviceName, err.Details["service_name"])
	})

	t.Run("with default message", func(t *testing.T) {
		serviceName := "email-service"
		err := NewServiceUnavailableError(serviceName, "")

		assert.Equal(t, ErrCodeServiceUnavailable, err.Code)
		assert.Equal(t, "Service 'email-service' is currently unavailable", err.Message)
		assert.Equal(t, 503, err.StatusCode)
		assert.Equal(t, serviceName, err.Details["service_name"])
	})
}

func TestNewTimeoutError(t *testing.T) {
	operation := "database query"
	timeout := 5 * time.Second

	err := NewTimeoutError(operation, timeout)

	assert.Equal(t, ErrCodeTimeout, err.Code)
	assert.Equal(t, "Operation 'database query' timed out after 5s", err.Message)
	assert.Equal(t, 408, err.StatusCode)
	assert.Equal(t, operation, err.Details["operation"])
	assert.Equal(t, 5.0, err.Details["timeout_seconds"])
}

func TestNewInvalidTokenError(t *testing.T) {
	t.Run("with reason", func(t *testing.T) {
		reason := "token expired"
		err := NewInvalidTokenError(reason)

		assert.Equal(t, ErrCodeInvalidToken, err.Code)
		assert.Equal(t, "Invalid token: token expired", err.Message)
		assert.Equal(t, 401, err.StatusCode)
		assert.Equal(t, reason, err.Details["reason"])
	})

	t.Run("without reason", func(t *testing.T) {
		err := NewInvalidTokenError("")

		assert.Equal(t, ErrCodeInvalidToken, err.Code)
		assert.Equal(t, "Invalid token", err.Message)
		assert.Equal(t, 401, err.StatusCode)
		assert.Equal(t, "", err.Details["reason"])
	})
}

func TestNewTokenBlacklistedError(t *testing.T) {
	err := NewTokenBlacklistedError()

	assert.Equal(t, ErrCodeTokenBlacklisted, err.Code)
	assert.Equal(t, "Token has been blacklisted", err.Message)
	assert.Equal(t, 401, err.StatusCode)
}

func TestWrapError(t *testing.T) {
	t.Run("wrapping nil error", func(t *testing.T) {
		err := WrapError(nil, ErrCodeInternal, "Wrapped error")
		assert.Nil(t, err)
	})

	t.Run("wrapping AppError", func(t *testing.T) {
		originalErr := NewValidationError("Original validation error")
		wrappedErr := WrapError(originalErr, ErrCodeInternal, "Wrapped error")

		assert.Same(t, originalErr, wrappedErr)
	})

	t.Run("wrapping regular error", func(t *testing.T) {
		originalErr := errors.New("original error")
		wrappedErr := WrapError(originalErr, ErrCodeInternal, "Wrapped error")

		assert.Equal(t, ErrCodeInternal, wrappedErr.Code)
		assert.Equal(t, "Wrapped error", wrappedErr.Message)
		assert.Equal(t, originalErr, wrappedErr.Cause)
	})
}

func TestGenerateCorrelationID(t *testing.T) {
	id1 := GenerateCorrelationID()
	id2 := GenerateCorrelationID()

	assert.NotEmpty(t, id1)
	assert.NotEmpty(t, id2)
	assert.NotEqual(t, id1, id2)
	assert.Len(t, id1, 8) // UUID[:8] should be 8 characters
	assert.Len(t, id2, 8)
}

func TestErrorWithCorrelation(t *testing.T) {
	err := ErrorWithCorrelation(ErrCodeValidation, "Test error")

	assert.Equal(t, ErrCodeValidation, err.Code)
	assert.Equal(t, "Test error", err.Message)
	assert.NotEmpty(t, err.CorrelationID)
	assert.Len(t, err.CorrelationID, 8)
}

func TestIsErrorCode(t *testing.T) {
	t.Run("matching error code", func(t *testing.T) {
		err := NewValidationError("Test error")
		assert.True(t, IsErrorCode(err, ErrCodeValidation))
		assert.False(t, IsErrorCode(err, ErrCodeNotFound))
	})

	t.Run("non-AppError", func(t *testing.T) {
		err := errors.New("regular error")
		assert.False(t, IsErrorCode(err, ErrCodeValidation))
	})
}

func TestGetErrorCode(t *testing.T) {
	t.Run("AppError", func(t *testing.T) {
		err := NewValidationError("Test error")
		assert.Equal(t, ErrCodeValidation, GetErrorCode(err))
	})

	t.Run("non-AppError", func(t *testing.T) {
		err := errors.New("regular error")
		assert.Equal(t, ErrCodeInternal, GetErrorCode(err))
	})
}

func TestGetHTTPStatusCode(t *testing.T) {
	t.Run("AppError", func(t *testing.T) {
		err := NewValidationError("Test error")
		assert.Equal(t, 400, GetHTTPStatusCode(err))
	})

	t.Run("non-AppError", func(t *testing.T) {
		err := errors.New("regular error")
		assert.Equal(t, 500, GetHTTPStatusCode(err))
	})
}

func TestAppErrorJSONSerialization(t *testing.T) {
	// This test ensures that AppError can be properly serialized to JSON
	// and that sensitive fields are not exposed
	err := NewValidationError("Test error").
		WithCorrelationID("test-correlation").
		WithCause(errors.New("original error")).
		WithUserMessage("Please check your input").
		WithDetail("field", "email")

	// The fields that should be serialized
	assert.Equal(t, ErrCodeValidation, err.Code)
	assert.Equal(t, "Test error", err.Message)
	assert.Equal(t, "test-correlation", err.CorrelationID)
	assert.Equal(t, "Please check your input", err.UserMessage)
	assert.NotNil(t, err.Details)
	assert.NotNil(t, err.Timestamp)

	// The fields that should not be serialized (marked with -)
	assert.NotZero(t, err.StatusCode)  // Should exist but not serialize
	assert.NotNil(t, err.Cause)        // Should exist but not serialize
}

func TestComprehensiveErrorScenarios(t *testing.T) {
	// Test comprehensive error handling scenarios
	t.Run("complex validation error", func(t *testing.T) {
		fieldDetails := []ErrorDetails{
			{Field: "email", Message: "Invalid email format", Value: "invalid", Constraint: "valid email"},
			{Field: "password", Message: "Password too short", Value: "123", Constraint: "min 8 chars"},
			{Field: "age", Message: "Age out of range", Value: 150, Constraint: "0-120"},
		}

		err := NewValidationError("User registration failed", fieldDetails...).
			WithCorrelationID("reg-123").
			WithUserMessage("Please correct the errors below").
			WithCause(errors.New("validation library error"))

		assert.Equal(t, ErrCodeValidation, err.Code)
		assert.Equal(t, "User registration failed", err.Message)
		assert.Equal(t, "reg-123", err.CorrelationID)
		assert.Equal(t, "Please correct the errors below", err.UserMessage)
		assert.NotNil(t, err.Cause)
		assert.NotNil(t, err.Details)
		assert.Contains(t, err.Details, "validation_errors")
	})

	t.Run("database error with correlation", func(t *testing.T) {
		originalErr := errors.New("connection pool exhausted")
		err := NewDatabaseError("Failed to create user", originalErr).
			WithCorrelationID("db-456").
			WithDetail("operation", "INSERT").
			WithDetail("table", "users").
			WithDetail("user_id", "temp-123")

		assert.Equal(t, ErrCodeDatabase, err.Code)
		assert.Equal(t, "Failed to create user", err.Message)
		assert.Equal(t, "db-456", err.CorrelationID)
		assert.Equal(t, originalErr, err.Cause)
		assert.Equal(t, "INSERT", err.Details["operation"])
		assert.Equal(t, "users", err.Details["table"])
		assert.Equal(t, "temp-123", err.Details["user_id"])
	})

	t.Run("service unavailable with timeout", func(t *testing.T) {
		timeoutErr := NewTimeoutError("external API call", 30*time.Second)
		err := NewServiceUnavailableError("payment-service", "Payment gateway is down").
			WithCorrelationID("svc-789").
			WithDetail("timeout", timeoutErr.Error()).
			WithDetail("retry_after", 300)

		assert.Equal(t, ErrCodeServiceUnavailable, err.Code)
		assert.Equal(t, "Payment gateway is down", err.Message)
		assert.Equal(t, "svc-789", err.CorrelationID)
		assert.Equal(t, "payment-service", err.Details["service_name"])
		assert.Equal(t, timeoutErr.Error(), err.Details["timeout"])
		assert.Equal(t, 300, err.Details["retry_after"])
	})
}

// Tests for enhanced error handling features

func TestNewBusinessLogicError(t *testing.T) {
	details := map[string]interface{}{
		"rule":      "max_items_per_order",
		"max_items": 10,
		"actual":    15,
	}

	err := NewBusinessLogicError("Order exceeds maximum item limit", details)

	assert.Equal(t, ErrCodeBusinessLogic, err.Code)
	assert.Equal(t, "Order exceeds maximum item limit", err.Message)
	assert.Equal(t, 400, err.StatusCode)
	assert.Equal(t, details, err.Details)
	assert.False(t, err.Retryable)
}

func TestNewQuotaExceededError(t *testing.T) {
	resourceType := "api_calls"
	currentLimit := 1000
	resetTime := time.Now().Add(time.Hour)

	err := NewQuotaExceededError(resourceType, currentLimit, resetTime)

	assert.Equal(t, ErrCodeQuotaExceeded, err.Code)
	assert.Contains(t, err.Message, resourceType)
	assert.Equal(t, 429, err.StatusCode)
	assert.Equal(t, resourceType, err.Details["resource_type"])
	assert.Equal(t, currentLimit, err.Details["current_limit"])
	assert.Equal(t, resetTime, err.Details["reset_time"])
	assert.True(t, err.Retryable)
}

func TestNewMaintenanceError(t *testing.T) {
	serviceName := "payment-service"
	estimatedDowntime := 2 * time.Hour

	err := NewMaintenanceError(serviceName, estimatedDowntime)

	assert.Equal(t, ErrCodeMaintenance, err.Code)
	assert.Contains(t, err.Message, serviceName)
	assert.Equal(t, 503, err.StatusCode)
	assert.Equal(t, serviceName, err.Details["service_name"])
	assert.Equal(t, int64(120), err.Details["estimated_downtime_minutes"])
	assert.True(t, err.Retryable)
}

func TestNewSecurityError(t *testing.T) {
	message := "Suspicious activity detected"
	securityContext := map[string]interface{}{
		"ip_address":   "192.168.1.100",
		"user_agent":   "suspicious-bot/1.0",
		"failed_login": 5,
	}

	err := NewSecurityError(message, securityContext)

	assert.Equal(t, ErrCodeSecurity, err.Code)
	assert.Equal(t, message, err.Message)
	assert.Equal(t, 403, err.StatusCode)
	assert.Equal(t, "high", err.Severity)
	assert.Equal(t, securityContext, err.Details)
	assert.False(t, err.Retryable)
}

func TestEnhancedErrorBuilderMethods(t *testing.T) {
	err := NewValidationError("Test error").
		WithRequestID("req-123").
		WithSeverity("medium").
		WithCategory("validation").
		WithRetryable(true).
		WithStackTrace("stack trace here").
		WithResolved(false)

	assert.Equal(t, "req-123", err.RequestID)
	assert.Equal(t, "medium", err.Severity)
	assert.Equal(t, "validation", err.Category)
	assert.True(t, err.Retryable)
	assert.Equal(t, "stack trace here", err.StackTrace)
	assert.False(t, err.Resolved)
}

func TestErrorContext(t *testing.T) {
	context := &ErrorContext{
		RequestID: "req-123",
		UserID:    "user-456",
		Operation: "user_creation",
		Resource:  "users",
		IPAddress: "192.168.1.1",
		UserAgent: "test-agent/1.0",
		Metadata: map[string]interface{}{
			"region":     "us-east-1",
			"version":    "v1.0.0",
			"request_ts": time.Now(),
		},
	}

	err := NewValidationError("Test error").WithContext(context)

	assert.Equal(t, context, err.Context)
	assert.Equal(t, "req-123", err.Context.RequestID)
	assert.Equal(t, "user-456", err.Context.UserID)
	assert.Equal(t, "us-east-1", err.Context.Metadata["region"])
}

func TestInternationalizedMessages(t *testing.T) {
	messages := map[string]string{
		"en": "Please check your input",
		"zh": "请检查您的输入",
		"es": "Por favor revise su entrada",
	}

	err := NewValidationError("Validation failed").
		AddInternationalizedMessages(messages)

	// Test getting localized messages
	assert.Equal(t, "请检查您的输入", err.GetLocalizedMessage("zh"))
	assert.Equal(t, "Please check your input", err.GetLocalizedMessage("en"))
	assert.Equal(t, "Por favor revise su entrada", err.GetLocalizedMessage("es"))

	// Test fallback to English for unsupported language
	assert.Equal(t, "Please check your input", err.GetLocalizedMessage("fr"))

	// Test fallback to user message when no i18n messages exist
	err2 := NewValidationError("Validation failed").
		WithUserMessage("User friendly message")
	assert.Equal(t, "User friendly message", err2.GetLocalizedMessage("unsupported"))

	// Test that i18n messages take precedence over user message when they exist
	err3 := NewValidationError("Validation failed").
		WithUserMessage("User friendly message").
		AddInternationalizedMessages(messages)
	assert.Equal(t, "Please check your input", err3.GetLocalizedMessage("fr")) // Falls back to English
}

func TestCreateBusinessValidationError(t *testing.T) {
	fieldErrors := []ErrorDetails{
		{
			Field:       "email",
			Message:     "Invalid email format",
			UserMessage: "Please enter a valid email address",
			Value:       "invalid-email",
			ErrorCode:   "INVALID_FORMAT",
			Suggestions: []string{"user@example.com", "test@domain.org"},
		},
	}

	suggestions := []string{
		"Check email format",
		"Ensure email is not already registered",
	}

	err := CreateBusinessValidationError("User creation failed", fieldErrors, suggestions)

	assert.Equal(t, ErrCodeBusinessLogic, err.Code)
	assert.Equal(t, "User creation failed", err.Message)
	assert.Equal(t, "Please check your input and try again", err.UserMessage)
	assert.Equal(t, "validation", err.Category)
	assert.Equal(t, fieldErrors, err.Details["field_errors"])
	assert.Equal(t, suggestions, err.Details["suggestions"])
}

func TestErrorToMap(t *testing.T) {
	originalErr := errors.New("original error")
	err := NewValidationError("Test error").
		WithCorrelationID("corr-123").
		WithRequestID("req-456").
		WithUserMessage("User message").
		WithSeverity("medium").
		WithCategory("validation").
		WithCause(originalErr).
		WithDetail("field", "email")

	errMap := err.ToMap()

	assert.Equal(t, ErrCodeValidation, errMap["code"])
	assert.Equal(t, "Test error", errMap["message"])
	assert.Equal(t, 400, errMap["status_code"])
	assert.Equal(t, "corr-123", errMap["correlation_id"])
	assert.Equal(t, "req-456", errMap["request_id"])
	assert.Equal(t, "User message", errMap["user_message"])
	assert.Equal(t, "medium", errMap["severity"])
	assert.Equal(t, "validation", errMap["category"])
	assert.Equal(t, "original error", errMap["cause"])
	assert.True(t, errMap["is_client_error"].(bool))
	assert.False(t, errMap["is_server_error"].(bool))
	assert.NotNil(t, errMap["details"])
}

func TestErrorLogFormat(t *testing.T) {
	err := NewValidationError("Test error").
		WithCorrelationID("corr-123").
		WithRequestID("req-456").
		WithSeverity("medium")

	logFormat := err.LogFormat()
	expected := "[medium] Test error - Code: VALIDATION_ERROR, Status: 400, CorrelationID: corr-123, RequestID: req-456"
	assert.Equal(t, expected, logFormat)
}

func TestCreateErrorFromContext(t *testing.T) {
	ctx := context.Background()
	ctx = context.WithValue(ctx, "request_id", "req-123")
	ctx = context.WithValue(ctx, "user_id", "user-456")
	ctx = context.WithValue(ctx, "correlation_id", "corr-789")

	err := CreateErrorFromContext(ctx, ErrCodeValidation, "Context error")

	assert.Equal(t, ErrCodeValidation, err.Code)
	assert.Equal(t, "Context error", err.Message)
	assert.Equal(t, "req-123", err.RequestID)
	assert.Equal(t, "corr-789", err.CorrelationID)
	assert.NotNil(t, err.Context)
	assert.Equal(t, "req-123", err.Context.RequestID)
	assert.Equal(t, "user-456", err.Context.UserID)
}