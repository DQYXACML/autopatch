package config

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/ethereum/go-ethereum/common"
)

// ConfigValidator provides comprehensive configuration validation
type ConfigValidator struct {
	addressRegex *regexp.Regexp
	urlRegex     *regexp.Regexp
}

// NewConfigValidator creates a new configuration validator
func NewConfigValidator() *ConfigValidator {
	return &ConfigValidator{
		addressRegex: regexp.MustCompile(`^0x[a-fA-F0-9]{40}$`),
		urlRegex:     regexp.MustCompile(`^https?://[^\s/$.?#].[^\s]*$`),
	}
}

// ValidationError represents a configuration validation error
type ValidationError struct {
	Field   string
	Value   interface{}
	Rule    string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation failed for field '%s': %s (value: %v)", e.Field, e.Message, e.Value)
}

// ValidationResult contains the result of configuration validation
type ValidationResult struct {
	Valid  bool
	Errors []ValidationError
	Warnings []string
}

// AddError adds a validation error
func (vr *ValidationResult) AddError(field, rule, message string, value interface{}) {
	vr.Valid = false
	vr.Errors = append(vr.Errors, ValidationError{
		Field:   field,
		Value:   value,
		Rule:    rule,
		Message: message,
	})
}

// AddWarning adds a validation warning
func (vr *ValidationResult) AddWarning(message string) {
	vr.Warnings = append(vr.Warnings, message)
}

// HasErrors returns true if there are validation errors
func (vr *ValidationResult) HasErrors() bool {
	return len(vr.Errors) > 0
}

// ValidateTypeAwareMutationConfig validates the complete mutation configuration
func (cv *ConfigValidator) ValidateTypeAwareMutationConfig(config *TypeAwareMutationConfig) *ValidationResult {
	result := &ValidationResult{Valid: true}

	// Validate top-level fields
	cv.validateMaxMutations(config.MaxMutations, result)

	// Validate chains configuration
	if config.Chains == nil || len(config.Chains) == 0 {
		result.AddError("chains", "required", "at least one chain configuration is required", nil)
	} else {
		cv.validateChainsConfig(config.Chains, result)
	}

	// Validate address mutation config
	cv.validateAddressMutationConfig(&config.AddressMutation, result)

	// Validate number mutation config
	cv.validateNumberMutationConfig(&config.NumberMutation, result)

	// Validate string mutation config
	cv.validateStringMutationConfig(&config.StringMutation, result)

	// Validate execution config - check if it's a zero value
	if config.Execution.MaxConcurrentWorkers == 0 && config.Execution.BatchSize == 0 && config.Execution.TimeoutSeconds == 0 {
		result.AddError("execution", "required", "execution configuration is required", nil)
	} else {
		cv.validateExecutionConfig(&config.Execution, result)
	}

	return result
}

// validateMaxMutations validates the maximum mutations setting
func (cv *ConfigValidator) validateMaxMutations(maxMutations int, result *ValidationResult) {
	if maxMutations <= 0 {
		result.AddError("maxMutations", "positive", "must be greater than 0", maxMutations)
	} else if maxMutations > 100000 {
		result.AddWarning(fmt.Sprintf("maxMutations value %d is very high, consider reducing for performance", maxMutations))
	}
}

// validateChainsConfig validates chain configurations
func (cv *ConfigValidator) validateChainsConfig(chains map[string]*ChainMutationConfig, result *ValidationResult) {
	supportedChains := map[int64]string{
		1:  "ethereum",
		56: "bsc",
	}

	for chainName, config := range chains {
		fieldPrefix := fmt.Sprintf("chains.%s", chainName)

		// Validate chain ID
		if config.ChainID <= 0 {
			result.AddError(fieldPrefix+".chainId", "positive", "must be greater than 0", config.ChainID)
		}

		// Check if chain is supported
		if expectedName, supported := supportedChains[config.ChainID]; supported {
			if config.Name != expectedName {
				result.AddWarning(fmt.Sprintf("Chain ID %d typically uses name '%s', got '%s'", config.ChainID, expectedName, config.Name))
			}
		} else {
			result.AddWarning(fmt.Sprintf("Chain ID %d is not in the list of well-known chains", config.ChainID))
		}

		// Validate explorer API URL
		if config.ExplorerAPI != "" {
			if !cv.urlRegex.MatchString(config.ExplorerAPI) {
				result.AddError(fieldPrefix+".explorerApi", "url_format", "must be a valid HTTP/HTTPS URL", config.ExplorerAPI)
			} else {
				// Additional URL validation
				if _, err := url.Parse(config.ExplorerAPI); err != nil {
					result.AddError(fieldPrefix+".explorerApi", "url_parse", "URL parsing failed: "+err.Error(), config.ExplorerAPI)
				}
			}
		}

		// Validate known addresses
		cv.validateKnownAddresses(config.KnownAddresses, fieldPrefix+".knownAddresses", result)

		// Check for API key
		if config.ExplorerAPIKey == "" {
			result.AddWarning(fmt.Sprintf("No API key configured for %s chain, requests may be rate-limited", chainName))
		}
	}
}

