# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

AutoPatch is an Ethereum transaction analysis and replay system that detects attack patterns and generates protection mechanisms. The system monitors blockchain transactions, replays them with mutations, and creates on-chain protection rules.

## Core Architecture

The codebase is organized into several key components:

### Main Components
- **autopatch.go**: Entry point that orchestrates the synchronizer, storage parser, and transaction tracing
- **cmd/autopatch/**: CLI application with an "index" command that runs the main indexing service
- **tracing/**: Core transaction replay and mutation analysis engine
- **synchronizer/**: Blockchain synchronization and node communication
- **database/**: PostgreSQL data layer with GORM for attack transactions and protected contracts
- **storage/**: Storage pattern analysis and parsing
- **txmgr/**: Transaction management and Ethereum interaction utilities

### Key Subsystems

**Transaction Replay Engine (tracing/)**:
- `AttackReplayer`: Main component that replays attack transactions with mutations
- `ExecutionEngine`: Handles transaction execution with tracing capabilities  
- `MutationManager`: Generates step-based modifications to transaction inputs/storage
- `StateManager`: Manages EVM state creation and manipulation
- `PrestateManager`: Handles transaction pre-state extraction

**Blockchain Integration**:
- Uses go-ethereum for EVM interaction and transaction processing
- Supports multiple RPC endpoints for transaction tracing and state access
- Integrates with PostgreSQL for persistent storage of analysis results

## Development Commands

### Build
```bash
make autopatch          # Build the main binary
go build ./cmd/autopatch # Alternative build command
```

### Testing
```bash
make test               # Run all tests
go test -v ./...       # Verbose test output
go test ./tracing/...  # Test specific package
```

### Linting
```bash
make lint              # Run golangci-lint on all packages
golangci-lint run ./...
```

### Smart Contract Bindings
```bash
make bindings          # Generate Go bindings from ABI artifacts
make binding-vrf       # Generate StorageScan contract bindings
```

## Configuration

The system uses a configuration structure loaded via CLI flags:
- Database configurations for master/slave PostgreSQL instances
- Chain configuration including RPC URLs, starting block heights, contract addresses
- Private keys for transaction signing and contract interaction

## Running the System

```bash
./autopatch index --chain-rpc-url <RPC_URL> --chain-id <CHAIN_ID> --starting-height <HEIGHT>
```

The main workflow:
1. Synchronizer monitors blockchain for new transactions
2. Storage parser analyzes contract storage patterns  
3. AttackReplayer replays suspicious transactions with mutations
4. Protection rules are generated and can be deployed on-chain

## Testing Framework

The tracing package includes comprehensive tests in `txReplayer_test.go` that demonstrate:
- Transaction replay with mutation collection
- Mutation transaction sending to contracts
- Integration with real blockchain networks (BSC, Holesky testnet)

## Database Schema

Key entities:
- `AttackTx`: Records of detected attack transactions
- `ProtectedTx`: Protected transaction patterns
- `ProtectedStorage`: Storage-level protection rules
- Block and address tracking for synchronization

## Important Notes

- The system requires PostgreSQL for persistent storage
- EVM tracing capabilities require archive node access or debug API support
- Transaction mutation testing can generate significant network traffic
- Private key management is required for sending protection transactions