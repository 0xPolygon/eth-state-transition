package runtime

import (
	"errors"
	"math/big"

	"github.com/0xPolygon/eth-state-transition/types"
	"github.com/ethereum/evmc/v10/bindings/go/evmc"
)

// Host is the execution host
type Host interface {
	AccountExists(addr evmc.Address) bool
	GetStorage(addr evmc.Address, key evmc.Hash) types.Hash

	SetStorage(addr types.Address, key types.Hash, value types.Hash) evmc.StorageStatus
	GetBalance(addr evmc.Address) *big.Int
	GetCodeSize(addr evmc.Address) int
	GetCodeHash(addr evmc.Address) evmc.Hash
	GetCode(addr evmc.Address) []byte
	Selfdestruct(addr evmc.Address, beneficiary evmc.Address)
	GetTxContext() evmc.TxContext
	GetBlockHash(number int64) evmc.Hash
	EmitLog(addr evmc.Address, topics []types.Hash, data []byte)
	Callx(*Contract) *ExecutionResult
	Empty(addr evmc.Address) bool

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
	Caller      types.Address
	Depth       int
	Value       *big.Int
	Input       []byte
	Gas         uint64
	Static      bool
	Salt        types.Hash
}

func NewContract(typ evmc.CallKind, depth int, from types.Address, to types.Address, value *big.Int, gas uint64, code []byte) *Contract {
	f := &Contract{
		Type:        typ,
		Caller:      from,
		CodeAddress: to,
		Address:     to,
		Gas:         gas,
		Value:       value,
		Code:        code,
		Depth:       depth,
	}
	return f
}

func NewContractCreation(depth int, from types.Address, to types.Address, value *big.Int, gas uint64, code []byte) *Contract {
	c := NewContract(evmc.Create, depth, from, to, value, gas, code)
	return c
}

func NewContractCall(depth int, from types.Address, to types.Address, value *big.Int, gas uint64, code []byte, input []byte) *Contract {
	c := NewContract(evmc.Call, depth, from, to, value, gas, code)
	c.Input = input
	return c
}
