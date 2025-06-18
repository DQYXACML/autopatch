// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package bindings

import (
	"errors"
	"math/big"
	"strings"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
)

// Reference imports to suppress errors if they are not otherwise used.
var (
	_ = errors.New
	_ = big.NewInt
	_ = strings.NewReader
	_ = ethereum.NotFound
	_ = bind.Bind
	_ = common.Big1
	_ = types.BloomLookup
	_ = event.NewSubscription
	_ = abi.ConvertType
)

// StorageScanMetaData contains all meta data concerning the StorageScan contract.
var StorageScanMetaData = &bind.MetaData{
	ABI: "[{\"type\":\"constructor\",\"inputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"setAddr1\",\"inputs\":[{\"name\":\"_v\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"setB1\",\"inputs\":[{\"name\":\"_v\",\"type\":\"bytes1\",\"internalType\":\"bytes1\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"setB2\",\"inputs\":[{\"name\":\"_v\",\"type\":\"bytes8\",\"internalType\":\"bytes8\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"setB3\",\"inputs\":[{\"name\":\"_v\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"setBool1\",\"inputs\":[{\"name\":\"_v\",\"type\":\"bool\",\"internalType\":\"bool\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"setBool2\",\"inputs\":[{\"name\":\"_v\",\"type\":\"bool\",\"internalType\":\"bool\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"setEntity\",\"inputs\":[{\"name\":\"_id\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"_value\",\"type\":\"string\",\"internalType\":\"string\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"setInt1\",\"inputs\":[{\"name\":\"_v\",\"type\":\"int8\",\"internalType\":\"int8\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"setInt2\",\"inputs\":[{\"name\":\"_v\",\"type\":\"int128\",\"internalType\":\"int128\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"setInt3\",\"inputs\":[{\"name\":\"_v\",\"type\":\"int256\",\"internalType\":\"int256\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"setMapping1\",\"inputs\":[{\"name\":\"key\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"value\",\"type\":\"string\",\"internalType\":\"string\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"setMapping2\",\"inputs\":[{\"name\":\"key\",\"type\":\"string\",\"internalType\":\"string\"},{\"name\":\"value\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"setMapping3\",\"inputs\":[{\"name\":\"key\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"value\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"setMapping4\",\"inputs\":[{\"name\":\"key\",\"type\":\"int256\",\"internalType\":\"int256\"},{\"name\":\"value\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"setMapping5\",\"inputs\":[{\"name\":\"key\",\"type\":\"bytes1\",\"internalType\":\"bytes1\"},{\"name\":\"value\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"setMapping6\",\"inputs\":[{\"name\":\"key\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"_id\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"_value\",\"type\":\"string\",\"internalType\":\"string\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"setSlice1At\",\"inputs\":[{\"name\":\"idx\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"_v\",\"type\":\"uint8\",\"internalType\":\"uint8\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"setSlice2At\",\"inputs\":[{\"name\":\"idx\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"_v\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"setSlice3At\",\"inputs\":[{\"name\":\"idx\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"_v\",\"type\":\"bool\",\"internalType\":\"bool\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"setSlice4At\",\"inputs\":[{\"name\":\"idx\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"_v\",\"type\":\"string\",\"internalType\":\"string\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"setSlice5At\",\"inputs\":[{\"name\":\"idx\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"_id\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"_value\",\"type\":\"string\",\"internalType\":\"string\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"setString1\",\"inputs\":[{\"name\":\"_v\",\"type\":\"string\",\"internalType\":\"string\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"setString2\",\"inputs\":[{\"name\":\"_v\",\"type\":\"string\",\"internalType\":\"string\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"setUint1\",\"inputs\":[{\"name\":\"_v\",\"type\":\"uint8\",\"internalType\":\"uint8\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"setUint2\",\"inputs\":[{\"name\":\"_v\",\"type\":\"uint128\",\"internalType\":\"uint128\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"setUint3\",\"inputs\":[{\"name\":\"_v\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"}]",
	Bin: "0x5f80546001600160881b03199081166180f81790915561010060019081556002805490921661800817909155670123456789abcef160039081556004805461ffff191690921790915560c060405260809081526261626360e81b60a0526005906100699082610a3a565b506040518060800160405280605581526020016117f4605591396006906100909082610a3a565b5060078054686279746532000000616001600160481b03199091161790557f737472696e672062797465732063616e6e6f7420657863656564203332000000600855600980546001600160a01b031916732729e5dfdeecb92c884470ef6cad9e844e34502d1790556040805160a08101825260018152600260208201526003918101919091526004606082015260056080820181905261013291600c91610699565b506040805160a08101825261010081526101016020820152610102918101919091526101036060820152610104608082015261017290600d90600561073a565b506040805160a08101825260018082525f60208301819052928201839052606082015260808101919091526101ab90600e906005610779565b5060405180604001604052806040518060400160405280600381526020016261626360e81b81525081526020016040518060800160405280605581526020016117f460559139905261020190600f9060026107db565b506040805160a08101825260018152600260208201526003918101919091526004606082015260056080820181905261023c9160119161082b565b506040805160a08101825261010081526101016020820152610102918101919091526101036060820152610104608082015261027c90601290600561087c565b506040805160a08101825260018082525f60208301819052928201839052606082015260808101919091526102b59060179060056108af565b5060405180604001604052806040518060400160405280600381526020016261626360e81b81525081526020016040518060800160405280605581526020016117f460559139905261030b9060189060026108ff565b50348015610317575f5ffd5b506001600a55604080518082019091526006815265656e7469747960d01b6020820152600b906103479082610a3a565b506040805180820182526001808252825180840190935260078352660736c69636535360cc1b60208481019190915282019283526010805491820181555f52815160029091025f5160206117d45f395f51905f5281019182559251919290915f5160206117b45f395f51905f52909101906103c29082610a3a565b5050604080518082018252600280825282518084019093526007835266736c696365353160c81b6020848101919091528201928352601080546001810182555f91909152825191025f5160206117d45f395f51905f5281019182559251919350915f5160206117b45f395f51905f52019061043d9082610a3a565b505050604051806040016040528060018152602001604051806040016040528060078152602001660617272617935360cc1b815250815250601a5f6002811061048857610488610af4565b600202015f820151815f015560208201518160010190816104a99190610a3a565b50905050604051806040016040528060028152602001604051806040016040528060078152602001666172726179353160c81b815250815250601a6001600281106104f6576104f6610af4565b600202015f820151815f015560208201518160010190816105179190610a3a565b50506040805180820190915260088152676d617070696e673160c01b60208083019190915260015f52601e90527f873299c6a6c39b8b92f01922bb622df4a3236ea2876aac2da76f6c092cf7e98f91506105719082610a3a565b506001601f604051610591906736b0b83834b7339960c11b815260080190565b9081526040805160209281900383018120939093556009546001600160a01b03165f9081528280528181206001908190557f5258bfb9612346a4ce42648707f55b17f8106d34d69ddcc829dc89248a979dba8190557f476b6aadbe0907fa7fa6044b0bcf027c48d1d1fc348715afca0b3b65864fc82f819055848301835284528151808301909252600882526736b0b83834b7339b60c11b82840152828401918252607b9052602390915281517ffbb0d4226db9ebca43acffd2b54948e6e2b00e3e194eb2c665cdd7e04764191c90815590517ffbb0d4226db9ebca43acffd2b54948e6e2b00e3e194eb2c665cdd7e04764191d906106909082610a3a565b50905050610b08565b828054828255905f5260205f2090601f0160209004810192821561072a579160200282015f5b838211156106fc57835183826101000a81548160ff021916908360ff16021790555092602001926001016020815f010492830192600103026106bf565b80156107285782816101000a81549060ff02191690556001016020815f010492830192600103026106fc565b505b50610736929150610938565b5090565b828054828255905f5260205f2090810192821561072a579160200282015b8281111561072a578251829061ffff16905591602001919060010190610758565b828054828255905f5260205f2090601f0160209004810192821561072a579160200282015f5b838211156106fc57835183826101000a81548160ff02191690831515021790555092602001926001016020815f0104928301926001030261079f565b828054828255905f5260205f2090810192821561081f579160200282015b8281111561081f578251829061080f9082610a3a565b50916020019190600101906107f9565b5061073692915061094c565b60018301918390821561072a579160200282015f838211156106fc57835183826101000a81548160ff021916908360ff16021790555092602001926001016020815f010492830192600103026106bf565b826005810192821561072a579160200282018281111561072a578251829061ffff16905591602001919060010190610758565b60018301918390821561072a579160200282015f838211156106fc57835183826101000a81548160ff02191690831515021790555092602001926001016020815f0104928301926001030261079f565b826002810192821561081f579160200282015b8281111561081f57825182906109289082610a3a565b5091602001919060010190610912565b5b80821115610736575f8155600101610939565b80821115610736575f61095f8282610968565b5060010161094c565b508054610974906109b6565b5f825580601f10610983575050565b601f0160209004905f5260205f209081019061099f9190610938565b50565b634e487b7160e01b5f52604160045260245ffd5b600181811c908216806109ca57607f821691505b6020821081036109e857634e487b7160e01b5f52602260045260245ffd5b50919050565b601f821115610a3557805f5260205f20601f840160051c81016020851015610a135750805b601f840160051c820191505b81811015610a32575f8155600101610a1f565b50505b505050565b81516001600160401b03811115610a5357610a536109a2565b610a6781610a6184546109b6565b846109ee565b6020601f821160018114610a99575f8315610a825750848201515b5f19600385901b1c1916600184901b178455610a32565b5f84815260208120601f198516915b82811015610ac85787850151825560209485019460019092019101610aa8565b5084821015610ae557868401515f19600387901b60f8161c191681555b50505050600190811b01905550565b634e487b7160e01b5f52603260045260245ffd5b610c9f80610b155f395ff3fe608060405234801561000f575f5ffd5b5060043610610187575f3560e01c8063708945b0116100d9578063bb3da88311610093578063dd00c4da1161006e578063dd00c4da146103f6578063e02c1cd01461041e578063e6cee3eb14610431578063ecc55e4b14610461575f5ffd5b8063bb3da8831461039f578063bd62f4e5146103b2578063c4844149146103d3575f5ffd5b8063708945b01461030a578063856309451461031d5780638db6f52b1461033057806398e0912214610343578063a044bcd914610356578063ab4bea6b1461038c575f5ffd5b806341a406b81161014457806360dbd8191161011f57806360dbd81914610296578063688388fd146102a9578063698ccd3a146102d35780636f4b210e146102f7575f5ffd5b806341a406b81461023b57806342d407ec1461024e57806351db4a3314610283575f5ffd5b8063067befb41461018b57806307bc5764146101b1578063233b9f8d146101c457806329da97a7146101f6578063377e41eb146102155780633da5a1ec14610228575b5f5ffd5b6101af6101993660046106e7565b6007805460ff191660f89290921c919091179055565b005b6101af6101bf36600461074c565b610489565b6101af6101d236600461079b565b6007805460c09290921c6101000268ffffffffffffffff0019909216919091179055565b6101af6102043660046107c2565b5f9182526021602052604090912055565b6101af6102233660046107f1565b610517565b6101af61023636600461081b565b600355565b6101af610249366004610832565b610556565b6101af61025c36600461087a565b5f80546001600160801b0390921661010002610100600160881b0319909216919091179055565b6101af6102913660046107c2565b61057e565b6101af6102a436600461081b565b600155565b6101af6102b736600461089a565b6001600160f81b03199091165f90815260226020526040902055565b6101af6102e13660046108d2565b6002805460ff191660ff92909216919091179055565b6101af6103053660046108eb565b6105a1565b6101af61031836600461074c565b6105e1565b6101af61032b36600461090c565b610652565b6101af61033e36600461090c565b610670565b6101af61035136600461081b565b600855565b6101af610364366004610954565b600280546001600160801b0390921661010002610100600160881b0319909216919091179055565b6101af61039a36600461090c565b61069a565b6101af6103ad36600461097a565b6106ac565b6101af6103c03660046109b9565b6004805460ff1916911515919091179055565b6101af6103e13660046109d2565b5f805460ff191660ff92909216919091179055565b6101af610404366004610a07565b6001600160a01b039091165f908152602080526040902055565b6101af61042c36600461097a565b6106be565b6101af61043f366004610a21565b600980546001600160a01b0319166001600160a01b0392909216919091179055565b6101af61046f3660046109b9565b600480549115156101000261ff0019909216919091179055565b604051806040016040528084815260200183838080601f0160208091040260200160405190810160405280939291908181526020018383808284375f92019190915250505091525060108054869081106104e5576104e5610a3a565b905f5260205f2090600202015f820151815f0155602082015181600101908161050e9190610ae5565b50505050505050565b80600e838154811061052b5761052b610a3a565b905f5260205f2090602091828204019190066101000a81548160ff0219169083151502179055505050565b80601f8484604051610569929190610ba0565b90815260405190819003602001902055505050565b80600d838154811061059257610592610a3a565b5f918252602090912001555050565b80600c83815481106105b5576105b5610a3a565b905f5260205f2090602091828204019190066101000a81548160ff021916908360ff1602179055505050565b604051806040016040528084815260200183838080601f0160208091040260200160405190810160405280939291908181526020018383808284375f9201829052509390945250508681526023602090815260409091208351815590830151909150600182019061050e9082610ae5565b5f838152601e6020526040902061066a828483610baf565b50505050565b8181600f858154811061068557610685610a3a565b905f5260205f2001918261066a929190610baf565b600a839055600b61066a828483610baf565b60056106b9828483610baf565b505050565b60066106b9828483610baf565b80356001600160f81b0319811681146106e2575f5ffd5b919050565b5f602082840312156106f7575f5ffd5b610700826106cb565b9392505050565b5f5f83601f840112610717575f5ffd5b50813567ffffffffffffffff81111561072e575f5ffd5b602083019150836020828501011115610745575f5ffd5b9250929050565b5f5f5f5f6060858703121561075f575f5ffd5b8435935060208501359250604085013567ffffffffffffffff811115610783575f5ffd5b61078f87828801610707565b95989497509550505050565b5f602082840312156107ab575f5ffd5b81356001600160c01b031981168114610700575f5ffd5b5f5f604083850312156107d3575f5ffd5b50508035926020909101359150565b803580151581146106e2575f5ffd5b5f5f60408385031215610802575f5ffd5b82359150610812602084016107e2565b90509250929050565b5f6020828403121561082b575f5ffd5b5035919050565b5f5f5f60408486031215610844575f5ffd5b833567ffffffffffffffff81111561085a575f5ffd5b61086686828701610707565b909790965060209590950135949350505050565b5f6020828403121561088a575f5ffd5b813580600f0b8114610700575f5ffd5b5f5f604083850312156108ab575f5ffd5b6108b4836106cb565b946020939093013593505050565b803560ff811681146106e2575f5ffd5b5f602082840312156108e2575f5ffd5b610700826108c2565b5f5f604083850312156108fc575f5ffd5b82359150610812602084016108c2565b5f5f5f6040848603121561091e575f5ffd5b83359250602084013567ffffffffffffffff81111561093b575f5ffd5b61094786828701610707565b9497909650939450505050565b5f60208284031215610964575f5ffd5b81356001600160801b0381168114610700575f5ffd5b5f5f6020838503121561098b575f5ffd5b823567ffffffffffffffff8111156109a1575f5ffd5b6109ad85828601610707565b90969095509350505050565b5f602082840312156109c9575f5ffd5b610700826107e2565b5f602082840312156109e2575f5ffd5b8135805f0b8114610700575f5ffd5b80356001600160a01b03811681146106e2575f5ffd5b5f5f60408385031215610a18575f5ffd5b6108b4836109f1565b5f60208284031215610a31575f5ffd5b610700826109f1565b634e487b7160e01b5f52603260045260245ffd5b634e487b7160e01b5f52604160045260245ffd5b600181811c90821680610a7657607f821691505b602082108103610a9457634e487b7160e01b5f52602260045260245ffd5b50919050565b601f8211156106b957805f5260205f20601f840160051c81016020851015610abf5750805b601f840160051c820191505b81811015610ade575f8155600101610acb565b5050505050565b815167ffffffffffffffff811115610aff57610aff610a4e565b610b1381610b0d8454610a62565b84610a9a565b6020601f821160018114610b45575f8315610b2e5750848201515b5f19600385901b1c1916600184901b178455610ade565b5f84815260208120601f198516915b82811015610b745787850151825560209485019460019092019101610b54565b5084821015610b9157868401515f19600387901b60f8161c191681555b50505050600190811b01905550565b818382375f9101908152919050565b67ffffffffffffffff831115610bc757610bc7610a4e565b610bdb83610bd58354610a62565b83610a9a565b5f601f841160018114610c0c575f8515610bf55750838201355b5f19600387901b1c1916600186901b178355610ade565b5f83815260208120601f198716915b82811015610c3b5786850135825560209485019460019092019101610c1b565b5086821015610c57575f1960f88860031b161c19848701351681555b505060018560011b018355505050505056fea26469706673582212207c277aa51c4704af6c6b1aec81f7b6f139d841674e43d158fc643d2467dc554864736f6c634300081c00331b6847dc741a1b0cd08d278845f9d819d87b734759afb55fe2de5cb82a9ae6731b6847dc741a1b0cd08d278845f9d819d87b734759afb55fe2de5cb82a9ae672736f6c696469747920697320616e206f626a6563742d6f7269656e7465642c20686967682d6c6576656c206c616e677561676520666f7220696d706c656d656e74696e6720736d61727420636f6e7472616374732e",
}

