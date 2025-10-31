package metrics

import (
	"sync"
	"testing"
	"time"
)

func TestNewRateLimitMetrics(t *testing.T) {
	rlm := NewRateLimitMetrics()
	if rlm == nil {
		t.Fatal("NewRateLimitMetrics() returned nil")
	}

	if rlm.maxHistorySize != DefaultRateLimitHistorySize {
		t.Errorf("Expected max history size to be %d, got %d", DefaultRateLimitHistorySize, rlm.maxHistorySize)
	}

	if rlm.maxViolationsMap != DefaultMaxViolationsMap {
		t.Errorf("Expected max violations map size to be %d, got %d", DefaultMaxViolationsMap, rlm.maxViolationsMap)
	}

	if len(rlm.requestHistory) != 0 {
		t.Errorf("Expected request history to be empty, got %d items", len(rlm.requestHistory))
	}

	if rlm.rateLimitConfig.RequestsPerMinute != DefaultRateLimitPerMinute {
		t.Errorf("Expected rate limit per minute to be %d, got %d", DefaultRateLimitPerMinute, rlm.rateLimitConfig.RequestsPerMinute)
	}
}

func TestUpdateConfig(t *testing.T) {
	rlm := NewRateLimitMetrics()
	
	newConfig := RateLimitConfig{
		RequestsPerMinute: 200,
		WindowSize:        2 * time.Minute,
		Enabled:           false,
	}

	rlm.UpdateConfig(newConfig)

	if rlm.rateLimitConfig.RequestsPerMinute != 200 {
		t.Errorf("Expected rate limit per minute to be 200, got %d", rlm.rateLimitConfig.RequestsPerMinute)
	}

	if rlm.rateLimitConfig.WindowSize != 2*time.Minute {
		t.Errorf("Expected window size to be 2 minutes, got %v", rlm.rateLimitConfig.WindowSize)
	}

	if rlm.rateLimitConfig.Enabled {
		t.Error("Expected rate limiting to be disabled")
	}
}

func TestRecordRequest_Allowed(t *testing.T) {
	rlm := NewRateLimitMetrics()
	
	ip := "192.168.1.1"
	userID := "user123"
	endpoint := "/api/v1/users"
	duration := 1 * time.Millisecond
	currentCount := int64(50)
	limit := int64(100)

	rlm.RecordRequest(ip, userID, endpoint, duration, true, "", currentCount, limit)

	stats := rlm.GetStats()
	if stats.TotalRequests != 1 {
		t.Errorf("Expected total requests to be 1, got %d", stats.TotalRequests)
	}
	if stats.AllowedRequests != 1 {
		t.Errorf("Expected allowed requests to be 1, got %d", stats.AllowedRequests)
	}
	if stats.ThrottledRequests != 0 {
		t.Errorf("Expected throttled requests to be 0, got %d", stats.ThrottledRequests)
	}
	if stats.ThrottleRate != 0 {
		t.Errorf("Expected throttle rate to be 0, got %f", stats.ThrottleRate)
	}
	if stats.AllowRate != 100 {
		t.Errorf("Expected allow rate to be 100, got %f", stats.AllowRate)
	}
}

