package mutation

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	abiPkg "github.com/DQYXACML/autopatch/tracing/abi"
)

// TypeAwareMutator Type-aware mutator
type TypeAwareMutator struct {
	chainID    *big.Int
	abiManager *abiPkg.ABIManager
}

// NewTypeAwareMutator Create type-aware mutator
func NewTypeAwareMutator(chainID *big.Int, abiManager *abiPkg.ABIManager) *TypeAwareMutator {
	return &TypeAwareMutator{
		chainID:    chainID,
		abiManager: abiManager,
	}
}

// MutateByType Mutate parameter according to ABI type
func (m *TypeAwareMutator) MutateByType(argType abi.Type, originalValue interface{}, variant int) (interface{}, error) {
	switch argType.T {
	case abi.AddressTy:
		return m.mutateAddress(originalValue.(common.Address), variant)
	case abi.UintTy:
		return m.mutateUint(originalValue, argType.Size, variant)
	case abi.IntTy:
		return m.mutateInt(originalValue, argType.Size, variant)
	case abi.BoolTy:
		return m.mutateBool(originalValue.(bool)), nil
	case abi.StringTy:
		return m.mutateString(originalValue.(string), variant), nil
	case abi.BytesTy:
		return m.mutateBytes(originalValue.([]byte), variant), nil
	case abi.FixedBytesTy:
		return m.mutateFixedBytes(originalValue, argType.Size, variant), nil
	case abi.ArrayTy, abi.SliceTy:
		return m.mutateArray(originalValue, argType, variant)
	default:
		fmt.Printf("⚠️  Unknown type for mutation: %s, using original value\n", argType.String())
		return originalValue, nil
	}
}

// mutateAddress Address mutation strategies
func (m *TypeAwareMutator) mutateAddress(addr common.Address, variant int) (common.Address, error) {
	strategies := []func(common.Address, int) common.Address{
		m.flipAddressBytes,     // Flip specific bytes
		m.useKnownAddress,      // Use known addresses
		m.generateNearbyAddr,   // Generate nearby addresses
		m.useZeroAddress,       // Use zero address
		m.randomizeAddress,     // Randomize address
	}

	strategy := strategies[variant%len(strategies)]
	return strategy(addr, variant), nil
}

// flipAddressBytes Flip specific bytes of address
func (m *TypeAwareMutator) flipAddressBytes(addr common.Address, variant int) common.Address {
	newAddr := addr
	bytesToFlip := []int{19, 18, 17, 16} // Start from the last byte

	flipCount := (variant % 3) + 1 // Flip 1-3 bytes
	for i := 0; i < flipCount && i < len(bytesToFlip); i++ {
		byteIndex := bytesToFlip[i]
		step := int8((variant + i) % 256)
		newAddr[byteIndex] = byte(int8(newAddr[byteIndex]) + step)
	}

	return newAddr
}

// useKnownAddress Use known common addresses on chain
func (m *TypeAwareMutator) useKnownAddress(addr common.Address, variant int) common.Address {
	var knownAddresses []common.Address

	switch m.chainID.Int64() {
	case 1: // Ethereum
		knownAddresses = []common.Address{
			common.HexToAddress("0x0000000000000000000000000000000000000000"), // Zero address
			common.HexToAddress("0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2"), // WETH
			common.HexToAddress("0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48"), // USDC
			common.HexToAddress("0xdAC17F958D2ee523a2206206994597C13D831ec7"), // USDT
			common.HexToAddress("0x7a250d5630B4cF539739dF2C5dAcb4c659F2488D"), // Uniswap Router
		}
	case 56: // BSC
		knownAddresses = []common.Address{
			common.HexToAddress("0x0000000000000000000000000000000000000000"), // Zero address
			common.HexToAddress("0xbb4CdB9CBd36B01bD1cBaEBF2De08d9173bc095c"), // WBNB
			common.HexToAddress("0xe9e7CEA3DedcA5984780Bafc599bD69ADd087D56"), // BUSD
			common.HexToAddress("0x10ED43C718714eb63d5aA57B78B54704E256024E"), // PancakeSwap Router
			common.HexToAddress("0x8894E0a0c962CB723c1976a4421c95949bE2D4E3"), // BETO Token
		}
	default:
		// Default address pool
		knownAddresses = []common.Address{
			common.HexToAddress("0x0000000000000000000000000000000000000000"),
			common.HexToAddress("0x1111111111111111111111111111111111111111"),
		}
	}

	if len(knownAddresses) == 0 {
		return addr
	}

	return knownAddresses[variant%len(knownAddresses)]
}

