package utils

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// ConcurrencyManager manages thread safety across components
type ConcurrencyManager struct {
	// Component-specific locks
	abiLock         sync.RWMutex
	strategyLock    sync.RWMutex
	executionLock   sync.RWMutex
	cacheLock       sync.RWMutex
	
	// Atomic counters for statistics
	activeOperations int64
	totalOperations  int64
	errorCount       int64
	
	// Rate limiting
	rateLimiter chan struct{}
	maxConcurrent int
	
	// Context for graceful shutdown
	ctx context.Context
	cancel context.CancelFunc
	
	// Shutdown coordination
	shutdownOnce sync.Once
	wg           sync.WaitGroup
}

// NewConcurrencyManager creates a new concurrency manager
func NewConcurrencyManager(maxConcurrent int) *ConcurrencyManager {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &ConcurrencyManager{
		rateLimiter:   make(chan struct{}, maxConcurrent),
		maxConcurrent: maxConcurrent,
		ctx:           ctx,
		cancel:        cancel,
	}
}

// AcquireOperation acquires a slot for operation execution
func (cm *ConcurrencyManager) AcquireOperation(ctx context.Context) error {
	select {
	case cm.rateLimiter <- struct{}{}:
		atomic.AddInt64(&cm.activeOperations, 1)
		atomic.AddInt64(&cm.totalOperations, 1)
		cm.wg.Add(1)
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-cm.ctx.Done():
		return cm.ctx.Err()
	}
}

// ReleaseOperation releases a slot after operation completion
func (cm *ConcurrencyManager) ReleaseOperation() {
	<-cm.rateLimiter
	atomic.AddInt64(&cm.activeOperations, -1)
	cm.wg.Done()
}

// WithABILock executes function with ABI lock protection
func (cm *ConcurrencyManager) WithABILock(fn func() error, readOnly bool) error {
	if readOnly {
		cm.abiLock.RLock()
		defer cm.abiLock.RUnlock()
	} else {
		cm.abiLock.Lock()
		defer cm.abiLock.Unlock()
	}
	
	err := fn()
	if err != nil {
		atomic.AddInt64(&cm.errorCount, 1)
	}
	return err
}

// WithStrategyLock executes function with strategy lock protection
func (cm *ConcurrencyManager) WithStrategyLock(fn func() error, readOnly bool) error {
	if readOnly {
		cm.strategyLock.RLock()
		defer cm.strategyLock.RUnlock()
	} else {
		cm.strategyLock.Lock()
		defer cm.strategyLock.Unlock()
	}
	
	err := fn()
	if err != nil {
		atomic.AddInt64(&cm.errorCount, 1)
	}
	return err
}

// WithExecutionLock executes function with execution lock protection
func (cm *ConcurrencyManager) WithExecutionLock(fn func() error, readOnly bool) error {
	if readOnly {
		cm.executionLock.RLock()
		defer cm.executionLock.RUnlock()
	} else {
		cm.executionLock.Lock()
		defer cm.executionLock.Unlock()
	}
	
	err := fn()
	if err != nil {
		atomic.AddInt64(&cm.errorCount, 1)
	}
	return err
}

// WithCacheLock executes function with cache lock protection
func (cm *ConcurrencyManager) WithCacheLock(fn func() error, readOnly bool) error {
	if readOnly {
		cm.cacheLock.RLock()
		defer cm.cacheLock.RUnlock()
	} else {
		cm.cacheLock.Lock()
		defer cm.cacheLock.Unlock()
	}
	
	err := fn()
	if err != nil {
		atomic.AddInt64(&cm.errorCount, 1)
	}
	return err
}

// ExecuteWithTimeout executes function with timeout and concurrency control
func (cm *ConcurrencyManager) ExecuteWithTimeout(
	fn func() error,
	timeout time.Duration,
) error {
	ctx, cancel := context.WithTimeout(cm.ctx, timeout)
	defer cancel()
	
	// Acquire operation slot
	if err := cm.AcquireOperation(ctx); err != nil {
		return NewError(ErrorTypeTimeout, "Failed to acquire operation slot").
			AddContext("timeout", timeout).
			AddContext("active_operations", atomic.LoadInt64(&cm.activeOperations))
	}
	defer cm.ReleaseOperation()
	
	// Execute function in goroutine
	errChan := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				errChan <- NewError(ErrorTypeExecution, "Panic during execution").
					AddContext("panic", r)
			}
		}()
		errChan <- fn()
	}()
	
	// Wait for completion or timeout
	select {
	case err := <-errChan:
		return err
	case <-ctx.Done():
		return WrapError(ErrorTypeTimeout, "Operation timed out", ctx.Err()).
			AddContext("timeout", timeout).
			AddContext("recoverable", false)
	}
}

// GetStats returns concurrency statistics
func (cm *ConcurrencyManager) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"active_operations": atomic.LoadInt64(&cm.activeOperations),
		"total_operations":  atomic.LoadInt64(&cm.totalOperations),
		"error_count":       atomic.LoadInt64(&cm.errorCount),
		"max_concurrent":    cm.maxConcurrent,
		"success_rate":      cm.calculateSuccessRate(),
	}
}

