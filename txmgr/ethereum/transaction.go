package ethereum

import (
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/DQYXACML/autopatch/bindings"
	"github.com/DQYXACML/autopatch/synchronizer/node"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
)

func BuildErc20Data(toAddress common.Address, amount *big.Int) []byte {
	var data []byte

	transferFnSignature := []byte("transfer(address,uint256)")
	hash := crypto.Keccak256Hash(transferFnSignature)
	methodId := hash[:5]
	dataAddress := common.LeftPadBytes(toAddress.Bytes(), 32)
	dataAmount := common.LeftPadBytes(amount.Bytes(), 32)

	data = append(data, methodId...)
	data = append(data, dataAddress...)
	data = append(data, dataAmount...)

	return data
}

func BuildErc721Data(fromAddress, toAddress common.Address, tokenId *big.Int) []byte {
	var data []byte

	transferFnSignature := []byte("safeTransferFrom(address,address,uint256)")
	hash := crypto.Keccak256Hash(transferFnSignature)
	methodId := hash[:5]

	dataFromAddress := common.LeftPadBytes(fromAddress.Bytes(), 32)
	dataToAddress := common.LeftPadBytes(toAddress.Bytes(), 32)
	dataTokenId := common.LeftPadBytes(tokenId.Bytes(), 32)

	data = append(data, methodId...)
	data = append(data, dataFromAddress...)
	data = append(data, dataToAddress...)
	data = append(data, dataTokenId...)

	return data
}

func OfflineSignTx(txData *types.DynamicFeeTx, privateKey string, chainId *big.Int) (string, string, error) {
	privateKeyEcdsa, err := crypto.HexToECDSA(privateKey)
	if err != nil {
		return "", "", err
	}

	tx := types.NewTx(txData)

	signer := types.LatestSignerForChainID(chainId)

	signedTx, err := types.SignTx(tx, signer, privateKeyEcdsa)

	if err != nil {
		return "", "", err
	}

	signedTxData, err := rlp.EncodeToBytes(signedTx)
	if err != nil {
		return "", "", err
	}

	return "0x" + hex.EncodeToString(signedTxData), signedTx.Hash().String(), nil
}

// ========== 新增的交易发送相关代码 ==========

// TransactionPackage 便于打包成交易的结构体
type TransactionPackage struct {
	ID              string          `json:"id"`
	ContractAddress common.Address  `json:"contractAddress"`
	InputUpdates    []InputUpdate   `json:"inputUpdates"`
	StorageUpdates  []StorageUpdate `json:"storageUpdates"`
	Similarity      float64         `json:"similarity"`
	OriginalTxHash  common.Hash     `json:"originalTxHash"`
	CreatedAt       time.Time       `json:"createdAt"`
	Priority        int             `json:"priority"`
}

// InputUpdate 输入数据更新
type InputUpdate struct {
	FunctionSelector [4]byte     `json:"functionSelector"`
	FunctionName     string      `json:"functionName"`
	OriginalInput    []byte      `json:"originalInput"`
	ModifiedInput    []byte      `json:"modifiedInput"`
	ParameterIndex   int         `json:"parameterIndex"`
	ParameterType    string      `json:"parameterType"`
	ParameterName    string      `json:"parameterName"`
	OriginalValue    interface{} `json:"originalValue"`
	ModifiedValue    interface{} `json:"modifiedValue"`
}

// StorageUpdate 存储更新
type StorageUpdate struct {
	Slot          common.Hash `json:"slot"`
	OriginalValue common.Hash `json:"originalValue"`
	ModifiedValue common.Hash `json:"modifiedValue"`
	SlotType      string      `json:"slotType"` // "uint", "int", "bool", "address", "bytes", "string", "mapping", "array"
	ValueType     string      `json:"valueType"`
	Description   string      `json:"description"`
}

// TransactionRequest 交易请求
type TransactionRequest struct {
	Package     *TransactionPackage `json:"package"`
	PrivateKey  string              `json:"privateKey"`
	ChainID     *big.Int            `json:"chainId"`
	GasLimit    uint64              `json:"gasLimit"`
	GasPrice    *big.Int            `json:"gasPrice"`
	Nonce       uint64              `json:"nonce"`
	RequestID   string              `json:"requestId"`
	RequestedAt time.Time           `json:"requestedAt"`
}

