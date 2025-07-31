# CLAUDE.md
用中文交流
This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

AutoPatch is an Ethereum transaction tracing and analysis tool that:
- Traces and analyzes Ethereum transactions using debug APIs
- Synchronizes blockchain data and stores it in PostgreSQL  
- Parses and analyzes smart contract storage
- Provides transaction replay and mutation capabilities

## Commands

### Build
```bash
make autopatch              # Build the main autopatch binary
make bindings               # Generate Go bindings from Solidity ABI
```

### Testing
```bash
make test                   # Run all tests with verbose output
go test ./tracing -v        # Run tests for a specific package
```

### Linting
```bash
make lint                   # Run golangci-lint on all Go files
```

### Clean
```bash
make clean                  # Remove built binaries
```

### Run the Application
```bash
./autopatch index           # Run the indexing service
./autopatch version         # Print version information
```

## Architecture

### Core Components

1. **AutoPatch** (`autopatch.go`): Main application coordinator that manages:
   - Database connections
   - Synchronizer for blockchain data
   - Storage parser for contract analysis
   - Lifecycle management (Start/Stop)

2. **Tracing Module** (`tracing/`): Transaction analysis engine
   - `txReplayer.go`: Replays transactions with modifications
   - `execution_engine.go`: Executes transactions in simulated environment
   - `state_manager.go`: Manages blockchain state during replay
   - `mutation_manager.go`: Handles transaction mutations
   - `call_tracer.go`: Traces EVM call stack
   - `customTracer.go`: Custom EVM opcode tracer

3. **Synchronizer** (`synchronizer/`): Blockchain data synchronization
   - Fetches headers, blocks, and transactions
   - Manages connection to Ethereum nodes
   - Handles chain reorganizations

4. **Storage Module** (`storage/`): Smart contract storage analysis
   - Parses storage layouts
   - Analyzes mappings, arrays, and structs
   - Uses StorageScan contract for on-chain queries

5. **Database** (`database/`): PostgreSQL data persistence
   - Uses GORM for ORM functionality
   - Custom serializers for Ethereum types (addresses, uint256, RLP)
   - Worker modules for different data types

### Key Design Patterns

- **Lifecycle Management**: Components implement `cliapp.Lifecycle` interface with Start/Stop methods
- **Configuration**: Centralized config loading via flags and config files
- **Error Handling**: Consistent error propagation with context
- **Logging**: Uses go-ethereum's structured logging throughout

### Database Schema

The application uses PostgreSQL with migrations in `migrations/`. Key tables include:
- Block and transaction data
- Protected addresses and storage
- Attack transaction analysis results

### External Dependencies

- `go-ethereum`: Core Ethereum libraries
- `gorm`: Database ORM
- `urfave/cli`: Command-line interface
- Custom StorageScan Solidity contract for on-chain storage queries