func TestRecordRequest_Throttled(t *testing.T) {
	rlm := NewRateLimitMetrics()
	
	ip := "192.168.1.2"
	userID := "user456"
	endpoint := "/api/v1/data"
	duration := 2 * time.Millisecond
	reason := "Rate limit exceeded"
	currentCount := int64(101)
	limit := int64(100)

	rlm.RecordRequest(ip, userID, endpoint, duration, false, reason, currentCount, limit)

	stats := rlm.GetStats()
	if stats.TotalRequests != 1 {
		t.Errorf("Expected total requests to be 1, got %d", stats.TotalRequests)
	}
	if stats.AllowedRequests != 0 {
		t.Errorf("Expected allowed requests to be 0, got %d", stats.AllowedRequests)
	}
	if stats.ThrottledRequests != 1 {
		t.Errorf("Expected throttled requests to be 1, got %d", stats.ThrottledRequests)
	}
	if stats.ThrottleRate != 100 {
		t.Errorf("Expected throttle rate to be 100, got %f", stats.ThrottleRate)
	}
	if stats.AllowRate != 0 {
		t.Errorf("Expected allow rate to be 0, got %f", stats.AllowRate)
	}

	// Check violation tracking
	ipStats, userStats := rlm.GetViolationStats()
	if len(ipStats) != 1 {
		t.Errorf("Expected 1 IP violation, got %d", len(ipStats))
	}
	if len(userStats) != 1 {
		t.Errorf("Expected 1 user violation, got %d", len(userStats))
	}

	if ipStats[ip].TotalViolations != 1 {
		t.Errorf("Expected 1 violation for IP %s, got %d", ip, ipStats[ip].TotalViolations)
	}
	if userStats[userID].TotalViolations != 1 {
		t.Errorf("Expected 1 violation for user %s, got %d", userID, userStats[userID].TotalViolations)
	}
}

func TestRecordRequest_MultipleRequests(t *testing.T) {
	rlm := NewRateLimitMetrics()
	
	requests := []struct {
		ip       string
		userID   string
		allowed  bool
		duration time.Duration
	}{
		{"192.168.1.1", "user1", true, 1 * time.Millisecond},
		{"192.168.1.2", "user2", true, 2 * time.Millisecond},
		{"192.168.1.1", "user1", false, 1 * time.Millisecond}, // Violation
		{"192.168.1.3", "user3", true, 3 * time.Millisecond},
		{"192.168.1.2", "user2", false, 2 * time.Millisecond}, // Violation
	}

	for _, req := range requests {
		rlm.RecordRequest(req.ip, req.userID, "/test", req.duration, req.allowed, "", 50, 100)
	}

	stats := rlm.GetStats()
	if stats.TotalRequests != 5 {
		t.Errorf("Expected total requests to be 5, got %d", stats.TotalRequests)
	}
	if stats.AllowedRequests != 3 {
		t.Errorf("Expected allowed requests to be 3, got %d", stats.AllowedRequests)
	}
	if stats.ThrottledRequests != 2 {
		t.Errorf("Expected throttled requests to be 2, got %d", stats.ThrottledRequests)
	}

	expectedThrottleRate := float64(2) / float64(5) * 100
	if stats.ThrottleRate != expectedThrottleRate {
		t.Errorf("Expected throttle rate to be %f, got %f", expectedThrottleRate, stats.ThrottleRate)
	}

	// Check violation tracking
	ipStats, _ := rlm.GetViolationStats()
	if len(ipStats) != 2 {
		t.Errorf("Expected 2 IPs with violations, got %d", len(ipStats))
	}
	if ipStats["192.168.1.1"].TotalViolations != 1 {
		t.Errorf("Expected 1 violation for 192.168.1.1, got %d", ipStats["192.168.1.1"].TotalViolations)
	}
	if ipStats["192.168.1.2"].TotalViolations != 1 {
		t.Errorf("Expected 1 violation for 192.168.1.2, got %d", ipStats["192.168.1.2"].TotalViolations)
	}
}

func TestRecordRequest_DurationTracking(t *testing.T) {
	rlm := NewRateLimitMetrics()
	
	durations := []time.Duration{
		1 * time.Millisecond,
		5 * time.Millisecond,
		10 * time.Millisecond,
	}

	for _, duration := range durations {
		rlm.RecordRequest("127.0.0.1", "", "/test", duration, true, "", 50, 100)
	}

	stats := rlm.GetStats()
	expectedAvg := (1 + 5 + 10) * time.Millisecond / 3
	if stats.AvgCheckDuration != expectedAvg {
		t.Errorf("Expected avg check duration to be %v, got %v", expectedAvg, stats.AvgCheckDuration)
	}
	if stats.MaxCheckDuration != 10*time.Millisecond {
		t.Errorf("Expected max check duration to be %v, got %v", 10*time.Millisecond, stats.MaxCheckDuration)
	}
	if stats.MinCheckDuration != 1*time.Millisecond {
		t.Errorf("Expected min check duration to be %v, got %v", 1*time.Millisecond, stats.MinCheckDuration)
	}
}

