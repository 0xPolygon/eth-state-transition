package state

import (
	"fmt"
	"math"
	"math/big"

	"github.com/0xPolygon/eth-state-transition/precompiled"
	"github.com/0xPolygon/eth-state-transition/runtime"
	"github.com/0xPolygon/eth-state-transition/runtime/evm"
	"github.com/0xPolygon/eth-state-transition/types"
	"github.com/ethereum/evmc/v10/bindings/go/evmc"
	"github.com/umbracle/go-web3"
)

const (
	spuriousDragonMaxCodeSize = 24576

	// Per transaction not creating a contract
	TxGas uint64 = 21000

	// Per transaction that creates a contract
	TxGasContractCreation uint64 = 53000
)

var emptyCodeHashTwo = types.BytesToHash(web3.Keccak256(nil))

// GetHashByNumber returns the hash function of a block number
type GetHashByNumber = func(i uint64) types.Hash

type GetHashByNumberHelper = func(num uint64, hash types.Hash) GetHashByNumber

type Transition struct {
	// forks are the enabled forks for this transition
	rev evmc.Revision

	// txn is the transaction of changes
	txn *Txn

	// ctx is the block context
	ctx TxContext

	// GetHash GetHashByNumberHelper
	getHash GetHashByNumber
}

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

// NewExecutor creates a new executor
func NewTransition(rev evmc.Revision, ctx TxContext, snap Snapshot) *Transition {
	txn := NewTxn(snap)
	txn.rev = rev

	transition := &Transition{
		ctx: ctx,
		txn: txn,
		rev: rev,
	}

	// by default for getHash use a simple one
	transition.getHash = func(n uint64) types.Hash {
		return types.BytesToHash(web3.Keccak256([]byte(big.NewInt(int64(n)).String())))
	}

	return transition
}

func (e *Transition) Commit() []*Object {
	return e.txn.Commit()
}

func (t *Transition) SetGetHash(helper GetHashByNumberHelper) {
	t.getHash = helper(uint64(t.ctx.Number), t.ctx.Hash)
}

func (t *Transition) Txn() *Txn {
	return t.txn
}

// Write writes another transaction to the executor
func (t *Transition) Write(msg *Message) (*Output, error) {
	output, err := t.applyImpl(msg)
	if err != nil {
		return nil, err
	}

	if t.isRevision(evmc.Byzantium) {
		// The suicided accounts are set as deleted for the next iteration
		t.txn.CleanDeleteObjects(true)
	} else {
		// TODO: If byzntium is enabled you need a special step to commit the data yourself
		t.txn.CleanDeleteObjects(t.isRevision(evmc.TangerineWhistle))
	}

	return output, nil
}

// Apply applies a new transaction
func (t *Transition) applyImpl(msg *Message) (*Output, error) {
	if err := t.preCheck(msg); err != nil {
		return nil, err
	}
	output := t.apply(msg)
	t.postCheck(msg, output)
	return output, nil
}

var (
	ErrNonceIncorrect        = fmt.Errorf("incorrect nonce")
	ErrNotEnoughFundsForGas  = fmt.Errorf("not enough funds to cover gas costs")
	ErrBlockLimitReached     = fmt.Errorf("gas limit reached in the pool")
	ErrBlockLimitExceeded    = fmt.Errorf("transaction's gas limit exceeds block gas limit")
	ErrIntrinsicGasOverflow  = fmt.Errorf("overflow in intrinsic gas calculation")
	ErrNotEnoughIntrinsicGas = fmt.Errorf("not enough gas supplied for intrinsic gas costs")
	ErrNotEnoughFunds        = fmt.Errorf("not enough funds for transfer with given value")
)

func (t *Transition) isRevision(rev evmc.Revision) bool {
	return rev <= t.rev
}