// generateNearbyAddr Generate nearby addresses
func (m *TypeAwareMutator) generateNearbyAddr(addr common.Address, variant int) common.Address {
	newAddr := addr
	step := int64(variant%1000 + 1) // Step size 1-1000

	// Convert address to big.Int for arithmetic
	addrBig := new(big.Int).SetBytes(addr.Bytes())
	
	if variant%2 == 0 {
		addrBig.Add(addrBig, big.NewInt(step))
	} else {
		addrBig.Sub(addrBig, big.NewInt(step))
	}

	// Ensure not exceeding address range
	maxAddr := new(big.Int).Lsh(big.NewInt(1), 160) // 2^160
	addrBig.Mod(addrBig, maxAddr)

	copy(newAddr[:], addrBig.Bytes())
	return newAddr
}

// useZeroAddress Use zero address
func (m *TypeAwareMutator) useZeroAddress(addr common.Address, variant int) common.Address {
	return common.Address{}
}

// randomizeAddress Randomize address
func (m *TypeAwareMutator) randomizeAddress(addr common.Address, variant int) common.Address {
	newAddr := common.Address{}
	rand.Read(newAddr[:])
	return newAddr
}

// mutateUint Unsigned integer mutation
func (m *TypeAwareMutator) mutateUint(value interface{}, size int, variant int) (interface{}, error) {
	switch v := value.(type) {
	case *big.Int:
		return m.mutateBigUint(v, variant), nil
	case uint8:
		return m.mutateUint8(v, variant), nil
	case uint16:
		return m.mutateUint16(v, variant), nil
	case uint32:
		return m.mutateUint32(v, variant), nil
	case uint64:
		return m.mutateUint64(v, variant), nil
	default:
		return value, fmt.Errorf("unsupported uint type: %T", value)
	}
}

// mutateBigUint Big integer mutation
func (m *TypeAwareMutator) mutateBigUint(value *big.Int, variant int) *big.Int {
	strategies := []func(*big.Int, int) *big.Int{
		m.addStepToBigInt,       // Step increment
		m.multiplyBigInt,        // Multiplication mutation
		m.setBoundaryValue,      // Boundary values
		m.setBitPatterns,        // Bit patterns
		m.setSpecialValues,      // Special values
	}

	strategy := strategies[variant%len(strategies)]
	return strategy(value, variant)
}

// addStepToBigInt Step increment
func (m *TypeAwareMutator) addStepToBigInt(value *big.Int, variant int) *big.Int {
	steps := []*big.Int{
		big.NewInt(1), big.NewInt(10), big.NewInt(100), big.NewInt(1000),
		big.NewInt(-1), big.NewInt(-10), big.NewInt(-100), big.NewInt(-1000),
	}
	
	step := steps[variant%len(steps)]
	result := new(big.Int).Add(value, step)
	
	// Ensure non-negative
	if result.Sign() < 0 {
		result = big.NewInt(0)
	}
	
	return result
}

// multiplyBigInt Multiplication mutation
func (m *TypeAwareMutator) multiplyBigInt(value *big.Int, variant int) *big.Int {
	multipliers := []*big.Int{
		big.NewInt(2), big.NewInt(10), big.NewInt(0), // 0 for clearing
	}
	
	multiplier := multipliers[variant%len(multipliers)]
	if multiplier.Sign() == 0 {
		return big.NewInt(0)
	}
	
	return new(big.Int).Mul(value, multiplier)
}

// setBoundaryValue Set boundary values
func (m *TypeAwareMutator) setBoundaryValue(value *big.Int, variant int) *big.Int {
	boundaries := []*big.Int{
		big.NewInt(0),           // Minimum value
		big.NewInt(1),           // Minimum positive value
		big.NewInt(255),         // uint8 max value
		big.NewInt(65535),       // uint16 max value
		big.NewInt(4294967295),  // uint32 max value
	}
	
	return boundaries[variant%len(boundaries)]
}

