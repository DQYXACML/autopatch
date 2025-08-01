package utils

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// MetricsCollector collects and manages performance metrics
type MetricsCollector struct {
	mu sync.RWMutex
	
	// Operation counters
	mutationAttempts    int64
	mutationSuccesses   int64
	mutationFailures    int64
	abiCacheHits        int64
	abiCacheMisses      int64
	executionAttempts   int64
	executionSuccesses  int64
	
	// Timing metrics
	avgMutationTime     time.Duration
	avgExecutionTime    time.Duration
	avgABIFetchTime     time.Duration
	
	// Performance buckets for latency distribution
	mutationLatencyBuckets map[string]int64
	executionLatencyBuckets map[string]int64
	
	// Resource usage metrics
	peakMemoryUsage     int64
	currentGoroutines   int64
	
	// Strategy performance tracking
	strategyMetrics     map[string]*StrategyMetrics
	
	// Rate metrics
	mutationsPerSecond  float64
	successRate         float64
	
	// System metrics
	startTime           time.Time
	lastResetTime       time.Time
}

// StrategyMetrics tracks performance for individual mutation strategies
type StrategyMetrics struct {
	Name            string        `json:"name"`
	Attempts        int64         `json:"attempts"`
	Successes       int64         `json:"successes"`
	AvgSimilarity   float64       `json:"avgSimilarity"`
	AvgExecutionTime time.Duration `json:"avgExecutionTime"`
	LastUsed        time.Time     `json:"lastUsed"`
	SuccessRate     float64       `json:"successRate"`
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector() *MetricsCollector {
	now := time.Now()
	return &MetricsCollector{
		mutationLatencyBuckets:  make(map[string]int64),
		executionLatencyBuckets: make(map[string]int64),
		strategyMetrics:         make(map[string]*StrategyMetrics),
		startTime:              now,
		lastResetTime:          now,
	}
}

// RecordMutationAttempt records a mutation attempt
func (mc *MetricsCollector) RecordMutationAttempt(duration time.Duration, success bool, strategy string) {
	atomic.AddInt64(&mc.mutationAttempts, 1)
	
	if success {
		atomic.AddInt64(&mc.mutationSuccesses, 1)
	} else {
		atomic.AddInt64(&mc.mutationFailures, 1)
	}
	
	// Update timing metrics
	mc.updateAverageTime(&mc.avgMutationTime, duration)
	
	// Update latency buckets
	mc.updateLatencyBucket(mc.mutationLatencyBuckets, duration)
	
	// Update strategy metrics
	mc.updateStrategyMetrics(strategy, duration, success, 0.0)
	
	// Update rate metrics
	mc.updateRateMetrics()
}

// RecordExecutionAttempt records an execution attempt
func (mc *MetricsCollector) RecordExecutionAttempt(duration time.Duration, success bool, similarity float64) {
	atomic.AddInt64(&mc.executionAttempts, 1)
	
	if success {
		atomic.AddInt64(&mc.executionSuccesses, 1)
	}
	
	// Update timing metrics
	mc.updateAverageTime(&mc.avgExecutionTime, duration)
	
	// Update latency buckets
	mc.updateLatencyBucket(mc.executionLatencyBuckets, duration)
}

// RecordABIOperation records ABI cache operations
func (mc *MetricsCollector) RecordABIOperation(duration time.Duration, cacheHit bool) {
	if cacheHit {
		atomic.AddInt64(&mc.abiCacheHits, 1)
	} else {
		atomic.AddInt64(&mc.abiCacheMisses, 1)
		mc.updateAverageTime(&mc.avgABIFetchTime, duration)
	}
}

// UpdateResourceUsage updates resource usage metrics
func (mc *MetricsCollector) UpdateResourceUsage(memoryUsage int64, goroutines int64) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	
	if memoryUsage > mc.peakMemoryUsage {
		mc.peakMemoryUsage = memoryUsage
	}
	
	atomic.StoreInt64(&mc.currentGoroutines, goroutines)
}

// updateAverageTime updates running average of duration
func (mc *MetricsCollector) updateAverageTime(avg *time.Duration, newDuration time.Duration) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	
	// Simple exponential moving average with alpha=0.1
	*avg = time.Duration(float64(*avg)*0.9 + float64(newDuration)*0.1)
}

