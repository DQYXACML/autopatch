package utils

import (
	"fmt"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

func TestEnhancedErrorHandling(t *testing.T) {
	// Test basic error creation
	t.Run("BasicError", func(t *testing.T) {
		err := NewError(ErrorTypeConfig, "Test configuration error")
		if err.Type != ErrorTypeConfig {
			t.Errorf("Expected error type %s, got %s", ErrorTypeConfig, err.Type)
		}
		if err.Message != "Test configuration error" {
			t.Errorf("Expected message 'Test configuration error', got '%s'", err.Message)
		}
	})

	// Test error wrapping
	t.Run("ErrorWrapping", func(t *testing.T) {
		originalErr := fmt.Errorf("original error")
		wrappedErr := WrapError(ErrorTypeNetwork, "Network issue", originalErr)
		
		if wrappedErr.OriginalErr != originalErr {
			t.Error("Original error not preserved")
		}
		
		if wrappedErr.Unwrap() != originalErr {
			t.Error("Unwrap() doesn't return original error")
		}
	})

	// Test context addition
	t.Run("ContextAddition", func(t *testing.T) {
		err := NewError(ErrorTypeAPI, "API error").
			AddContext("status_code", 404).
			AddContext("endpoint", "/api/test")
		
		if err.Context["status_code"] != 404 {
			t.Error("Status code context not set correctly")
		}
		
		if err.Context["endpoint"] != "/api/test" {
			t.Error("Endpoint context not set correctly")
		}
	})

	// Test predefined error constructors
	t.Run("NetworkError", func(t *testing.T) {
		originalErr := fmt.Errorf("connection refused")
		err := NewNetworkError("Connection failed", originalErr)
		
		if err.Type != ErrorTypeNetwork {
			t.Error("Network error type not set correctly")
		}
		
		if !err.Context["recoverable"].(bool) {
			t.Error("Network error should be recoverable")
		}
	})

	// Test API error with different status codes
	t.Run("APIError", func(t *testing.T) {
		// Test rate limiting (429)
		rateLimitErr := NewAPIError("Rate limited", 429, nil)
		if !rateLimitErr.Context["recoverable"].(bool) {
			t.Error("Rate limit error should be recoverable")
		}
		
		// Test server error (500)
		serverErr := NewAPIError("Server error", 500, nil)
		if !serverErr.Context["recoverable"].(bool) {
			t.Error("Server error should be recoverable")
		}
		
		// Test client error (400)
		clientErr := NewAPIError("Bad request", 400, nil)
		if clientErr.Context["recoverable"].(bool) {
			t.Error("Client error should not be recoverable")
		}
	})

	// Test contract error
	t.Run("ContractError", func(t *testing.T) {
		addr := common.HexToAddress("0x1234567890123456789012345678901234567890")
		originalErr := fmt.Errorf("contract not found")
		
		err := NewContractError("Contract verification failed", addr, originalErr)
		
		if err.Type != ErrorTypeContract {
			t.Error("Contract error type not set correctly")
		}
		
		if err.Context["contract_address"] != addr.Hex() {
			t.Error("Contract address context not set correctly")
		}
	})
}

func TestErrorRecovery(t *testing.T) {
	// Test retry logic
	t.Run("RetryLogic", func(t *testing.T) {
		recovery := NewErrorRecovery()
		recovery.MaxRetries = 2
		recovery.BaseDelay = time.Millisecond // Fast for testing
		
		attempts := 0
		err := recovery.RetryWithRecovery(func() error {
			attempts++
			if attempts < 3 {
				return NewNetworkError("Temporary failure", nil)
			}
			return nil // Success on third attempt
		})
		
		if err != nil {
			t.Errorf("Expected success after retries, got error: %v", err)
		}
		
		if attempts != 3 {
			t.Errorf("Expected 3 attempts, got %d", attempts)
		}
	})

	// Test non-retryable error
	t.Run("NonRetryableError", func(t *testing.T) {
		recovery := NewErrorRecovery()
		recovery.MaxRetries = 2
		
		attempts := 0
		err := recovery.RetryWithRecovery(func() error {
			attempts++
			return NewConfigError("Invalid configuration", "field")
		})
		
		if err == nil {
			t.Error("Expected error to persist")
		}
		
		if attempts != 1 {
			t.Errorf("Expected 1 attempt for non-retryable error, got %d", attempts)
		}
	})

	// Test max retries exceeded
	t.Run("MaxRetriesExceeded", func(t *testing.T) {
		recovery := NewErrorRecovery()
		recovery.MaxRetries = 2
		recovery.BaseDelay = time.Millisecond
		
		attempts := 0
		err := recovery.RetryWithRecovery(func() error {
			attempts++
			return NewNetworkError("Persistent failure", nil)
		})
		
		if err == nil {
			t.Error("Expected error after max retries exceeded")
		}
		
		if attempts != 3 { // Initial attempt + 2 retries
			t.Errorf("Expected 3 attempts, got %d", attempts)
		}
	})

	// Test exponential backoff
	t.Run("ExponentialBackoff", func(t *testing.T) {
		recovery := NewErrorRecovery()
		recovery.BaseDelay = 10 * time.Millisecond
		recovery.MaxDelay = 100 * time.Millisecond
		
		// Test delay calculation
		delay1 := recovery.GetRetryDelay(0)
		delay2 := recovery.GetRetryDelay(1)
		delay3 := recovery.GetRetryDelay(2)
		
		if delay1 != 10*time.Millisecond {
			t.Errorf("Expected first delay 10ms, got %v", delay1)
		}
		
		if delay2 != 20*time.Millisecond {
			t.Errorf("Expected second delay 20ms, got %v", delay2)
		}
		
		if delay3 != 40*time.Millisecond {
			t.Errorf("Expected third delay 40ms, got %v", delay3)
		}
		
		// Test max delay cap
		delay10 := recovery.GetRetryDelay(10)
		if delay10 != recovery.MaxDelay {
			t.Errorf("Expected max delay %v, got %v", recovery.MaxDelay, delay10)
		}
	})
}

func TestErrorFormatting(t *testing.T) {
	// Test error string formatting
	t.Run("ErrorString", func(t *testing.T) {
		originalErr := fmt.Errorf("original error")
		err := WrapError(ErrorTypeNetwork, "Network failure", originalErr)
		
		expected := "[network] Network failure: original error"
		if err.Error() != expected {
			t.Errorf("Expected error string '%s', got '%s'", expected, err.Error())
		}
	})

	// Test error string without original error
	t.Run("ErrorStringNoOriginal", func(t *testing.T) {
		err := NewError(ErrorTypeConfig, "Configuration missing")
		
		expected := "[config] Configuration missing"
		if err.Error() != expected {
			t.Errorf("Expected error string '%s', got '%s'", expected, err.Error())
		}
	})
}