// setBitPatterns Set bit patterns
func (m *TypeAwareMutator) setBitPatterns(value *big.Int, variant int) *big.Int {
	patterns := []*big.Int{
		big.NewInt(0xFFFFFFFF),     // All 1s pattern
		big.NewInt(0xAAAAAAAA),     // Alternating pattern
		big.NewInt(0x55555555),     // Another alternating pattern
	}
	
	return patterns[variant%len(patterns)]
}

// setSpecialValues Set special values
func (m *TypeAwareMutator) setSpecialValues(value *big.Int, variant int) *big.Int {
	// Special mutations based on original value
	if value.Sign() == 0 {
		return big.NewInt(1) // 0 -> 1
	}
	
	// Powers of 2
	powers := []int64{1, 2, 4, 8, 16, 32, 64, 128, 256}
	power := powers[variant%len(powers)]
	return big.NewInt(power)
}

// mutateUint8 uint8 mutation
func (m *TypeAwareMutator) mutateUint8(value uint8, variant int) uint8 {
	strategies := []uint8{
		value + 1, value - 1, 0, 255, value ^ 0xFF, value << 1, value >> 1,
	}
	
	return strategies[variant%len(strategies)]
}

// mutateUint16 uint16 mutation  
func (m *TypeAwareMutator) mutateUint16(value uint16, variant int) uint16 {
	strategies := []uint16{
		value + 1, value - 1, 0, 65535, value ^ 0xFFFF, value << 1, value >> 1,
	}
	
	return strategies[variant%len(strategies)]
}

// mutateUint32 uint32 mutation
func (m *TypeAwareMutator) mutateUint32(value uint32, variant int) uint32 {
	strategies := []uint32{
		value + 1, value - 1, 0, 4294967295, value ^ 0xFFFFFFFF, value << 1, value >> 1,
	}
	
	return strategies[variant%len(strategies)]
}

// mutateUint64 uint64 mutation
func (m *TypeAwareMutator) mutateUint64(value uint64, variant int) uint64 {
	strategies := []uint64{
		value + 1, value - 1, 0, 18446744073709551615, value ^ 0xFFFFFFFFFFFFFFFF, value << 1, value >> 1,
	}
	
	return strategies[variant%len(strategies)]
}

// mutateInt Signed integer mutation
func (m *TypeAwareMutator) mutateInt(value interface{}, size int, variant int) (interface{}, error) {
	switch v := value.(type) {
	case *big.Int:
		return m.mutateBigInt(v, variant), nil
	case int8:
		return m.mutateInt8(v, variant), nil
	case int16:
		return m.mutateInt16(v, variant), nil
	case int32:
		return m.mutateInt32(v, variant), nil
	case int64:
		return m.mutateInt64(v, variant), nil
	default:
		return value, fmt.Errorf("unsupported int type: %T", value)
	}
}

// mutateBigInt Big signed integer mutation
func (m *TypeAwareMutator) mutateBigInt(value *big.Int, variant int) *big.Int {
	strategies := []func(*big.Int, int) *big.Int{
		m.addSignedStep,         // Signed step
		m.negateBigInt,          // Take negation
		m.setSignedBoundary,     // Signed boundary values
	}

	strategy := strategies[variant%len(strategies)]
	return strategy(value, variant)
}

// addSignedStep Signed step increment
func (m *TypeAwareMutator) addSignedStep(value *big.Int, variant int) *big.Int {
	steps := []*big.Int{
		big.NewInt(1), big.NewInt(-1), big.NewInt(10), big.NewInt(-10),
		big.NewInt(100), big.NewInt(-100), big.NewInt(1000), big.NewInt(-1000),
	}
	
	step := steps[variant%len(steps)]
	return new(big.Int).Add(value, step)
}

// negateBigInt Take negation
func (m *TypeAwareMutator) negateBigInt(value *big.Int, variant int) *big.Int {
	return new(big.Int).Neg(value)
}

