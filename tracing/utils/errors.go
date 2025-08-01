package utils

import (
	"errors"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

// ErrorType represents different categories of errors
type ErrorType string

const (
	// Network and API errors
	ErrorTypeNetwork    ErrorType = "network"
	ErrorTypeAPI        ErrorType = "api"
	ErrorTypeTimeout    ErrorType = "timeout"
	
	// Configuration and setup errors
	ErrorTypeConfig     ErrorType = "config"
	ErrorTypeInit       ErrorType = "initialization"
	ErrorTypeValidation ErrorType = "validation"
	
	// Data processing errors
	ErrorTypeEncoding   ErrorType = "encoding"
	ErrorTypeDecoding   ErrorType = "decoding"
	ErrorTypeParsing    ErrorType = "parsing"
	
	// Business logic errors
	ErrorTypeMutation   ErrorType = "mutation"
	ErrorTypeExecution  ErrorType = "execution"
	ErrorTypeStorage    ErrorType = "storage"
	ErrorTypeContract   ErrorType = "contract"
	
	// Resource errors
	ErrorTypeNotFound   ErrorType = "not_found"
	ErrorTypeAccess     ErrorType = "access"
	ErrorTypeQuota      ErrorType = "quota"
)

// AutoPatchError represents an enhanced error with context and recovery information
type AutoPatchError struct {
	Type         ErrorType
	Message      string
	OriginalErr  error
	Context      map[string]interface{}
	Timestamp    time.Time
	Recoverable  bool
	SuggestedFix string
}

// Error implements the error interface
func (e *AutoPatchError) Error() string {
	if e.OriginalErr != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Type, e.Message, e.OriginalErr)
	}
	return fmt.Sprintf("[%s] %s", e.Type, e.Message)
}

// Unwrap implements the error unwrapping interface
func (e *AutoPatchError) Unwrap() error {
	return e.OriginalErr
}

// Is implements error checking
func (e *AutoPatchError) Is(target error) bool {
	var targetErr *AutoPatchError
	if errors.As(target, &targetErr) {
		return e.Type == targetErr.Type
	}
	return false
}

// AddContext adds contextual information to the error
func (e *AutoPatchError) AddContext(key string, value interface{}) *AutoPatchError {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context[key] = value
	return e
}

// NewError creates a new AutoPatchError
func NewError(errType ErrorType, message string) *AutoPatchError {
	return &AutoPatchError{
		Type:      errType,
		Message:   message,
		Timestamp: time.Now(),
		Context:   make(map[string]interface{}),
	}
}

// WrapError wraps an existing error with AutoPatchError
func WrapError(errType ErrorType, message string, originalErr error) *AutoPatchError {
	return &AutoPatchError{
		Type:        errType,
		Message:     message,
		OriginalErr: originalErr,
		Timestamp:   time.Now(),
		Context:     make(map[string]interface{}),
	}
}

// Predefined error constructors for common scenarios

// NewNetworkError creates a network-related error
func NewNetworkError(message string, originalErr error) *AutoPatchError {
	return WrapError(ErrorTypeNetwork, message, originalErr).
		AddContext("recoverable", true).
		AddContext("suggested_fix", "Check network connectivity and retry")
}

// NewAPIError creates an API-related error
func NewAPIError(message string, statusCode int, originalErr error) *AutoPatchError {
	err := WrapError(ErrorTypeAPI, message, originalErr).
		AddContext("status_code", statusCode)
	
	if statusCode == 429 {
		err.AddContext("recoverable", true).
			AddContext("suggested_fix", "Rate limited - wait and retry with exponential backoff")
	} else if statusCode >= 500 {
		err.AddContext("recoverable", true).
			AddContext("suggested_fix", "Server error - retry after delay")
	} else {
		err.AddContext("recoverable", false).
			AddContext("suggested_fix", "Check API parameters and authentication")
	}
	
	return err
}

// NewConfigError creates a configuration-related error
func NewConfigError(message string, field string) *AutoPatchError {
	return NewError(ErrorTypeConfig, message).
		AddContext("field", field).
		AddContext("recoverable", false).
		AddContext("suggested_fix", "Check configuration file and environment variables")
}

// NewContractError creates a contract-related error
func NewContractError(message string, contractAddr common.Address, originalErr error) *AutoPatchError {
	return WrapError(ErrorTypeContract, message, originalErr).
		AddContext("contract_address", contractAddr.Hex()).
		AddContext("recoverable", false).
		AddContext("suggested_fix", "Verify contract address and ABI")
}

// NewMutationError creates a mutation-related error
func NewMutationError(message string, mutationID string, originalErr error) *AutoPatchError {
	return WrapError(ErrorTypeMutation, message, originalErr).
		AddContext("mutation_id", mutationID).
		AddContext("recoverable", true).
		AddContext("suggested_fix", "Try different mutation parameters")
}

// ErrorRecovery provides recovery suggestions and retry logic
type ErrorRecovery struct {
	MaxRetries    int
	BaseDelay     time.Duration
	MaxDelay      time.Duration
	RetryableTypes map[ErrorType]bool
}

// NewErrorRecovery creates a new error recovery handler
func NewErrorRecovery() *ErrorRecovery {
	return &ErrorRecovery{
		MaxRetries: 3,
		BaseDelay:  time.Second,
		MaxDelay:   30 * time.Second,
		RetryableTypes: map[ErrorType]bool{
			ErrorTypeNetwork: true,
			ErrorTypeTimeout: true,
			ErrorTypeAPI:     true,
		},
	}
}

// ShouldRetry determines if an error should be retried
func (r *ErrorRecovery) ShouldRetry(err error, attempt int) bool {
	if attempt >= r.MaxRetries {
		return false
	}
	
	var apErr *AutoPatchError
	if errors.As(err, &apErr) {
		// Check if error type is retryable
		if retryable, exists := r.RetryableTypes[apErr.Type]; exists && retryable {
			return true
		}
		
		// Check if error is marked as recoverable
		if recoverable, exists := apErr.Context["recoverable"].(bool); exists && recoverable {
			return true
		}
	}
	
	return false
}

// GetRetryDelay calculates the delay before the next retry
func (r *ErrorRecovery) GetRetryDelay(attempt int) time.Duration {
	delay := r.BaseDelay * time.Duration(1<<uint(attempt)) // Exponential backoff
	if delay > r.MaxDelay {
		delay = r.MaxDelay
	}
	return delay
}

// RetryWithRecovery executes a function with retry logic
func (r *ErrorRecovery) RetryWithRecovery(operation func() error) error {
	var lastErr error
	
	for attempt := 0; attempt <= r.MaxRetries; attempt++ {
		if attempt > 0 {
			delay := r.GetRetryDelay(attempt - 1)
			time.Sleep(delay)
		}
		
		err := operation()
		if err == nil {
			return nil
		}
		
		lastErr = err
		if !r.ShouldRetry(err, attempt) {
			break
		}
	}
	
	return lastErr
}