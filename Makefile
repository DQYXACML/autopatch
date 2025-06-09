VRF_ABI_ARTIFACT := ./abis/DappLinkVRFManager.sol/DappLinkVRFManager.json
BLS_FACTORY_ABI_ARTIFACT := ./abis/DappLinkVRFFactory.sol/DappLinkVRFFactory.json
BLS_ABI_ARTIFACT := ./abis/BLSApkRegistry.sol/BLSApkRegistry.json


vrf-node:
	env GO111MODULE=on go build  ./cmd/vrf-node

clean:
	rm vrf-node

test:
	go test -v ./...

lint:
	golangci-lint run ./...

bindings: binding-vrf binding-bls binding-factory


binding-vrf:
	$(eval temp := $(shell mktemp))

	cat $(VRF_ABI_ARTIFACT) \
    	| jq -r .bytecode.object > $(temp)

	cat $(VRF_ABI_ARTIFACT) \
		| jq .abi \
		| abigen --pkg bindings \
		--abi - \
		--out bindings/dapplinkvrfmanager.go \
		--type DappLinkVRFManager \
		--bin $(temp)

		rm $(temp)

binding-bls:
	$(eval temp := $(shell mktemp))

	cat $(BLS_ABI_ARTIFACT) \
    	| jq -r .bytecode.object > $(temp)

	cat $(BLS_ABI_ARTIFACT) \
		| jq .abi \
		| abigen --pkg bindings \
		--abi - \
		--out bindings/blsapkregistry.go \
		--type BLSApkRegistry \
		--bin $(temp)

		rm $(temp)


binding-factory:
	$(eval temp := $(shell mktemp))

	cat $(BLS_FACTORY_ABI_ARTIFACT) \
    	| jq -r .bytecode.object > $(temp)

	cat $(BLS_FACTORY_ABI_ARTIFACT) \
		| jq .abi \
		| abigen --pkg bindings \
		--abi - \
		--out bindings/dapplinkvrffactory.go \
		--type DappLinkVRFFactory \
		--bin $(temp)

		rm $(temp)


.PHONY: \
	vrf-node \
	bindings \
	binding-vrf \
	binding-bls \
	clean \
	test \
	lint