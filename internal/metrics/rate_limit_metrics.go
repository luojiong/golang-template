package metrics

import (
	"log"
	"sync"
	"sync/atomic"
	"time"
)

// RateLimitMetrics tracks rate limiting effectiveness statistics
type RateLimitMetrics struct {
	mu sync.RWMutex

	// Request counters
	totalRequests      uint64
	throttledRequests  uint64
	allowedRequests    uint64

	// Rate limit violation tracking by IP and user
	ipViolations     map[string]*ViolationTracker
	userViolations   map[string]*ViolationTracker
	maxViolationsMap int

	// Request history for recent performance analysis
	requestHistory []RateLimitRequest
	maxHistorySize int

	// Rate limit configuration tracking
	rateLimitConfig RateLimitConfig

	// Performance tracking
	totalCheckDuration int64 // in nanoseconds
	maxCheckDuration    int64 // in nanoseconds
	minCheckDuration    int64 // in nanoseconds
}

// RateLimitRequest represents a single rate limit request/check
type RateLimitRequest struct {
	IP          string        `json:"ip"`
	UserID      string        `json:"user_id,omitempty"`
	Endpoint    string        `json:"endpoint"`
	Duration    time.Duration `json:"duration"`
	Allowed     bool          `json:"allowed"`
	Reason      string        `json:"reason,omitempty"`
	Timestamp   time.Time     `json:"timestamp"`
	WindowSize  time.Duration `json:"window_size"`
	CurrentCount int64        `json:"current_count"`
	Limit        int64        `json:"limit"`
}

// ViolationTracker tracks rate limit violations for a specific identifier
type ViolationTracker struct {
	Identifier       string        `json:"identifier"`        // IP or user ID
	TotalViolations  uint64        `json:"total_violations"`
	LastViolation    time.Time     `json:"last_violation"`
	ViolationHistory []time.Time   `json:"violation_history"`
	FirstViolation   time.Time     `json:"first_violation"`
}

// RateLimitConfig represents the current rate limiting configuration
type RateLimitConfig struct {
	RequestsPerMinute int           `json:"requests_per_minute"`
	WindowSize        time.Duration `json:"window_size"`
	Enabled           bool          `json:"enabled"`
}

// RateLimitStats represents aggregated rate limit statistics
type RateLimitStats struct {
	TotalRequests     uint64                    `json:"total_requests"`
	ThrottledRequests uint64                    `json:"throttled_requests"`
	AllowedRequests   uint64                    `json:"allowed_requests"`
	ThrottleRate      float64                   `json:"throttle_rate"`
	AllowRate         float64                   `json:"allow_rate"`
	AvgCheckDuration  time.Duration             `json:"avg_check_duration"`
	MaxCheckDuration  time.Duration             `json:"max_check_duration"`
	MinCheckDuration  time.Duration             `json:"min_check_duration"`
	TopViolatingIPs   []ViolationTracker        `json:"top_violating_ips,omitempty"`
	TopViolatingUsers []ViolationTracker        `json:"top_violating_users,omitempty"`
	RecentRequests    []RateLimitRequest        `json:"recent_requests,omitempty"`
	Configuration     RateLimitConfig           `json:"configuration"`
	EffectiveRate     float64                   `json:"effective_rate"` // Effectiveness score
}

// RateLimitEffectiveness represents detailed effectiveness metrics
type RateLimitEffectiveness struct {
	RequestsPerSecond    float64   `json:"requests_per_second"`
	ThrottleRate         float64   `json:"throttle_rate"`
	EffectivenessScore   float64   `json:"effectiveness_score"`   // 0-100, higher is better
	ViolationHotspots    []Hotspot `json:"violation_hotspots"`
	AverageCheckTime     time.Duration `json:"average_check_time"`
	P95CheckTime         time.Duration `json:"p95_check_time"`
	ConfiguredRPS        float64   `json:"configured_rps"`
	ActualRPS            float64   `json:"actual_rps"`
}