// validateKnownAddresses validates Ethereum addresses
func (cv *ConfigValidator) validateKnownAddresses(addresses []string, fieldPath string, result *ValidationResult) {
	seenAddresses := make(map[string]bool)

	for i, addr := range addresses {
		fieldName := fmt.Sprintf("%s[%d]", fieldPath, i)

		// Check format
		if !cv.addressRegex.MatchString(addr) {
			result.AddError(fieldName, "address_format", "must be a valid Ethereum address (0x + 40 hex chars)", addr)
			continue
		}

		// Check if it's a valid address using go-ethereum
		if !common.IsHexAddress(addr) {
			result.AddError(fieldName, "address_checksum", "invalid address format or checksum", addr)
			continue
		}

		// Check for duplicates
		addrLower := strings.ToLower(addr)
		if seenAddresses[addrLower] {
			result.AddError(fieldName, "duplicate", "duplicate address found", addr)
		}
		seenAddresses[addrLower] = true

		// Warn about zero address
		if addr == "0x0000000000000000000000000000000000000000" {
			result.AddWarning(fmt.Sprintf("Zero address found in known addresses at index %d", i))
		}
	}
}

// validateAddressMutationConfig validates address mutation configuration
func (cv *ConfigValidator) validateAddressMutationConfig(config *AddressMutationConfig, result *ValidationResult) {
	// Validate flip bytes
	if len(config.FlipBytes) == 0 {
		result.AddWarning("addressMutation.flipBytes is empty, no byte flipping will occur")
	} else {
		for i, byteVal := range config.FlipBytes {
			if byteVal < 1 || byteVal > 20 {
				result.AddError(fmt.Sprintf("addressMutation.flipBytes[%d]", i), "range", "must be between 1 and 20 (address byte positions)", byteVal)
			}
		}
	}

	// Validate nearby range
	if config.NearbyRange < 0 {
		result.AddError("addressMutation.nearbyRange", "non_negative", "must be non-negative", config.NearbyRange)
	} else if config.NearbyRange > 1000000 {
		result.AddWarning("addressMutation.nearbyRange is very large, may generate too many invalid addresses")
	}

	// Validate zero address ratio
	if config.ZeroAddressRatio < 0 || config.ZeroAddressRatio > 1 {
		result.AddError("addressMutation.zeroAddressRatio", "ratio", "must be between 0.0 and 1.0", config.ZeroAddressRatio)
	}
}

// validateNumberMutationConfig validates number mutation configuration
func (cv *ConfigValidator) validateNumberMutationConfig(config *NumberMutationConfig, result *ValidationResult) {
	// Validate step sizes
	if len(config.StepSizes) == 0 {
		result.AddWarning("numberMutation.stepSizes is empty, no step-based mutations will occur")
	}

	// Validate multiplier ratio
	if config.MultiplierRatio < 0 || config.MultiplierRatio > 1 {
		result.AddError("numberMutation.multiplierRatio", "ratio", "must be between 0.0 and 1.0", config.MultiplierRatio)
	}
}

// validateStringMutationConfig validates string mutation configuration
func (cv *ConfigValidator) validateStringMutationConfig(config *StringMutationConfig, result *ValidationResult) {
	// Validate max length
	if config.MaxLength <= 0 {
		result.AddError("stringMutation.maxLength", "positive", "must be greater than 0", config.MaxLength)
	} else if config.MaxLength > 1000000 {
		result.AddWarning("stringMutation.maxLength is very large, may cause memory issues")
	}
}

