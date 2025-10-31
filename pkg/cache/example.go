package cache

import (
	"context"
	"fmt"
	"log"
	"time"
)

// ExampleRedisCache demonstrates how to use the Redis cache
func ExampleRedisCache() {
	// Create cache configuration
	config := DefaultRedisConfig()
	config.Host = "localhost"
	config.Port = 6379
	config.Prefix = "app:cache:"

	// Initialize cache
	cache, err := NewRedisCache(config)
	if err != nil {
		log.Printf("Failed to connect to Redis: %v", err)
		return
	}
	defer cache.Close()

	ctx := context.Background()

	// Example 1: Basic Set/Get operations
	fmt.Println("=== Basic Set/Get Operations ===")
	err = cache.Set(ctx, "user:123", map[string]interface{}{
		"name":  "John Doe",
		"email": "john@example.com",
		"age":   30,
	}, 5*time.Minute)
	if err != nil {
		log.Printf("Failed to set user: %v", err)
		return
	}

	user, found := cache.Get(ctx, "user:123")
	if found {
		fmt.Printf("User found: %v\n", user)
	} else {
		fmt.Println("User not found")
	}

	// Example 2: Get with TTL
	fmt.Println("\n=== Get with TTL ===")
	user, ttl, found := cache.GetWithTTL(ctx, "user:123")
	if found {
		fmt.Printf("User found: %v, TTL: %v\n", user, ttl)
	}

	// Example 3: Check if key exists
	fmt.Println("\n=== Check Key Existence ===")
	exists, err := cache.Exists(ctx, "user:123")
	if err != nil {
		log.Printf("Failed to check existence: %v", err)
		return
	}
	fmt.Printf("User 123 exists: %v\n", exists)

	// Example 4: Set if not exists
	fmt.Println("\n=== Set If Not Exists ===")
	success, err := cache.SetIfNotExists(ctx, "user:123", "new user", 5*time.Minute)
	if err != nil {
		log.Printf("Failed to set if not exists: %v", err)
		return
	}
	fmt.Printf("Set new user (should be false): %v\n", success)

	success, err = cache.SetIfNotExists(ctx, "user:456", "another user", 5*time.Minute)
	if err != nil {
		log.Printf("Failed to set if not exists: %v", err)
		return
	}
	fmt.Printf("Set another user (should be true): %v\n", success)

	// Example 5: Multiple operations
	fmt.Println("\n=== Multiple Operations ===")
	users := map[string]interface{}{
		"session:abc": map[string]string{"user_id": "123", "role": "admin"},
		"session:def": map[string]string{"user_id": "456", "role": "user"},
		"settings:theme": "dark",
	}

	err = cache.SetMultiple(ctx, users, 10*time.Minute)
	if err != nil {
		log.Printf("Failed to set multiple items: %v", err)
		return
	}

	keys := []string{"session:abc", "session:def", "session:xyz"} // session:xyz doesn't exist
	results, err := cache.GetMultiple(ctx, keys)
	if err != nil {
		log.Printf("Failed to get multiple items: %v", err)
		return
	}

	fmt.Printf("Multiple results: %v\n", results)

	// Example 6: Increment/Decrement
	fmt.Println("\n=== Increment/Decrement Operations ===")
	err = cache.Set(ctx, "counter", int64(0), 0) // No expiration
	if err != nil {
		log.Printf("Failed to set counter: %v", err)
		return
	}

	// Increment counter
	newValue, err := cache.Increment(ctx, "counter", 1)
	if err != nil {
		log.Printf("Failed to increment counter: %v", err)
		return
	}
	fmt.Printf("Counter after increment: %d\n", newValue)

	// Increment again
	newValue, err = cache.Increment(ctx, "counter", 5)
	if err != nil {
		log.Printf("Failed to increment counter: %v", err)
		return
	}
	fmt.Printf("Counter after second increment: %d\n", newValue)

	// Decrement
	newValue, err = cache.Decrement(ctx, "counter", 2)
	if err != nil {
		log.Printf("Failed to decrement counter: %v", err)
		return
	}
	fmt.Printf("Counter after decrement: %d\n", newValue)

	// Example 7: List keys with pattern
	fmt.Println("\n=== List Keys ===")
	allKeys, err := cache.Keys(ctx, "*")
	if err != nil {
		log.Printf("Failed to list keys: %v", err)
		return
	}
	fmt.Printf("All keys: %v\n", allKeys)

	userKeys, err := cache.Keys(ctx, "user:*")
	if err != nil {
		log.Printf("Failed to list user keys: %v", err)
		return
	}
	fmt.Printf("User keys: %v\n", userKeys)

	// Example 8: Delete operations
	fmt.Println("\n=== Delete Operations ===")
	err = cache.Delete(ctx, "user:123")
	if err != nil {
		log.Printf("Failed to delete user: %v", err)
		return
	}
	fmt.Println("Deleted user:123")

	// Delete multiple keys
	err = cache.DeleteMultiple(ctx, []string{"session:abc", "session:def"})
	if err != nil {
		log.Printf("Failed to delete multiple sessions: %v", err)
		return
	}
	fmt.Println("Deleted sessions")

	// Clean up all cache entries (use with caution!)
	fmt.Println("\n=== Clear All Cache ===")
	err = cache.Clear(ctx)
	if err != nil {
		log.Printf("Failed to clear cache: %v", err)
		return
	}
	fmt.Println("Cleared all cache entries")
}

// ExampleRedisCacheWithExistingClient shows how to use cache with an existing Redis client
func ExampleRedisCacheWithExistingClient() {
	// This example shows how to use the cache with an existing Redis client
	// that you might already have in your application

	// Suppose you already have a Redis client
	// rdb := redis.NewClient(&redis.Options{...})

	// You can create a cache instance with it
	// cache := NewRedisCacheWithClient(rdb, "myapp:")

	// Then use it normally
	// cache.Set(ctx, "key", "value", time.Minute)

	fmt.Println("This example requires an existing Redis client")
}