// calculateSuccessRate calculates the success rate
func (cm *ConcurrencyManager) calculateSuccessRate() float64 {
	total := atomic.LoadInt64(&cm.totalOperations)
	errors := atomic.LoadInt64(&cm.errorCount)
	
	if total == 0 {
		return 1.0
	}
	
	return float64(total-errors) / float64(total)
}

// Shutdown gracefully shuts down the concurrency manager
func (cm *ConcurrencyManager) Shutdown(timeout time.Duration) error {
	var shutdownErr error
	
	cm.shutdownOnce.Do(func() {
		// Cancel context to stop new operations
		cm.cancel()
		
		// Wait for existing operations with timeout
		done := make(chan struct{})
		go func() {
			cm.wg.Wait()
			close(done)
		}()
		
		select {
		case <-done:
			shutdownErr = nil
		case <-time.After(timeout):
			shutdownErr = NewError(ErrorTypeTimeout, "Shutdown timeout exceeded").
				AddContext("timeout", timeout).
				AddContext("active_operations", atomic.LoadInt64(&cm.activeOperations))
		}
	})
	
	return shutdownErr
}

// SafeCache provides thread-safe caching with TTL support
type SafeCache struct {
	mu    sync.RWMutex
	cache map[string]*CacheEntry
	ttl   time.Duration
}

// CacheEntry represents a cache entry with expiration
type CacheEntry struct {
	Value     interface{}
	ExpiresAt time.Time
}

// NewSafeCache creates a new thread-safe cache
func NewSafeCache(ttl time.Duration) *SafeCache {
	return &SafeCache{
		cache: make(map[string]*CacheEntry),
		ttl:   ttl,
	}
}

// Get retrieves a value from cache
func (sc *SafeCache) Get(key string) (interface{}, bool) {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	
	entry, exists := sc.cache[key]
	if !exists {
		return nil, false
	}
	
	// Check expiration
	if time.Now().After(entry.ExpiresAt) {
		// Entry expired, but don't delete here to avoid lock upgrade
		return nil, false
	}
	
	return entry.Value, true
}

// Set stores a value in cache
func (sc *SafeCache) Set(key string, value interface{}) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	
	sc.cache[key] = &CacheEntry{
		Value:     value,
		ExpiresAt: time.Now().Add(sc.ttl),
	}
}

// Delete removes a value from cache
func (sc *SafeCache) Delete(key string) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	
	delete(sc.cache, key)
}

// Clear removes all entries from cache
func (sc *SafeCache) Clear() {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	
	sc.cache = make(map[string]*CacheEntry)
}

// CleanExpired removes expired entries
func (sc *SafeCache) CleanExpired() {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	
	now := time.Now()
	for key, entry := range sc.cache {
		if now.After(entry.ExpiresAt) {
			delete(sc.cache, key)
		}
	}
}

// Size returns the number of entries in cache
func (sc *SafeCache) Size() int {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	
	return len(sc.cache)
}

// Keys returns all keys in cache
func (sc *SafeCache) Keys() []string {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	
	keys := make([]string, 0, len(sc.cache))
	for key := range sc.cache {
		keys = append(keys, key)
	}
	return keys
}

// WorkerPool provides a thread-safe worker pool for executing tasks
type WorkerPool struct {
	workers    int
	taskQueue  chan func()
	resultChan chan WorkerTaskResult
	wg         sync.WaitGroup
	ctx        context.Context
	cancel     context.CancelFunc
	started    int32
}

// WorkerTaskResult represents the result of a worker task
type WorkerTaskResult struct {
	TaskID string
	Result interface{}
	Error  error
	Duration time.Duration
}

// NewWorkerPool creates a new worker pool
func NewWorkerPool(workers int, bufferSize int) *WorkerPool {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &WorkerPool{
		workers:    workers,
		taskQueue:  make(chan func(), bufferSize),
		resultChan: make(chan WorkerTaskResult, bufferSize),
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Start starts the worker pool
func (wp *WorkerPool) Start() {
	if !atomic.CompareAndSwapInt32(&wp.started, 0, 1) {
		return // Already started
	}
	
	for i := 0; i < wp.workers; i++ {
		wp.wg.Add(1)
		go wp.worker(i)
	}
}

// worker is the worker goroutine
func (wp *WorkerPool) worker(id int) {
	defer wp.wg.Done()
	
	for {
		select {
		case task := <-wp.taskQueue:
			task()
		case <-wp.ctx.Done():
			return
		}
	}
}

// Submit submits a task to the worker pool
func (wp *WorkerPool) Submit(taskID string, task func() (interface{}, error)) {
	wp.taskQueue <- func() {
		start := time.Now()
		result, err := task()
		
		wp.resultChan <- WorkerTaskResult{
			TaskID:   taskID,
			Result:   result,
			Error:    err,
			Duration: time.Since(start),
		}
	}
}

// Results returns the result channel
func (wp *WorkerPool) Results() <-chan WorkerTaskResult {
	return wp.resultChan
}

// Stop stops the worker pool
func (wp *WorkerPool) Stop() {
	wp.cancel()
	wp.wg.Wait()
	close(wp.taskQueue)
	close(wp.resultChan)
}