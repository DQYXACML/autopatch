package config

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"gopkg.in/yaml.v3"
)

// TestConfig 测试配置结构
type TestConfig struct {
	// RPC 配置
	RPCURL string `json:"rpcURL" yaml:"rpcURL"`

	// 数据库配置
	DBConfig DBConfig `json:"dbConfig" yaml:"dbConfig"`

	// 测试相关配置
	TxHash              string `json:"txHash" yaml:"txHash"`
	ContractAddress     string `json:"contractAddress" yaml:"contractAddress"`
	ProtectContractAddr string `json:"protectContractAddr" yaml:"protectContractAddr"`

	// 测试场景名称
	Scenario string `json:"scenario" yaml:"scenario"`
}

// TestConfigFile 测试配置文件结构
type TestConfigFile struct {
	Default   TestConfig            `json:"default" yaml:"default"`
	Scenarios map[string]TestConfig `json:"scenarios" yaml:"scenarios"`
	TestCases []TestCase            `json:"testCases" yaml:"testCases"`
}

// TestCase 测试用例结构
type TestCase struct {
	Name                string `json:"name" yaml:"name"`
	Description         string `json:"description" yaml:"description"`
	TxHash              string `json:"txHash" yaml:"txHash"`
	ContractAddress     string `json:"contractAddress" yaml:"contractAddress"`
	ProtectContractAddr string `json:"protectContractAddr" yaml:"protectContractAddr"`
}

// 默认配置值
var defaultTestConfig = TestConfig{
	RPCURL: "https://tame-fluent-sun.bsc.quiknode.pro/3bfeef12ccb71c4a015a62771a98e51733b09924",
	DBConfig: DBConfig{
		Host:     "localhost",
		Port:     5432,
		Name:     "postgres",
		User:     "root",
		Password: "postgres",
	},
	TxHash:              "0x2a65254b41b42f39331a0bcc9f893518d6b106e80d9a476b8ca3816325f4a150",
	ContractAddress:     "0x9967407a5B9177E234d7B493AF8ff4A46771BEdf",
	ProtectContractAddr: "0x95e92b09b89cf31fa9f1eca4109a85f88eb08531",
	Scenario:            "default",
}

// 命令行标志
var (
	testRPCURL              = flag.String("test-rpc-url", "", "RPC URL for testing")
	testTxHash              = flag.String("test-tx-hash", "", "Transaction hash for testing")
	testContractAddr        = flag.String("test-contract-addr", "", "Contract address for testing")
	testProtectContractAddr = flag.String("test-protect-contract-addr", "", "Protected contract address for testing")
	testDBHost              = flag.String("test-db-host", "", "Database host for testing")
	testDBPort              = flag.Int("test-db-port", 0, "Database port for testing")
	testDBName              = flag.String("test-db-name", "", "Database name for testing")
	testDBUser              = flag.String("test-db-user", "", "Database user for testing")
	testDBPassword          = flag.String("test-db-password", "", "Database password for testing")
	testScenario            = flag.String("test-scenario", "", "Test scenario name")
)

// LoadTestConfig 加载测试配置
// 优先级：命令行参数 > 环境变量 > 配置文件 > 默认值
func LoadTestConfig() (*TestConfig, error) {
	// 解析命令行参数（如果还没有解析过）
	if !flag.Parsed() {
		flag.Parse()
	}

	// 从默认配置开始
	config := defaultTestConfig

	// 1. 尝试从配置文件加载
	configPath := os.Getenv("AUTOPATCH_TEST_CONFIG")
	if configPath == "" {
		// 尝试默认路径
		defaultPaths := []string{
			"conf/test_config.yaml",
			"conf/test_config.yml",
			"test_config.yaml",
			"test_config.yml",
			"test_config.json",
		}
		for _, path := range defaultPaths {
			if _, err := os.Stat(path); err == nil {
				configPath = path
				break
			}
		}
	}

	if configPath != "" {
		fileConfig, err := loadConfigFromFile(configPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load config file: %v", err)
		}

		// 如果指定了场景，使用场景配置
		scenario := getStringValue(*testScenario, "AUTOPATCH_TEST_SCENARIO", "")
		if scenario != "" && scenario != "default" {
			if scenarioConfig, ok := fileConfig.Scenarios[scenario]; ok {
				mergeConfig(&config, &scenarioConfig)
			} else {
				return nil, fmt.Errorf("scenario '%s' not found in config file", scenario)
			}
		} else {
			// 使用默认配置
			mergeConfig(&config, &fileConfig.Default)
		}
	}

	// 2. 从环境变量覆盖
	config.RPCURL = getStringValue(*testRPCURL, "AUTOPATCH_TEST_RPC_URL", config.RPCURL)
	config.TxHash = getStringValue(*testTxHash, "AUTOPATCH_TEST_TX_HASH", config.TxHash)
	config.ContractAddress = getStringValue(*testContractAddr, "AUTOPATCH_TEST_CONTRACT_ADDRESS", config.ContractAddress)
	config.ProtectContractAddr = getStringValue(*testProtectContractAddr, "AUTOPATCH_TEST_PROTECT_CONTRACT_ADDRESS", config.ProtectContractAddr)

	// 数据库配置
	config.DBConfig.Host = getStringValue(*testDBHost, "AUTOPATCH_TEST_DB_HOST", config.DBConfig.Host)
	config.DBConfig.Port = getIntValue(*testDBPort, "AUTOPATCH_TEST_DB_PORT", config.DBConfig.Port)
	config.DBConfig.Name = getStringValue(*testDBName, "AUTOPATCH_TEST_DB_NAME", config.DBConfig.Name)
	config.DBConfig.User = getStringValue(*testDBUser, "AUTOPATCH_TEST_DB_USER", config.DBConfig.User)
	config.DBConfig.Password = getStringValue(*testDBPassword, "AUTOPATCH_TEST_DB_PASSWORD", config.DBConfig.Password)

	return &config, nil
}

