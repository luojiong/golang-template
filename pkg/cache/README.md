# Cache Package

This package provides a comprehensive caching interface with Redis implementation for the go-server project.

## Features

- **Comprehensive Interface**: Full-featured cache interface with Get, Set, Delete, Exists, Clear operations
- **TTL Support**: Configurable time-to-live for cache entries
- **Batch Operations**: Support for multiple key operations (GetMultiple, SetMultiple, DeleteMultiple)
- **Atomic Operations**: SetIfNotExists, Increment, Decrement with atomic guarantees
- **JSON Handling**: Automatic JSON marshaling/unmarshaling for complex data types
- **Connection Pooling**: Redis client with configurable connection pooling
- **Error Handling**: Comprehensive error handling with proper error wrapping
- **Context Support**: Full support for context cancellation and timeouts
- **Key Prefixing**: Automatic key prefixing to avoid key collisions
- **Pattern Matching**: Support for key pattern matching (wildcards)

## Installation

The package uses Redis as the underlying cache storage. Make sure you have Redis installed and running.

```bash
# Add the dependency (already included in go.mod)
go get github.com/redis/go-redis/v9
```

## Usage

### Basic Setup

```go
import (
    "context"
    "time"
    "go-server/pkg/cache"
)

// Create cache with default configuration
config := cache.DefaultRedisConfig()
config.Host = "localhost"
config.Port = 6379
config.Prefix = "myapp:"

cache, err := cache.NewRedisCache(config)
if err != nil {
    log.Fatal(err)
}
defer cache.Close()

ctx := context.Background()
```

### Basic Operations

```go
// Set a value with 5-minute TTL
err := cache.Set(ctx, "user:123", userObject, 5*time.Minute)

// Get a value
value, found := cache.Get(ctx, "user:123")
if found {
    fmt.Printf("User: %v\n", value)
}

// Check if key exists
exists, err := cache.Exists(ctx, "user:123")

// Delete a key
err := cache.Delete(ctx, "user:123")
```

### TTL Operations

```go
// Get value with remaining TTL
value, ttl, found := cache.GetWithTTL(ctx, "user:123")
if found {
    fmt.Printf("Value: %v, TTL: %v\n", value, ttl)
}

// Set TTL for existing key
err := cache.SetTTL(ctx, "user:123", 10*time.Minute)

// Get TTL for a key
ttl, err := cache.GetTTL(ctx, "user:123")
```

### Batch Operations

```go
// Set multiple values
items := map[string]interface{}{
    "user:1": userData1,
    "user:2": userData2,
    "user:3": userData3,
}
err := cache.SetMultiple(ctx, items, 10*time.Minute)

// Get multiple values
keys := []string{"user:1", "user:2", "user:3", "user:4"} // user:4 doesn't exist
results, err := cache.GetMultiple(ctx, keys)
// results contains only found keys

// Delete multiple keys
err = cache.DeleteMultiple(ctx, []string{"user:1", "user:2"})
```

### Atomic Operations

```go
// Set only if key doesn't exist
success, err := cache.SetIfNotExists(ctx, "config:theme", "dark", 24*time.Hour)

// Increment counter
newValue, err := cache.Increment(ctx, "page:views", 1)

// Decrement counter
newValue, err := cache.Decrement(ctx, "inventory:stock", 5)
```

### Pattern Matching

```go
// List all keys
allKeys, err := cache.Keys(ctx, "*")

// List user keys
userKeys, err := cache.Keys(ctx, "user:*")

// List session keys
sessionKeys, err := cache.Keys(ctx, "session:*")
```

### Complex Data Types

The cache automatically handles JSON marshaling/unmarshaling for complex types:

```go
type User struct {
    ID    string `json:"id"`
    Name  string `json:"name"`
    Email string `json:"email"`
}

user := User{
    ID:    "123",
    Name:  "John Doe",
    Email: "john@example.com",
}

// Store complex object
err := cache.Set(ctx, "user:123", user, 1*time.Hour)

// Retrieve complex object
value, found := cache.Get(ctx, "user:123")
if found {
    // value will be map[string]interface{} representing the JSON
    userData := value.(map[string]interface{})
    fmt.Printf("User name: %v\n", userData["name"])
}
```

## Configuration

The `RedisConfig` struct provides comprehensive configuration options:

```go
type RedisConfig struct {
    Host         string        // Redis server host (default: "localhost")
    Port         int          // Redis server port (default: 6379)
    Password     string       // Redis password (default: "")
    DB           int          // Redis database number (default: 0)
    Prefix       string       // Key prefix (default: "cache:")
    PoolSize     int          // Connection pool size (default: 10)
    MinIdleConns int          // Minimum idle connections (default: 5)
    DialTimeout  time.Duration // Connection timeout (default: 5s)
    ReadTimeout  time.Duration // Read timeout (default: 3s)
    WriteTimeout time.Duration // Write timeout (default: 3s)
    PoolTimeout  time.Duration // Pool timeout (default: 4s)
}
```

## Error Handling

All cache operations return errors that wrap the underlying Redis errors with additional context:

```go
err := cache.Set(ctx, "key", "value", time.Minute)
if err != nil {
    // Error will include operation context
    log.Printf("Failed to set cache: %w", err)
}
```

## Interface

The cache implements the `Cache` interface, making it easy to mock for testing:

```go
type Cache interface {
    Get(ctx context.Context, key string) (interface{}, bool)
    GetWithTTL(ctx context.Context, key string) (interface{}, time.Duration, bool)
    Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
    SetMultiple(ctx context.Context, items map[string]interface{}, ttl time.Duration) error
    Delete(ctx context.Context, key string) error
    DeleteMultiple(ctx context.Context, keys []string) error
    Exists(ctx context.Context, key string) (bool, error)
    Clear(ctx context.Context) error
    Keys(ctx context.Context, pattern string) ([]string, error)
    GetMultiple(ctx context.Context, keys []string) (map[string]interface{}, error)
    SetIfNotExists(ctx context.Context, key string, value interface{}, ttl time.Duration) (bool, error)
    Increment(ctx context.Context, key string, amount int64) (int64, error)
    Decrement(ctx context.Context, key string, amount int64) (int64, error)
    Close() error
}
```

## Best Practices

1. **Use Context**: Always pass context with appropriate timeouts
2. **Handle Errors**: Check and handle all returned errors
3. **Set TTL**: Always set appropriate TTL to prevent memory bloat
4. **Use Prefixes**: Use meaningful prefixes to organize cache keys
5. **Batch Operations**: Use batch operations for better performance
6. **Connection Pool**: Configure appropriate pool size for your load
7. **Resource Cleanup**: Always call `Close()` when done with the cache

## Testing

The package includes comprehensive tests. Tests require a running Redis instance and will be skipped if Redis is not available:

```bash
# Run tests
go test ./pkg/cache -v

# Run tests with Redis running on localhost:6379
go test ./pkg/cache -v
```

## Examples

See `example.go` for comprehensive usage examples covering all features of the cache package.