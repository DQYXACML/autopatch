package config

import (
	"os"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// 测试加载YAML配置文件
	t.Run("LoadYAMLConfig", func(t *testing.T) {
		// 设置环境变量指向YAML配置文件
		os.Setenv("AUTOPATCH_TEST_CONFIG", "../conf/test_config.yaml")
		defer os.Unsetenv("AUTOPATCH_TEST_CONFIG")

		config, err := LoadTestConfig()
		if err != nil {
			t.Fatalf("Failed to load YAML config: %v", err)
		}

		// 验证默认配置加载
		if config.RPCURL == "" {
			t.Error("RPC URL should not be empty")
		}
		if config.DBConfig.Host == "" {
			t.Error("DB Host should not be empty")
		}
		if config.TxHash == "" {
			t.Error("TxHash should not be empty")
		}

		t.Logf("Successfully loaded config: RPC=%s, DB=%s:%d", 
			config.RPCURL, config.DBConfig.Host, config.DBConfig.Port)
	})

	// 测试加载特定场景配置
	t.Run("LoadScenarioConfig", func(t *testing.T) {
		os.Setenv("AUTOPATCH_TEST_CONFIG", "../conf/test_config.yaml")
		defer os.Unsetenv("AUTOPATCH_TEST_CONFIG")

		config, err := LoadTestConfigForScenario("bsc")
		if err != nil {
			t.Fatalf("Failed to load scenario config: %v", err)
		}

		// BSC场景应该有不同的数据库配置
		if config.DBConfig.Host != "172.23.216.120" {
			t.Errorf("Expected BSC DB host to be 172.23.216.120, got %s", config.DBConfig.Host)
		}
	})

	// 测试自动检测配置文件
	t.Run("AutoDetectConfig", func(t *testing.T) {
		// 不设置环境变量，让它自动检测
		os.Unsetenv("AUTOPATCH_TEST_CONFIG")

		config, err := LoadTestConfig()
		if err != nil {
			t.Fatalf("Failed to auto-detect config: %v", err)
		}

		if config.RPCURL == "" {
			t.Error("RPC URL should not be empty with auto-detected config")
		}
	})

	// 测试JSON配置文件兼容性
	t.Run("LoadJSONConfig", func(t *testing.T) {
		if _, err := os.Stat("../test_config.example.json"); err == nil {
			os.Setenv("AUTOPATCH_TEST_CONFIG", "../test_config.example.json")
			defer os.Unsetenv("AUTOPATCH_TEST_CONFIG")

			config, err := LoadTestConfig()
			if err != nil {
				t.Fatalf("Failed to load JSON config: %v", err)
			}

			if config.RPCURL == "" {
				t.Error("RPC URL should not be empty from JSON config")
			}
			t.Log("JSON config compatibility confirmed")
		} else {
			t.Skip("JSON config file not found, skipping compatibility test")
		}
	})
}