// setSignedBoundary Signed boundary values
func (m *TypeAwareMutator) setSignedBoundary(value *big.Int, variant int) *big.Int {
	boundaries := []*big.Int{
		big.NewInt(-128),        // int8 min value
		big.NewInt(127),         // int8 max value
		big.NewInt(-32768),      // int16 min value
		big.NewInt(32767),       // int16 max value
		big.NewInt(-2147483648), // int32 min value
		big.NewInt(2147483647),  // int32 max value
		big.NewInt(0),           // Zero value
		big.NewInt(-1),          // -1
		big.NewInt(1),           // 1
	}
	
	return boundaries[variant%len(boundaries)]
}

// mutateInt8 int8 mutation
func (m *TypeAwareMutator) mutateInt8(value int8, variant int) int8 {
	strategies := []int8{
		value + 1, value - 1, 0, 127, -128, -value, value ^ 0x7F,
	}
	
	return strategies[variant%len(strategies)]
}

// mutateInt16 int16 mutation
func (m *TypeAwareMutator) mutateInt16(value int16, variant int) int16 {
	strategies := []int16{
		value + 1, value - 1, 0, 32767, -32768, -value, value ^ 0x7FFF,
	}
	
	return strategies[variant%len(strategies)]
}

// mutateInt32 int32 mutation
func (m *TypeAwareMutator) mutateInt32(value int32, variant int) int32 {
	strategies := []int32{
		value + 1, value - 1, 0, 2147483647, -2147483648, -value, value ^ 0x7FFFFFFF,
	}
	
	return strategies[variant%len(strategies)]
}

// mutateInt64 int64 mutation
func (m *TypeAwareMutator) mutateInt64(value int64, variant int) int64 {
	strategies := []int64{
		value + 1, value - 1, 0, 9223372036854775807, -9223372036854775808, -value, value ^ 0x7FFFFFFFFFFFFFFF,
	}
	
	return strategies[variant%len(strategies)]
}

// mutateBool Boolean value mutation
func (m *TypeAwareMutator) mutateBool(value bool) bool {
	return !value // Simple flip
}

// mutateString String mutation
func (m *TypeAwareMutator) mutateString(value string, variant int) string {
	strategies := []func(string, int) string{
		m.appendString,      // Append content
		m.prependString,     // Prepend content
		m.truncateString,    // Truncate
		m.replaceString,     // Replace
		m.emptyString,       // Clear
		m.repeatString,      // Repeat
		m.addSpecialChars,   // Add special characters
	}

	strategy := strategies[variant%len(strategies)]
	return strategy(value, variant)
}

// appendString Append content
func (m *TypeAwareMutator) appendString(value string, variant int) string {
	suffixes := []string{"_modified", "_test", "_mutated", "123", "x"}
	suffix := suffixes[variant%len(suffixes)]
	return value + suffix
}

// prependString Prepend content  
func (m *TypeAwareMutator) prependString(value string, variant int) string {
	prefixes := []string{"test_", "modified_", "x_", "0x"}
	prefix := prefixes[variant%len(prefixes)]
	return prefix + value
}

// truncateString Truncate string
func (m *TypeAwareMutator) truncateString(value string, variant int) string {
	if len(value) <= 1 {
		return ""
	}
	
	lengths := []int{0, 1, len(value)/2, len(value)-1}
	length := lengths[variant%len(lengths)]
	
	if length >= len(value) {
		return value
	}
	
	return value[:length]
}

// replaceString Replace content
func (m *TypeAwareMutator) replaceString(value string, variant int) string {
	replacements := []string{"", "test", "modified", "x", "0"}
	return replacements[variant%len(replacements)]
}

// emptyString Clear string
func (m *TypeAwareMutator) emptyString(value string, variant int) string {
	return ""
}

// repeatString Repeat string
func (m *TypeAwareMutator) repeatString(value string, variant int) string {
	if len(value) == 0 {
		return "x"
	}
	
	repeats := []int{2, 3, 10}
	repeat := repeats[variant%len(repeats)]
	
	// Limit length to avoid too long strings
	if len(value)*repeat > 1000 {
		repeat = 1000 / len(value)
		if repeat < 1 {
			repeat = 1
		}
	}
	
	return strings.Repeat(value, repeat)
}