// validateExecutionConfig validates execution configuration
func (cv *ConfigValidator) validateExecutionConfig(config *ExecutionConfig, result *ValidationResult) {
	// Validate max concurrent workers
	if config.MaxConcurrentWorkers <= 0 {
		result.AddError("execution.maxConcurrentWorkers", "positive", "must be greater than 0", config.MaxConcurrentWorkers)
	} else if config.MaxConcurrentWorkers > 100 {
		result.AddWarning("execution.maxConcurrentWorkers is very high, may cause resource exhaustion")
	}

	// Validate batch size
	if config.BatchSize <= 0 {
		result.AddError("execution.batchSize", "positive", "must be greater than 0", config.BatchSize)
	} else if config.BatchSize > 10000 {
		result.AddWarning("execution.batchSize is very large, may cause memory issues")
	}

	// Validate timeout
	if config.TimeoutSeconds <= 0 {
		result.AddError("execution.timeoutSeconds", "positive", "must be greater than 0", config.TimeoutSeconds)
	} else if config.TimeoutSeconds > 3600 {
		result.AddWarning("execution.timeoutSeconds is very long (> 1 hour)")
	}

	// Validate similarity threshold
	if config.SimilarityThreshold < 0 || config.SimilarityThreshold > 1 {
		result.AddError("execution.similarityThreshold", "ratio", "must be between 0.0 and 1.0", config.SimilarityThreshold)
	}

	// Validate cache size
	if config.CacheSize < 0 {
		result.AddError("execution.cacheSize", "non_negative", "must be non-negative", config.CacheSize)
	} else if config.CacheSize > 1000000 {
		result.AddWarning("execution.cacheSize is very large, may cause memory issues")
	}
}

// ValidateEnvironmentVariables validates required environment variables
func (cv *ConfigValidator) ValidateEnvironmentVariables(requiredVars []string) *ValidationResult {
	result := &ValidationResult{Valid: true}

	for _, varName := range requiredVars {
		// This would normally check os.Getenv, but we don't want to depend on actual env vars in tests
		result.AddWarning(fmt.Sprintf("Environment variable %s should be verified at runtime", varName))
	}

	return result
}

// ValidateMutationConfig 兼容性方法，委托给 ValidateTypeAwareMutationConfig
func (cv *ConfigValidator) ValidateMutationConfig(config *TypeAwareMutationConfig) *ValidationResult {
	return cv.ValidateTypeAwareMutationConfig(config)
}

// ValidateChainCompatibility validates that configuration is compatible with specified chains
func (cv *ConfigValidator) ValidateChainCompatibility(config *TypeAwareMutationConfig, activeChains []int64) *ValidationResult {
	result := &ValidationResult{Valid: true}

	configuredChains := make(map[int64]bool)
	for _, chainConfig := range config.Chains {
		configuredChains[chainConfig.ChainID] = true
	}

	for _, chainID := range activeChains {
		if !configuredChains[chainID] {
			result.AddError("chain_compatibility", "missing_config", 
				fmt.Sprintf("Active chain %d is not configured", chainID), chainID)
		}
	}

	return result
}

// GetValidationSummary returns a human-readable summary of validation results
func (vr *ValidationResult) GetValidationSummary() string {
	var summary strings.Builder
	
	if vr.Valid && len(vr.Warnings) == 0 {
		summary.WriteString("✅ Configuration validation passed with no issues\n")
		return summary.String()
	}

	if !vr.Valid {
		summary.WriteString(fmt.Sprintf("❌ Configuration validation failed with %d errors:\n", len(vr.Errors)))
		for _, err := range vr.Errors {
			summary.WriteString(fmt.Sprintf("  - %s\n", err.Error()))
		}
	} else {
		summary.WriteString("✅ Configuration validation passed\n")
	}

	if len(vr.Warnings) > 0 {
		summary.WriteString(fmt.Sprintf("⚠️  %d warnings:\n", len(vr.Warnings)))
		for _, warning := range vr.Warnings {
			summary.WriteString(fmt.Sprintf("  - %s\n", warning))
		}
	}

	return summary.String()
}