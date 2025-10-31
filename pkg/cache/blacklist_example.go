package cache

import (
	"context"
	"fmt"
	"log"
	"time"

	"go-server/pkg/auth"
)

// ExampleBlacklistUsage demonstrates how to use the JWT blacklist service
func ExampleBlacklistUsage() {
	// 1. Create a Redis cache (or use your existing cache)
	redisConfig := DefaultRedisConfig()
	redisCache, err := NewRedisCache(redisConfig)
	if err != nil {
		log.Fatalf("Failed to create Redis cache: %v", err)
	}
	defer redisCache.Close()

	// 2. Create JWT manager
	jwtManager := auth.NewJWTManager("your-secret-key", 24) // 24 hours expiration

	// 3. Create blacklist service
	blacklistConfig := DefaultBlacklistConfig()
	blacklistService := NewBlacklistService(redisCache, jwtManager, blacklistConfig)

	ctx := context.Background()

	// 4. Generate a JWT token for a user
	token, err := jwtManager.GenerateToken("user123", "john_doe", "john@example.com")
	if err != nil {
		log.Fatalf("Failed to generate token: %v", err)
	}

	fmt.Printf("Generated token: %s\n", token)

	// 5. Validate the token (should succeed)
	claims, err := blacklistService.ValidateTokenWithBlacklist(ctx, token)
	if err != nil {
		log.Fatalf("Token validation failed: %v", err)
	}
	fmt.Printf("Token validated successfully for user: %s\n", claims.Username)

	// 6. Check if token is blacklisted (should be false)
	isBlacklisted, err := blacklistService.IsBlacklisted(ctx, token)
	if err != nil {
		log.Fatalf("Failed to check blacklist: %v", err)
	}
	fmt.Printf("Token is blacklisted: %t\n", isBlacklisted)

	// 7. Add token to blacklist (simulate user logout)
	err = blacklistService.AddToBlacklist(ctx, token)
	if err != nil {
		log.Fatalf("Failed to add token to blacklist: %v", err)
	}
	fmt.Println("Token added to blacklist")

	// 8. Check if token is blacklisted again (should be true)
	isBlacklisted, err = blacklistService.IsBlacklisted(ctx, token)
	if err != nil {
		log.Fatalf("Failed to check blacklist: %v", err)
	}
	fmt.Printf("Token is blacklisted: %t\n", isBlacklisted)

	// 9. Try to validate the blacklisted token (should fail)
	_, err = blacklistService.ValidateTokenWithBlacklist(ctx, token)
	if err != nil {
		fmt.Printf("Token validation failed as expected: %v\n", err)
	}

	// 10. Get blacklist size
	size, err := blacklistService.GetBlacklistSize(ctx)
	if err != nil {
		log.Fatalf("Failed to get blacklist size: %v", err)
	}
	fmt.Printf("Current blacklist size: %d\n", size)

	// 11. Remove token from blacklist (if needed)
	err = blacklistService.RemoveFromBlacklist(ctx, token)
	if err != nil {
		log.Fatalf("Failed to remove token from blacklist: %v", err)
	}
	fmt.Println("Token removed from blacklist")

	// 12. Validate token again (should succeed)
	claims, err = blacklistService.ValidateTokenWithBlacklist(ctx, token)
	if err != nil {
		log.Fatalf("Token validation failed: %v", err)
	}
	fmt.Printf("Token validation succeeded again for user: %s\n", claims.Username)
}