// updateLatencyBucket updates latency distribution buckets
func (mc *MetricsCollector) updateLatencyBucket(buckets map[string]int64, duration time.Duration) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	
	ms := duration.Milliseconds()
	var bucket string
	
	switch {
	case ms < 10:
		bucket = "<10ms"
	case ms < 50:
		bucket = "10-50ms"
	case ms < 100:
		bucket = "50-100ms"
	case ms < 500:
		bucket = "100-500ms"
	case ms < 1000:
		bucket = "500ms-1s"
	case ms < 5000:
		bucket = "1-5s"
	default:
		bucket = ">5s"
	}
	
	buckets[bucket]++
}

// updateStrategyMetrics updates strategy-specific metrics
func (mc *MetricsCollector) updateStrategyMetrics(strategy string, duration time.Duration, success bool, similarity float64) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	
	metrics, exists := mc.strategyMetrics[strategy]
	if !exists {
		metrics = &StrategyMetrics{
			Name: strategy,
		}
		mc.strategyMetrics[strategy] = metrics
	}
	
	metrics.Attempts++
	if success {
		metrics.Successes++
	}
	
	// Update averages
	metrics.AvgExecutionTime = time.Duration(float64(metrics.AvgExecutionTime)*0.9 + float64(duration)*0.1)
	metrics.AvgSimilarity = metrics.AvgSimilarity*0.9 + similarity*0.1
	metrics.SuccessRate = float64(metrics.Successes) / float64(metrics.Attempts)
	metrics.LastUsed = time.Now()
}

// updateRateMetrics updates rate-based metrics
func (mc *MetricsCollector) updateRateMetrics() {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	
	elapsed := time.Since(mc.lastResetTime).Seconds()
	if elapsed > 0 {
		attempts := atomic.LoadInt64(&mc.mutationAttempts)
		successes := atomic.LoadInt64(&mc.mutationSuccesses)
		
		mc.mutationsPerSecond = float64(attempts) / elapsed
		if attempts > 0 {
			mc.successRate = float64(successes) / float64(attempts)
		}
	}
}

// GetMetrics returns a comprehensive metrics snapshot
func (mc *MetricsCollector) GetMetrics() *MetricsSnapshot {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	
	// Create deep copy of strategy metrics
	strategyMetricsCopy := make(map[string]*StrategyMetrics)
	for name, metrics := range mc.strategyMetrics {
		strategyMetricsCopy[name] = &StrategyMetrics{
			Name:             metrics.Name,
			Attempts:         metrics.Attempts,
			Successes:        metrics.Successes,
			AvgSimilarity:    metrics.AvgSimilarity,
			AvgExecutionTime: metrics.AvgExecutionTime,
			LastUsed:         metrics.LastUsed,
			SuccessRate:      metrics.SuccessRate,
		}
	}
	
	// Create deep copy of latency buckets
	mutationBucketsCopy := make(map[string]int64)
	for bucket, count := range mc.mutationLatencyBuckets {
		mutationBucketsCopy[bucket] = count
	}
	
	executionBucketsCopy := make(map[string]int64)
	for bucket, count := range mc.executionLatencyBuckets {
		executionBucketsCopy[bucket] = count
	}
	
	return &MetricsSnapshot{
		// Operation counters
		MutationAttempts:   atomic.LoadInt64(&mc.mutationAttempts),
		MutationSuccesses:  atomic.LoadInt64(&mc.mutationSuccesses),
		MutationFailures:   atomic.LoadInt64(&mc.mutationFailures),
		ABICacheHits:       atomic.LoadInt64(&mc.abiCacheHits),
		ABICacheMisses:     atomic.LoadInt64(&mc.abiCacheMisses),
		ExecutionAttempts:  atomic.LoadInt64(&mc.executionAttempts),
		ExecutionSuccesses: atomic.LoadInt64(&mc.executionSuccesses),
		
		// Timing metrics
		AvgMutationTime:  mc.avgMutationTime,
		AvgExecutionTime: mc.avgExecutionTime,
		AvgABIFetchTime:  mc.avgABIFetchTime,
		
		// Latency distributions
		MutationLatencyBuckets:  mutationBucketsCopy,
		ExecutionLatencyBuckets: executionBucketsCopy,
		
		// Resource usage
		PeakMemoryUsage:   mc.peakMemoryUsage,
		CurrentGoroutines: atomic.LoadInt64(&mc.currentGoroutines),
		
		// Strategy metrics
		StrategyMetrics: strategyMetricsCopy,
		
		// Rate metrics
		MutationsPerSecond: mc.mutationsPerSecond,
		SuccessRate:        mc.successRate,
		
		// Time metrics
		Uptime:        time.Since(mc.startTime),
		LastResetTime: mc.lastResetTime,
		Timestamp:     time.Now(),
	}
}