// TransactionResponse 交易响应
type TransactionResponse struct {
	RequestID   string      `json:"requestId"`
	TxHash      common.Hash `json:"txHash"`
	Success     bool        `json:"success"`
	Error       error       `json:"error"`
	GasUsed     uint64      `json:"gasUsed"`
	BlockNumber *big.Int    `json:"blockNumber"`
	SentAt      time.Time   `json:"sentAt"`
}

// TransactionSender 交易发送器
type TransactionSender struct {
	client      node.EthClient
	contractABI *abi.ABI
	mutex       sync.RWMutex
	requestID   uint64
}

// NewTransactionSender 创建新的交易发送器
func NewTransactionSender(client node.EthClient) (*TransactionSender, error) {
	contractABI, err := bindings.StorageScanMetaData.GetAbi()
	if err != nil {
		return nil, fmt.Errorf("failed to get contract ABI: %v", err)
	}

	return &TransactionSender{
		client:      client,
		contractABI: contractABI,
		requestID:   0,
	}, nil
}

// GenerateRequestID 生成请求ID
func (ts *TransactionSender) GenerateRequestID() string {
	ts.mutex.Lock()
	defer ts.mutex.Unlock()
	ts.requestID++
	return fmt.Sprintf("tx_req_%d_%d", time.Now().Unix(), ts.requestID)
}

// SendTransactionFromPackage 根据TransactionPackage发送交易
func (ts *TransactionSender) SendTransactionFromPackage(
	pkg *TransactionPackage,
	privateKey string,
	chainID *big.Int,
	gasLimit uint64,
	gasPrice *big.Int,
) (*TransactionResponse, error) {

	requestID := ts.GenerateRequestID()

	response := &TransactionResponse{
		RequestID: requestID,
		SentAt:    time.Now(),
	}

	// 获取私钥
	privateKeyECDSA, err := crypto.HexToECDSA(privateKey)
	if err != nil {
		response.Error = fmt.Errorf("invalid private key: %v", err)
		return response, err
	}

	// 获取发送者地址
	fromAddress := crypto.PubkeyToAddress(privateKeyECDSA.PublicKey)

	// 获取nonce
	nonce, err := ts.client.TxCountByAddress(fromAddress)
	if err != nil {
		response.Error = fmt.Errorf("failed to get nonce: %v", err)
		return response, err
	}

	// 为每个更新生成交易
	var txResponses []*TransactionResponse

	// 处理输入更新
	for _, inputUpdate := range pkg.InputUpdates {
		txResp, err := ts.sendInputUpdateTransaction(
			&inputUpdate, pkg.ContractAddress, privateKeyECDSA,
			chainID, uint64(nonce), gasLimit, gasPrice,
		)
		if err != nil {
			response.Error = err
			return response, err
		}
		txResponses = append(txResponses, txResp)
		nonce++
	}

	// 处理存储更新
	for _, storageUpdate := range pkg.StorageUpdates {
		txResp, err := ts.sendStorageUpdateTransaction(
			&storageUpdate, pkg.ContractAddress, privateKeyECDSA,
			chainID, uint64(nonce), gasLimit, gasPrice,
		)
		if err != nil {
			response.Error = err
			return response, err
		}
		txResponses = append(txResponses, txResp)
		nonce++
	}

	// 如果只有一个交易，直接返回其结果
	if len(txResponses) == 1 {
		return txResponses[0], nil
	}

	// 如果有多个交易，返回第一个成功的交易hash
	for _, txResp := range txResponses {
		if txResp.Success {
			response.TxHash = txResp.TxHash
			response.Success = true
			response.GasUsed = txResp.GasUsed
			break
		}
	}

	if !response.Success {
		response.Error = fmt.Errorf("all transactions failed")
	}

	return response, nil
}