func (t *Transition) preCheck(msg *Message) error {
	// 1. the nonce of the message caller is correct
	nonce := t.txn.GetNonce(msg.From)
	if nonce != msg.Nonce {
		return ErrNonceIncorrect
	}

	// 2. deduct the upfront max gas cost to cover transaction fee(gaslimit * gasprice)
	upfrontGasCost := new(big.Int).Set(msg.GasPrice)
	upfrontGasCost.Mul(upfrontGasCost, new(big.Int).SetUint64(msg.Gas))

	err := t.txn.SubBalance(msg.From, upfrontGasCost)
	if err != nil {
		if err == runtime.ErrNotEnoughFunds {
			return ErrNotEnoughFundsForGas
		}
		return err
	}

	// 4. there is no overflow when calculating intrinsic gas
	intrinsicGasCost, err := TransactionGasCost(msg, t.isRevision(evmc.Homestead), t.isRevision(evmc.Istanbul))
	if err != nil {
		return err
	}

	// 5. the purchased gas is enough to cover intrinsic usage
	gasLeft := msg.Gas - intrinsicGasCost
	// Because we are working with unsigned integers for gas, the `>` operator is used instead of the more intuitive `<`
	if gasLeft > msg.Gas {
		return ErrNotEnoughIntrinsicGas
	}

	// 6. caller has enough balance to cover asset transfer for **topmost** call
	if balance := t.txn.GetBalance(evmc.Address(msg.From)); balance.Cmp(msg.Value) < 0 {
		return ErrNotEnoughFunds
	}

	msg.Gas = gasLeft
	return nil
}

func (t *Transition) postCheck(msg *Message, output *Output) {
	var gasUsed uint64

	intrinsicGasCost, _ := TransactionGasCost(msg, t.isRevision(evmc.Homestead), t.isRevision(evmc.Istanbul))
	msg.Gas += intrinsicGasCost

	// Update gas used depending on the refund.
	refund := t.txn.GetRefund()
	{
		gasUsed = msg.Gas - output.GasLeft
		maxRefund := gasUsed / 2
		// Refund can go up to half the gas used
		if refund > maxRefund {
			refund = maxRefund
		}

		output.GasLeft += refund
		gasUsed -= refund
	}

	gasPrice := new(big.Int).Set(msg.GasPrice)

	// refund the sender
	remaining := new(big.Int).Mul(new(big.Int).SetUint64(output.GasLeft), gasPrice)
	t.txn.AddBalance(msg.From, remaining)

	// pay the coinbase for the transaction
	coinbaseFee := new(big.Int).Mul(new(big.Int).SetUint64(gasUsed), gasPrice)
	t.txn.AddBalance(t.ctx.Coinbase, coinbaseFee)
}

func (t *Transition) apply(msg *Message) *Output {
	gasPrice := new(big.Int).Set(msg.GasPrice)
	value := new(big.Int).Set(msg.Value)

	// Override the context and set the specific transaction fields
	t.ctx.GasPrice = types.BytesToHash(gasPrice.Bytes())
	t.ctx.Origin = msg.From

	var retValue []byte
	var gasLeft int64
	var err error

	if msg.IsContractCreation() {
		address := CreateAddress(msg.From, t.txn.GetNonce(msg.From))
		contract := runtime.NewContractCreation(0, msg.From, address, value, msg.Gas, msg.Input)
		retValue, gasLeft, err = t.applyCreate(contract)
	} else {
		t.txn.IncrNonce(msg.From)
		c := runtime.NewContractCall(0, msg.From, *msg.To, value, msg.Gas, t.txn.GetCode(evmc.Address(*msg.To)), msg.Input)
		retValue, gasLeft, err = t.applyCall(c, evmc.Call)
	}

	output := &Output{
		ReturnValue: retValue,
		Logs:        t.txn.Logs(),
		GasLeft:     uint64(gasLeft),
	}

	if err != nil {
		output.Success = false
	} else {
		output.Success = true
	}

	// if the transaction created a contract, store the creation address in the receipt.
	if msg.To == nil {
		output.ContractAddress = CreateAddress(msg.From, msg.Nonce)
	}

	return output
}

var (
	five  = types.StringToAddress("5")
	six   = types.StringToAddress("6")
	seven = types.StringToAddress("7")
	eight = types.StringToAddress("8")
	nine  = types.StringToAddress("9")
)

func (t *Transition) isPrecompiled(codeAddr types.Address) bool {
	if _, ok := precompiled.Contracts[codeAddr]; !ok {
		return false
	}

	// byzantium precompiles
	switch codeAddr {
	case five:
		fallthrough
	case six:
		fallthrough
	case seven:
		fallthrough
	case eight:
		return t.isRevision(evmc.Byzantium)
	}

	// istanbul precompiles
	switch codeAddr {
	case nine:
		return t.isRevision(evmc.Istanbul)
	}

	return true
}