// Hotspot represents a rate limit violation hotspot
type Hotspot struct {
	Identifier    string    `json:"identifier"`     // IP or user ID
	Type          string    `json:"type"`           // "ip" or "user"
	ViolationCount int     `json:"violation_count"`
	ViolationRate  float64  `json:"violation_rate"`
	LastViolation  time.Time `json:"last_violation"`
}

// Constants for rate limiting monitoring
const (
	DefaultRateLimitPerMinute = 100 // REQ-MW-001: 100 requests/minute per IP
	DefaultRateLimitHistorySize = 5000
	DefaultMaxViolationsMap    = 10000
	DefaultWindowSize          = time.Minute
)

// NewRateLimitMetrics creates a new rate limit metrics instance
func NewRateLimitMetrics() *RateLimitMetrics {
	return &RateLimitMetrics{
		ipViolations:     make(map[string]*ViolationTracker),
		userViolations:   make(map[string]*ViolationTracker),
		maxViolationsMap: DefaultMaxViolationsMap,
		maxHistorySize:   DefaultRateLimitHistorySize,
		requestHistory:   make([]RateLimitRequest, 0),
		rateLimitConfig: RateLimitConfig{
			RequestsPerMinute: DefaultRateLimitPerMinute,
			WindowSize:        DefaultWindowSize,
			Enabled:           true,
		},
	}
}

// UpdateConfig updates the rate limit configuration being tracked
func (rlm *RateLimitMetrics) UpdateConfig(config RateLimitConfig) {
	rlm.mu.Lock()
	defer rlm.mu.Unlock()
	rlm.rateLimitConfig = config
}

// RecordRequest records a rate limit request check
func (rlm *RateLimitMetrics) RecordRequest(ip, userID, endpoint string, duration time.Duration, allowed bool, reason string, currentCount, limit int64) {
	// Update atomic counters
	atomic.AddUint64(&rlm.totalRequests, 1)
	atomic.AddInt64(&rlm.totalCheckDuration, int64(duration))

	// Update min/max durations
	maxDuration := atomic.LoadInt64(&rlm.maxCheckDuration)
	if duration > time.Duration(maxDuration) {
		atomic.CompareAndSwapInt64(&rlm.maxCheckDuration, maxDuration, int64(duration))
	}

	minDuration := atomic.LoadInt64(&rlm.minCheckDuration)
	if minDuration == 0 || duration < time.Duration(minDuration) {
		atomic.CompareAndSwapInt64(&rlm.minCheckDuration, minDuration, int64(duration))
	}

	// Update allowed/throttled counters
	if allowed {
		atomic.AddUint64(&rlm.allowedRequests, 1)
	} else {
		atomic.AddUint64(&rlm.throttledRequests, 1)
		// Track violations
		rlm.trackViolation(ip, userID)
	}

	// Record request in history
	rlm.recordRequest(RateLimitRequest{
		IP:           ip,
		UserID:       userID,
		Endpoint:     endpoint,
		Duration:     duration,
		Allowed:      allowed,
		Reason:       reason,
		Timestamp:    time.Now(),
		WindowSize:   rlm.rateLimitConfig.WindowSize,
		CurrentCount: currentCount,
		Limit:        limit,
	})
}

// trackViolation tracks rate limit violations by IP and user
func (rlm *RateLimitMetrics) trackViolation(ip, userID string) {
	rlm.mu.Lock()
	defer rlm.mu.Unlock()

	now := time.Now()

	// Track IP violations
	if ip != "" {
		if rlm.ipViolations[ip] == nil {
			rlm.ipViolations[ip] = &ViolationTracker{
				Identifier: ip,
				FirstViolation: now,
				ViolationHistory: make([]time.Time, 0),
			}
		}
		
		tracker := rlm.ipViolations[ip]
		tracker.TotalViolations++
		tracker.LastViolation = now
		tracker.ViolationHistory = append(tracker.ViolationHistory, now)

		// Trim violation history to reasonable size
		if len(tracker.ViolationHistory) > 100 {
			tracker.ViolationHistory = tracker.ViolationHistory[50:] // Keep last 50
		}
	}

	// Track user violations
	if userID != "" {
		if rlm.userViolations[userID] == nil {
			rlm.userViolations[userID] = &ViolationTracker{
				Identifier: userID,
				FirstViolation: now,
				ViolationHistory: make([]time.Time, 0),
			}
		}
		
		tracker := rlm.userViolations[userID]
		tracker.TotalViolations++
		tracker.LastViolation = now
		tracker.ViolationHistory = append(tracker.ViolationHistory, now)

		// Trim violation history to reasonable size
		if len(tracker.ViolationHistory) > 100 {
			tracker.ViolationHistory = tracker.ViolationHistory[50:] // Keep last 50
		}
	}

	// Cleanup old violations to prevent memory leaks
	rlm.cleanupOldViolations()
}