// MetricsSnapshot represents a point-in-time snapshot of metrics
type MetricsSnapshot struct {
	// Operation counters
	MutationAttempts   int64 `json:"mutationAttempts"`
	MutationSuccesses  int64 `json:"mutationSuccesses"`
	MutationFailures   int64 `json:"mutationFailures"`
	ABICacheHits       int64 `json:"abiCacheHits"`
	ABICacheMisses     int64 `json:"abiCacheMisses"`
	ExecutionAttempts  int64 `json:"executionAttempts"`
	ExecutionSuccesses int64 `json:"executionSuccesses"`
	
	// Timing metrics
	AvgMutationTime  time.Duration `json:"avgMutationTime"`
	AvgExecutionTime time.Duration `json:"avgExecutionTime"`
	AvgABIFetchTime  time.Duration `json:"avgABIFetchTime"`
	
	// Latency distributions
	MutationLatencyBuckets  map[string]int64 `json:"mutationLatencyBuckets"`
	ExecutionLatencyBuckets map[string]int64 `json:"executionLatencyBuckets"`
	
	// Resource usage
	PeakMemoryUsage   int64 `json:"peakMemoryUsage"`
	CurrentGoroutines int64 `json:"currentGoroutines"`
	
	// Strategy metrics
	StrategyMetrics map[string]*StrategyMetrics `json:"strategyMetrics"`
	
	// Rate metrics
	MutationsPerSecond float64 `json:"mutationsPerSecond"`
	SuccessRate        float64 `json:"successRate"`
	
	// Time metrics
	Uptime        time.Duration `json:"uptime"`
	LastResetTime time.Time     `json:"lastResetTime"`
	Timestamp     time.Time     `json:"timestamp"`
}

// ResetCounters resets all counters (useful for benchmarking specific periods)
func (mc *MetricsCollector) ResetCounters() {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	
	atomic.StoreInt64(&mc.mutationAttempts, 0)
	atomic.StoreInt64(&mc.mutationSuccesses, 0)
	atomic.StoreInt64(&mc.mutationFailures, 0)
	atomic.StoreInt64(&mc.abiCacheHits, 0)
	atomic.StoreInt64(&mc.abiCacheMisses, 0)
	atomic.StoreInt64(&mc.executionAttempts, 0)
	atomic.StoreInt64(&mc.executionSuccesses, 0)
	
	// Reset latency buckets
	mc.mutationLatencyBuckets = make(map[string]int64)
	mc.executionLatencyBuckets = make(map[string]int64)
	
	// Reset strategy metrics
	mc.strategyMetrics = make(map[string]*StrategyMetrics)
	
	mc.lastResetTime = time.Now()
}