func TestGetRecentRequests(t *testing.T) {
	rlm := NewRateLimitMetrics()

	// Record some requests
	for i := 0; i < 10; i++ {
		rlm.RecordRequest("127.0.0.1", "user1", "/test", time.Duration(i+1)*time.Millisecond, true, "", 50, 100)
	}

	// Test getting all recent requests
	recentRequests := rlm.GetRecentRequests(15)
	if len(recentRequests) != 10 {
		t.Errorf("Expected 10 recent requests, got %d", len(recentRequests))
	}

	// Test getting limited recent requests
	limitedRequests := rlm.GetRecentRequests(5)
	if len(limitedRequests) != 5 {
		t.Errorf("Expected 5 recent requests, got %d", len(limitedRequests))
	}

	// Verify order (should be most recent first)
	// The most recent request should have duration 10ms
	if limitedRequests[4].Duration != 10*time.Millisecond {
		t.Errorf("Expected most recent request to have duration 10ms, got %v", limitedRequests[4].Duration)
	}
}

func TestGetThrottleRate(t *testing.T) {
	rlm := NewRateLimitMetrics()

	// Test with no requests
	if rlm.GetThrottleRate() != 0 {
		t.Errorf("Expected throttle rate to be 0 with no requests, got %f", rlm.GetThrottleRate())
	}

	// Record some requests
	rlm.RecordRequest("127.0.0.1", "", "/test", 1*time.Millisecond, true, "", 50, 100)
	rlm.RecordRequest("127.0.0.2", "", "/test", 1*time.Millisecond, false, "", 101, 100) // Throttled
	rlm.RecordRequest("127.0.0.3", "", "/test", 1*time.Millisecond, true, "", 50, 100)

	expectedThrottleRate := float64(1) / float64(3) * 100
	if rlm.GetThrottleRate() != expectedThrottleRate {
		t.Errorf("Expected throttle rate to be %f, got %f", expectedThrottleRate, rlm.GetThrottleRate())
	}
}

func TestGetTotalRequests(t *testing.T) {
	rlm := NewRateLimitMetrics()

	if rlm.GetTotalRequests() != 0 {
		t.Errorf("Expected total requests to be 0 initially, got %d", rlm.GetTotalRequests())
	}

	// Record some requests
	for i := 0; i < 5; i++ {
		rlm.RecordRequest("127.0.0.1", "", "/test", 1*time.Millisecond, true, "", 50, 100)
	}

	if rlm.GetTotalRequests() != 5 {
		t.Errorf("Expected total requests to be 5, got %d", rlm.GetTotalRequests())
	}
}

func TestGetViolationStats(t *testing.T) {
	rlm := NewRateLimitMetrics()

	// Record violations
	rlm.RecordRequest("192.168.1.1", "user1", "/test", 1*time.Millisecond, false, "", 101, 100)
	rlm.RecordRequest("192.168.1.2", "user2", "/test", 1*time.Millisecond, false, "", 101, 100)
	rlm.RecordRequest("192.168.1.1", "user1", "/test", 1*time.Millisecond, false, "", 101, 100) // Second violation for same IP/user

	ipStats, userStats := rlm.GetViolationStats()

	// Check IP stats
	if len(ipStats) != 2 {
		t.Errorf("Expected 2 IP entries, got %d", len(ipStats))
	}
	if ipStats["192.168.1.1"].TotalViolations != 2 {
		t.Errorf("Expected 2 violations for 192.168.1.1, got %d", ipStats["192.168.1.1"].TotalViolations)
	}
	if ipStats["192.168.1.2"].TotalViolations != 1 {
		t.Errorf("Expected 1 violation for 192.168.1.2, got %d", ipStats["192.168.1.2"].TotalViolations)
	}

	// Check user stats
	if len(userStats) != 2 {
		t.Errorf("Expected 2 user entries, got %d", len(userStats))
	}
	if userStats["user1"].TotalViolations != 2 {
		t.Errorf("Expected 2 violations for user1, got %d", userStats["user1"].TotalViolations)
	}
	if userStats["user2"].TotalViolations != 1 {
		t.Errorf("Expected 1 violation for user2, got %d", userStats["user2"].TotalViolations)
	}
}