// cleanupOldViolations removes old violations to prevent memory leaks
func (rlm *RateLimitMetrics) cleanupOldViolations() {
	cutoff := time.Now().Add(-24 * time.Hour) // Keep last 24 hours

	// Cleanup IP violations
	for ip, tracker := range rlm.ipViolations {
		if tracker.LastViolation.Before(cutoff) {
			delete(rlm.ipViolations, ip)
		}
	}

	// Cleanup user violations
	for userID, tracker := range rlm.userViolations {
		if tracker.LastViolation.Before(cutoff) {
			delete(rlm.userViolations, userID)
		}
	}

	// Ensure maps don't grow too large
	if len(rlm.ipViolations) > rlm.maxViolationsMap {
		rlm.trimViolationsMap(rlm.ipViolations)
	}
	if len(rlm.userViolations) > rlm.maxViolationsMap {
		rlm.trimViolationsMap(rlm.userViolations)
	}
}

// trimViolationsMap trims the violations map to a reasonable size
func (rlm *RateLimitMetrics) trimViolationsMap(violations map[string]*ViolationTracker) {
	type violationEntry struct {
		identifier string
		lastViolation time.Time
	}

	// Collect all violations with their last violation time
	var allViolations []violationEntry
	for id, tracker := range violations {
		allViolations = append(allViolations, violationEntry{
			identifier: id,
			lastViolation: tracker.LastViolation,
		})
	}

	// Sort by last violation time (oldest first)
	for i := 0; i < len(allViolations); i++ {
		for j := i + 1; j < len(allViolations); j++ {
			if allViolations[i].lastViolation.After(allViolations[j].lastViolation) {
				allViolations[i], allViolations[j] = allViolations[j], allViolations[i]
			}
		}
	}

	// Keep only the most recent violations
	keepCount := rlm.maxViolationsMap / 2
	for i := 0; i < len(allViolations)-keepCount; i++ {
		delete(violations, allViolations[i].identifier)
	}
}

// recordRequest records a request in the history
func (rlm *RateLimitMetrics) recordRequest(request RateLimitRequest) {
	rlm.mu.Lock()
	defer rlm.mu.Unlock()

	// Add new request
	rlm.requestHistory = append(rlm.requestHistory, request)

	// Trim history if it exceeds max size
	if len(rlm.requestHistory) > rlm.maxHistorySize {
		// Remove oldest requests (keep only the last maxHistorySize)
		rlm.requestHistory = rlm.requestHistory[rlm.maxHistorySize/2:]
	}
}

