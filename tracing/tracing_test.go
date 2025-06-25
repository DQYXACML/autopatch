package tracing

import (
	"github.com/DQYXACML/autopatch/bindings"
	"github.com/ethereum/go-ethereum/common"
	"log"

	"testing"
)

func TestRelayTx(t *testing.T) {
	replayer, err := NewTransactionReplayer("https://lb.drpc.org/ogrpc?network=holesky&dkey=Avduh2iIjEAksBUYtd4wP1NUPObEnwYR76WEFhW5UfFk")
	if err != nil {
		log.Fatal(err)
	}
	contractAddr := common.HexToAddress("0xD38BdaC0C7059ca14226DD5AC4dC994597ea428A")
	txHash := common.HexToHash("0xb7577fe30a648ab9f20f4d322b66080bb7e393abb0dcba6a96d928624f6d9432")

	// 使用新的动态生成存储布局方法，替代手动定义
	// layout := &StorageLayout{
	// 	Variables: []StorageVariable{
	// 		{Name: "int1", Type: "int8", Slot: 0, Offset: 0, Size: 1, IsSigned: true},
	// 		{Name: "int2", Type: "int128", Slot: 0, Offset: 1, Size: 16, IsSigned: true},
	// 	},
	// }
	// replayer.AddStorageLayout(contractAddr, layout)

	// 新方法：从ABI动态生成存储布局
	err = replayer.GenerateStorageLayoutFromABI(contractAddr, bindings.StorageScanMetaData)
	if err != nil {
		log.Printf("Warning: Failed to generate storage layout from ABI: %v", err)
		log.Printf("Falling back to predefined layout for StorageScan contract...")

		// 如果动态生成失败，使用预定义的 StorageScan 布局作为后备
		replayer.CreateStorageLayoutForStorageScan(contractAddr)
	} else {
		log.Printf("Successfully generated storage layout from ABI")
	}

	err = replayer.AddAdvancedModifierFromBinding(contractAddr, bindings.StorageScanMetaData)
	if err != nil {
		log.Fatalf("Failed to add advanced modifier: %v", err)
		return
	}

	setInt1Mod := &FunctionModification{
		FunctionName: "setInt1",
		ParameterMods: []ParameterModification{
			{
				ParameterIndex: 0,
				ParameterName:  "_v",
				NewValue:       9,
			},
		},
	}
	err = replayer.AddFunctionModification(contractAddr, setInt1Mod)
	if err != nil {
		log.Fatalf("Failed to add function modification: %v", err)
		return
	}

	err = replayer.ReplayTransaction(txHash)
	if err != nil {
		log.Fatalf("Failed to replay transaction: %v", err)
		return
	}

	log.Println("=== POST-EXECUTION TRACE ANALYSIS ===")

	// Get execution paths
	paths := replayer.GetExecutionTrace()
	log.Printf("Captured %d execution paths\n", len(paths))
}

// 新增：测试动态存储布局生成功能
func TestDynamicStorageLayoutGeneration(t *testing.T) {
	replayer, err := NewTransactionReplayer("https://lb.drpc.org/ogrpc?network=holesky&dkey=Avduh2iIjEAksBUYtd4wP1NUPObEnwYR76WEFhW5UfFk")
	if err != nil {
		t.Fatal(err)
	}

	contractAddr := common.HexToAddress("0xD38BdaC0C7059ca14226DD5AC4dC994597ea428A")

	// 测试从ABI生成存储布局
	err = replayer.GenerateStorageLayoutFromABI(contractAddr, bindings.StorageScanMetaData)
	if err != nil {
		t.Fatalf("Failed to generate storage layout from ABI: %v", err)
	}

	// 验证生成的布局
	layout := replayer.storageLayouts[contractAddr]
	if layout == nil {
		t.Fatal("Storage layout was not generated")
	}

	log.Printf("Generated storage layout with %d variables:", len(layout.Variables))
	for i, variable := range layout.Variables {
		log.Printf("  Variable %d: %s (%s) - Slot: %d, Offset: %d, Size: %d, IsSigned: %t",
			i+1, variable.Name, variable.Type, variable.Slot, variable.Offset, variable.Size, variable.IsSigned)
	}

	// 验证特定变量是否存在
	expectedVariables := []string{"addr1", "b1", "b2", "b3", "bool1", "bool2", "int1", "int2", "int3", "string1", "string2", "uint1", "uint2", "uint3"}
	foundVariables := make(map[string]bool)

	for _, variable := range layout.Variables {
		foundVariables[variable.Name] = true
	}

	for _, expected := range expectedVariables {
		if !foundVariables[expected] {
			log.Printf("Warning: Expected variable '%s' not found in generated layout", expected)
		} else {
			log.Printf("✓ Found expected variable: %s", expected)
		}
	}
}