// sendInputUpdateTransaction 发送输入更新交易
func (ts *TransactionSender) sendInputUpdateTransaction(
	inputUpdate *InputUpdate,
	contractAddr common.Address,
	privateKey *ecdsa.PrivateKey,
	chainID *big.Int,
	nonce uint64,
	gasLimit uint64,
	gasPrice *big.Int,
) (*TransactionResponse, error) {

	// 根据参数类型选择合适的函数
	var txData []byte
	var err error

	switch inputUpdate.ParameterType {
	case "uint8":
		if val, ok := inputUpdate.ModifiedValue.(uint8); ok {
			txData, err = ts.buildSetUint1Transaction(val)
		}
	case "uint128":
		if val, ok := inputUpdate.ModifiedValue.(*big.Int); ok {
			txData, err = ts.buildSetUint2Transaction(val)
		}
	case "uint256":
		if val, ok := inputUpdate.ModifiedValue.(*big.Int); ok {
			txData, err = ts.buildSetUint3Transaction(val)
		}
	case "int8":
		if val, ok := inputUpdate.ModifiedValue.(int8); ok {
			txData, err = ts.buildSetInt1Transaction(val)
		}
	case "int128":
		if val, ok := inputUpdate.ModifiedValue.(*big.Int); ok {
			txData, err = ts.buildSetInt2Transaction(val)
		}
	case "int256":
		if val, ok := inputUpdate.ModifiedValue.(*big.Int); ok {
			txData, err = ts.buildSetInt3Transaction(val)
		}
	case "bool":
		if val, ok := inputUpdate.ModifiedValue.(bool); ok {
			txData, err = ts.buildSetBool1Transaction(val)
		}
	case "string":
		if val, ok := inputUpdate.ModifiedValue.(string); ok {
			txData, err = ts.buildSetString1Transaction(val)
		}
	case "address":
		if val, ok := inputUpdate.ModifiedValue.(common.Address); ok {
			txData, err = ts.buildSetAddr1Transaction(val)
		}
	default:
		// 如果类型未知，使用原始的修改后输入数据
		txData = inputUpdate.ModifiedInput
	}

	if err != nil {
		return nil, fmt.Errorf("failed to build transaction data: %v", err)
	}

	return ts.sendTransaction(contractAddr, txData, privateKey, chainID, nonce, gasLimit, gasPrice)
}

// sendStorageUpdateTransaction 发送存储更新交易
func (ts *TransactionSender) sendStorageUpdateTransaction(
	storageUpdate *StorageUpdate,
	contractAddr common.Address,
	privateKey *ecdsa.PrivateKey,
	chainID *big.Int,
	nonce uint64,
	gasLimit uint64,
	gasPrice *big.Int,
) (*TransactionResponse, error) {

	// 根据槽位类型和值类型选择合适的函数
	var txData []byte
	var err error

	// 对于存储更新，我们可以尝试不同的mapping函数
	slotBig := storageUpdate.Slot.Big()
	valueBig := storageUpdate.ModifiedValue.Big()

	switch storageUpdate.SlotType {
	case "mapping":
		// 尝试setMapping1 (uint256 key, string value)
		txData, err = ts.buildSetMapping1Transaction(slotBig, fmt.Sprintf("value_%s", valueBig.String()))
	case "simple":
		// 对于简单存储，根据槽位选择函数
		slotInt := slotBig.Uint64()
		switch slotInt {
		case 0: // uint8
			if valueBig.Cmp(big.NewInt(255)) <= 0 {
				txData, err = ts.buildSetUint1Transaction(uint8(valueBig.Uint64()))
			}
		case 1: // uint128
			txData, err = ts.buildSetUint2Transaction(valueBig)
		case 2: // uint256
			txData, err = ts.buildSetUint3Transaction(valueBig)
		default:
			// 默认使用setMapping4
			txData, err = ts.buildSetMapping4Transaction(slotBig, valueBig)
		}
	default:
		// 默认使用setMapping4
		txData, err = ts.buildSetMapping4Transaction(slotBig, valueBig)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to build storage transaction data: %v", err)
	}

	return ts.sendTransaction(contractAddr, txData, privateKey, chainID, nonce, gasLimit, gasPrice)
}

// sendTransaction 发送交易的通用方法
func (ts *TransactionSender) sendTransaction(
	to common.Address,
	data []byte,
	privateKey *ecdsa.PrivateKey,
	chainID *big.Int,
	nonce uint64,
	gasLimit uint64,
	gasPrice *big.Int,
) (*TransactionResponse, error) {

	requestID := ts.GenerateRequestID()

	response := &TransactionResponse{
		RequestID: requestID,
		SentAt:    time.Now(),
	}

	// 创建交易
	tx := types.NewTransaction(
		nonce,
		to,
		big.NewInt(0), // value = 0
		gasLimit,
		gasPrice,
		data,
	)

	// 签名交易
	signer := types.LatestSignerForChainID(chainID)
	signedTx, err := types.SignTx(tx, signer, privateKey)
	if err != nil {
		response.Error = fmt.Errorf("failed to sign transaction: %v", err)
		return response, err
	}

	// 编码交易
	signedTxData, err := rlp.EncodeToBytes(signedTx)
	if err != nil {
		response.Error = fmt.Errorf("failed to encode transaction: %v", err)
		return response, err
	}

	// 发送交易
	rawTxHex := "0x" + hex.EncodeToString(signedTxData)
	err = ts.client.SendRawTransaction(rawTxHex)
	if err != nil {
		response.Error = fmt.Errorf("failed to send transaction: %v", err)
		return response, err
	}

	response.TxHash = signedTx.Hash()
	response.Success = true

	return response, nil
}