// ExampleBlacklistServiceInMiddleware demonstrates how to integrate the blacklist service in HTTP middleware
func ExampleBlacklistServiceInMiddleware() {
	// This is a conceptual example showing how you might use the blacklist service
	// in an HTTP authentication middleware

	// Setup
	redisConfig := DefaultRedisConfig()
	redisCache, err := NewRedisCache(redisConfig)
	if err != nil {
		log.Fatalf("Failed to create Redis cache: %v", err)
	}
	defer redisCache.Close()

	// jwtManager := auth.NewJWTManager("your-secret-key", 24)
	// blacklistService := NewBlacklistService(redisCache, jwtManager, nil)

	// Middleware function (pseudo-code)
	/*
		func authMiddleware(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Extract token from Authorization header
				authHeader := r.Header.Get("Authorization")
				if authHeader == "" {
					http.Error(w, "Missing authorization header", http.StatusUnauthorized)
					return
				}

				// Remove "Bearer " prefix
				tokenString := strings.TrimPrefix(authHeader, "Bearer ")

				// Validate token with blacklist check
				claims, err := blacklistService.ValidateTokenWithBlacklist(r.Context(), tokenString)
				if err != nil {
					http.Error(w, "Invalid or blacklisted token", http.StatusUnauthorized)
					return
				}

				// Add user context to request
				ctx := context.WithValue(r.Context(), "user", claims)
				next.ServeHTTP(w, r.WithContext(ctx))
			})
		}
	*/

	// Logout endpoint (pseudo-code)
	/*
		func logoutHandler(w http.ResponseWriter, r *http.Request) {
			// Extract token from request
			tokenString := extractTokenFromRequest(r)

			// Add token to blacklist
			err := blacklistService.AddToBlacklist(r.Context(), tokenString)
			if err != nil {
				http.Error(w, "Failed to logout", http.StatusInternalServerError)
				return
			}

			// Return success response
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"message": "Logged out successfully"})
		}
	*/

	fmt.Println("Example: Blacklist service can be integrated into HTTP middleware")
	fmt.Println("1. Use ValidateTokenWithBlacklist() in auth middleware")
	fmt.Println("2. Use AddToBlacklist() in logout endpoints")
	fmt.Println("3. Tokens are automatically removed from blacklist when they expire")
}

// ExampleBatchOperations demonstrates batch operations with the blacklist service
func ExampleBatchOperations() {
	redisConfig := DefaultRedisConfig()
	redisCache, err := NewRedisCache(redisConfig)
	if err != nil {
		log.Fatalf("Failed to create Redis cache: %v", err)
	}
	defer redisCache.Close()

	jwtManager := auth.NewJWTManager("your-secret-key", 24)
	blacklistService := NewBlacklistService(redisCache, jwtManager, nil)

	ctx := context.Background()

	// Generate multiple tokens for different users
	tokens := make([]string, 3)
	for i := 0; i < 3; i++ {
		userID := fmt.Sprintf("user%d", i+1)
		username := fmt.Sprintf("user%d", i+1)
		email := fmt.Sprintf("user%d@example.com", i+1)

		token, err := jwtManager.GenerateToken(userID, username, email)
		if err != nil {
			log.Fatalf("Failed to generate token %d: %v", i+1, err)
		}
		tokens[i] = token
	}

	fmt.Printf("Generated %d tokens\n", len(tokens))

	// Add all tokens to blacklist in a single operation
	err = blacklistService.AddMultipleToBlacklist(ctx, tokens)
	if err != nil {
		log.Fatalf("Failed to add multiple tokens to blacklist: %v", err)
	}
	fmt.Printf("Added %d tokens to blacklist\n", len(tokens))

	// Verify all tokens are blacklisted
	for i, token := range tokens {
		isBlacklisted, err := blacklistService.IsBlacklisted(ctx, token)
		if err != nil {
			log.Fatalf("Failed to check if token %d is blacklisted: %v", i+1, err)
		}
		fmt.Printf("Token %d is blacklisted: %t\n", i+1, isBlacklisted)
	}

	// Get blacklist size
	size, err := blacklistService.GetBlacklistSize(ctx)
	if err != nil {
		log.Fatalf("Failed to get blacklist size: %v", err)
	}
	fmt.Printf("Blacklist size: %d\n", size)

	// Clear all tokens
	err = blacklistService.ClearBlacklist(ctx)
	if err != nil {
		log.Fatalf("Failed to clear blacklist: %v", err)
	}
	fmt.Println("Cleared blacklist")
}

// ExampleCleanupRoutine demonstrates how to set up a cleanup routine
func ExampleCleanupRoutine() {
	redisConfig := DefaultRedisConfig()
	redisCache, err := NewRedisCache(redisConfig)
	if err != nil {
		log.Fatalf("Failed to create Redis cache: %v", err)
	}
	defer redisCache.Close()

	jwtManager := auth.NewJWTManager("your-secret-key", 24)
	blacklistConfig := DefaultBlacklistConfig()
	blacklistService := NewBlacklistService(redisCache, jwtManager, blacklistConfig)

	ctx := context.Background()

	// Start cleanup routine
	go func() {
		ticker := time.NewTicker(blacklistConfig.CleanupInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				err := blacklistService.CleanupExpiredTokens(ctx)
				if err != nil {
					log.Printf("Failed to cleanup expired tokens: %v", err)
				} else {
					log.Println("Successfully cleaned up expired tokens")
				}
			}
		}
	}()

	fmt.Println("Cleanup routine started. Expired tokens will be cleaned up automatically.")
	fmt.Println("Note: In Redis, expired keys are automatically removed, so this cleanup is mostly for housekeeping.")
}
