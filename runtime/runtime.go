package runtime

import (
	"errors"
	"math/big"

	"github.com/0xPolygon/eth-state-transition/types"
	"github.com/ethereum/evmc/v10/bindings/go/evmc"
)

// TxContext is the context of the transaction
type TxContext struct {
	Hash       types.Hash
	GasPrice   types.Hash
	Origin     types.Address
	Coinbase   types.Address
	Number     int64
	Timestamp  int64
	GasLimit   int64
	ChainID    int64
	Difficulty types.Hash
}

// Host is the execution host
type Host interface {
	AccountExists(addr types.Address) bool
	GetStorage(addr types.Address, key types.Hash) types.Hash
	SetStorage(addr types.Address, key types.Hash, value types.Hash) evmc.StorageStatus
	GetBalance(addr types.Address) *big.Int
	GetCodeSize(addr types.Address) int
	GetCodeHash(addr types.Address) types.Hash
	GetCode(addr types.Address) []byte
	Selfdestruct(addr types.Address, beneficiary types.Address)
	GetTxContext() TxContext
	GetBlockHash(number int64) types.Hash
	EmitLog(addr types.Address, topics []types.Hash, data []byte)
	Callx(*Contract) *ExecutionResult
	Empty(addr types.Address) bool

	Cally(kind evmc.CallKind,
		recipient types.Address, sender types.Address, value types.Hash, input []byte, gas int64, depth int,
		static bool, salt types.Hash, codeAddress types.Address) (output []byte, gasLeft int64, createAddr types.Address, err error)
}

// ExecutionResult includes all output after executing given evm
// message no matter the execution itself is successful or not.
type ExecutionResult struct {
	ReturnValue   []byte // Returned data from the runtime (function result or data supplied with revert opcode)
	GasLeft       uint64 // Total gas left as result of execution
	GasUsed       uint64 // Total gas used as result of execution
	Err           error  // Any error encountered during the execution, listed below
	CreateAddress types.Address
}

func (r *ExecutionResult) Succeeded() bool {
	return r.Err == nil
}

func (r *ExecutionResult) Failed() bool {
	return r.Err != nil
}

func (r *ExecutionResult) Reverted() bool {
	return r.Err == ErrExecutionReverted
}

var (
	ErrOutOfGas                 = errors.New("out of gas")
	ErrStackOverflow            = errors.New("stack overflow")
	ErrStackUnderflow           = errors.New("stack underflow")
	ErrNotEnoughFunds           = errors.New("not enough funds")
	ErrInsufficientBalance      = errors.New("insufficient balance for transfer")
	ErrMaxCodeSizeExceeded      = errors.New("evm: max code size exceeded")
	ErrContractAddressCollision = errors.New("contract address collision")
	ErrDepth                    = errors.New("max call depth exceeded")
	ErrExecutionReverted        = errors.New("execution was reverted")
	ErrCodeStoreOutOfGas        = errors.New("contract creation code storage out of gas")
)

// Contract is the instance being called
type Contract struct {
	Code        []byte
	Type        evmc.CallKind
	CodeAddress types.Address
	Address     types.Address
	Origin      types.Address
	Caller      types.Address
	Depth       int
	Value       *big.Int
	Input       []byte
	Gas         uint64
	Static      bool
	Salt        types.Hash
}

func NewContract(typ evmc.CallKind, depth int, origin types.Address, from types.Address, to types.Address, value *big.Int, gas uint64, code []byte) *Contract {
	f := &Contract{
		Type:        typ,
		Caller:      from,
		Origin:      origin,
		CodeAddress: to,
		Address:     to,
		Gas:         gas,
		Value:       value,
		Code:        code,
		Depth:       depth,
	}
	return f
}

func NewContractCreation(depth int, origin types.Address, from types.Address, to types.Address, value *big.Int, gas uint64, code []byte) *Contract {
	c := NewContract(evmc.Create, depth, origin, from, to, value, gas, code)
	return c
}

func NewContractCall(depth int, origin types.Address, from types.Address, to types.Address, value *big.Int, gas uint64, code []byte, input []byte) *Contract {
	c := NewContract(evmc.Call, depth, origin, from, to, value, gas, code)
	c.Input = input
	return c
}