// GetStats returns current rate limit statistics
func (rlm *RateLimitMetrics) GetStats() RateLimitStats {
	totalRequests := atomic.LoadUint64(&rlm.totalRequests)
	throttledRequests := atomic.LoadUint64(&rlm.throttledRequests)
	allowedRequests := atomic.LoadUint64(&rlm.allowedRequests)
	totalCheckDuration := atomic.LoadInt64(&rlm.totalCheckDuration)
	maxCheckDuration := atomic.LoadInt64(&rlm.maxCheckDuration)
	minCheckDuration := atomic.LoadInt64(&rlm.minCheckDuration)

	// Calculate rates
	throttleRate := float64(0)
	allowRate := float64(0)
	effectivenessScore := float64(0)

	if totalRequests > 0 {
		throttleRate = float64(throttledRequests) / float64(totalRequests) * 100
		allowRate = float64(allowedRequests) / float64(totalRequests) * 100
		
		// Calculate effectiveness score
		// Higher effectiveness when throttling is controlled (not too high, not too low)
		// Optimal range is 1-10% throttling rate
		if throttleRate <= 1.0 {
			effectivenessScore = 100.0
		} else if throttleRate <= 10.0 {
			effectivenessScore = 90.0 - (throttleRate - 1.0) * 10.0 // 90-100%
		} else if throttleRate <= 25.0 {
			effectivenessScore = 80.0 - (throttleRate - 10.0) * 2.0 // 50-90%
		} else {
			effectivenessScore = 50.0 - (throttleRate - 25.0) // Lower effectiveness for high throttling
		}
		if effectivenessScore < 0 {
			effectivenessScore = 0
		}
	}

	// Calculate average check duration
	avgCheckDuration := time.Duration(0)
	if totalRequests > 0 {
		avgCheckDuration = time.Duration(totalCheckDuration / int64(totalRequests))
	}

	// Get top violators
	rlm.mu.RLock()
	topIPs := rlm.getTopViolators(rlm.ipViolations, 10)
	topUsers := rlm.getTopViolators(rlm.userViolations, 10)
	
	recentRequests := make([]RateLimitRequest, len(rlm.requestHistory))
	copy(recentRequests, rlm.requestHistory)
	config := rlm.rateLimitConfig
	rlm.mu.RUnlock()

	return RateLimitStats{
		TotalRequests:     totalRequests,
		ThrottledRequests: throttledRequests,
		AllowedRequests:   allowedRequests,
		ThrottleRate:      throttleRate,
		AllowRate:         allowRate,
		AvgCheckDuration:  avgCheckDuration,
		MaxCheckDuration:  time.Duration(maxCheckDuration),
		MinCheckDuration:  time.Duration(minCheckDuration),
		TopViolatingIPs:   topIPs,
		TopViolatingUsers: topUsers,
		RecentRequests:    recentRequests,
		Configuration:     config,
		EffectiveRate:     effectivenessScore,
	}
}

// getTopViolators returns the top violators from a violations map
func (rlm *RateLimitMetrics) getTopViolators(violations map[string]*ViolationTracker, limit int) []ViolationTracker {
	var violators []ViolationTracker
	for _, tracker := range violations {
		violators = append(violators, *tracker)
	}

	// Sort by total violations (highest first)
	for i := 0; i < len(violators); i++ {
		for j := i + 1; j < len(violators); j++ {
			if violators[i].TotalViolations < violators[j].TotalViolations {
				violators[i], violators[j] = violators[j], violators[i]
			}
		}
	}

	// Return top violators
	if limit > len(violators) {
		limit = len(violators)
	}
	return violators[:limit]
}

// GetRecentRequests returns the most recent rate limit requests
func (rlm *RateLimitMetrics) GetRecentRequests(limit int) []RateLimitRequest {
	rlm.mu.RLock()
	defer rlm.mu.RUnlock()

	if limit <= 0 || limit > len(rlm.requestHistory) {
		limit = len(rlm.requestHistory)
	}

	// Return the last 'limit' requests
	start := len(rlm.requestHistory) - limit
	if start < 0 {
		start = 0
	}

	recentRequests := make([]RateLimitRequest, limit)
	copy(recentRequests, rlm.requestHistory[start:])
	return recentRequests
}

// GetThrottleRate returns the current throttle rate as a percentage
func (rlm *RateLimitMetrics) GetThrottleRate() float64 {
	totalRequests := atomic.LoadUint64(&rlm.totalRequests)
	throttledRequests := atomic.LoadUint64(&rlm.throttledRequests)

	if totalRequests == 0 {
		return 0
	}

	return float64(throttledRequests) / float64(totalRequests) * 100
}

// GetTotalRequests returns the total number of requests
func (rlm *RateLimitMetrics) GetTotalRequests() uint64 {
	return atomic.LoadUint64(&rlm.totalRequests)
}