// StorageScanABI is the input ABI used to generate the binding from.
// Deprecated: Use StorageScanMetaData.ABI instead.
var StorageScanABI = StorageScanMetaData.ABI

// StorageScanBin is the compiled bytecode used for deploying new contracts.
// Deprecated: Use StorageScanMetaData.Bin instead.
var StorageScanBin = StorageScanMetaData.Bin

// DeployStorageScan deploys a new Ethereum contract, binding an instance of StorageScan to it.
func DeployStorageScan(auth *bind.TransactOpts, backend bind.ContractBackend) (common.Address, *types.Transaction, *StorageScan, error) {
	parsed, err := StorageScanMetaData.GetAbi()
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	if parsed == nil {
		return common.Address{}, nil, nil, errors.New("GetABI returned nil")
	}

	address, tx, contract, err := bind.DeployContract(auth, *parsed, common.FromHex(StorageScanBin), backend)
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	return address, tx, &StorageScan{StorageScanCaller: StorageScanCaller{contract: contract}, StorageScanTransactor: StorageScanTransactor{contract: contract}, StorageScanFilterer: StorageScanFilterer{contract: contract}}, nil
}

// StorageScan is an auto generated Go binding around an Ethereum contract.
type StorageScan struct {
	StorageScanCaller     // Read-only binding to the contract
	StorageScanTransactor // Write-only binding to the contract
	StorageScanFilterer   // Log filterer for contract events
}