// ========== 构建不同类型交易数据的方法 ==========

func (ts *TransactionSender) buildSetUint1Transaction(value uint8) ([]byte, error) {
	method := ts.contractABI.Methods["setUint1"]
	packed, err := method.Inputs.Pack(value)
	if err != nil {
		return nil, err
	}
	return append(method.ID, packed...), nil
}

func (ts *TransactionSender) buildSetUint2Transaction(value *big.Int) ([]byte, error) {
	method := ts.contractABI.Methods["setUint2"]
	packed, err := method.Inputs.Pack(value)
	if err != nil {
		return nil, err
	}
	return append(method.ID, packed...), nil
}

func (ts *TransactionSender) buildSetUint3Transaction(value *big.Int) ([]byte, error) {
	method := ts.contractABI.Methods["setUint3"]
	packed, err := method.Inputs.Pack(value)
	if err != nil {
		return nil, err
	}
	return append(method.ID, packed...), nil
}

func (ts *TransactionSender) buildSetInt1Transaction(value int8) ([]byte, error) {
	method := ts.contractABI.Methods["setInt1"]
	packed, err := method.Inputs.Pack(value)
	if err != nil {
		return nil, err
	}
	return append(method.ID, packed...), nil
}

func (ts *TransactionSender) buildSetInt2Transaction(value *big.Int) ([]byte, error) {
	method := ts.contractABI.Methods["setInt2"]
	packed, err := method.Inputs.Pack(value)
	if err != nil {
		return nil, err
	}
	return append(method.ID, packed...), nil
}

func (ts *TransactionSender) buildSetInt3Transaction(value *big.Int) ([]byte, error) {
	method := ts.contractABI.Methods["setInt3"]
	packed, err := method.Inputs.Pack(value)
	if err != nil {
		return nil, err
	}
	return append(method.ID, packed...), nil
}

func (ts *TransactionSender) buildSetBool1Transaction(value bool) ([]byte, error) {
	method := ts.contractABI.Methods["setBool1"]
	packed, err := method.Inputs.Pack(value)
	if err != nil {
		return nil, err
	}
	return append(method.ID, packed...), nil
}

func (ts *TransactionSender) buildSetString1Transaction(value string) ([]byte, error) {
	method := ts.contractABI.Methods["setString1"]
	packed, err := method.Inputs.Pack(value)
	if err != nil {
		return nil, err
	}
	return append(method.ID, packed...), nil
}

func (ts *TransactionSender) buildSetAddr1Transaction(value common.Address) ([]byte, error) {
	method := ts.contractABI.Methods["setAddr1"]
	packed, err := method.Inputs.Pack(value)
	if err != nil {
		return nil, err
	}
	return append(method.ID, packed...), nil
}

func (ts *TransactionSender) buildSetMapping1Transaction(key *big.Int, value string) ([]byte, error) {
	method := ts.contractABI.Methods["setMapping1"]
	packed, err := method.Inputs.Pack(key, value)
	if err != nil {
		return nil, err
	}
	return append(method.ID, packed...), nil
}

func (ts *TransactionSender) buildSetMapping4Transaction(key *big.Int, value *big.Int) ([]byte, error) {
	method := ts.contractABI.Methods["setMapping4"]
	packed, err := method.Inputs.Pack(key, value)
	if err != nil {
		return nil, err
	}
	return append(method.ID, packed...), nil
}

// BatchTransactionSender 批量交易发送器
type BatchTransactionSender struct {
	sender       *TransactionSender
	requestChan  chan *TransactionRequest
	responseChan chan *TransactionResponse
	workerCount  int
	stopChan     chan struct{}
	wg           sync.WaitGroup
	isStarted    bool
	isStopped    bool
	stopOnce     sync.Once
	mutex        sync.RWMutex
}