// GetViolationStats returns detailed violation statistics by IP and user
func (rlm *RateLimitMetrics) GetViolationStats() (map[string]ViolationTracker, map[string]ViolationTracker) {
	rlm.mu.RLock()
	defer rlm.mu.RUnlock()

	ipStats := make(map[string]ViolationTracker)
	for ip, tracker := range rlm.ipViolations {
		ipStats[ip] = *tracker
	}

	userStats := make(map[string]ViolationTracker)
	for userID, tracker := range rlm.userViolations {
		userStats[userID] = *tracker
	}

	return ipStats, userStats
}

// GetEffectivenessMetrics calculates detailed effectiveness metrics
func (rlm *RateLimitMetrics) GetEffectivenessMetrics(timeWindow time.Duration) RateLimitEffectiveness {
	rlm.mu.RLock()
	defer rlm.mu.RUnlock()

	now := time.Now()
	cutoff := now.Add(-timeWindow)

	// Filter requests within time window
	var recentRequests []RateLimitRequest
	for _, req := range rlm.requestHistory {
		if req.Timestamp.After(cutoff) {
			recentRequests = append(recentRequests, req)
		}
	}

	if len(recentRequests) == 0 {
		return RateLimitEffectiveness{
			ConfiguredRPS: float64(rlm.rateLimitConfig.RequestsPerMinute) / 60.0,
		}
	}

	// Calculate metrics
	totalRequests := len(recentRequests)
	throttledCount := 0
	var totalCheckDuration time.Duration
	var checkDurations []time.Duration

	for _, req := range recentRequests {
		totalCheckDuration += req.Duration
		checkDurations = append(checkDurations, req.Duration)
		if !req.Allowed {
			throttledCount++
		}
	}

	// Calculate rates
	requestsPerSecond := float64(totalRequests) / timeWindow.Seconds()
	throttleRate := float64(throttledCount) / float64(totalRequests) * 100
	avgCheckTime := totalCheckDuration / time.Duration(totalRequests)

	// Calculate P95 check time
	if len(checkDurations) > 0 {
		// Simple insertion sort for small slices
		for i := 1; i < len(checkDurations); i++ {
			key := checkDurations[i]
			j := i - 1
			for j >= 0 && checkDurations[j] > key {
				checkDurations[j+1] = checkDurations[j]
				j--
			}
			checkDurations[j+1] = key
		}
	}

	p95CheckTime := time.Duration(0)
	if len(checkDurations) > 0 {
		p95Index := int(float64(len(checkDurations)) * 0.95)
		if p95Index >= len(checkDurations) {
			p95Index = len(checkDurations) - 1
		}
		p95CheckTime = checkDurations[p95Index]
	}

	// Calculate effectiveness score
	effectivenessScore := rlm.calculateEffectivenessScore(throttleRate)

	// Identify violation hotspots
	hotspots := rlm.identifyHotspots(recentRequests, cutoff)

	return RateLimitEffectiveness{
		RequestsPerSecond:  requestsPerSecond,
		ThrottleRate:       throttleRate,
		EffectivenessScore: effectivenessScore,
		ViolationHotspots:  hotspots,
		AverageCheckTime:   avgCheckTime,
		P95CheckTime:       p95CheckTime,
		ConfiguredRPS:      float64(rlm.rateLimitConfig.RequestsPerMinute) / 60.0,
		ActualRPS:          requestsPerSecond,
	}
}

// calculateEffectivenessScore calculates the effectiveness score based on throttle rate
func (rlm *RateLimitMetrics) calculateEffectivenessScore(throttleRate float64) float64 {
	if throttleRate <= 1.0 {
		return 100.0
	} else if throttleRate <= 10.0 {
		// Linear decrease from 100% at 1% to 90% at 10%
		return 100.0 - (throttleRate - 1.0) * (10.0 / 9.0)
	} else if throttleRate <= 25.0 {
		// Linear decrease from 90% at 10% to 50% at 25%
		return 90.0 - (throttleRate - 10.0) * (40.0 / 15.0)
	} else {
		score := 50.0 - (throttleRate - 25.0)
		if score < 0 {
			score = 0
		}
		return score
	}
}

