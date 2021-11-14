package itrie

import (
	state "github.com/0xPolygon/eth-state-transition"
	"github.com/0xPolygon/eth-state-transition/types"
)

// this is a wrapper to represent the new snapshot entity

type Snapshot struct {
	state    *State
	trieRoot *Trie
}

func (s *Snapshot) GetCode(hash types.Hash) ([]byte, bool) {
	return s.state.GetCode(hash)
}

func (s *Snapshot) GetStorage(root types.Hash, key types.Hash) types.Hash {
	return s.trieRoot.GetStorage(root, key)
}

func (s *Snapshot) GetAccount(addr types.Address) (*types.Account, error) {
	return s.trieRoot.GetAccount(addr)
}

func (s *Snapshot) Commit(objs []*state.Object) (state.Snapshot, []byte) {
	return s.trieRoot.Commit(objs)
}