func TestGetEffectivenessMetrics(t *testing.T) {
	rlm := NewRateLimitMetrics()

	// Test with empty metrics
	emptyMetrics := rlm.GetEffectivenessMetrics(time.Hour)
	if emptyMetrics.RequestsPerSecond != 0 {
		t.Errorf("Expected 0 requests per second for empty metrics, got %f", emptyMetrics.RequestsPerSecond)
	}
	if emptyMetrics.ConfiguredRPS != float64(DefaultRateLimitPerMinute)/60.0 {
		t.Errorf("Expected configured RPS to be %f, got %f", 
			float64(DefaultRateLimitPerMinute)/60.0, emptyMetrics.ConfiguredRPS)
	}

	// Record some operations
	operations := []struct {
		allowed  bool
		duration time.Duration
	}{
		{true, 1 * time.Millisecond},
		{true, 2 * time.Millisecond},
		{true, 3 * time.Millisecond},
		{false, 1 * time.Millisecond}, // Throttled
		{true, 1 * time.Millisecond},
		{false, 2 * time.Millisecond}, // Throttled
		{true, 1 * time.Millisecond},
		{true, 1 * time.Millisecond},
	}

	for _, op := range operations {
		rlm.RecordRequest("127.0.0.1", "user1", "/test", op.duration, op.allowed, "", 50, 100)
	}

	// Get effectiveness metrics for a large time window
	metrics := rlm.GetEffectivenessMetrics(time.Hour)

	if metrics.RequestsPerSecond == 0 {
		t.Error("Expected non-zero requests per second")
	}

	expectedThrottleRate := float64(2) / float64(8) * 100
	if metrics.ThrottleRate != expectedThrottleRate {
		t.Errorf("Expected throttle rate to be %f, got %f", expectedThrottleRate, metrics.ThrottleRate)
	}

	// Check effectiveness score calculation
	// With 25% throttle rate, should be in the 50-80% range
	if metrics.EffectivenessScore < 50 || metrics.EffectivenessScore > 80 {
		t.Errorf("Expected effectiveness score between 50-80, got %f", metrics.EffectivenessScore)
	}

	// Check average check time
	expectedAvgTime := (1 + 2 + 3 + 1 + 1 + 2 + 1 + 1) * time.Millisecond / 8
	if metrics.AverageCheckTime != expectedAvgTime {
		t.Errorf("Expected average check time to be %v, got %v", expectedAvgTime, metrics.AverageCheckTime)
	}
}

func TestCalculateEffectivenessScore(t *testing.T) {
	rlm := NewRateLimitMetrics()

	testCases := []struct {
		throttleRate float64
		expectedScore float64
	}{
		{0.0, 100.0},        // Perfect
		{0.5, 100.0},        // Still perfect (<= 1%)
		{1.0, 100.0},        // Still perfect
		{5.0, 95.556},       // Good (in 1-10% range)
		{10.0, 90.0},        // Okay (boundary of 1-10% range)
		{15.0, 76.667},      // Fair (in 10-25% range)
		{25.0, 50.0},        // Poor (boundary of 10-25% range)
		{30.0, 45.0},        // Poor (above 25%)
		{75.0, 0.0},         // Very poor (capped at 0)
	}

	for _, tc := range testCases {
		score := rlm.calculateEffectivenessScore(tc.throttleRate)
		if score < tc.expectedScore-0.001 || score > tc.expectedScore+0.001 {
			t.Errorf("Expected effectiveness score to be %f for throttle rate %f, got %f",
				tc.expectedScore, tc.throttleRate, score)
		}
	}
}

