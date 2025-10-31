package auth

import (
	"context"
	"fmt"
	"log"
)

// ExampleJWTManagerWithBlacklist demonstrates how to use the enhanced JWT manager with blacklist support
func ExampleJWTManagerWithBlacklist() {
	// Create a JWT manager without blacklist (backward compatibility)
	jwtManager := NewJWTManager("your-secret-key", 24)

	// Generate a token
	token, err := jwtManager.GenerateToken("user123", "johndoe", "john@example.com")
	if err != nil {
		log.Printf("Failed to generate token: %v", err)
		return
	}

	fmt.Printf("Generated token: %s\n", token)

	// Validate the token (without blacklist checking)
	claims, err := jwtManager.ValidateToken(token)
	if err != nil {
		log.Printf("Failed to validate token: %v", err)
		return
	}

	fmt.Printf("Validated claims - UserID: %s, Username: %s\n", claims.UserID, claims.Username)

	// Now let's create a JWT manager with blacklist support
	// Note: In a real application, you would provide a BlacklistChecker implementation
	// For this example, we'll use nil to show the pattern

	// Create JWT manager with blacklist support (using nil for demonstration)
	// In practice, you would pass a BlacklistChecker implementation
	jwtManagerWithBlacklist := NewJWTManagerWithBlacklist("your-secret-key", 24, nil)

	// Generate and validate token with blacklist support
	token2, err := jwtManagerWithBlacklist.GenerateToken("user456", "janedoe", "jane@example.com")
	if err != nil {
		log.Printf("Failed to generate token: %v", err)
		return
	}

	// Validate the token (would check blacklist if BlacklistChecker was provided)
	claims2, err := jwtManagerWithBlacklist.ValidateToken(token2)
	if err != nil {
		log.Printf("Failed to validate token: %v", err)
		return
	}

	fmt.Printf("Enhanced JWT manager validated - UserID: %s, Username: %s\n", claims2.UserID, claims2.Username)
	fmt.Println("Enhanced JWT manager with blacklist support is ready!")
}

// ExampleBlacklistUsage demonstrates the blacklist usage pattern
func ExampleBlacklistUsage() {
	// This example shows how you would use the JWT manager with blacklist in practice

	// 1. Create a BlacklistChecker implementation (e.g., from cache package)
	// blacklistChecker := cache.NewBlacklistService(...)

	// 2. Create JWT manager with blacklist support
	jwtManager := NewJWTManagerWithBlacklist("your-secret-key", 24, nil) // nil for demo

	// 3. Generate and validate tokens normally
	token, _ := jwtManager.GenerateToken("user123", "username", "email@example.com")
	claims, _ := jwtManager.ValidateToken(token) // This would check blacklist if BlacklistChecker was provided

	fmt.Printf("Token validated for user: %s\n", claims.Username)

	// 4. When you want to blacklist a token (e.g., on logout)
	// ctx := context.Background()
	// blacklistChecker.AddToBlacklist(ctx, token) // This would be done via the BlacklistChecker

	// 5. Subsequent validation attempts would fail
	// _, err := jwtManager.ValidateToken(token) // Returns "token is blacklisted" error

	// 6. If the blacklist service is unavailable, validation continues gracefully
	// The JWT manager will log a warning but still validate the token's signature

	fmt.Println("Blacklist usage pattern demonstrated")
}

// ExampleContextualValidation shows how to use contextual validation
func ExampleContextualValidation() {
	// Create JWT manager
	jwtManager := NewJWTManager("your-secret-key", 24)

	// Generate token
	token, err := jwtManager.GenerateToken("user123", "johndoe", "john@example.com")
	if err != nil {
		log.Printf("Failed to generate token: %v", err)
		return
	}

	// Use contextual validation (useful for timeouts, cancellation, etc.)
	ctx := context.Background()
	claims, err := jwtManager.ValidateTokenWithContext(ctx, token)
	if err != nil {
		log.Printf("Failed to validate token: %v", err)
		return
	}

	fmt.Printf("Contextual validation successful - UserID: %s\n", claims.UserID)
}