// LoadTestConfigForScenario 为特定场景加载测试配置
func LoadTestConfigForScenario(scenario string) (*TestConfig, error) {
	*testScenario = scenario
	return LoadTestConfig()
}

// GetTxHash 获取交易哈希
func (tc *TestConfig) GetTxHash() common.Hash {
	return common.HexToHash(tc.TxHash)
}

// GetContractAddress 获取合约地址
func (tc *TestConfig) GetContractAddress() common.Address {
	return common.HexToAddress(tc.ContractAddress)
}

// GetProtectContractAddress 获取保护合约地址
func (tc *TestConfig) GetProtectContractAddress() common.Address {
	return common.HexToAddress(tc.ProtectContractAddr)
}

// GetDSN 获取数据库连接字符串
func (tc *TestConfig) GetDSN() string {
	return fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=disable",
		tc.DBConfig.Host,
		tc.DBConfig.User,
		tc.DBConfig.Password,
		tc.DBConfig.Name,
		tc.DBConfig.Port,
	)
}

// 辅助函数

func loadConfigFromFile(path string) (*TestConfigFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config TestConfigFile
	ext := strings.ToLower(filepath.Ext(path))

	switch ext {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &config); err != nil {
			return nil, fmt.Errorf("failed to parse YAML: %v", err)
		}
	case ".json":
		if err := json.Unmarshal(data, &config); err != nil {
			return nil, fmt.Errorf("failed to parse JSON: %v", err)
		}
	default:
		return nil, fmt.Errorf("unsupported config file format: %s", ext)
	}

	return &config, nil
}

func mergeConfig(target, source *TestConfig) {
	if source.RPCURL != "" {
		target.RPCURL = source.RPCURL
	}
	if source.TxHash != "" {
		target.TxHash = source.TxHash
	}
	if source.ContractAddress != "" {
		target.ContractAddress = source.ContractAddress
	}
	if source.ProtectContractAddr != "" {
		target.ProtectContractAddr = source.ProtectContractAddr
	}
	if source.Scenario != "" {
		target.Scenario = source.Scenario
	}

	// 合并数据库配置
	if source.DBConfig.Host != "" {
		target.DBConfig.Host = source.DBConfig.Host
	}
	if source.DBConfig.Port != 0 {
		target.DBConfig.Port = source.DBConfig.Port
	}
	if source.DBConfig.Name != "" {
		target.DBConfig.Name = source.DBConfig.Name
	}
	if source.DBConfig.User != "" {
		target.DBConfig.User = source.DBConfig.User
	}
	if source.DBConfig.Password != "" {
		target.DBConfig.Password = source.DBConfig.Password
	}
}

func getStringValue(flagValue, envVar, defaultValue string) string {
	// 优先使用命令行参数
	if flagValue != "" {
		return flagValue
	}
	// 其次使用环境变量
	if envValue := os.Getenv(envVar); envValue != "" {
		return envValue
	}
	// 最后使用默认值
	return defaultValue
}

func getIntValue(flagValue int, envVar string, defaultValue int) int {
	// 优先使用命令行参数
	if flagValue != 0 {
		return flagValue
	}
	// 其次使用环境变量
	if envValue := os.Getenv(envVar); envValue != "" {
		if intValue, err := strconv.Atoi(envValue); err == nil {
			return intValue
		}
	}
	// 最后使用默认值
	return defaultValue
}