// 新增：比较动态生成和手动定义的布局
func TestCompareStorageLayouts(t *testing.T) {
	replayer, err := NewTransactionReplayer("https://lb.drpc.org/ogrpc?network=holesky&dkey=Avduh2iIjEAksBUYtd4wP1NUPObEnwYR76WEFhW5UfFk")
	if err != nil {
		t.Fatal(err)
	}

	contractAddr1 := common.HexToAddress("0xD38BdaC0C7059ca14226DD5AC4dC994597ea428A")
	contractAddr2 := common.HexToAddress("0xD38BdaC0C7059ca14226DD5AC4dC994597ea428B") // 不同地址用于比较

	// 动态生成布局
	err = replayer.GenerateStorageLayoutFromABI(contractAddr1, bindings.StorageScanMetaData)
	if err != nil {
		t.Fatalf("Failed to generate storage layout from ABI: %v", err)
	}

	// 使用预定义布局
	replayer.CreateStorageLayoutForStorageScan(contractAddr2)

	// 获取两个布局进行比较
	dynamicLayout := replayer.storageLayouts[contractAddr1]
	predefinedLayout := replayer.storageLayouts[contractAddr2]

	if dynamicLayout == nil || predefinedLayout == nil {
		t.Fatal("One or both layouts are missing")
	}

	log.Printf("Dynamic layout variables: %d", len(dynamicLayout.Variables))
	log.Printf("Predefined layout variables: %d", len(predefinedLayout.Variables))

	// 创建映射以便比较
	dynamicVars := make(map[string]StorageVariable)
	predefinedVars := make(map[string]StorageVariable)

	for _, v := range dynamicLayout.Variables {
		dynamicVars[v.Name] = v
	}

	for _, v := range predefinedLayout.Variables {
		predefinedVars[v.Name] = v
	}

	// 比较共同的变量
	for name, dynamicVar := range dynamicVars {
		if predefinedVar, exists := predefinedVars[name]; exists {
			if dynamicVar.Type == predefinedVar.Type &&
				dynamicVar.Slot == predefinedVar.Slot &&
				dynamicVar.Offset == predefinedVar.Offset &&
				dynamicVar.Size == predefinedVar.Size &&
				dynamicVar.IsSigned == predefinedVar.IsSigned {
				log.Printf("✓ Variable '%s' matches between dynamic and predefined layouts", name)
			} else {
				log.Printf("❌ Variable '%s' differs:", name)
				log.Printf("  Dynamic:    Type:%s, Slot:%d, Offset:%d, Size:%d, IsSigned:%t",
					dynamicVar.Type, dynamicVar.Slot, dynamicVar.Offset, dynamicVar.Size, dynamicVar.IsSigned)
				log.Printf("  Predefined: Type:%s, Slot:%d, Offset:%d, Size:%d, IsSigned:%t",
					predefinedVar.Type, predefinedVar.Slot, predefinedVar.Offset, predefinedVar.Size, predefinedVar.IsSigned)
			}
		} else {
			log.Printf("⚠ Variable '%s' found in dynamic layout but not in predefined layout", name)
		}
	}

	// 检查预定义布局中是否有动态布局没有的变量
	for name := range predefinedVars {
		if _, exists := dynamicVars[name]; !exists {
			log.Printf("⚠ Variable '%s' found in predefined layout but not in dynamic layout", name)
		}
	}
}
