package utils

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestConcurrencyManager(t *testing.T) {
	// Test basic concurrency control
	t.Run("BasicConcurrencyControl", func(t *testing.T) {
		cm := NewConcurrencyManager(3) // Max 3 concurrent operations
		
		var activeCount int32
		var maxActive int32
		var mu sync.Mutex
		
		// Start 10 operations concurrently
		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()
				
				err := cm.AcquireOperation(ctx)
				if err != nil {
					t.Errorf("Failed to acquire operation %d: %v", id, err)
					return
				}
				defer cm.ReleaseOperation()
				
				// Track maximum concurrent operations
				mu.Lock()
				activeCount++
				if activeCount > maxActive {
					maxActive = activeCount
				}
				current := activeCount
				mu.Unlock()
				
				// Simulate work
				time.Sleep(100 * time.Millisecond)
				
				mu.Lock()
				activeCount--
				mu.Unlock()
				
				if current > 3 {
					t.Errorf("Exceeded maximum concurrent operations: %d", current)
				}
			}(i)
		}
		
		wg.Wait()
		
		if maxActive > 3 {
			t.Errorf("Maximum concurrent operations exceeded: %d", maxActive)
		}
		
		stats := cm.GetStats()
		if stats["total_operations"].(int64) != 10 {
			t.Errorf("Expected 10 total operations, got %d", stats["total_operations"])
		}
	})

	// Test timeout handling
	t.Run("TimeoutHandling", func(t *testing.T) {
		cm := NewConcurrencyManager(1) // Only 1 concurrent operation
		
		// Occupy the single slot
		ctx1, cancel1 := context.WithTimeout(context.Background(), time.Second)
		defer cancel1()
		
		err := cm.AcquireOperation(ctx1)
		if err != nil {
			t.Fatalf("Failed to acquire first operation: %v", err)
		}
		
		// Try to acquire another slot with short timeout
		ctx2, cancel2 := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel2()
		
		err = cm.AcquireOperation(ctx2)
		if err == nil {
			cm.ReleaseOperation()
			t.Error("Expected timeout error, but operation succeeded")
		}
		
		cm.ReleaseOperation() // Release first slot
	})

	// Test graceful shutdown
	t.Run("GracefulShutdown", func(t *testing.T) {
		cm := NewConcurrencyManager(5)
		
		// Start a long-running operation
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			
			err := cm.AcquireOperation(ctx)
			if err != nil {
				return
			}
			defer cm.ReleaseOperation()
			
			time.Sleep(200 * time.Millisecond)
		}()
		
		// Give operation time to start
		time.Sleep(50 * time.Millisecond)
		
		// Shutdown with sufficient timeout
		err := cm.Shutdown(500 * time.Millisecond)
		if err != nil {
			t.Errorf("Shutdown failed: %v", err)
		}
		
		wg.Wait()
	})
}

