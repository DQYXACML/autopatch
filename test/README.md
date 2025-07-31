# AutoPatch 测试配置指南

本文档介绍如何配置和运行 AutoPatch 的测试，特别是 `tracing` 包中的交易重放测试。

## 概述

AutoPatch 的测试现在支持灵活的配置方式，你可以通过以下方式配置测试参数：

1. **环境变量**（推荐）
2. **配置文件**
3. **命令行参数**

配置优先级：命令行参数 > 环境变量 > 配置文件 > 默认值

## 配置项说明

### 主要配置项

- `rpcURL`: 以太坊节点的 RPC URL
- `txHash`: 要重放的交易哈希
- `contractAddress`: 目标合约地址
- `protectContractAddr`: 保护合约地址
- `dbConfig`: 数据库连接配置
  - `host`: 数据库主机
  - `port`: 数据库端口
  - `name`: 数据库名称
  - `user`: 数据库用户名
  - `password`: 数据库密码

## 使用方法

### 方法 1：使用环境变量（推荐）

```bash
# 设置环境变量
export AUTOPATCH_TEST_RPC_URL="https://lb.drpc.org/holesky/..."
export AUTOPATCH_TEST_TX_HASH="0x2a65254b41b42f39331a0bcc9f893518d6b106e80d9a476b8ca3816325f4a150"
export AUTOPATCH_TEST_CONTRACT_ADDRESS="0x9967407a5B9177E234d7B493AF8ff4A46771BEdf"
export AUTOPATCH_TEST_PROTECT_CONTRACT_ADDRESS="0x95e92b09b89cf31fa9f1eca4109a85f88eb08531"
export AUTOPATCH_TEST_DB_HOST="localhost"
export AUTOPATCH_TEST_DB_PORT="5432"
export AUTOPATCH_TEST_DB_NAME="postgres"
export AUTOPATCH_TEST_DB_USER="postgres"
export AUTOPATCH_TEST_DB_PASSWORD="postgres"

# 运行测试
go test -v ./tracing
```

### 方法 2：使用配置文件

1. 复制示例配置文件：
```bash
cp test_config.example.json test_config.json
```

2. 编辑 `test_config.json`，修改为你的配置值

3. 运行测试：
```bash
# 使用默认配置
export AUTOPATCH_TEST_CONFIG="./test_config.json"
go test -v ./tracing

# 使用特定场景配置
export AUTOPATCH_TEST_CONFIG="./test_config.json"
export AUTOPATCH_TEST_SCENARIO="holesky"
go test -v ./tracing
```

### 方法 3：使用命令行参数

```bash
go test -v ./tracing -args \
  -test-rpc-url="https://..." \
  -test-tx-hash="0x..." \
  -test-contract-addr="0x..." \
  -test-db-host="localhost"
```

### 方法 4：混合使用

你可以组合使用多种方式，例如：

```bash
# 使用配置文件作为基础，通过环境变量覆盖特定值
export AUTOPATCH_TEST_CONFIG="./test_config.json"
export AUTOPATCH_TEST_TX_HASH="0x1234..."  # 覆盖配置文件中的交易哈希
go test -v ./tracing
```

## 运行特定测试

### 测试交易重放和变异

```bash
# 完整测试（包括发送交易）
go test -v -run TestRelayTx ./tracing

# 只测试重放和收集变异（不发送交易）
go test -v -run TestRelayTxWithoutSending ./tracing

# 只测试交易发送功能
go test -v -run TestTransactionSendingOnly ./tracing
```

## 配置文件格式

配置文件使用 JSON 格式，支持多个测试场景：

```json
{
  "default": {
    "rpcURL": "https://...",
    "dbConfig": {
      "host": "localhost",
      "port": 5432,
      "name": "postgres",
      "user": "postgres",
      "password": "postgres"
    },
    "txHash": "0x...",
    "contractAddress": "0x...",
    "protectContractAddr": "0x..."
  },
  "scenarios": {
    "holesky": { ... },
    "bsc": { ... },
    "local": { ... }
  }
}
```

## 最佳实践

1. **生产环境测试**：使用环境变量，避免在代码中暴露敏感信息
   ```bash
   export AUTOPATCH_TEST_DB_PASSWORD="${SECURE_PASSWORD}"
   ```

2. **本地开发**：创建 `.env` 文件（记得添加到 `.gitignore`）
   ```bash
   source .env
   go test -v ./tracing
   ```

3. **CI/CD**：在 CI 环境中设置环境变量或使用安全的配置管理

4. **多环境测试**：使用配置文件的场景功能
   ```bash
   # 测试 Holesky 网络
   export AUTOPATCH_TEST_SCENARIO="holesky"
   go test -v ./tracing

   # 测试 BSC 网络
   export AUTOPATCH_TEST_SCENARIO="bsc"
   go test -v ./tracing
   ```

## 故障排除

### 常见问题

1. **数据库连接失败**
   - 检查数据库是否运行
   - 验证连接参数是否正确
   - 确保数据库用户有适当的权限

2. **RPC 连接失败**
   - 验证 RPC URL 是否正确
   - 检查网络连接
   - 确认 RPC 节点是否支持所需的 API

3. **交易不存在**
   - 确认交易哈希在指定的网络上存在
   - 检查 RPC 节点是否同步到包含该交易的区块

### 调试技巧

1. 打印当前配置：
```go
config, _ := LoadTestConfig()
fmt.Printf("Current config: %+v\n", config)
```

2. 检查环境变量：
```bash
env | grep AUTOPATCH_TEST
```

3. 使用详细日志：
```bash
go test -v ./tracing -count=1
```

## 示例脚本

### 快速测试脚本

创建 `test.sh`：

```bash
#!/bin/bash

# 加载本地配置
if [ -f .env ]; then
    source .env
fi

# 设置默认值
export AUTOPATCH_TEST_RPC_URL="${AUTOPATCH_TEST_RPC_URL:-https://lb.drpc.org/holesky/...}"
export AUTOPATCH_TEST_DB_HOST="${AUTOPATCH_TEST_DB_HOST:-localhost}"
export AUTOPATCH_TEST_DB_PORT="${AUTOPATCH_TEST_DB_PORT:-5432}"
export AUTOPATCH_TEST_DB_NAME="${AUTOPATCH_TEST_DB_NAME:-postgres}"
export AUTOPATCH_TEST_DB_USER="${AUTOPATCH_TEST_DB_USER:-postgres}"
export AUTOPATCH_TEST_DB_PASSWORD="${AUTOPATCH_TEST_DB_PASSWORD:-postgres}"

# 运行测试
echo "Running AutoPatch tests..."
go test -v ./tracing -count=1
```

使用：
```bash
chmod +x test.sh
./test.sh
```

## 更多信息

- 查看 `tracing/test_config.go` 了解配置加载的实现细节
- 参考 `test_config.example.json` 了解完整的配置选项
- 查看项目的 `CLAUDE.md` 了解更多项目信息