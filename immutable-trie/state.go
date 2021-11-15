package itrie

import (
	"fmt"

	lru "github.com/hashicorp/golang-lru"

	"bytes"

	state "github.com/0xPolygon/eth-state-transition"
	"github.com/0xPolygon/eth-state-transition/helper"
	"github.com/0xPolygon/eth-state-transition/types"
	"github.com/umbracle/fastrlp"
	"golang.org/x/crypto/sha3"
)

type State struct {
	storage Storage
	cache   *lru.Cache
}

func NewState(storage Storage) *State {
	cache, _ := lru.New(128)

	s := &State{
		storage: storage,
		cache:   cache,
	}
	return s
}

func (s *State) SetCode(hash types.Hash, code []byte) {
	s.storage.SetCode(hash, code)
}

func (s *State) GetCode(hash types.Hash) ([]byte, bool) {
	return s.storage.GetCode(hash)
}

func (s *State) NewSnapshot() state.SnapshotWriter {
	t := NewTrie()
	t.state = s
	t.storage = s.storage

	return &Snapshot{
		state:    s,
		trieRoot: t,
	}
}

func (s *State) NewSnapshotAt(root types.Hash) (state.SnapshotWriter, error) {
	if root == types.EmptyRootHash {
		// empty state
		return s.NewSnapshot(), nil
	}

	tt, ok := s.cache.Get(root)
	if ok {
		t := tt.(*Trie)
		t.state = s

		return &Snapshot{state: s, trieRoot: tt.(*Trie)}, nil
	}
	n, ok, err := GetNode(root.Bytes(), s.storage)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("state not found at hash %s", root)
	}
	t := &Trie{
		root:    n,
		state:   s,
		storage: s.storage,
	}
	return &Snapshot{
		state:    s,
		trieRoot: t,
	}, nil
}

func (s *State) AddState(root types.Hash, t *Trie) {
	s.cache.Add(root, t)
}

// this is a wrapper to represent the new snapshot entity

type Snapshot struct {
	state    *State
	trieRoot *Trie
}

func (s *Snapshot) GetCode(hash types.Hash) ([]byte, bool) {
	return s.state.GetCode(hash)
}

var accountArenaPool fastrlp.ArenaPool

var stateArenaPool fastrlp.ArenaPool // TODO, Remove once we do update in fastrlp

var stateStateParserPool fastrlp.ParserPool

func (s *Snapshot) GetStorage(root types.Hash, raw types.Hash) types.Hash {

	// Load trie from memory if there is some state
	var dummySnap *Snapshot
	if root == types.EmptyRootHash {
		dummySnap = s.state.NewSnapshot().(*Snapshot)
	} else {
		xx, err := s.state.NewSnapshotAt(root)
		if err != nil {
			panic(err)
		}
		dummySnap = xx.(*Snapshot)
	}

	key := helper.Keccak256(raw.Bytes())

	val, ok := dummySnap.trieRoot.Get(key)
	if !ok {
		return types.Hash{}
	}

	p := stateStateParserPool.Get()
	defer stateStateParserPool.Put(p)

	v, err := p.Parse(val)
	if err != nil {
		return types.Hash{}
	}

	res := []byte{}
	if res, err = v.GetBytes(res[:0]); err != nil {
		return types.Hash{}
	}

	return types.BytesToHash(res)
}

func (s *Snapshot) GetAccount(addr types.Address) (*types.Account, error) {
	data, ok := s.trieRoot.Get(helper.Keccak256(addr.Bytes()))
	if !ok {
		return nil, nil
	}

	var err error
	var account types.Account
	if err = account.UnmarshalRlp(data); err != nil {
		return nil, err
	}
	return &account, nil
}

func (s *Snapshot) Commit(objs []*state.Object) (state.SnapshotWriter, []byte) {

	// Create an insertion batch for all the entries
	batch := s.state.storage.Batch()

	tt := s.trieRoot.Txn()
	tt.batch = batch

	arena := accountArenaPool.Get()
	defer accountArenaPool.Put(arena)

	ar1 := stateArenaPool.Get()
	defer stateArenaPool.Put(ar1)

	for _, obj := range objs {
		if obj.Deleted {
			tt.Delete(hashit(obj.Address.Bytes()))
		} else {

			account := types.Account{
				Balance:  obj.Balance,
				Nonce:    obj.Nonce,
				CodeHash: obj.CodeHash.Bytes(),
				Root:     obj.Root, // old root
			}

			if len(obj.Storage) != 0 {
				localSnapshot, err := s.state.NewSnapshotAt(obj.Root)
				if err != nil {
					panic(err)
				}

				localTxn := localSnapshot.(*Snapshot).trieRoot.Txn()
				localTxn.batch = batch

				for _, entry := range obj.Storage {
					k := hashit(entry.Key)
					if entry.Deleted {
						localTxn.Delete(k)
					} else {
						vv := ar1.NewBytes(bytes.TrimLeft(entry.Val, "\x00"))
						localTxn.Insert(k, vv.MarshalTo(nil))
					}
				}

				accountStateRoot, _ := localTxn.Hash()
				accountStateTrie := localTxn.Commit()

				// Add this to the cache
				s.state.AddState(types.BytesToHash(accountStateRoot), accountStateTrie)

				account.Root = types.BytesToHash(accountStateRoot)
			}

			if obj.DirtyCode {
				s.state.SetCode(obj.CodeHash, obj.Code)
			}

			vv := account.MarshalWith(arena)
			data := vv.MarshalTo(nil)

			tt.Insert(hashit(obj.Address.Bytes()), data)
			arena.Reset()
		}
	}

	root, _ := tt.Hash()

	nTrie := tt.Commit()
	nTrie.state = s.state
	nTrie.storage = s.state.storage

	// Write all the entries to db
	batch.Write()

	s.state.AddState(types.BytesToHash(root), nTrie)
	return &Snapshot{state: s.state, trieRoot: nTrie}, root
}

func hashit(k []byte) []byte {
	h := sha3.NewLegacyKeccak256()
	h.Write(k)
	return h.Sum(nil)
}
