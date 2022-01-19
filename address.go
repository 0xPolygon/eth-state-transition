package state

import (
	"github.com/0xPolygon/eth-state-transition/types"
	"github.com/umbracle/fastrlp"
	"golang.org/x/crypto/sha3"
)

var addressPool fastrlp.ArenaPool

// CreateAddress creates an Ethereum address.
func CreateAddress(addr types.Address, nonce uint64) types.Address {
	a := addressPool.Get()
	defer addressPool.Put(a)

	v := a.NewArray()
	v.Set(a.NewBytes(addr.Bytes()))
	v.Set(a.NewUint(nonce))

	dst := v.MarshalTo(nil)
	dst = Keccak256(dst)[12:]

	return types.BytesToAddress(dst)
}

var create2Prefix = []byte{0xff}

// CreateAddress2 creates an Ethereum address following the CREATE2 Opcode.
func CreateAddress2(addr types.Address, salt [32]byte, inithash []byte) types.Address {
	return types.BytesToAddress(Keccak256(create2Prefix, addr.Bytes(), salt[:], Keccak256(inithash))[12:])
}

// Keccak256 calculates the Keccak256
func Keccak256(v ...[]byte) []byte {
	h := sha3.NewLegacyKeccak256()
	for _, i := range v {
		h.Write(i)
	}
	return h.Sum(nil)
}
