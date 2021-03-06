package state

import (
	"math/big"

	"github.com/0xPolygon/eth-state-transition/types"
)

var (
	// EmptyRootHash is the root when there are no transactions
	EmptyRootHash = types.StringToHash("0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421")
)

type Transaction struct {
	Nonce    uint64
	GasPrice *big.Int
	Gas      uint64
	To       *types.Address
	Value    *big.Int
	Input    []byte
	Hash     types.Hash
	From     types.Address
}

func (t *Transaction) IsContractCreation() bool {
	return t.To == nil
}

func (t *Transaction) Copy() *Transaction {
	tt := new(Transaction)
	*tt = *t

	tt.GasPrice = new(big.Int)
	tt.GasPrice.Set(t.GasPrice)

	tt.Value = new(big.Int)
	tt.Value.Set(t.Value)

	tt.Input = make([]byte, len(t.Input))
	copy(tt.Input[:], t.Input[:])
	return tt
}
