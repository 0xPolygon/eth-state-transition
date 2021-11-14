package state

import (
	"bytes"
	"math/big"

	iradix "github.com/hashicorp/go-immutable-radix"

	"github.com/0xPolygon/eth-state-transition/helper"
	"github.com/0xPolygon/eth-state-transition/types"
)

type Snapshot interface {
	GetCode(hash types.Hash) ([]byte, bool)
	GetStorage(root types.Hash, key types.Hash) types.Hash
	GetAccount(addr types.Address) (*types.Account, error)
	Commit(objs []*Object) (Snapshot, []byte)
}

var emptyCodeHash = helper.Keccak256(nil)

// StateObject is the internal representation of the account
type StateObject struct {
	Account   *types.Account
	Code      []byte
	Suicide   bool
	Deleted   bool
	DirtyCode bool
	Txn       *iradix.Txn
}

func (s *StateObject) Empty() bool {
	return s.Account.Nonce == 0 && s.Account.Balance.Sign() == 0 && bytes.Equal(s.Account.CodeHash, emptyCodeHash)
}

// Copy makes a copy of the state object
func (s *StateObject) Copy() *StateObject {
	ss := new(StateObject)

	// copy account
	ss.Account = s.Account.Copy()

	ss.Suicide = s.Suicide
	ss.Deleted = s.Deleted
	ss.DirtyCode = s.DirtyCode
	ss.Code = s.Code

	if s.Txn != nil {
		ss.Txn = s.Txn.CommitOnly().Txn()
	}
	//if s.Trie != nil {
	//	ss.Trie = s.Trie
	//}
	return ss
}

// Object is the serialization of the radix object (can be merged to StateObject?).
type Object struct {
	Address  types.Address
	CodeHash types.Hash
	Balance  *big.Int
	Root     types.Hash
	Nonce    uint64
	Deleted  bool

	// TODO: Move this to executor
	DirtyCode bool
	Code      []byte

	Storage []*StorageObject
}

// StorageObject is an entry in the storage
type StorageObject struct {
	Deleted bool
	Key     []byte
	Val     []byte
}