// StorageScanCaller is an auto generated read-only Go binding around an Ethereum contract.
type StorageScanCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// StorageScanTransactor is an auto generated write-only Go binding around an Ethereum contract.
type StorageScanTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// StorageScanFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type StorageScanFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// StorageScanSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type StorageScanSession struct {
	Contract     *StorageScan      // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// StorageScanCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type StorageScanCallerSession struct {
	Contract *StorageScanCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts      // Call options to use throughout this session
}

// StorageScanTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type StorageScanTransactorSession struct {
	Contract     *StorageScanTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts      // Transaction auth options to use throughout this session
}

// StorageScanRaw is an auto generated low-level Go binding around an Ethereum contract.
type StorageScanRaw struct {
	Contract *StorageScan // Generic contract binding to access the raw methods on
}

// StorageScanCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type StorageScanCallerRaw struct {
	Contract *StorageScanCaller // Generic read-only contract binding to access the raw methods on
}

// StorageScanTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type StorageScanTransactorRaw struct {
	Contract *StorageScanTransactor // Generic write-only contract binding to access the raw methods on
}

// NewStorageScan creates a new instance of StorageScan, bound to a specific deployed contract.
func NewStorageScan(address common.Address, backend bind.ContractBackend) (*StorageScan, error) {
	contract, err := bindStorageScan(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &StorageScan{StorageScanCaller: StorageScanCaller{contract: contract}, StorageScanTransactor: StorageScanTransactor{contract: contract}, StorageScanFilterer: StorageScanFilterer{contract: contract}}, nil
}

// NewStorageScanCaller creates a new read-only instance of StorageScan, bound to a specific deployed contract.
func NewStorageScanCaller(address common.Address, caller bind.ContractCaller) (*StorageScanCaller, error) {
	contract, err := bindStorageScan(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &StorageScanCaller{contract: contract}, nil
}

// NewStorageScanTransactor creates a new write-only instance of StorageScan, bound to a specific deployed contract.
func NewStorageScanTransactor(address common.Address, transactor bind.ContractTransactor) (*StorageScanTransactor, error) {
	contract, err := bindStorageScan(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &StorageScanTransactor{contract: contract}, nil
}

// NewStorageScanFilterer creates a new log filterer instance of StorageScan, bound to a specific deployed contract.
func NewStorageScanFilterer(address common.Address, filterer bind.ContractFilterer) (*StorageScanFilterer, error) {
	contract, err := bindStorageScan(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &StorageScanFilterer{contract: contract}, nil
}

// bindStorageScan binds a generic wrapper to an already deployed contract.
func bindStorageScan(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := StorageScanMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_StorageScan *StorageScanRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _StorageScan.Contract.StorageScanCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_StorageScan *StorageScanRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _StorageScan.Contract.StorageScanTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_StorageScan *StorageScanRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _StorageScan.Contract.StorageScanTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_StorageScan *StorageScanCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _StorageScan.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_StorageScan *StorageScanTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _StorageScan.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_StorageScan *StorageScanTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _StorageScan.Contract.contract.Transact(opts, method, params...)
}

// SetAddr1 is a paid mutator transaction binding the contract method 0xe6cee3eb.
//
// Solidity: function setAddr1(address _v) returns()
func (_StorageScan *StorageScanTransactor) SetAddr1(opts *bind.TransactOpts, _v common.Address) (*types.Transaction, error) {
	return _StorageScan.contract.Transact(opts, "setAddr1", _v)
}

// SetAddr1 is a paid mutator transaction binding the contract method 0xe6cee3eb.
//
// Solidity: function setAddr1(address _v) returns()
func (_StorageScan *StorageScanSession) SetAddr1(_v common.Address) (*types.Transaction, error) {
	return _StorageScan.Contract.SetAddr1(&_StorageScan.TransactOpts, _v)
}

// SetAddr1 is a paid mutator transaction binding the contract method 0xe6cee3eb.
//
// Solidity: function setAddr1(address _v) returns()
func (_StorageScan *StorageScanTransactorSession) SetAddr1(_v common.Address) (*types.Transaction, error) {
	return _StorageScan.Contract.SetAddr1(&_StorageScan.TransactOpts, _v)
}

// SetB1 is a paid mutator transaction binding the contract method 0x067befb4.
//
// Solidity: function setB1(bytes1 _v) returns()
func (_StorageScan *StorageScanTransactor) SetB1(opts *bind.TransactOpts, _v [1]byte) (*types.Transaction, error) {
	return _StorageScan.contract.Transact(opts, "setB1", _v)
}

// SetB1 is a paid mutator transaction binding the contract method 0x067befb4.
//
// Solidity: function setB1(bytes1 _v) returns()
func (_StorageScan *StorageScanSession) SetB1(_v [1]byte) (*types.Transaction, error) {
	return _StorageScan.Contract.SetB1(&_StorageScan.TransactOpts, _v)
}

// SetB1 is a paid mutator transaction binding the contract method 0x067befb4.
//
// Solidity: function setB1(bytes1 _v) returns()
func (_StorageScan *StorageScanTransactorSession) SetB1(_v [1]byte) (*types.Transaction, error) {
	return _StorageScan.Contract.SetB1(&_StorageScan.TransactOpts, _v)
}

// SetB2 is a paid mutator transaction binding the contract method 0x233b9f8d.
//
// Solidity: function setB2(bytes8 _v) returns()
func (_StorageScan *StorageScanTransactor) SetB2(opts *bind.TransactOpts, _v [8]byte) (*types.Transaction, error) {
	return _StorageScan.contract.Transact(opts, "setB2", _v)
}

// SetB2 is a paid mutator transaction binding the contract method 0x233b9f8d.
//
// Solidity: function setB2(bytes8 _v) returns()
func (_StorageScan *StorageScanSession) SetB2(_v [8]byte) (*types.Transaction, error) {
	return _StorageScan.Contract.SetB2(&_StorageScan.TransactOpts, _v)
}

// SetB2 is a paid mutator transaction binding the contract method 0x233b9f8d.
//
// Solidity: function setB2(bytes8 _v) returns()
func (_StorageScan *StorageScanTransactorSession) SetB2(_v [8]byte) (*types.Transaction, error) {
	return _StorageScan.Contract.SetB2(&_StorageScan.TransactOpts, _v)
}

// SetB3 is a paid mutator transaction binding the contract method 0x98e09122.
//
// Solidity: function setB3(bytes32 _v) returns()
func (_StorageScan *StorageScanTransactor) SetB3(opts *bind.TransactOpts, _v [32]byte) (*types.Transaction, error) {
	return _StorageScan.contract.Transact(opts, "setB3", _v)
}

// SetB3 is a paid mutator transaction binding the contract method 0x98e09122.
//
// Solidity: function setB3(bytes32 _v) returns()
func (_StorageScan *StorageScanSession) SetB3(_v [32]byte) (*types.Transaction, error) {
	return _StorageScan.Contract.SetB3(&_StorageScan.TransactOpts, _v)
}

// SetB3 is a paid mutator transaction binding the contract method 0x98e09122.
//
// Solidity: function setB3(bytes32 _v) returns()
func (_StorageScan *StorageScanTransactorSession) SetB3(_v [32]byte) (*types.Transaction, error) {
	return _StorageScan.Contract.SetB3(&_StorageScan.TransactOpts, _v)
}

// SetBool1 is a paid mutator transaction binding the contract method 0xbd62f4e5.
//
// Solidity: function setBool1(bool _v) returns()
func (_StorageScan *StorageScanTransactor) SetBool1(opts *bind.TransactOpts, _v bool) (*types.Transaction, error) {
	return _StorageScan.contract.Transact(opts, "setBool1", _v)
}

// SetBool1 is a paid mutator transaction binding the contract method 0xbd62f4e5.
//
// Solidity: function setBool1(bool _v) returns()
func (_StorageScan *StorageScanSession) SetBool1(_v bool) (*types.Transaction, error) {
	return _StorageScan.Contract.SetBool1(&_StorageScan.TransactOpts, _v)
}

// SetBool1 is a paid mutator transaction binding the contract method 0xbd62f4e5.
//
// Solidity: function setBool1(bool _v) returns()
func (_StorageScan *StorageScanTransactorSession) SetBool1(_v bool) (*types.Transaction, error) {
	return _StorageScan.Contract.SetBool1(&_StorageScan.TransactOpts, _v)
}

// SetBool2 is a paid mutator transaction binding the contract method 0xecc55e4b.
//
// Solidity: function setBool2(bool _v) returns()
func (_StorageScan *StorageScanTransactor) SetBool2(opts *bind.TransactOpts, _v bool) (*types.Transaction, error) {
	return _StorageScan.contract.Transact(opts, "setBool2", _v)
}

// SetBool2 is a paid mutator transaction binding the contract method 0xecc55e4b.
//
// Solidity: function setBool2(bool _v) returns()
func (_StorageScan *StorageScanSession) SetBool2(_v bool) (*types.Transaction, error) {
	return _StorageScan.Contract.SetBool2(&_StorageScan.TransactOpts, _v)
}

// SetBool2 is a paid mutator transaction binding the contract method 0xecc55e4b.
//
// Solidity: function setBool2(bool _v) returns()
func (_StorageScan *StorageScanTransactorSession) SetBool2(_v bool) (*types.Transaction, error) {
	return _StorageScan.Contract.SetBool2(&_StorageScan.TransactOpts, _v)
}

// SetEntity is a paid mutator transaction binding the contract method 0xab4bea6b.
//
// Solidity: function setEntity(uint256 _id, string _value) returns()
func (_StorageScan *StorageScanTransactor) SetEntity(opts *bind.TransactOpts, _id *big.Int, _value string) (*types.Transaction, error) {
	return _StorageScan.contract.Transact(opts, "setEntity", _id, _value)
}

// SetEntity is a paid mutator transaction binding the contract method 0xab4bea6b.
//
// Solidity: function setEntity(uint256 _id, string _value) returns()
func (_StorageScan *StorageScanSession) SetEntity(_id *big.Int, _value string) (*types.Transaction, error) {
	return _StorageScan.Contract.SetEntity(&_StorageScan.TransactOpts, _id, _value)
}

// SetEntity is a paid mutator transaction binding the contract method 0xab4bea6b.
//
// Solidity: function setEntity(uint256 _id, string _value) returns()
func (_StorageScan *StorageScanTransactorSession) SetEntity(_id *big.Int, _value string) (*types.Transaction, error) {
	return _StorageScan.Contract.SetEntity(&_StorageScan.TransactOpts, _id, _value)
}

// SetInt1 is a paid mutator transaction binding the contract method 0xc4844149.
//
// Solidity: function setInt1(int8 _v) returns()
func (_StorageScan *StorageScanTransactor) SetInt1(opts *bind.TransactOpts, _v int8) (*types.Transaction, error) {
	return _StorageScan.contract.Transact(opts, "setInt1", _v)
}

// SetInt1 is a paid mutator transaction binding the contract method 0xc4844149.
//
// Solidity: function setInt1(int8 _v) returns()
func (_StorageScan *StorageScanSession) SetInt1(_v int8) (*types.Transaction, error) {
	return _StorageScan.Contract.SetInt1(&_StorageScan.TransactOpts, _v)
}

// SetInt1 is a paid mutator transaction binding the contract method 0xc4844149.
//
// Solidity: function setInt1(int8 _v) returns()
func (_StorageScan *StorageScanTransactorSession) SetInt1(_v int8) (*types.Transaction, error) {
	return _StorageScan.Contract.SetInt1(&_StorageScan.TransactOpts, _v)
}

// SetInt2 is a paid mutator transaction binding the contract method 0x42d407ec.
//
// Solidity: function setInt2(int128 _v) returns()
func (_StorageScan *StorageScanTransactor) SetInt2(opts *bind.TransactOpts, _v *big.Int) (*types.Transaction, error) {
	return _StorageScan.contract.Transact(opts, "setInt2", _v)
}

// SetInt2 is a paid mutator transaction binding the contract method 0x42d407ec.
//
// Solidity: function setInt2(int128 _v) returns()
func (_StorageScan *StorageScanSession) SetInt2(_v *big.Int) (*types.Transaction, error) {
	return _StorageScan.Contract.SetInt2(&_StorageScan.TransactOpts, _v)
}

// SetInt2 is a paid mutator transaction binding the contract method 0x42d407ec.
//
// Solidity: function setInt2(int128 _v) returns()
func (_StorageScan *StorageScanTransactorSession) SetInt2(_v *big.Int) (*types.Transaction, error) {
	return _StorageScan.Contract.SetInt2(&_StorageScan.TransactOpts, _v)
}

// SetInt3 is a paid mutator transaction binding the contract method 0x60dbd819.
//
// Solidity: function setInt3(int256 _v) returns()
func (_StorageScan *StorageScanTransactor) SetInt3(opts *bind.TransactOpts, _v *big.Int) (*types.Transaction, error) {
	return _StorageScan.contract.Transact(opts, "setInt3", _v)
}

// SetInt3 is a paid mutator transaction binding the contract method 0x60dbd819.
//
// Solidity: function setInt3(int256 _v) returns()
func (_StorageScan *StorageScanSession) SetInt3(_v *big.Int) (*types.Transaction, error) {
	return _StorageScan.Contract.SetInt3(&_StorageScan.TransactOpts, _v)
}

// SetInt3 is a paid mutator transaction binding the contract method 0x60dbd819.
//
// Solidity: function setInt3(int256 _v) returns()
func (_StorageScan *StorageScanTransactorSession) SetInt3(_v *big.Int) (*types.Transaction, error) {
	return _StorageScan.Contract.SetInt3(&_StorageScan.TransactOpts, _v)
}

// SetMapping1 is a paid mutator transaction binding the contract method 0x85630945.
//
// Solidity: function setMapping1(uint256 key, string value) returns()
func (_StorageScan *StorageScanTransactor) SetMapping1(opts *bind.TransactOpts, key *big.Int, value string) (*types.Transaction, error) {
	return _StorageScan.contract.Transact(opts, "setMapping1", key, value)
}

// SetMapping1 is a paid mutator transaction binding the contract method 0x85630945.
//
// Solidity: function setMapping1(uint256 key, string value) returns()
func (_StorageScan *StorageScanSession) SetMapping1(key *big.Int, value string) (*types.Transaction, error) {
	return _StorageScan.Contract.SetMapping1(&_StorageScan.TransactOpts, key, value)
}

// SetMapping1 is a paid mutator transaction binding the contract method 0x85630945.
//
// Solidity: function setMapping1(uint256 key, string value) returns()
func (_StorageScan *StorageScanTransactorSession) SetMapping1(key *big.Int, value string) (*types.Transaction, error) {
	return _StorageScan.Contract.SetMapping1(&_StorageScan.TransactOpts, key, value)
}

// SetMapping2 is a paid mutator transaction binding the contract method 0x41a406b8.
//
// Solidity: function setMapping2(string key, uint256 value) returns()
func (_StorageScan *StorageScanTransactor) SetMapping2(opts *bind.TransactOpts, key string, value *big.Int) (*types.Transaction, error) {
	return _StorageScan.contract.Transact(opts, "setMapping2", key, value)
}

// SetMapping2 is a paid mutator transaction binding the contract method 0x41a406b8.
//
// Solidity: function setMapping2(string key, uint256 value) returns()
func (_StorageScan *StorageScanSession) SetMapping2(key string, value *big.Int) (*types.Transaction, error) {
	return _StorageScan.Contract.SetMapping2(&_StorageScan.TransactOpts, key, value)
}

// SetMapping2 is a paid mutator transaction binding the contract method 0x41a406b8.
//
// Solidity: function setMapping2(string key, uint256 value) returns()
func (_StorageScan *StorageScanTransactorSession) SetMapping2(key string, value *big.Int) (*types.Transaction, error) {
	return _StorageScan.Contract.SetMapping2(&_StorageScan.TransactOpts, key, value)
}

// SetMapping3 is a paid mutator transaction binding the contract method 0xdd00c4da.
//
// Solidity: function setMapping3(address key, uint256 value) returns()
func (_StorageScan *StorageScanTransactor) SetMapping3(opts *bind.TransactOpts, key common.Address, value *big.Int) (*types.Transaction, error) {
	return _StorageScan.contract.Transact(opts, "setMapping3", key, value)
}

// SetMapping3 is a paid mutator transaction binding the contract method 0xdd00c4da.
//
// Solidity: function setMapping3(address key, uint256 value) returns()
func (_StorageScan *StorageScanSession) SetMapping3(key common.Address, value *big.Int) (*types.Transaction, error) {
	return _StorageScan.Contract.SetMapping3(&_StorageScan.TransactOpts, key, value)
}

// SetMapping3 is a paid mutator transaction binding the contract method 0xdd00c4da.
//
// Solidity: function setMapping3(address key, uint256 value) returns()
func (_StorageScan *StorageScanTransactorSession) SetMapping3(key common.Address, value *big.Int) (*types.Transaction, error) {
	return _StorageScan.Contract.SetMapping3(&_StorageScan.TransactOpts, key, value)
}

// SetMapping4 is a paid mutator transaction binding the contract method 0x29da97a7.
//
// Solidity: function setMapping4(int256 key, uint256 value) returns()
func (_StorageScan *StorageScanTransactor) SetMapping4(opts *bind.TransactOpts, key *big.Int, value *big.Int) (*types.Transaction, error) {
	return _StorageScan.contract.Transact(opts, "setMapping4", key, value)
}

// SetMapping4 is a paid mutator transaction binding the contract method 0x29da97a7.
//
// Solidity: function setMapping4(int256 key, uint256 value) returns()
func (_StorageScan *StorageScanSession) SetMapping4(key *big.Int, value *big.Int) (*types.Transaction, error) {
	return _StorageScan.Contract.SetMapping4(&_StorageScan.TransactOpts, key, value)
}

// SetMapping4 is a paid mutator transaction binding the contract method 0x29da97a7.
//
// Solidity: function setMapping4(int256 key, uint256 value) returns()
func (_StorageScan *StorageScanTransactorSession) SetMapping4(key *big.Int, value *big.Int) (*types.Transaction, error) {
	return _StorageScan.Contract.SetMapping4(&_StorageScan.TransactOpts, key, value)
}

// SetMapping5 is a paid mutator transaction binding the contract method 0x688388fd.
//
// Solidity: function setMapping5(bytes1 key, uint256 value) returns()
func (_StorageScan *StorageScanTransactor) SetMapping5(opts *bind.TransactOpts, key [1]byte, value *big.Int) (*types.Transaction, error) {
	return _StorageScan.contract.Transact(opts, "setMapping5", key, value)
}

// SetMapping5 is a paid mutator transaction binding the contract method 0x688388fd.
//
// Solidity: function setMapping5(bytes1 key, uint256 value) returns()
func (_StorageScan *StorageScanSession) SetMapping5(key [1]byte, value *big.Int) (*types.Transaction, error) {
	return _StorageScan.Contract.SetMapping5(&_StorageScan.TransactOpts, key, value)
}

// SetMapping5 is a paid mutator transaction binding the contract method 0x688388fd.
//
// Solidity: function setMapping5(bytes1 key, uint256 value) returns()
func (_StorageScan *StorageScanTransactorSession) SetMapping5(key [1]byte, value *big.Int) (*types.Transaction, error) {
	return _StorageScan.Contract.SetMapping5(&_StorageScan.TransactOpts, key, value)
}

// SetMapping6 is a paid mutator transaction binding the contract method 0x708945b0.
//
// Solidity: function setMapping6(uint256 key, uint256 _id, string _value) returns()
func (_StorageScan *StorageScanTransactor) SetMapping6(opts *bind.TransactOpts, key *big.Int, _id *big.Int, _value string) (*types.Transaction, error) {
	return _StorageScan.contract.Transact(opts, "setMapping6", key, _id, _value)
}

// SetMapping6 is a paid mutator transaction binding the contract method 0x708945b0.
//
// Solidity: function setMapping6(uint256 key, uint256 _id, string _value) returns()
func (_StorageScan *StorageScanSession) SetMapping6(key *big.Int, _id *big.Int, _value string) (*types.Transaction, error) {
	return _StorageScan.Contract.SetMapping6(&_StorageScan.TransactOpts, key, _id, _value)
}

// SetMapping6 is a paid mutator transaction binding the contract method 0x708945b0.
//
// Solidity: function setMapping6(uint256 key, uint256 _id, string _value) returns()
func (_StorageScan *StorageScanTransactorSession) SetMapping6(key *big.Int, _id *big.Int, _value string) (*types.Transaction, error) {
	return _StorageScan.Contract.SetMapping6(&_StorageScan.TransactOpts, key, _id, _value)
}

// SetSlice1At is a paid mutator transaction binding the contract method 0x6f4b210e.
//
// Solidity: function setSlice1At(uint256 idx, uint8 _v) returns()
func (_StorageScan *StorageScanTransactor) SetSlice1At(opts *bind.TransactOpts, idx *big.Int, _v uint8) (*types.Transaction, error) {
	return _StorageScan.contract.Transact(opts, "setSlice1At", idx, _v)
}

// SetSlice1At is a paid mutator transaction binding the contract method 0x6f4b210e.
//
// Solidity: function setSlice1At(uint256 idx, uint8 _v) returns()
func (_StorageScan *StorageScanSession) SetSlice1At(idx *big.Int, _v uint8) (*types.Transaction, error) {
	return _StorageScan.Contract.SetSlice1At(&_StorageScan.TransactOpts, idx, _v)
}

// SetSlice1At is a paid mutator transaction binding the contract method 0x6f4b210e.
//
// Solidity: function setSlice1At(uint256 idx, uint8 _v) returns()
func (_StorageScan *StorageScanTransactorSession) SetSlice1At(idx *big.Int, _v uint8) (*types.Transaction, error) {
	return _StorageScan.Contract.SetSlice1At(&_StorageScan.TransactOpts, idx, _v)
}

// SetSlice2At is a paid mutator transaction binding the contract method 0x51db4a33.
//
// Solidity: function setSlice2At(uint256 idx, uint256 _v) returns()
func (_StorageScan *StorageScanTransactor) SetSlice2At(opts *bind.TransactOpts, idx *big.Int, _v *big.Int) (*types.Transaction, error) {
	return _StorageScan.contract.Transact(opts, "setSlice2At", idx, _v)
}

// SetSlice2At is a paid mutator transaction binding the contract method 0x51db4a33.
//
// Solidity: function setSlice2At(uint256 idx, uint256 _v) returns()
func (_StorageScan *StorageScanSession) SetSlice2At(idx *big.Int, _v *big.Int) (*types.Transaction, error) {
	return _StorageScan.Contract.SetSlice2At(&_StorageScan.TransactOpts, idx, _v)
}

// SetSlice2At is a paid mutator transaction binding the contract method 0x51db4a33.
//
// Solidity: function setSlice2At(uint256 idx, uint256 _v) returns()
func (_StorageScan *StorageScanTransactorSession) SetSlice2At(idx *big.Int, _v *big.Int) (*types.Transaction, error) {
	return _StorageScan.Contract.SetSlice2At(&_StorageScan.TransactOpts, idx, _v)
}

// SetSlice3At is a paid mutator transaction binding the contract method 0x377e41eb.
//
// Solidity: function setSlice3At(uint256 idx, bool _v) returns()
func (_StorageScan *StorageScanTransactor) SetSlice3At(opts *bind.TransactOpts, idx *big.Int, _v bool) (*types.Transaction, error) {
	return _StorageScan.contract.Transact(opts, "setSlice3At", idx, _v)
}

// SetSlice3At is a paid mutator transaction binding the contract method 0x377e41eb.
//
// Solidity: function setSlice3At(uint256 idx, bool _v) returns()
func (_StorageScan *StorageScanSession) SetSlice3At(idx *big.Int, _v bool) (*types.Transaction, error) {
	return _StorageScan.Contract.SetSlice3At(&_StorageScan.TransactOpts, idx, _v)
}

// SetSlice3At is a paid mutator transaction binding the contract method 0x377e41eb.
//
// Solidity: function setSlice3At(uint256 idx, bool _v) returns()
func (_StorageScan *StorageScanTransactorSession) SetSlice3At(idx *big.Int, _v bool) (*types.Transaction, error) {
	return _StorageScan.Contract.SetSlice3At(&_StorageScan.TransactOpts, idx, _v)
}

// SetSlice4At is a paid mutator transaction binding the contract method 0x8db6f52b.
//
// Solidity: function setSlice4At(uint256 idx, string _v) returns()
func (_StorageScan *StorageScanTransactor) SetSlice4At(opts *bind.TransactOpts, idx *big.Int, _v string) (*types.Transaction, error) {
	return _StorageScan.contract.Transact(opts, "setSlice4At", idx, _v)
}

// SetSlice4At is a paid mutator transaction binding the contract method 0x8db6f52b.
//
// Solidity: function setSlice4At(uint256 idx, string _v) returns()
func (_StorageScan *StorageScanSession) SetSlice4At(idx *big.Int, _v string) (*types.Transaction, error) {
	return _StorageScan.Contract.SetSlice4At(&_StorageScan.TransactOpts, idx, _v)
}

// SetSlice4At is a paid mutator transaction binding the contract method 0x8db6f52b.
//
// Solidity: function setSlice4At(uint256 idx, string _v) returns()
func (_StorageScan *StorageScanTransactorSession) SetSlice4At(idx *big.Int, _v string) (*types.Transaction, error) {
	return _StorageScan.Contract.SetSlice4At(&_StorageScan.TransactOpts, idx, _v)
}

// SetSlice5At is a paid mutator transaction binding the contract method 0x07bc5764.
//
// Solidity: function setSlice5At(uint256 idx, uint256 _id, string _value) returns()
func (_StorageScan *StorageScanTransactor) SetSlice5At(opts *bind.TransactOpts, idx *big.Int, _id *big.Int, _value string) (*types.Transaction, error) {
	return _StorageScan.contract.Transact(opts, "setSlice5At", idx, _id, _value)
}

// SetSlice5At is a paid mutator transaction binding the contract method 0x07bc5764.
//
// Solidity: function setSlice5At(uint256 idx, uint256 _id, string _value) returns()
func (_StorageScan *StorageScanSession) SetSlice5At(idx *big.Int, _id *big.Int, _value string) (*types.Transaction, error) {
	return _StorageScan.Contract.SetSlice5At(&_StorageScan.TransactOpts, idx, _id, _value)
}

// SetSlice5At is a paid mutator transaction binding the contract method 0x07bc5764.
//
// Solidity: function setSlice5At(uint256 idx, uint256 _id, string _value) returns()
func (_StorageScan *StorageScanTransactorSession) SetSlice5At(idx *big.Int, _id *big.Int, _value string) (*types.Transaction, error) {
	return _StorageScan.Contract.SetSlice5At(&_StorageScan.TransactOpts, idx, _id, _value)
}

// SetString1 is a paid mutator transaction binding the contract method 0xbb3da883.
//
// Solidity: function setString1(string _v) returns()
func (_StorageScan *StorageScanTransactor) SetString1(opts *bind.TransactOpts, _v string) (*types.Transaction, error) {
	return _StorageScan.contract.Transact(opts, "setString1", _v)
}

// SetString1 is a paid mutator transaction binding the contract method 0xbb3da883.
//
// Solidity: function setString1(string _v) returns()
func (_StorageScan *StorageScanSession) SetString1(_v string) (*types.Transaction, error) {
	return _StorageScan.Contract.SetString1(&_StorageScan.TransactOpts, _v)
}

// SetString1 is a paid mutator transaction binding the contract method 0xbb3da883.
//
// Solidity: function setString1(string _v) returns()
func (_StorageScan *StorageScanTransactorSession) SetString1(_v string) (*types.Transaction, error) {
	return _StorageScan.Contract.SetString1(&_StorageScan.TransactOpts, _v)
}

// SetString2 is a paid mutator transaction binding the contract method 0xe02c1cd0.
//
// Solidity: function setString2(string _v) returns()
func (_StorageScan *StorageScanTransactor) SetString2(opts *bind.TransactOpts, _v string) (*types.Transaction, error) {
	return _StorageScan.contract.Transact(opts, "setString2", _v)
}

// SetString2 is a paid mutator transaction binding the contract method 0xe02c1cd0.
//
// Solidity: function setString2(string _v) returns()
func (_StorageScan *StorageScanSession) SetString2(_v string) (*types.Transaction, error) {
	return _StorageScan.Contract.SetString2(&_StorageScan.TransactOpts, _v)
}

// SetString2 is a paid mutator transaction binding the contract method 0xe02c1cd0.
//
// Solidity: function setString2(string _v) returns()
func (_StorageScan *StorageScanTransactorSession) SetString2(_v string) (*types.Transaction, error) {
	return _StorageScan.Contract.SetString2(&_StorageScan.TransactOpts, _v)
}

// SetUint1 is a paid mutator transaction binding the contract method 0x698ccd3a.
//
// Solidity: function setUint1(uint8 _v) returns()
func (_StorageScan *StorageScanTransactor) SetUint1(opts *bind.TransactOpts, _v uint8) (*types.Transaction, error) {
	return _StorageScan.contract.Transact(opts, "setUint1", _v)
}

// SetUint1 is a paid mutator transaction binding the contract method 0x698ccd3a.
//
// Solidity: function setUint1(uint8 _v) returns()
func (_StorageScan *StorageScanSession) SetUint1(_v uint8) (*types.Transaction, error) {
	return _StorageScan.Contract.SetUint1(&_StorageScan.TransactOpts, _v)
}

// SetUint1 is a paid mutator transaction binding the contract method 0x698ccd3a.
//
// Solidity: function setUint1(uint8 _v) returns()
func (_StorageScan *StorageScanTransactorSession) SetUint1(_v uint8) (*types.Transaction, error) {
	return _StorageScan.Contract.SetUint1(&_StorageScan.TransactOpts, _v)
}

// SetUint2 is a paid mutator transaction binding the contract method 0xa044bcd9.
//
// Solidity: function setUint2(uint128 _v) returns()
func (_StorageScan *StorageScanTransactor) SetUint2(opts *bind.TransactOpts, _v *big.Int) (*types.Transaction, error) {
	return _StorageScan.contract.Transact(opts, "setUint2", _v)
}

// SetUint2 is a paid mutator transaction binding the contract method 0xa044bcd9.
//
// Solidity: function setUint2(uint128 _v) returns()
func (_StorageScan *StorageScanSession) SetUint2(_v *big.Int) (*types.Transaction, error) {
	return _StorageScan.Contract.SetUint2(&_StorageScan.TransactOpts, _v)
}

// SetUint2 is a paid mutator transaction binding the contract method 0xa044bcd9.
//
// Solidity: function setUint2(uint128 _v) returns()
func (_StorageScan *StorageScanTransactorSession) SetUint2(_v *big.Int) (*types.Transaction, error) {
	return _StorageScan.Contract.SetUint2(&_StorageScan.TransactOpts, _v)
}

// SetUint3 is a paid mutator transaction binding the contract method 0x3da5a1ec.
//
// Solidity: function setUint3(uint256 _v) returns()
func (_StorageScan *StorageScanTransactor) SetUint3(opts *bind.TransactOpts, _v *big.Int) (*types.Transaction, error) {
	return _StorageScan.contract.Transact(opts, "setUint3", _v)
}

// SetUint3 is a paid mutator transaction binding the contract method 0x3da5a1ec.
//
// Solidity: function setUint3(uint256 _v) returns()
func (_StorageScan *StorageScanSession) SetUint3(_v *big.Int) (*types.Transaction, error) {
	return _StorageScan.Contract.SetUint3(&_StorageScan.TransactOpts, _v)
}

// SetUint3 is a paid mutator transaction binding the contract method 0x3da5a1ec.
//
// Solidity: function setUint3(uint256 _v) returns()
func (_StorageScan *StorageScanTransactorSession) SetUint3(_v *big.Int) (*types.Transaction, error) {
	return _StorageScan.Contract.SetUint3(&_StorageScan.TransactOpts, _v)
}
