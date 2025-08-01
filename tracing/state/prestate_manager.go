package state

import (
	"context"
	"fmt"
	"math/big"

	"github.com/DQYXACML/autopatch/database/utils"
	gethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	tracingUtils "github.com/DQYXACML/autopatch/tracing/utils"
)

// PrestateManager 管理预状态获取和处理
type PrestateManager struct {
	client *ethclient.Client
}

// NewPrestateManager 创建预状态管理器
func NewPrestateManager(client *ethclient.Client) *PrestateManager {
	return &PrestateManager{
		client: client,
	}
}

// GetTransactionPrestateWithAllContracts 获取交易的预状态，保存所有合约的存储
func (pm *PrestateManager) GetTransactionPrestateWithAllContracts(txHash gethCommon.Hash) (tracingUtils.PrestateResult, map[gethCommon.Address]map[gethCommon.Hash]gethCommon.Hash, error) {
	fmt.Printf("=== GETTING PRESTATE WITH ALL CONTRACTS ===\n")

	config := map[string]interface{}{
		"tracer": "prestateTracer",
		"tracerConfig": map[string]interface{}{
			"diffMode": false,
		},
		"timeout": "60s",
	}

	var result tracingUtils.PrestateResult
	err := pm.client.Client().CallContext(context.Background(), &result,
		"debug_traceTransaction", txHash, config)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get prestate: %v", err)
	}

	// 创建所有合约的存储映射
	allContractsStorage := make(map[gethCommon.Address]map[gethCommon.Hash]gethCommon.Hash)

	// 遍历所有账户，保存它们的存储
	for addr, account := range result {
		if account != nil && len(account.Storage) > 0 {
			allContractsStorage[addr] = make(map[gethCommon.Hash]gethCommon.Hash)
			for slot, value := range account.Storage {
				allContractsStorage[addr][slot] = value
			}
			fmt.Printf("💾 Saved storage for contract %s: %d slots\n", addr.Hex(), len(account.Storage))
		}
	}

	fmt.Printf("📦 Total contracts with storage: %d\n", len(allContractsStorage))
	return result, allContractsStorage, nil
}

// GetPrestate 获取交易的预状态，返回ContractState格式
func (pm *PrestateManager) GetPrestate(txHash gethCommon.Hash) (map[gethCommon.Address]*utils.ContractState, error) {
	prestateResult, _, err := pm.GetTransactionPrestateWithAllContracts(txHash)
	if err != nil {
		return nil, err
	}
	
	contractStates := make(map[gethCommon.Address]*utils.ContractState)
	
	// 转换为ContractState格式
	for addr, account := range prestateResult {
		if account != nil {
			var balance *big.Int
			if account.Balance != nil {
				balance = (*big.Int)(account.Balance)
			}
			
			contractStates[addr] = &utils.ContractState{
				Address: addr,
				Storage: account.Storage,
				Code:    account.Code,
				Balance: balance,
				Nonce:   account.Nonce,
			}
		}
	}
	
	return contractStates, nil
}