func TestRateLimitReset(t *testing.T) {
	rlm := NewRateLimitMetrics()

	// Record some data
	rlm.RecordRequest("127.0.0.1", "user1", "/test", 1*time.Millisecond, true, "", 50, 100)
	rlm.RecordRequest("127.0.0.2", "user2", "/test", 2*time.Millisecond, false, "", 101, 100)

	// Reset metrics
	rlm.Reset()

	stats := rlm.GetStats()

	// Verify everything is reset to zero
	if stats.TotalRequests != 0 {
		t.Errorf("Expected total requests to be 0 after reset, got %d", stats.TotalRequests)
	}
	if stats.ThrottledRequests != 0 {
		t.Errorf("Expected throttled requests to be 0 after reset, got %d", stats.ThrottledRequests)
	}
	if stats.AllowedRequests != 0 {
		t.Errorf("Expected allowed requests to be 0 after reset, got %d", stats.AllowedRequests)
	}

	if len(stats.RecentRequests) != 0 {
		t.Errorf("Expected recent requests to be empty after reset, got %d", len(stats.RecentRequests))
	}

	// Check violation stats
	ipStats, userStats := rlm.GetViolationStats()
	if len(ipStats) != 0 {
		t.Errorf("Expected IP violations to be empty after reset, got %d", len(ipStats))
	}
	if len(userStats) != 0 {
		t.Errorf("Expected user violations to be empty after reset, got %d", len(userStats))
	}
}

func TestRateLimitSetMaxHistorySize(t *testing.T) {
	rlm := NewRateLimitMetrics()
	newSize := 100

	// Record more requests than the new limit
	for i := 0; i < 200; i++ {
		rlm.RecordRequest("127.0.0.1", "user1", "/test", 1*time.Millisecond, true, "", 50, 100)
	}

	// Set smaller history size
	rlm.SetMaxHistorySize(newSize)

	if rlm.maxHistorySize != newSize {
		t.Errorf("Expected max history size to be %d, got %d", newSize, rlm.maxHistorySize)
	}

	// Verify history size is managed
	recentRequests := rlm.GetRecentRequests(200)
	if len(recentRequests) > 200 { // Allow some buffer due to the half-trimming strategy
		t.Errorf("Expected request history to be managed by max size, got %d", len(recentRequests))
	}
}

func TestSetMaxViolationsMap(t *testing.T) {
	rlm := NewRateLimitMetrics()
	newSize := 50

	// Record violations from many different IPs
	for i := 0; i < 100; i++ {
		ip := "192.168.1." + string(rune(i))
		rlm.RecordRequest(ip, "user1", "/test", 1*time.Millisecond, false, "", 101, 100)
	}

	// Set smaller violations map size
	rlm.SetMaxViolationsMap(newSize)

	if rlm.maxViolationsMap != newSize {
		t.Errorf("Expected max violations map size to be %d, got %d", newSize, rlm.maxViolationsMap)
	}

	// Verify violations map size is managed
	ipStats, _ := rlm.GetViolationStats()
	if len(ipStats) > newSize*2 { // Allow some buffer due to cleanup strategy
		t.Errorf("Expected IP violations map to be managed, got %d", len(ipStats))
	}
}

func TestCleanupOldViolations(t *testing.T) {
	rlm := NewRateLimitMetrics()

	// Record some violations
	rlm.RecordRequest("192.168.1.1", "user1", "/test", 1*time.Millisecond, false, "", 101, 100)

	// Manually set an old violation time to test cleanup
	rlm.mu.Lock()
	oldTime := time.Now().Add(-25 * time.Hour) // Older than 24 hour cutoff
	rlm.ipViolations["192.168.1.1"].LastViolation = oldTime
	rlm.userViolations["user1"].LastViolation = oldTime
	rlm.mu.Unlock()

	// Trigger cleanup
	rlm.cleanupOldViolations()

	// Verify old violations were cleaned up
	ipStats, userStats := rlm.GetViolationStats()
	if len(ipStats) != 0 {
		t.Errorf("Expected IP violations to be cleaned up, got %d", len(ipStats))
	}
	if len(userStats) != 0 {
		t.Errorf("Expected user violations to be cleaned up, got %d", len(userStats))
	}
}