func TestSafeCache(t *testing.T) {
	// Test basic cache operations
	t.Run("BasicOperations", func(t *testing.T) {
		cache := NewSafeCache(100 * time.Millisecond)
		
		// Test Set and Get
		cache.Set("key1", "value1")
		value, exists := cache.Get("key1")
		if !exists || value != "value1" {
			t.Errorf("Expected 'value1', got %v (exists: %v)", value, exists)
		}
		
		// Test non-existent key
		_, exists = cache.Get("nonexistent")
		if exists {
			t.Error("Expected key to not exist")
		}
		
		// Test Delete
		cache.Delete("key1")
		_, exists = cache.Get("key1")
		if exists {
			t.Error("Expected key to be deleted")
		}
	})

	// Test TTL expiration
	t.Run("TTLExpiration", func(t *testing.T) {
		cache := NewSafeCache(50 * time.Millisecond)
		
		cache.Set("ttl_key", "ttl_value")
		
		// Should exist immediately
		_, exists := cache.Get("ttl_key")
		if !exists {
			t.Error("Key should exist immediately after set")
		}
		
		// Wait for expiration
		time.Sleep(100 * time.Millisecond)
		
		// Should be expired
		_, exists = cache.Get("ttl_key")
		if exists {
			t.Error("Key should be expired")
		}
	})

	// Test concurrent access
	t.Run("ConcurrentAccess", func(t *testing.T) {
		cache := NewSafeCache(time.Minute)
		
		var wg sync.WaitGroup
		// Start multiple goroutines for concurrent access
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				key := fmt.Sprintf("key_%d", id)
				value := fmt.Sprintf("value_%d", id)
				
				// Set value
				cache.Set(key, value)
				
				// Get value
				retrieved, exists := cache.Get(key)
				if !exists || retrieved != value {
					t.Errorf("Concurrent access failed for %s", key)
				}
			}(i)
		}
		
		wg.Wait()
		
		// Verify all keys exist
		if cache.Size() != 100 {
			t.Errorf("Expected 100 keys, got %d", cache.Size())
		}
	})

	// Test Clear and CleanExpired
	t.Run("ClearOperations", func(t *testing.T) {
		cache := NewSafeCache(time.Minute)
		
		// Add some entries
		for i := 0; i < 10; i++ {
			cache.Set(fmt.Sprintf("key_%d", i), fmt.Sprintf("value_%d", i))
		}
		
		if cache.Size() != 10 {
			t.Errorf("Expected 10 entries, got %d", cache.Size())
		}
		
		// Clear all
		cache.Clear()
		if cache.Size() != 0 {
			t.Errorf("Expected 0 entries after clear, got %d", cache.Size())
		}
		
		// Test CleanExpired
		cache2 := NewSafeCache(50 * time.Millisecond)
		cache2.Set("temp1", "value1")
		cache2.Set("temp2", "value2")
		
		time.Sleep(100 * time.Millisecond) // Wait for expiration
		
		cache2.Set("permanent", "value3") // Add non-expired entry
		
		cache2.CleanExpired()
		
		// Only permanent entry should remain
		if cache2.Size() != 1 {
			t.Errorf("Expected 1 entry after cleaning expired, got %d", cache2.Size())
		}
		
		_, exists := cache2.Get("permanent")
		if !exists {
			t.Error("Permanent entry should still exist")
		}
	})
}

func TestWorkerPool(t *testing.T) {
	// Test basic worker pool functionality
	t.Run("BasicWorkerPool", func(t *testing.T) {
		pool := NewWorkerPool(3, 10)
		pool.Start()
		defer pool.Stop()
		
		// Submit tasks
		taskCount := 10
		for i := 0; i < taskCount; i++ {
			taskID := fmt.Sprintf("task_%d", i)
			value := i
			pool.Submit(taskID, func() (interface{}, error) {
				time.Sleep(10 * time.Millisecond)
				return value * 2, nil
			})
		}
		
		// Collect results
		results := make(map[string]interface{})
		timeout := time.After(time.Second)
		
		for len(results) < taskCount {
			select {
			case result := <-pool.Results():
				if result.Error != nil {
					t.Errorf("Task %s failed: %v", result.TaskID, result.Error)
				} else {
					results[result.TaskID] = result.Result
				}
			case <-timeout:
				t.Fatal("Timeout waiting for results")
			}
		}
		
		// Verify all results
		if len(results) != taskCount {
			t.Errorf("Expected %d results, got %d", taskCount, len(results))
		}
	})

	// Test error handling
	t.Run("ErrorHandling", func(t *testing.T) {
		pool := NewWorkerPool(2, 5)
		pool.Start()
		defer pool.Stop()
		
		// Submit a task that will fail
		pool.Submit("error_task", func() (interface{}, error) {
			return nil, fmt.Errorf("intentional error")
		})
		
		// Wait for result
		select {
		case result := <-pool.Results():
			if result.Error == nil {
				t.Error("Expected error but got none")
			}
			if result.Error.Error() != "intentional error" {
				t.Errorf("Expected 'intentional error', got '%v'", result.Error)
			}
		case <-time.After(time.Second):
			t.Fatal("Timeout waiting for error result")
		}
	})
}