func (t *Transition) run(contract *runtime.Contract) *runtime.ExecutionResult {
	if t.isPrecompiled(contract.CodeAddress) {
		return precompiled.Run(contract.CodeAddress, contract.Input, contract.Gas, t.rev)
	}

	evm := evm.EVM{
		Host: t,
		Rev:  t.rev,
	}
	return evm.Run(contract)
}

func (t *Transition) transfer(from, to types.Address, amount *big.Int) error {
	if amount == nil {
		return nil
	}

	if err := t.txn.SubBalance(from, amount); err != nil {
		if err == runtime.ErrNotEnoughFunds {
			return runtime.ErrInsufficientBalance
		}
		return err
	}

	t.txn.AddBalance(to, amount)
	return nil
}

func (t *Transition) applyCall(c *runtime.Contract, callType evmc.CallKind) ([]byte, int64, error) {
	snapshot := t.txn.Snapshot()
	t.txn.TouchAccount(c.Address)

	if callType == evmc.Call {
		// Transfers only allowed on calls
		if err := t.transfer(c.Caller, c.Address, c.Value); err != nil {
			return nil, int64(c.Gas), err
		}
	}

	result := t.run(c)
	if result.Failed() {
		t.txn.RevertToSnapshot(snapshot)
	}

	return result.ReturnValue, int64(result.GasLeft), result.Err
}

var emptyHash types.Hash

func (t *Transition) hasCodeOrNonce(addr types.Address) bool {
	nonce := t.txn.GetNonce(addr)
	if nonce != 0 {
		return true
	}
	codeHash := t.txn.GetCodeHash(evmc.Address(addr))
	if codeHash != emptyCodeHashTwo && codeHash != emptyHash {
		return true
	}
	return false
}

func (t *Transition) applyCreate(c *runtime.Contract) ([]byte, int64, error) {
	gasLimit := c.Gas

	var address types.Address
	if c.Type == evmc.Create {
		address = CreateAddress(c.Caller, t.GetNonce(c.Caller))
	} else if c.Type == evmc.Create2 {
		address = CreateAddress2(c.Caller, c.Salt, c.Code)
	} else {
		panic("X1")
	}

	c.CodeAddress = address
	c.Address = address

	// Increment the nonce of the caller
	t.txn.IncrNonce(c.Caller)

	// Check if there if there is a collision and the address already exists
	if t.hasCodeOrNonce(c.Address) {
		return nil, 0, runtime.ErrContractAddressCollision
	}

	// Take snapshot of the current state
	snapshot := t.txn.Snapshot()

	if t.isRevision(evmc.TangerineWhistle) {
		// Force the creation of the account
		t.txn.CreateAccount(c.Address)
		t.txn.IncrNonce(c.Address)
	}

	// Transfer the value
	if err := t.transfer(c.Caller, c.Address, c.Value); err != nil {
		return nil, int64(gasLimit), err
	}

	result := t.run(c)

	if result.Failed() {
		t.txn.RevertToSnapshot(snapshot)
		return result.ReturnValue, int64(result.GasLeft), result.Err
	}

	if t.isRevision(evmc.TangerineWhistle) && len(result.ReturnValue) > spuriousDragonMaxCodeSize {
		// Contract size exceeds 'SpuriousDragon' size limit
		t.txn.RevertToSnapshot(snapshot)
		return nil, 0, runtime.ErrMaxCodeSizeExceeded
	}

	gasCost := uint64(len(result.ReturnValue)) * 200

	if result.GasLeft < gasCost {
		result.Err = runtime.ErrCodeStoreOutOfGas
		result.ReturnValue = nil

		// Out of gas creating the contract
		if t.isRevision(evmc.Homestead) {
			t.txn.RevertToSnapshot(snapshot)
			result.GasLeft = 0
		}

		return result.ReturnValue, int64(result.GasLeft), result.Err
	}

	result.GasLeft -= gasCost
	t.txn.SetCode(c.Address, result.ReturnValue)

	return result.ReturnValue, int64(result.GasLeft), result.Err
}

