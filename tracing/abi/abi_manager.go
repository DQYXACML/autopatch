package abi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/DQYXACML/autopatch/tracing/utils"
)

// ChainConfig Chain configuration
type ChainConfig struct {
	ChainID     int64  `json:"chainId"`
	Name        string `json:"name"`
	ExplorerAPI string `json:"explorerApi"`
	APIKey      string `json:"apiKey"`
}

// ABICache ABI cache structure
type ABICache struct {
	mu    sync.RWMutex
	cache map[string]*abi.ABI // chainID_address -> ABI
	dir   string              // Cache directory
}

// ABIManager ABI manager
type ABIManager struct {
	chains     map[int64]*ChainConfig
	cache      *ABICache
	httpClient *http.Client
}

// NewABIManager Create ABI manager
func NewABIManager(cacheDir string) *ABIManager {
	// Create cache directory
	if cacheDir == "" {
		cacheDir = "./abi_cache"
	}
	os.MkdirAll(cacheDir, 0755)

	// Initialize supported chains
	chains := map[int64]*ChainConfig{
		1: { // Ethereum mainnet
			ChainID:     1,
			Name:        "ethereum",
			ExplorerAPI: "https://api.etherscan.io/api",
			APIKey:      os.Getenv("ETHERSCAN_API_KEY"),
		},
		56: { // BSC mainnet
			ChainID:     56,
			Name:        "bsc",
			ExplorerAPI: "https://api.bscscan.com/api",
			APIKey:      os.Getenv("BSCSCAN_API_KEY"),
		},
	}

	return &ABIManager{
		chains: chains,
		cache: &ABICache{
			cache: make(map[string]*abi.ABI),
			dir:   cacheDir,
		},
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetContractABI Get contract ABI with enhanced error handling
func (m *ABIManager) GetContractABI(chainID *big.Int, address common.Address) (*abi.ABI, error) {
	cacheKey := fmt.Sprintf("%s_%s", chainID.String(), address.Hex())

	// 1. Check memory cache
	if cachedABI := m.cache.get(cacheKey); cachedABI != nil {
		fmt.Printf("📋 Found ABI in memory cache for %s on chain %s\n", address.Hex(), chainID.String())
		return cachedABI, nil
	}

	// 2. Check file cache
	if cachedABI := m.cache.loadFromFile(cacheKey); cachedABI != nil {
		fmt.Printf("📋 Found ABI in file cache for %s on chain %s\n", address.Hex(), chainID.String())
		m.cache.set(cacheKey, cachedABI) // Load to memory cache
		return cachedABI, nil
	}

	// 3. Fetch from block explorer
	chain, exists := m.chains[chainID.Int64()]
	if !exists {
		return nil, utils.NewConfigError("Unsupported chain ID", "chainID").
			AddContext("chain_id", chainID.String()).
			AddContext("supported_chains", []int64{1, 56}).
			AddContext("suggested_fix", "Add chain configuration or use supported chain")
	}

	if chain.APIKey == "" {
		fmt.Printf("⚠️  No API key for %s, trying without key\n", chain.Name)
	}

	contractABI, err := m.fetchABIFromExplorer(chain, address)
	if err != nil {
		// Enhance error with additional context
		var apErr *utils.AutoPatchError
		if errors.As(err, &apErr) {
			apErr.AddContext("operation", "fetch_abi").
				AddContext("cache_key", cacheKey)
			return nil, apErr
		}
		
		// Wrap non-AutoPatchError
		return nil, utils.WrapError(utils.ErrorTypeAPI, "Failed to fetch ABI from explorer", err).
			AddContext("chain", chain.Name).
			AddContext("contract_address", address.Hex()).
			AddContext("chain_id", chainID.String())
	}

	// 4. Cache results
	m.cache.set(cacheKey, contractABI)
	m.cache.saveToFile(cacheKey, contractABI)

	fmt.Printf("✅ Successfully fetched and cached ABI for %s on %s\n", address.Hex(), chain.Name)
	return contractABI, nil
}

// GetChainConfig Get chain configuration
func (m *ABIManager) GetChainConfig(chainID int64) (*ChainConfig, bool) {
	chain, exists := m.chains[chainID]
	return chain, exists
}

// SetAPIKey Set API key
func (m *ABIManager) SetAPIKey(chainID int64, apiKey string) {
	if chain, exists := m.chains[chainID]; exists {
		chain.APIKey = apiKey
	}
}

// fetchABIFromExplorer Fetch ABI from block explorer with enhanced error handling
func (m *ABIManager) fetchABIFromExplorer(chain *ChainConfig, address common.Address) (*abi.ABI, error) {
	// Build request URL
	url := fmt.Sprintf("%s?module=contract&action=getabi&address=%s", 
		chain.ExplorerAPI, address.Hex())
	
	if chain.APIKey != "" {
		url += "&apikey=" + chain.APIKey
	}

	fmt.Printf("🔍 Fetching ABI from %s for %s\n", chain.Name, address.Hex())

	// Create error recovery handler
	recovery := utils.NewErrorRecovery()
	
	var contractABI *abi.ABI
	err := recovery.RetryWithRecovery(func() error {
		// Send request with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return utils.NewNetworkError("Failed to create HTTP request", err).
				AddContext("url", url).
				AddContext("chain", chain.Name)
		}

		resp, err := m.httpClient.Do(req)
		if err != nil {
			if ctx.Err() == context.DeadlineExceeded {
				return utils.WrapError(utils.ErrorTypeTimeout, "Request timeout", err).
					AddContext("timeout_seconds", 15).
					AddContext("chain", chain.Name).
					AddContext("recoverable", true).
					AddContext("suggested_fix", "Increase timeout or check network connectivity")
			}
			return utils.NewNetworkError("Failed to fetch ABI from explorer", err).
				AddContext("chain", chain.Name).
				AddContext("url", url)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return utils.NewAPIError("HTTP error from block explorer", resp.StatusCode, nil).
				AddContext("chain", chain.Name).
				AddContext("url", url).
				AddContext("contract_address", address.Hex())
		}

		// Parse response
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return utils.WrapError(utils.ErrorTypeNetwork, "Failed to read response body", err).
				AddContext("chain", chain.Name).
				AddContext("recoverable", true)
		}

		var response struct {
			Status  string `json:"status"`
			Message string `json:"message"`
			Result  string `json:"result"`
		}

		if err := json.Unmarshal(body, &response); err != nil {
			return utils.WrapError(utils.ErrorTypeDecoding, "Failed to parse JSON response", err).
				AddContext("chain", chain.Name).
				AddContext("response_size", len(body)).
				AddContext("recoverable", false).
				AddContext("suggested_fix", "Check API response format")
		}

		if response.Status != "1" {
			if response.Message == "Contract source code not verified" {
				return utils.NewContractError("Contract source code not verified", address, nil).
					AddContext("chain", chain.Name).
					AddContext("suggested_fix", "Verify contract source code on block explorer")
			}
			return utils.NewAPIError("Block explorer API error", 0, nil).
				AddContext("api_message", response.Message).
				AddContext("chain", chain.Name).
				AddContext("recoverable", false)
		}

		// Parse ABI
		parsedABI, err := abi.JSON(strings.NewReader(response.Result))
		if err != nil {
			return utils.WrapError(utils.ErrorTypeParsing, "Failed to parse ABI JSON", err).
				AddContext("chain", chain.Name).
				AddContext("contract_address", address.Hex()).
				AddContext("abi_size", len(response.Result)).
				AddContext("recoverable", false).
				AddContext("suggested_fix", "Check if ABI format is valid")
		}

		contractABI = &parsedABI
		return nil
	})

	if err != nil {
		return nil, err
	}

	return contractABI, nil
}

