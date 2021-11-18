package state

import (
	"math/big"

	"github.com/0xPolygon/eth-state-transition/types"
)

type Transaction struct {
	Nonce    uint64
	GasPrice *big.Int
	Gas      uint64
	To       *types.Address
	Value    *big.Int
	Input    []byte
	V        []byte
	R        []byte
	S        []byte
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

	tt.R = make([]byte, len(t.R))
	copy(tt.R[:], t.R[:])
	tt.S = make([]byte, len(t.S))
	copy(tt.S[:], t.S[:])

	tt.Input = make([]byte, len(t.Input))
	copy(tt.Input[:], t.Input[:])
	return tt
}
