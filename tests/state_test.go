package tests

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"
	"testing"

	state "github.com/0xPolygon/eth-state-transition"
	itrie "github.com/0xPolygon/eth-state-transition/immutable-trie"
	"github.com/0xPolygon/eth-state-transition/types"
	"github.com/stretchr/testify/assert"
	"github.com/umbracle/go-web3"
)

var (
	stateTests       = "GeneralStateTests"
	legacyStateTests = "LegacyTests/Constantinople/GeneralStateTests"
)

type stateCase struct {
	Env         *env                              `json:"env"`
	Pre         map[types.Address]*GenesisAccount `json:"pre"`
	Post        map[string]postState              `json:"post"`
	Transaction *stTransaction                    `json:"transaction"`
}

type wrapper struct {
	cc map[types.Address]*GenesisAccount
	//code    map[types.Hash][]byte
	raw state.Snapshot
	//storage map[types.Hash]map[types.Hash]types.Hash
}

func newWrapper(raw state.Snapshot, cc map[types.Address]*GenesisAccount) *wrapper {
	w := &wrapper{
		cc:  cc,
		raw: raw,
		//code:    map[types.Hash][]byte{},
		//storage: map[types.Hash]map[types.Hash]types.Hash{},
	}
	return w
}

func (w *wrapper) GetCode(hash types.Hash, addr types.Address) ([]byte, bool) {
	acct, ok := w.cc[addr]
	if !ok {
		return nil, false
	}
	if len(acct.Code) == 0 {
		return nil, false
	}
	return acct.Code, true

	//code, ok := w.code[hash]
	//return code, ok
}

func (w *wrapper) GetStorage(addr types.Address, root types.Hash, key types.Hash) types.Hash {
	if root == state.EmptyRootHash {
		return types.Hash{}
	}
	acct, ok := w.cc[addr]
	if !ok {
		return types.Hash{}
	}
	val, ok := acct.Storage[key]
	if !ok {
		return types.Hash{}
	}
	return val
}

func (w *wrapper) GetAccount(addr types.Address) (*state.Account, error) {
	acct, err := w.raw.GetAccount(addr)
	if err != nil {
		return nil, err
	}
	if acct == nil {
		return nil, nil
	}
	newAcct := &state.Account{
		Balance:  acct.Balance,
		Nonce:    acct.Nonce,
		CodeHash: acct.CodeHash,
		Root:     acct.Root,
	}
	//if !bytes.Equal(newAcct.CodeHash, EmptyCodeHash) {
	//w.code[types.BytesToHash(newAcct.CodeHash)] = w.cc[addr].Code
	//}
	//if !bytes.Equal(acct.Root.Bytes(), EmptyStateHash.Bytes()) {
	//w.storage[newAcct.Root] = w.cc[addr].Storage
	//}

	/*
		if code := w.cc[addr].Code; len(code) != 0 {
			rand.Read(newAcct.CodeHash)
			w.code[types.BytesToHash(newAcct.CodeHash)] = code
		} else {
			newAcct.CodeHash = state.EmptyCodeHash.Bytes()
		}
	*/

	/*
		// fill the storage
		if len(w.cc[addr].Storage) != 0 {
			rand.Read(newAcct.Root[:])
			w.storage[newAcct.Root] = w.cc[addr].Storage
		} else {
			newAcct.Root = state.EmptyRootHash
		}
	*/

	/*
		fmt.Println(types.BytesToHash(state.EmptyCodeHash[:]))
		fmt.Println(state.EmptyRootHash)
		fmt.Println(types.BytesToHash(acct.CodeHash))
		fmt.Println(types.BytesToHash(newAcct.CodeHash))
	*/

	return newAcct, nil
}

