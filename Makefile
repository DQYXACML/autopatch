StorageScan_ABI_ARTIFACT := ./abis/StorageScan.sol/StorageScan.json


autopatch:
	env GO111MODULE=on go build  ./cmd/autopatch

clean:
	rm auto-patch

test:
	go test -v ./...

# 测试重放相关命令
test-replay:
	go test -v -run TestRelayTx ./tracing -count=1

test-replay-nosend:
	go test -v -run TestRelayTxWithoutSending ./tracing -count=1

test-send:
	go test -v -run TestTransactionSendingOnly ./tracing -count=1

test-tracing:
	go test -v ./tracing -count=1

# 使用配置文件运行测试
test-with-config:
	AUTOPATCH_TEST_CONFIG=./test_config.json go test -v ./tracing -count=1

# 使用特定场景运行测试
test-holesky:
	AUTOPATCH_TEST_CONFIG=./test_config.json AUTOPATCH_TEST_SCENARIO=holesky go test -v ./tracing -count=1

test-bsc:
	AUTOPATCH_TEST_CONFIG=./test_config.json AUTOPATCH_TEST_SCENARIO=bsc go test -v ./tracing -count=1

test-local:
	AUTOPATCH_TEST_CONFIG=./test_config.json AUTOPATCH_TEST_SCENARIO=local go test -v ./tracing -count=1

lint:
	golangci-lint run ./...

bindings: binding-vrf

binding-vrf:
	$(eval temp := $(shell mktemp))

	cat $(StorageScan_ABI_ARTIFACT) \
    	| jq -r .bytecode.object > $(temp)

	cat $(StorageScan_ABI_ARTIFACT) \
		| jq .abi \
		| abigen --pkg bindings \
		--abi - \
		--out bindings/storagescan.go \
		--type StorageScan \
		--bin $(temp)

		rm $(temp)


.PHONY: \
	autopatch \
	bindings \
	binding-vrf \
	clean \
	test \
	test-replay \
	test-replay-nosend \
	test-send \
	test-tracing \
	test-with-config \
	test-holesky \
	test-bsc \
	test-local \
	lint