// GetSummaryReport generates a human-readable performance summary
func (ms *MetricsSnapshot) GetSummaryReport() string {
	report := fmt.Sprintf("=== Performance Metrics Summary ===\n")
	report += fmt.Sprintf("Timestamp: %s\n", ms.Timestamp.Format("2006-01-02 15:04:05"))
	report += fmt.Sprintf("Uptime: %v\n\n", ms.Uptime)
	
	// Operation summary
	report += fmt.Sprintf("📊 Operations:\n")
	report += fmt.Sprintf("  Mutation Attempts: %d\n", ms.MutationAttempts)
	report += fmt.Sprintf("  Mutation Successes: %d\n", ms.MutationSuccesses)
	report += fmt.Sprintf("  Success Rate: %.2f%%\n", ms.SuccessRate*100)
	report += fmt.Sprintf("  Mutations/sec: %.2f\n\n", ms.MutationsPerSecond)
	
	// Timing summary
	report += fmt.Sprintf("⏱️  Timing:\n")
	report += fmt.Sprintf("  Avg Mutation Time: %v\n", ms.AvgMutationTime)
	report += fmt.Sprintf("  Avg Execution Time: %v\n", ms.AvgExecutionTime)
	report += fmt.Sprintf("  Avg ABI Fetch Time: %v\n\n", ms.AvgABIFetchTime)
	
	// Cache performance
	totalCacheOps := ms.ABICacheHits + ms.ABICacheMisses
	cacheHitRate := 0.0
	if totalCacheOps > 0 {
		cacheHitRate = float64(ms.ABICacheHits) / float64(totalCacheOps) * 100
	}
	report += fmt.Sprintf("💾 Cache Performance:\n")
	report += fmt.Sprintf("  Cache Hit Rate: %.2f%% (%d hits, %d misses)\n\n", 
		cacheHitRate, ms.ABICacheHits, ms.ABICacheMisses)
	
	// Resource usage
	report += fmt.Sprintf("🖥️  Resources:\n")
	report += fmt.Sprintf("  Peak Memory Usage: %d bytes\n", ms.PeakMemoryUsage)
	report += fmt.Sprintf("  Current Goroutines: %d\n\n", ms.CurrentGoroutines)
	
	// Top strategies
	report += fmt.Sprintf("🎯 Top Performing Strategies:\n")
	topStrategies := ms.GetTopStrategies(5)
	for i, strategy := range topStrategies {
		report += fmt.Sprintf("  %d. %s: %.2f%% success (%d/%d), avg similarity: %.3f\n",
			i+1, strategy.Name, strategy.SuccessRate*100, 
			strategy.Successes, strategy.Attempts, strategy.AvgSimilarity)
	}
	
	// Latency distribution
	report += fmt.Sprintf("\n📈 Mutation Latency Distribution:\n")
	for bucket, count := range ms.MutationLatencyBuckets {
		if count > 0 {
			percentage := float64(count) / float64(ms.MutationAttempts) * 100
			report += fmt.Sprintf("  %s: %d (%.1f%%)\n", bucket, count, percentage)
		}
	}
	
	return report
}

// GetTopStrategies returns the top performing strategies sorted by success rate
func (ms *MetricsSnapshot) GetTopStrategies(limit int) []*StrategyMetrics {
	strategies := make([]*StrategyMetrics, 0, len(ms.StrategyMetrics))
	
	for _, strategy := range ms.StrategyMetrics {
		if strategy.Attempts > 0 { // Only include strategies that have been used
			strategies = append(strategies, strategy)
		}
	}
	
	// Sort by success rate (descending)
	for i := 0; i < len(strategies)-1; i++ {
		for j := i + 1; j < len(strategies); j++ {
			if strategies[i].SuccessRate < strategies[j].SuccessRate {
				strategies[i], strategies[j] = strategies[j], strategies[i]
			}
		}
	}
	
	if limit > 0 && len(strategies) > limit {
		strategies = strategies[:limit]
	}
	
	return strategies
}

// GetPerformanceScore calculates an overall performance score (0-100)
func (ms *MetricsSnapshot) GetPerformanceScore() float64 {
	score := 0.0
	
	// Success rate contributes 40% to the score
	score += ms.SuccessRate * 40
	
	// Cache hit rate contributes 20% to the score
	totalCacheOps := ms.ABICacheHits + ms.ABICacheMisses
	if totalCacheOps > 0 {
		cacheHitRate := float64(ms.ABICacheHits) / float64(totalCacheOps)
		score += cacheHitRate * 20
	}
	
	// Mutations per second contributes 20% (normalized to 0-1 scale, assuming 10/sec is excellent)
	mutationRateScore := ms.MutationsPerSecond / 10.0
	if mutationRateScore > 1.0 {
		mutationRateScore = 1.0
	}
	score += mutationRateScore * 20
	
	// Average execution time contributes 20% (lower is better, normalized)
	if ms.AvgExecutionTime > 0 {
		// Assume 100ms is good, anything under 50ms is excellent
		timeScore := 1.0 - (float64(ms.AvgExecutionTime.Milliseconds()) / 100.0)
		if timeScore < 0 {
			timeScore = 0
		}
		if timeScore > 1 {
			timeScore = 1
		}
		score += timeScore * 20
	}
	
	return score
}