// ABICache related methods

func (c *ABICache) get(key string) *abi.ABI {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.cache[key]
}

func (c *ABICache) set(key string, contractABI *abi.ABI) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache[key] = contractABI
}

func (c *ABICache) loadFromFile(key string) *abi.ABI {
	filename := filepath.Join(c.dir, key+".json")
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil
	}

	var contractABI abi.ABI
	if err := json.Unmarshal(data, &contractABI); err != nil {
		return nil
	}

	return &contractABI
}

func (c *ABICache) saveToFile(key string, contractABI *abi.ABI) {
	filename := filepath.Join(c.dir, key+".json")
	data, err := json.Marshal(contractABI)
	if err != nil {
		return
	}

	os.WriteFile(filename, data, 0644)
}

// ClearCache Clear cache
func (m *ABIManager) ClearCache() {
	m.cache.mu.Lock()
	defer m.cache.mu.Unlock()
	
	m.cache.cache = make(map[string]*abi.ABI)
	
	// Clear file cache
	if entries, err := os.ReadDir(m.cache.dir); err == nil {
		for _, entry := range entries {
			if filepath.Ext(entry.Name()) == ".json" {
				os.Remove(filepath.Join(m.cache.dir, entry.Name()))
			}
		}
	}
	
	fmt.Printf("🧹 ABI cache cleared\n")
}

// GetCacheStats Get cache statistics
func (m *ABIManager) GetCacheStats() map[string]int {
	m.cache.mu.RLock()
	defer m.cache.mu.RUnlock()
	
	stats := map[string]int{
		"memory_cache_size": len(m.cache.cache),
	}
	
	// Count file cache
	if entries, err := os.ReadDir(m.cache.dir); err == nil {
		fileCount := 0
		for _, entry := range entries {
			if filepath.Ext(entry.Name()) == ".json" {
				fileCount++
			}
		}
		stats["file_cache_size"] = fileCount
	}
	
	return stats
}