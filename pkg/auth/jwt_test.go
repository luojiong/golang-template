package auth

import (
	"context"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// MockBlacklistChecker implements the BlacklistChecker interface for testing
type MockBlacklistChecker struct {
	blacklistedTokens map[string]bool
	shouldError       bool
}

func NewMockBlacklistChecker() *MockBlacklistChecker {
	return &MockBlacklistChecker{
		blacklistedTokens: make(map[string]bool),
		shouldError:       false,
	}
}

func (m *MockBlacklistChecker) IsBlacklisted(ctx context.Context, tokenString string) (bool, error) {
	if m.shouldError {
		return false, jwt.ErrInvalidKey
	}
	return m.blacklistedTokens[tokenString], nil
}

func (m *MockBlacklistChecker) AddBlacklistedToken(token string) {
	m.blacklistedTokens[token] = true
}

func (m *MockBlacklistChecker) SetShouldError(shouldError bool) {
	m.shouldError = shouldError
}

func TestJWTManager_WithoutBlacklist(t *testing.T) {
	secretKey := "test-secret-key"
	jwtManager := NewJWTManager(secretKey, 24)

	// Test token generation
	token, err := jwtManager.GenerateToken("user123", "testuser", "test@example.com")
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	// Test token validation
	claims, err := jwtManager.ValidateToken(token)
	if err != nil {
		t.Fatalf("Failed to validate token: %v", err)
	}

	if claims.UserID != "user123" {
		t.Errorf("Expected UserID 'user123', got '%s'", claims.UserID)
	}
	if claims.Username != "testuser" {
		t.Errorf("Expected Username 'testuser', got '%s'", claims.Username)
	}
	if claims.Email != "test@example.com" {
		t.Errorf("Expected Email 'test@example.com', got '%s'", claims.Email)
	}
}

func TestJWTManager_WithBlacklist(t *testing.T) {
	secretKey := "test-secret-key"
	mockBlacklist := NewMockBlacklistChecker()
	jwtManager := NewJWTManagerWithBlacklist(secretKey, 24, mockBlacklist)

	// Test token generation
	token, err := jwtManager.GenerateToken("user123", "testuser", "test@example.com")
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	// Test token validation (should succeed)
	claims, err := jwtManager.ValidateToken(token)
	if err != nil {
		t.Fatalf("Failed to validate token: %v", err)
	}

	if claims.UserID != "user123" {
		t.Errorf("Expected UserID 'user123', got '%s'", claims.UserID)
	}

	// Add token to blacklist
	mockBlacklist.AddBlacklistedToken(token)

	// Test token validation (should fail)
	_, err = jwtManager.ValidateToken(token)
	if err == nil {
		t.Error("Expected validation to fail for blacklisted token, but it succeeded")
	}

	if err.Error() != "token is blacklisted" {
		t.Errorf("Expected 'token is blacklisted' error, got '%v'", err)
	}
}

func TestJWTManager_BlacklistErrorFallback(t *testing.T) {
	secretKey := "test-secret-key"
	mockBlacklist := NewMockBlacklistChecker()
	jwtManager := NewJWTManagerWithBlacklist(secretKey, 24, mockBlacklist)

	// Test token generation
	token, err := jwtManager.GenerateToken("user123", "testuser", "test@example.com")
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	// Set blacklist to return an error
	mockBlacklist.SetShouldError(true)

	// Test token validation (should succeed due to graceful fallback)
	claims, err := jwtManager.ValidateToken(token)
	if err != nil {
		t.Fatalf("Failed to validate token with blacklist error: %v", err)
	}

	if claims.UserID != "user123" {
		t.Errorf("Expected UserID 'user123', got '%s'", claims.UserID)
	}
}

func TestJWTManager_ValidateTokenWithContext(t *testing.T) {
	secretKey := "test-secret-key"
	mockBlacklist := NewMockBlacklistChecker()
	jwtManager := NewJWTManagerWithBlacklist(secretKey, 24, mockBlacklist)

	// Test token generation
	token, err := jwtManager.GenerateToken("user123", "testuser", "test@example.com")
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	// Test token validation with context
	ctx := context.Background()
	claims, err := jwtManager.ValidateTokenWithContext(ctx, token)
	if err != nil {
		t.Fatalf("Failed to validate token with context: %v", err)
	}

	if claims.UserID != "user123" {
		t.Errorf("Expected UserID 'user123', got '%s'", claims.UserID)
	}
}

func TestJWTManager_InvalidToken(t *testing.T) {
	secretKey := "test-secret-key"
	jwtManager := NewJWTManager(secretKey, 24)

	// Test with invalid token
	invalidToken := "invalid.token.here"
	_, err := jwtManager.ValidateToken(invalidToken)
	if err == nil {
		t.Error("Expected validation to fail for invalid token, but it succeeded")
	}
}

func TestJWTManager_ExpiredToken(t *testing.T) {
	secretKey := "test-secret-key"
	jwtManager := NewJWTManager(secretKey, 0) // 0 hours = immediate expiration

	// Test token generation
	token, err := jwtManager.GenerateToken("user123", "testuser", "test@example.com")
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	// Wait a moment to ensure token is expired
	time.Sleep(1 * time.Millisecond)

	// Test token validation (should fail for expired token)
	_, err = jwtManager.ValidateToken(token)
	if err == nil {
		t.Error("Expected validation to fail for expired token, but it succeeded")
	}
}