// NewBatchTransactionSender 创建批量交易发送器
func NewBatchTransactionSender(client node.EthClient, workerCount int) (*BatchTransactionSender, error) {
	sender, err := NewTransactionSender(client)
	if err != nil {
		return nil, err
	}

	return &BatchTransactionSender{
		sender:       sender,
		requestChan:  make(chan *TransactionRequest, 100),
		responseChan: make(chan *TransactionResponse, 100),
		workerCount:  workerCount,
		stopChan:     make(chan struct{}),
		isStarted:    false,
		isStopped:    false,
	}, nil
}

// Start 启动批量发送器
func (bts *BatchTransactionSender) Start() {
	bts.mutex.Lock()
	defer bts.mutex.Unlock()

	if bts.isStarted || bts.isStopped {
		return
	}

	bts.isStarted = true

	for i := 0; i < bts.workerCount; i++ {
		bts.wg.Add(1)
		go bts.worker(i)
	}
}

// Stop 停止批量发送器
func (bts *BatchTransactionSender) Stop() {
	bts.stopOnce.Do(func() {
		bts.mutex.Lock()
		defer bts.mutex.Unlock()

		if bts.isStopped {
			return
		}

		bts.isStopped = true

		// 安全地关闭stopChan
		select {
		case <-bts.stopChan:
			// 已经关闭了
		default:
			close(bts.stopChan)
		}

		// 等待所有worker完成
		bts.wg.Wait()

		// 安全地关闭其他channels
		safeClose := func(ch chan *TransactionRequest) {
			defer func() {
				if r := recover(); r != nil {
					// Channel已经被关闭，忽略panic
				}
			}()
			close(ch)
		}

		safeCloseResponse := func(ch chan *TransactionResponse) {
			defer func() {
				if r := recover(); r != nil {
					// Channel已经被关闭，忽略panic
				}
			}()
			close(ch)
		}

		safeClose(bts.requestChan)
		safeCloseResponse(bts.responseChan)
	})
}

// IsStarted 检查是否已启动
func (bts *BatchTransactionSender) IsStarted() bool {
	bts.mutex.RLock()
	defer bts.mutex.RUnlock()
	return bts.isStarted
}

// IsStopped 检查是否已停止
func (bts *BatchTransactionSender) IsStopped() bool {
	bts.mutex.RLock()
	defer bts.mutex.RUnlock()
	return bts.isStopped
}

// SubmitRequest 提交交易请求
func (bts *BatchTransactionSender) SubmitRequest(req *TransactionRequest) {
	bts.mutex.RLock()
	defer bts.mutex.RUnlock()

	if bts.isStopped {
		return
	}

	select {
	case bts.requestChan <- req:
	case <-bts.stopChan:
	default:
		// 如果channel满了，丢弃请求
		fmt.Printf("Warning: Request channel is full, dropping request %s\n", req.RequestID)
	}
}

// GetResponseChan 获取响应通道
func (bts *BatchTransactionSender) GetResponseChan() <-chan *TransactionResponse {
	return bts.responseChan
}

// worker 工作协程
func (bts *BatchTransactionSender) worker(workerID int) {
	defer bts.wg.Done()

	for {
		select {
		case req := <-bts.requestChan:
			if req == nil {
				return
			}

			// 处理交易请求
			response, err := bts.sender.SendTransactionFromPackage(
				req.Package,
				req.PrivateKey,
				req.ChainID,
				req.GasLimit,
				req.GasPrice,
			)

			if err != nil {
				response = &TransactionResponse{
					RequestID: req.RequestID,
					Success:   false,
					Error:     err,
					SentAt:    time.Now(),
				}
			} else {
				response.RequestID = req.RequestID
			}

			// 发送响应
			select {
			case bts.responseChan <- response:
			case <-bts.stopChan:
				return
			default:
				// 如果response channel满了，记录日志但不阻塞
				fmt.Printf("Warning: Response channel is full, dropping response for request %s\n", req.RequestID)
			}

		case <-bts.stopChan:
			return
		}
	}
}

// CreateTransactionRequest 创建交易请求的辅助函数
func CreateTransactionRequest(
	pkg *TransactionPackage,
	privateKey string,
	chainID *big.Int,
	gasLimit uint64,
	gasPrice *big.Int,
	nonce uint64,
) *TransactionRequest {
	return &TransactionRequest{
		Package:     pkg,
		PrivateKey:  privateKey,
		ChainID:     chainID,
		GasLimit:    gasLimit,
		GasPrice:    gasPrice,
		Nonce:       nonce,
		RequestID:   fmt.Sprintf("req_%d", time.Now().UnixNano()),
		RequestedAt: time.Now(),
	}
}
