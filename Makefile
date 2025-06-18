StorageScan_ABI_ARTIFACT := ./abis/StorageScan.sol/StorageScan.json


autopatch:
	env GO111MODULE=on go build  ./cmd/autopatch

clean:
	rm auto-patch

test:
	go test -v ./...

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
	lint