// addSpecialChars Add special characters
func (m *TypeAwareMutator) addSpecialChars(value string, variant int) string {
	specialChars := []string{"\x00", "\n", "\r", "\t", "\"", "'", "\\", "%", "&", "<", ">"}
	char := specialChars[variant%len(specialChars)]
	return value + char
}

// mutateBytes Byte array mutation
func (m *TypeAwareMutator) mutateBytes(value []byte, variant int) []byte {
	if len(value) == 0 {
		return []byte("test")
	}

	strategies := []func([]byte, int) []byte{
		m.flipBytes,       // Flip bytes
		m.appendBytes,     // Append bytes
		m.truncateBytes,   // Truncate bytes
		m.replaceBytes,    // Replace bytes
		m.emptyBytes,      // Clear bytes
	}

	strategy := strategies[variant%len(strategies)]
	return strategy(value, variant)
}

// flipBytes Flip bytes
func (m *TypeAwareMutator) flipBytes(value []byte, variant int) []byte {
	newValue := make([]byte, len(value))
	copy(newValue, value)
	
	// Flip several bytes
	flipCount := (variant % 3) + 1
	for i := 0; i < flipCount && i < len(newValue); i++ {
		index := (variant + i) % len(newValue)
		newValue[index] ^= 0xFF
	}
	
	return newValue
}

// appendBytes Append bytes
func (m *TypeAwareMutator) appendBytes(value []byte, variant int) []byte {
	appendData := [][]byte{
		{0x00}, {0xFF}, {0xAA}, {0x55}, []byte("test"),
	}
	
	data := appendData[variant%len(appendData)]
	return append(value, data...)
}

// truncateBytes Truncate bytes
func (m *TypeAwareMutator) truncateBytes(value []byte, variant int) []byte {
	if len(value) <= 1 {
		return []byte{}
	}
	
	lengths := []int{0, 1, len(value)/2, len(value)-1}
	length := lengths[variant%len(lengths)]
	
	if length >= len(value) {
		return value
	}
	
	return value[:length]
}

// replaceBytes Replace bytes
func (m *TypeAwareMutator) replaceBytes(value []byte, variant int) []byte {
	replacements := [][]byte{
		{}, {0x00}, {0xFF}, []byte("test"), []byte("modified"),
	}
	
	return replacements[variant%len(replacements)]
}

// emptyBytes Clear bytes
func (m *TypeAwareMutator) emptyBytes(value []byte, variant int) []byte {
	return []byte{}
}

// mutateFixedBytes Fixed length byte mutation
func (m *TypeAwareMutator) mutateFixedBytes(value interface{}, size int, variant int) interface{} {
	// Determine type based on size
	switch size {
	case 1:
		if v, ok := value.([1]byte); ok {
			v[0] ^= byte(variant)
			return v
		}
	case 4:
		if v, ok := value.([4]byte); ok {
			v[variant%4] ^= byte(variant)
			return v
		}
	case 32:
		if v, ok := value.([32]byte); ok {
			v[variant%32] ^= byte(variant)
			return v
		}
	}
	
	return value
}

// mutateArray Array mutation  
func (m *TypeAwareMutator) mutateArray(value interface{}, argType abi.Type, variant int) (interface{}, error) {
	// TODO: Implement recursive mutation for arrays/slices
	// Return original value for now, can be extended later
	fmt.Printf("⚠️  Array mutation not implemented yet for type: %s\n", argType.String())
	return value, nil
}

// GetMutationStrategies Get number of mutation strategies for specified type
func (m *TypeAwareMutator) GetMutationStrategies(argType abi.Type) int {
	switch argType.T {
	case abi.AddressTy:
		return 5 // 5 address mutation strategies
	case abi.UintTy, abi.IntTy:
		return 5 // 5 numeric mutation strategies  
	case abi.BoolTy:
		return 1 // 1 boolean mutation strategy
	case abi.StringTy:
		return 7 // 7 string mutation strategies
	case abi.BytesTy:
		return 5 // 5 byte mutation strategies
	case abi.FixedBytesTy:
		return 3 // 3 fixed byte mutation strategies
	default:
		return 1 // Default 1 strategy
	}
}