func (t *Transition) SetStorage(addr types.Address, key types.Hash, value types.Hash) evmc.StorageStatus {
	return t.txn.SetStorage(addr, key, value)
}

func (t *Transition) GetTxContext() evmc.TxContext {
	chainID := new(big.Int).SetInt64(t.ctx.ChainID)
	cc := types.BytesToHash(chainID.Bytes())

	ctx := evmc.TxContext{
		GasPrice:   evmc.Hash(t.ctx.GasPrice),
		Origin:     evmc.Address(t.ctx.Origin),
		Coinbase:   evmc.Address(t.ctx.Coinbase),
		Number:     t.ctx.Number,
		Timestamp:  t.ctx.Timestamp,
		GasLimit:   t.ctx.GasLimit,
		Difficulty: evmc.Hash(t.ctx.Difficulty),
		ChainID:    evmc.Hash(cc),
	}
	return ctx
}

func (t *Transition) GetBlockHash(number int64) (res evmc.Hash) {
	return evmc.Hash(t.getHash(uint64(number)))
}

func (t *Transition) EmitLog(addr evmc.Address, topics []types.Hash, data []byte) {
	t.txn.EmitLog(addr, topics, data)
}

func (t *Transition) GetCodeSize(addr evmc.Address) int {
	return t.txn.GetCodeSize(addr)
}

func (t *Transition) GetCodeHash(addr evmc.Address) (res evmc.Hash) {
	return evmc.Hash(t.txn.GetCodeHash(addr))
}

func (t *Transition) GetCode(addr evmc.Address) []byte {
	return t.txn.GetCode(addr)
}

func (t *Transition) GetBalance(addr evmc.Address) *big.Int {
	return t.txn.GetBalance(addr)
}

func (t *Transition) GetStorage(addr evmc.Address, key evmc.Hash) types.Hash {
	return t.txn.GetState(addr, key)
}

func (t *Transition) AccountExists(addr evmc.Address) bool {
	return t.txn.Exist(addr)
}

func (t *Transition) Empty(addr evmc.Address) bool {
	return t.txn.Empty(addr)
}

func (t *Transition) GetNonce(addr types.Address) uint64 {
	return t.txn.GetNonce(addr)
}

func (t *Transition) Selfdestruct(addr evmc.Address, beneficiary evmc.Address) {
	if !t.txn.HasSuicided(types.Address(addr)) {
		t.txn.AddRefund(24000)
	}
	t.txn.AddBalance(types.Address(beneficiary), t.txn.GetBalance(evmc.Address(addr)))
	t.txn.Suicide(types.Address(addr))
}

func (t *Transition) Cally(kind evmc.CallKind,
	recipient types.Address, sender types.Address, value types.Hash, input []byte, gas int64, depth int,
	static bool, salt types.Hash, codeAddress types.Address) (output []byte, gasLeft int64, createAddr types.Address, err error) {

	return nil, 0, types.Address{}, nil
}

func (t *Transition) Callx(c *runtime.Contract) ([]byte, int64, error) {
	if c.Type == evmc.Create || c.Type == evmc.Create2 {
		return t.applyCreate(c)
	}
	return t.applyCall(c, c.Type)
}

func TransactionGasCost(msg *Message, isHomestead, isIstanbul bool) (uint64, error) {
	cost := uint64(0)

	// Contract creation is only paid on the homestead fork
	if msg.IsContractCreation() && isHomestead {
		cost += TxGasContractCreation
	} else {
		cost += TxGas
	}

	payload := msg.Input
	if len(payload) > 0 {
		zeros := uint64(0)
		for i := 0; i < len(payload); i++ {
			if payload[i] == 0 {
				zeros++
			}
		}

		nonZeros := uint64(len(payload)) - zeros
		nonZeroCost := uint64(68)
		if isIstanbul {
			nonZeroCost = 16
		}

		if (math.MaxUint64-cost)/nonZeroCost < nonZeros {
			return 0, ErrIntrinsicGasOverflow
		}

		cost += nonZeros * nonZeroCost

		if (math.MaxUint64-cost)/4 < zeros {
			return 0, ErrIntrinsicGasOverflow
		}

		cost += zeros * 4
	}

	return cost, nil
}