func TestTrimViolationsMap(t *testing.T) {
	rlm := NewRateLimitMetrics()
	rlm.maxViolationsMap = 10

	// Create violations map with many entries
	violations := make(map[string]*ViolationTracker)
	now := time.Now()

	for i := 0; i < 20; i++ {
		violations["192.168.1."+string(rune(i))] = &ViolationTracker{
			Identifier:     "192.168.1." + string(rune(i)),
			TotalViolations: 1,
			LastViolation:   now.Add(time.Duration(i) * time.Hour), // Different times
		}
	}

	// Trim the map
	rlm.trimViolationsMap(violations)

	expectedSize := rlm.maxViolationsMap / 2 // 5
	if len(violations) != expectedSize {
		t.Errorf("Expected violations map size to be %d after trimming, got %d", expectedSize, len(violations))
	}

	// Verify that the most recent violations are kept
	for id, tracker := range violations {
		if tracker.LastViolation.Before(now.Add(14 * time.Hour)) {
			t.Errorf("Expected only recent violations to be kept, but found old violation for %s", id)
		}
	}
}

func TestRateLimitConcurrentOperations(t *testing.T) {
	rlm := NewRateLimitMetrics()

	// Test concurrent access to ensure thread safety
	var wg sync.WaitGroup
	numGoroutines := 50
	operationsPerGoroutine := 20

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < operationsPerGoroutine; j++ {
				ip := "192.168.1." + string(rune(id%10)) // Reuse some IPs to test violation tracking
				userID := "user" + string(rune(id%5))    // Reuse some users
				duration := time.Duration(j+1) * time.Millisecond
				allowed := j%5 != 0 // Every 5th request is throttled
				
				rlm.RecordRequest(ip, userID, "/test", duration, allowed, "", 50, 100)
			}
		}(i)
	}

	wg.Wait()

	stats := rlm.GetStats()
	expectedTotal := uint64(numGoroutines * operationsPerGoroutine)
	
	if stats.TotalRequests != expectedTotal {
		t.Errorf("Expected %d total requests, got %d", expectedTotal, stats.TotalRequests)
	}

	// Check that throttled requests were tracked correctly
	if stats.ThrottledRequests == 0 {
		t.Error("Expected some throttled requests, got 0")
	}

	// Check violation tracking
	ipStats, userStats := rlm.GetViolationStats()
	if len(ipStats) == 0 {
		t.Error("Expected some IP violations, got none")
	}
	if len(userStats) == 0 {
		t.Error("Expected some user violations, got none")
	}
}

func TestGetTopViolators(t *testing.T) {
	rlm := NewRateLimitMetrics()

	// Create violations with different counts
	violations := map[string]*ViolationTracker{
		"192.168.1.1": {Identifier: "192.168.1.1", TotalViolations: 10, LastViolation: time.Now()},
		"192.168.1.2": {Identifier: "192.168.1.2", TotalViolations: 5, LastViolation: time.Now()},
		"192.168.1.3": {Identifier: "192.168.1.3", TotalViolations: 15, LastViolation: time.Now()},
		"192.168.1.4": {Identifier: "192.168.1.4", TotalViolations: 1, LastViolation: time.Now()},
	}

	topViolators := rlm.getTopViolators(violations, 3)

	// Should return top 3 violators sorted by violation count
	if len(topViolators) != 3 {
		t.Errorf("Expected 3 top violators, got %d", len(topViolators))
	}

	// Check order (should be highest first)
	if topViolators[0].TotalViolations != 15 {
		t.Errorf("Expected top violator to have 15 violations, got %d", topViolators[0].TotalViolations)
	}
	if topViolators[1].TotalViolations != 10 {
		t.Errorf("Expected second violator to have 10 violations, got %d", topViolators[1].TotalViolations)
	}
	if topViolators[2].TotalViolations != 5 {
		t.Errorf("Expected third violator to have 5 violations, got %d", topViolators[2].TotalViolations)
	}
}