// identifyHotspots identifies rate limit violation hotspots
func (rlm *RateLimitMetrics) identifyHotspots(requests []RateLimitRequest, cutoff time.Time) []Hotspot {
	ipViolations := make(map[string]int)
	userViolations := make(map[string]int)
	ipTotalRequests := make(map[string]int)
	userTotalRequests := make(map[string]int)

	for _, req := range requests {
		if !req.Allowed {
			if req.IP != "" {
				ipViolations[req.IP]++
			}
			if req.UserID != "" {
				userViolations[req.UserID]++
			}
		}
		
		if req.IP != "" {
			ipTotalRequests[req.IP]++
		}
		if req.UserID != "" {
			userTotalRequests[req.UserID]++
		}
	}

	var hotspots []Hotspot

	// Add IP hotspots
	for ip, violations := range ipViolations {
		total := ipTotalRequests[ip]
		if total >= 10 { // Only include IPs with significant traffic
			rate := float64(violations) / float64(total) * 100
			if rate > 5.0 { // Only include if violation rate is significant
				hotspots = append(hotspots, Hotspot{
					Identifier:     ip,
					Type:          "ip",
					ViolationCount: violations,
					ViolationRate:  rate,
					LastViolation:  rlm.getLastViolationTime(ip, true),
				})
			}
		}
	}

	// Add user hotspots
	for userID, violations := range userViolations {
		total := userTotalRequests[userID]
		if total >= 5 { // Only include users with some traffic
			rate := float64(violations) / float64(total) * 100
			if rate > 5.0 { // Only include if violation rate is significant
				hotspots = append(hotspots, Hotspot{
					Identifier:     userID,
					Type:          "user",
					ViolationCount: violations,
					ViolationRate:  rate,
					LastViolation:  rlm.getLastViolationTime(userID, false),
				})
			}
		}
	}

	// Sort by violation count (highest first)
	for i := 0; i < len(hotspots); i++ {
		for j := i + 1; j < len(hotspots); j++ {
			if hotspots[i].ViolationCount < hotspots[j].ViolationCount {
				hotspots[i], hotspots[j] = hotspots[j], hotspots[i]
			}
		}
	}

	// Return top 10 hotspots
	if len(hotspots) > 10 {
		hotspots = hotspots[:10]
	}

	return hotspots
}

// getLastViolationTime gets the last violation time for an identifier
func (rlm *RateLimitMetrics) getLastViolationTime(identifier string, isIP bool) time.Time {
	if isIP {
		if tracker, exists := rlm.ipViolations[identifier]; exists {
			return tracker.LastViolation
		}
	} else {
		if tracker, exists := rlm.userViolations[identifier]; exists {
			return tracker.LastViolation
		}
	}
	return time.Time{}
}

// Reset resets all metrics
func (rlm *RateLimitMetrics) Reset() {
	atomic.StoreUint64(&rlm.totalRequests, 0)
	atomic.StoreUint64(&rlm.throttledRequests, 0)
	atomic.StoreUint64(&rlm.allowedRequests, 0)
	atomic.StoreInt64(&rlm.totalCheckDuration, 0)
	atomic.StoreInt64(&rlm.maxCheckDuration, 0)
	atomic.StoreInt64(&rlm.minCheckDuration, 0)

	rlm.mu.Lock()
	rlm.ipViolations = make(map[string]*ViolationTracker)
	rlm.userViolations = make(map[string]*ViolationTracker)
	rlm.requestHistory = make([]RateLimitRequest, 0)
	rlm.mu.Unlock()

	log.Println("Rate limit metrics reset")
}

// SetMaxHistorySize sets the maximum number of requests to keep in history
func (rlm *RateLimitMetrics) SetMaxHistorySize(size int) {
	rlm.mu.Lock()
	defer rlm.mu.Unlock()

	rlm.maxHistorySize = size
	if len(rlm.requestHistory) > size {
		// Trim history to new size
		rlm.requestHistory = rlm.requestHistory[len(rlm.requestHistory)-size:]
	}
}

// SetMaxViolationsMap sets the maximum number of entries in violations maps
func (rlm *RateLimitMetrics) SetMaxViolationsMap(size int) {
	rlm.mu.Lock()
	defer rlm.mu.Unlock()

	rlm.maxViolationsMap = size
	rlm.cleanupOldViolations()
}