func RunSpecificTest(file string, t *testing.T, c stateCase, name, fork string, index int, p postEntry) {
	if fork == "EIP150" {
		// already self contained in the EIP 158
		//return
	}

	//if name != "RevertInCreateInInitCreate2" || fork != "Istanbul" {
	//	return
	//}

	//fmt.Println(state.EmptyRootHash)
	fmt.Println(file, name, fork, index)

	env := c.Env.ToEnv(t)

	// find the fork
	goahead, ok := Forks2[fork]
	if !ok {
		t.Fatalf("config %s not found", fork)
	}
	rev := goahead(int(env.Number))

	// fmt.Println("----------------")

	msg, err := c.Transaction.At(p.Indexes)
	if err != nil {
		t.Fatal(err)
	}

	snap, _ := buildState(t, c.Pre)

	runtimeCtx := env
	runtimeCtx.ChainID = 1

	wr := newWrapper(snap, c.Pre)
	transition := state.NewTransition(rev, runtimeCtx, wr)

	result, err := transition.Write(msg)
	assert.NoError(t, err)

	// txn.CleanDeleteObjects(forks.EIP158)
	objs := transition.Commit()
	//_, root := snap.Commit(objs)

	root, _ := computeRoot(c.Pre, objs)
	//if !bytes.Equal(root2, root) {
	//	panic("BAD")
	//}
	if !bytes.Equal(root, p.Root[:]) {
		t.Fatalf("root mismatch (%s %s %s %d): expected %s but found %s", file, name, fork, index, p.Root, hex.EncodeToString(root))
	}

	if logs := rlpHashLogs(result.Logs); !bytes.Equal(logs.Bytes(), p.Logs[:]) {
		t.Fatalf("logs mismatch (%s, %s %d): expected %s but found %s", name, fork, index, p.Logs, logs.String())
	}

	// panic("X")
}

var EmptyStateHash = types.StringToHash("0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421")

var EmptyCodeHash = web3.Keccak256(nil)

var zeroHash = types.Hash{}

func computeRoot(pre map[types.Address]*GenesisAccount, post []*state.Object) ([]byte, []byte) {
	s := itrie.NewArchiveState(itrie.NewMemoryStorage())
	snap := s.NewSnapshot()

	objs := []*state.Object{}
	for addr, data := range pre {
		single := &state.Object{
			Address:  addr,
			Balance:  data.Balance.Big(),
			Nonce:    data.Nonce.Uint64(),
			CodeHash: types.BytesToHash(EmptyCodeHash),
			Storage:  []*state.StorageObject{},
			Root:     EmptyStateHash,
		}
		if len(data.Code) != 0 {
			single.DirtyCode = true
			single.Code = data.Code
			single.CodeHash = types.BytesToHash(web3.Keccak256(data.Code))
		}

		for k, v := range data.Storage {
			entry := &state.StorageObject{
				Key: k.Bytes(),
			}
			if v == zeroHash {
				entry.Deleted = true
			} else {
				entry.Val = v.Bytes()
			}
			single.Storage = append(single.Storage, entry)
		}
		objs = append(objs, single)
	}

	objs = append(objs, post...)
	_, root := snap.Commit(objs)

	return root, nil
}

func TestState(t *testing.T) {
	long := []string{
		"static_Call50000",
		"static_Return50000",
		"static_Call1MB",
		"stQuadraticComplexityTest",
		"stTimeConsuming",
	}

	skip := []string{
		"RevertPrecompiledTouch",
		"failed_tx_xcf416c53",
	}

	// There are two folders in spec tests, one for the current tests for the Istanbul fork
	// and one for the legacy tests for the other forks
	folders, err := listFolders(stateTests, legacyStateTests)
	if err != nil {
		t.Fatal(err)
	}

	for _, folder := range folders {
		//t.Run(folder, func(t *testing.T) {
		files, err := listFiles(folder)
		if err != nil {
			t.Fatal(err)
		}

		for _, file := range files {
			if !strings.HasSuffix(file, ".json") {
				continue
			}

			if contains(long, file) && testing.Short() {
				t.Log("Long tests are skipped in short mode")
				continue
			}

			if contains(skip, file) {
				t.Log("Skip test")
				continue
			}

			data, err := ioutil.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}

			var c map[string]stateCase
			if err := json.Unmarshal(data, &c); err != nil {
				t.Fatal(err)
			}

			for name, i := range c {
				for fork, f := range i.Post {
					for indx, e := range f {
						RunSpecificTest(file, t, i, name, fork, indx, e)
					}
				}
			}
		}
		//})
	}
}

func rlpHashLogs(logs []*state.Log) (res types.Hash) {
	dst := web3.Keccak256(MarshalLogsWith(logs))
	return types.BytesToHash(dst)
}
