# 测试配置使用指南

## 概述
测试配置已从硬编码改为基于 YAML 文件的配置方式，支持多场景和动态管理。

## 配置文件位置
- 主配置文件: `conf/test_config.yaml`
- 示例配置: `conf/test_config.example.yaml`

## 配置结构
```yaml
# 默认配置
default:
  rpcURL: "..."
  dbConfig:
    host: localhost
    port: 5432
    ...
  txHash: "..."
  contractAddress: "..."
  protectContractAddr: "..."

# 场景配置
scenarios:
  holesky:
    ...
  bsc:
    ...
  local:
    ...

# 测试用例列表（可选）
testCases:
  - name: "测试名称"
    description: "测试描述"
    ...
```

## 使用方法

### 1. 使用默认配置
```bash
go test ./tracing
```

### 2. 指定配置文件
```bash
AUTOPATCH_TEST_CONFIG=conf/test_config.yaml go test ./tracing
```

### 3. 使用特定场景
```bash
go test ./tracing -test-scenario=bsc
```

### 4. 命令行参数覆盖
```bash
go test ./tracing -test-rpc-url="http://localhost:8545"
```

## 配置优先级
1. 命令行参数
2. 环境变量
3. 配置文件
4. 内置默认值

## 从 JSON 迁移到 YAML
使用提供的迁移工具：
```bash
go run cmd/json2yaml/main.go -input test_config.json -output conf/test_config.yaml
```

## 兼容性
- 同时支持 JSON 和 YAML 格式
- 自动检测配置文件格式
- 向后兼容现有测试代码