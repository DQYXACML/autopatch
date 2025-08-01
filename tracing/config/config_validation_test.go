package config

import (
	"testing"
)

func TestConfigValidator(t *testing.T) {
	validator := NewConfigValidator()

	// Test valid configuration
	t.Run("ValidConfiguration", func(t *testing.T) {
		config := &TypeAwareMutationConfig{
			EnableTypeAware:   true,
			FallbackToGeneric: true,
			MaxMutations:      1000,
			Chains: map[string]*ChainMutationConfig{
				"ethereum": {
					ChainID:         1,
					Name:            "ethereum",
					ExplorerAPIKey:  "test_key",
					ExplorerAPI:     "https://api.etherscan.io/api",
					KnownAddresses:  []string{"0x0000000000000000000000000000000000000000"},
					EnableTypeAware: true,
				},
			},
			AddressMutation: AddressMutationConfig{
				UseKnownAddresses: true,
				FlipBytes:         []int{1, 2, 4},
				NearbyRange:       1000,
				ZeroAddressRatio:  0.1,
			},
			NumberMutation: NumberMutationConfig{
				BoundaryValues:  true,
				StepSizes:       []int64{1, 10, 100},
				MultiplierRatio: 0.2,
				BitPatterns:     true,
			},
			StringMutation: StringMutationConfig{
				MaxLength:     1000,
				SpecialChars:  true,
				EncodingTests: true,
				Truncation:    true,
			},
			Execution: ExecutionConfig{
				MaxConcurrentWorkers: 8,
				BatchSize:            100,
				TimeoutSeconds:       30,
				SimilarityThreshold:  0.8,
				EnableEarlyPruning:   true,
				CacheSize:            10000,
			},
		}

		result := validator.ValidateTypeAwareMutationConfig(config)
		if !result.Valid {
			t.Errorf("Expected valid configuration, got errors: %v", result.Errors)
		}
	})

	// Test invalid max mutations
	t.Run("InvalidMaxMutations", func(t *testing.T) {
		config := &TypeAwareMutationConfig{
			MaxMutations: -1,
			Chains: map[string]*ChainMutationConfig{
				"ethereum": {
					ChainID:     1,
					Name:        "ethereum",
					ExplorerAPI: "https://api.etherscan.io/api",
				},
			},
			Execution: ExecutionConfig{
				MaxConcurrentWorkers: 8,
				BatchSize:            100,
				TimeoutSeconds:       30,
				SimilarityThreshold:  0.8,
			},
		}

		result := validator.ValidateTypeAwareMutationConfig(config)
		if result.Valid {
			t.Error("Expected invalid configuration due to negative maxMutations")
		}

		found := false
		for _, err := range result.Errors {
			if err.Field == "maxMutations" && err.Rule == "positive" {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected maxMutations validation error not found")
		}
	})

	// Test invalid addresses
	t.Run("InvalidAddresses", func(t *testing.T) {
		config := &TypeAwareMutationConfig{
			MaxMutations: 100,
			Chains: map[string]*ChainMutationConfig{
				"ethereum": {
					ChainID:     1,
					Name:        "ethereum",
					ExplorerAPI: "https://api.etherscan.io/api",
					KnownAddresses: []string{
						"invalid_address",
						"0x1234", // too short
						"0x0000000000000000000000000000000000000000", // valid
						"0x0000000000000000000000000000000000000000", // duplicate
					},
				},
			},
			Execution: ExecutionConfig{
				MaxConcurrentWorkers: 8,
				BatchSize:            100,
				TimeoutSeconds:       30,
				SimilarityThreshold:  0.8,
			},
		}

		result := validator.ValidateTypeAwareMutationConfig(config)
		if result.Valid {
			t.Error("Expected invalid configuration due to invalid addresses")
		}

		// Should have errors for invalid address format and duplicate
		addressErrors := 0
		for _, err := range result.Errors {
			if err.Rule == "address_format" || err.Rule == "duplicate" {
				addressErrors++
			}
		}
		if addressErrors < 2 {
			t.Errorf("Expected at least 2 address validation errors, got %d", addressErrors)
		}
	})

	// Test invalid URL
	t.Run("InvalidURL", func(t *testing.T) {
		config := &TypeAwareMutationConfig{
			MaxMutations: 100,
			Chains: map[string]*ChainMutationConfig{
				"ethereum": {
					ChainID:     1,
					Name:        "ethereum",
					ExplorerAPI: "not_a_url",
				},
			},
			Execution: ExecutionConfig{
				MaxConcurrentWorkers: 8,
				BatchSize:            100,
				TimeoutSeconds:       30,
				SimilarityThreshold:  0.8,
			},
		}

		result := validator.ValidateTypeAwareMutationConfig(config)
		if result.Valid {
			t.Error("Expected invalid configuration due to invalid URL")
		}

		found := false
		for _, err := range result.Errors {
			if err.Rule == "url_format" {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected URL format validation error not found")
		}
	})

	// Test invalid execution config
	t.Run("InvalidExecutionConfig", func(t *testing.T) {
		config := &TypeAwareMutationConfig{
			MaxMutations: 100,
			Chains: map[string]*ChainMutationConfig{
				"ethereum": {
					ChainID:     1,
					Name:        "ethereum",
					ExplorerAPI: "https://api.etherscan.io/api",
				},
			},
			Execution: ExecutionConfig{
				MaxConcurrentWorkers: 0,          // invalid
				BatchSize:            -1,         // invalid
				TimeoutSeconds:       0,          // invalid
				SimilarityThreshold:  1.5,        // invalid
				CacheSize:            -1,         // invalid
			},
		}

		result := validator.ValidateTypeAwareMutationConfig(config)
		if result.Valid {
			t.Error("Expected invalid configuration due to invalid execution config")
		}

		expectedErrors := []string{"positive", "positive", "positive", "ratio", "non_negative"}
		foundErrors := make(map[string]bool)

		for _, err := range result.Errors {
			foundErrors[err.Rule] = true
		}

		for _, expected := range expectedErrors {
			if !foundErrors[expected] {
				t.Errorf("Expected validation error with rule '%s' not found", expected)
			}
		}
	})

	// Test missing chains configuration
	t.Run("MissingChainsConfig", func(t *testing.T) {
		config := &TypeAwareMutationConfig{
			MaxMutations: 100,
			Execution: ExecutionConfig{
				MaxConcurrentWorkers: 8,
				BatchSize:            100,
				TimeoutSeconds:       30,
				SimilarityThreshold:  0.8,
			},
		}

		result := validator.ValidateTypeAwareMutationConfig(config)
		if result.Valid {
			t.Error("Expected invalid configuration due to missing chains")
		}

		found := false
		for _, err := range result.Errors {
			if err.Field == "chains" && err.Rule == "required" {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected chains required validation error not found")
		}
	})

	// Test missing execution configuration
	t.Run("MissingExecutionConfig", func(t *testing.T) {
		config := &TypeAwareMutationConfig{
			MaxMutations: 100,
			Chains: map[string]*ChainMutationConfig{
				"ethereum": {
					ChainID:     1,
					Name:        "ethereum",
					ExplorerAPI: "https://api.etherscan.io/api",
				},
			},
		}

		result := validator.ValidateTypeAwareMutationConfig(config)
		if result.Valid {
			t.Error("Expected invalid configuration due to missing execution config")
		}

		found := false
		for _, err := range result.Errors {
			if err.Field == "execution" && err.Rule == "required" {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected execution required validation error not found")
		}
	})
}

func TestAddressMutationValidation(t *testing.T) {
	validator := NewConfigValidator()

	// Test invalid flip bytes
	t.Run("InvalidFlipBytes", func(t *testing.T) {
		config := &AddressMutationConfig{
			FlipBytes:        []int{0, 21, -1}, // invalid values
			NearbyRange:      1000,
			ZeroAddressRatio: 0.1,
		}

		result := &ValidationResult{Valid: true}
		validator.validateAddressMutationConfig(config, result)

		if result.Valid {
			t.Error("Expected invalid configuration due to invalid flip bytes")
		}

		invalidCount := 0
		for _, err := range result.Errors {
			if err.Rule == "range" {
				invalidCount++
			}
		}
		if invalidCount != 3 {
			t.Errorf("Expected 3 range validation errors, got %d", invalidCount)
		}
	})

	// Test invalid ratios
	t.Run("InvalidRatios", func(t *testing.T) {
		config := &AddressMutationConfig{
			FlipBytes:        []int{1, 2},
			NearbyRange:      -1,   // invalid
			ZeroAddressRatio: 1.5,  // invalid
		}

		result := &ValidationResult{Valid: true}
		validator.validateAddressMutationConfig(config, result)

		if result.Valid {
			t.Error("Expected invalid configuration due to invalid ratios")
		}

		foundNonNegative := false
		foundRatio := false
		for _, err := range result.Errors {
			if err.Rule == "non_negative" {
				foundNonNegative = true
			}
			if err.Rule == "ratio" {
				foundRatio = true
			}
		}

		if !foundNonNegative {
			t.Error("Expected non_negative validation error not found")
		}
		if !foundRatio {
			t.Error("Expected ratio validation error not found")
		}
	})
}

func TestChainCompatibilityValidation(t *testing.T) {
	validator := NewConfigValidator()

	// Test compatible chains
	t.Run("CompatibleChains", func(t *testing.T) {
		config := &TypeAwareMutationConfig{
			Chains: map[string]*ChainMutationConfig{
				"ethereum": {ChainID: 1},
				"bsc":      {ChainID: 56},
			},
		}

		result := validator.ValidateChainCompatibility(config, []int64{1, 56})
		if !result.Valid {
			t.Errorf("Expected valid chain compatibility, got errors: %v", result.Errors)
		}
	})

	// Test incompatible chains
	t.Run("IncompatibleChains", func(t *testing.T) {
		config := &TypeAwareMutationConfig{
			Chains: map[string]*ChainMutationConfig{
				"ethereum": {ChainID: 1},
			},
		}

		result := validator.ValidateChainCompatibility(config, []int64{1, 56, 137}) // 56 and 137 not configured
		if result.Valid {
			t.Error("Expected invalid chain compatibility")
		}

		missingChains := 0
		for _, err := range result.Errors {
			if err.Rule == "missing_config" {
				missingChains++
			}
		}
		if missingChains != 2 {
			t.Errorf("Expected 2 missing chain errors, got %d", missingChains)
		}
	})
}

func TestValidationSummary(t *testing.T) {
	// Test successful validation summary
	t.Run("SuccessfulValidation", func(t *testing.T) {
		result := &ValidationResult{Valid: true}
		summary := result.GetValidationSummary()

		if !contains(summary, "✅ Configuration validation passed with no issues") {
			t.Error("Expected success message not found in summary")
		}
	})

	// Test validation with errors
	t.Run("ValidationWithErrors", func(t *testing.T) {
		result := &ValidationResult{Valid: false}
		result.AddError("testField", "testRule", "test error message", "testValue")

		summary := result.GetValidationSummary()

		if !contains(summary, "❌ Configuration validation failed") {
			t.Error("Expected error message not found in summary")
		}
		if !contains(summary, "test error message") {
			t.Error("Expected specific error message not found in summary")
		}
	})

	// Test validation with warnings
	t.Run("ValidationWithWarnings", func(t *testing.T) {
		result := &ValidationResult{Valid: true}
		result.AddWarning("test warning message")

		summary := result.GetValidationSummary()

		if !contains(summary, "✅ Configuration validation passed") {
			t.Error("Expected success message not found in summary")
		}
		if !contains(summary, "⚠️") {
			t.Error("Expected warning indicator not found in summary")
		}
		if !contains(summary, "test warning message") {
			t.Error("Expected warning message not found in summary")
		}
	})
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && 
		   (s == substr || 
		    (len(s) > len(substr) && 
		     (s[:len(substr)] == substr || 
		      s[len(s)-len(substr):] == substr || 
		      containsSubstring(s, substr))))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}