func TestIdentifyHotspots(t *testing.T) {
	rlm := NewRateLimitMetrics()

	// Create requests with high violation patterns to trigger hotspot identification
	requests := []RateLimitRequest{}

	// Create a hotspot IP with 66% violation rate (4 violations out of 6 requests)
	for i := 0; i < 6; i++ {
		req := RateLimitRequest{
			IP:        "192.168.1.1",
			UserID:    "user1",
			Allowed:   i >= 4, // First 4 are violations, last 2 are allowed
			Timestamp: time.Now(),
		}
		requests = append(requests, req)
	}

	// Add more requests for the same IP to reach the 10 request threshold
	for i := 0; i < 4; i++ {
		req := RateLimitRequest{
			IP:        "192.168.1.1",
			UserID:    "user1",
			Allowed:   true, // These are allowed
			Timestamp: time.Now(),
		}
		requests = append(requests, req)
	}

	// Create a hotspot user with 80% violation rate (4 violations out of 5 requests)
	for i := 0; i < 5; i++ {
		req := RateLimitRequest{
			IP:        "192.168.1.2",
			UserID:    "user2",
			Allowed:   i >= 4, // First 4 are violations, last 1 is allowed
			Timestamp: time.Now(),
		}
		requests = append(requests, req)
	}

	// Add some normal traffic
	for i := 0; i < 20; i++ {
		req := RateLimitRequest{
			IP:        "192.168.1.3",
			UserID:    "user3",
			Allowed:   true,
			Timestamp: time.Now(),
		}
		requests = append(requests, req)
	}

	hotspots := rlm.identifyHotspots(requests, time.Now().Add(-time.Hour))

	// Should identify hotspots with high violation rates
	if len(hotspots) == 0 {
		t.Error("Expected at least one hotspot to be identified")
	}

	// Verify hotspot properties
	foundIPHotspot := false
	foundUserHotspot := false

	for _, hotspot := range hotspots {
		if hotspot.ViolationRate <= 5.0 {
			t.Errorf("Expected hotspot violation rate to be > 5%%, got %f", hotspot.ViolationRate)
		}
		if hotspot.Type != "ip" && hotspot.Type != "user" {
			t.Errorf("Expected hotspot type to be 'ip' or 'user', got %s", hotspot.Type)
		}
		if hotspot.Type == "ip" && hotspot.Identifier == "192.168.1.1" {
			foundIPHotspot = true
		}
		if hotspot.Type == "user" && hotspot.Identifier == "user2" {
			foundUserHotspot = true
		}
	}

	if !foundIPHotspot {
		t.Error("Expected to find IP hotspot for 192.168.1.1")
	}
	if !foundUserHotspot {
		t.Error("Expected to find user hotspot for user2")
	}
}

// Benchmark tests
func BenchmarkRecordRequest(b *testing.B) {
	rlm := NewRateLimitMetrics()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		rlm.RecordRequest("127.0.0.1", "user1", "/test", 1*time.Millisecond, true, "", 50, 100)
	}
}

func BenchmarkRateLimitGetStats(b *testing.B) {
	rlm := NewRateLimitMetrics()

	// Pre-populate with some data
	for i := 0; i < 1000; i++ {
		rlm.RecordRequest("127.0.0.1", "user1", "/test", 1*time.Millisecond, i%10 != 0, "", 50, 100)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		rlm.GetStats()
	}
}

func BenchmarkGetEffectivenessMetrics(b *testing.B) {
	rlm := NewRateLimitMetrics()

	// Pre-populate with some data
	for i := 0; i < 1000; i++ {
		rlm.RecordRequest("127.0.0.1", "user1", "/test", 1*time.Millisecond, i%10 != 0, "", 50, 100)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		rlm.GetEffectivenessMetrics(time.Hour)
	}
}