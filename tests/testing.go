package tests

import (
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"math/big"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	state "github.com/0xPolygon/eth-state-transition"
	itrie "github.com/0xPolygon/eth-state-transition/immutable-trie"
	"github.com/0xPolygon/eth-state-transition/types"
	"github.com/ethereum/evmc/v10/bindings/go/evmc"
	"github.com/stretchr/testify/assert"
	"github.com/umbracle/fastrlp"
	"github.com/umbracle/go-web3/wallet"
)

// TESTS is the default location of the tests folder
const TESTS = "./tests"

type env struct {
	Coinbase   argAddr   `json:"currentCoinbase"`
	Difficulty argHash   `json:"currentDifficulty"`
	GasLimit   argUint64 `json:"currentGasLimit"`
	Number     argUint64 `json:"currentNumber"`
	Timestamp  argUint64 `json:"currentTimestamp"`
}

func (e *env) ToEnv(t *testing.T) state.TxContext {
	return state.TxContext{
		Coinbase:   types.Address(e.Coinbase),
		Difficulty: types.Hash(e.Difficulty),
		GasLimit:   int64(e.GasLimit.Uint64()),
		Number:     int64(e.Number.Uint64()),
		Timestamp:  int64(e.Timestamp.Uint64()),
	}
}

func buildState(t *testing.T, allocs map[types.Address]*GenesisAccount) (state.SnapshotWriter, types.Hash) {
	s := itrie.NewArchiveState(itrie.NewMemoryStorage())
	snap := s.NewSnapshot()

	txn := state.NewTxn(snap)

	for addr, alloc := range allocs {
		txn.CreateAccount(addr)
		txn.SetNonce(addr, alloc.Nonce.Uint64())
		txn.SetBalance(addr, alloc.Balance.Big())

		if len(alloc.Code) != 0 {
			txn.SetCode(addr, alloc.Code)
		}

		for k, v := range alloc.Storage {
			txn.SetState(addr, types.BytesToHash(k[:]), types.BytesToHash(v[:]))
		}
	}

	objs := txn.Commit()
	_, root := snap.Commit(objs)

	snap, err := s.NewSnapshotAt(types.BytesToHash(root))
	assert.NoError(t, err)

	return snap, types.BytesToHash(root)
}

type indexes struct {
	Data  int `json:"data"`
	Gas   int `json:"gas"`
	Value int `json:"value"`
}

type postEntry struct {
	Root    argHash `json:"hash"`
	Logs    argHash `json:"logs"`
	Indexes indexes `json:"indexes"`
}

type postState []postEntry

type stTransaction struct {
	Data      []argBytes  `json:"data"`
	GasLimit  []argUint64 `json:"gasLimit"`
	Value     []argBig    `json:"value"`
	GasPrice  argBig      `json:"gasPrice"`
	Nonce     argUint64   `json:"nonce"`
	SecretKey argBytes    `json:"secretKey"`
	To        string      `json:"to"`
}

func (t *stTransaction) At(i indexes) (*state.Message, error) {
	if i.Data > len(t.Data) {
		return nil, fmt.Errorf("data index %d out of bounds (%d)", i.Data, len(t.Data))
	}
	if i.Gas > len(t.GasLimit) {
		return nil, fmt.Errorf("gas index %d out of bounds (%d)", i.Gas, len(t.GasLimit))
	}
	if i.Value > len(t.Value) {
		return nil, fmt.Errorf("value index %d out of bounds (%d)", i.Value, len(t.Value))
	}

	msg := &state.Message{
		Nonce:    t.Nonce.Uint64(),
		Value:    t.Value[i.Value].Big(),
		Gas:      t.GasLimit[i.Gas].Uint64(),
		GasPrice: t.GasPrice.Big(),
		Input:    t.Data[i.Data],
	}
	if t.To != "" {
		address := types.StringToAddress(t.To)
		msg.To = &address
	}

	var from types.Address
	if len(t.SecretKey) > 0 {
		key, err := wallet.ParsePrivateKey(t.SecretKey)
		if err != nil {
			return nil, fmt.Errorf("invalid private key: %v", err)
		}
		from = types.Address(wallet.NewKey(key).Address())
	}

	msg.From = from
	return msg, nil
}

// forks

type blockB func(i int) evmc.Revision

var Forks2 = map[string]blockB{
	"Frontier": func(i int) evmc.Revision {
		return evmc.Frontier
	},
	"Homestead": func(i int) evmc.Revision {
		return evmc.Homestead
	},
	"EIP150": func(i int) evmc.Revision {
		return evmc.TangerineWhistle
	},
	"EIP158": func(i int) evmc.Revision {
		return evmc.TangerineWhistle
	},
	"Byzantium": func(i int) evmc.Revision {
		return evmc.Byzantium
	},
	"Constantinople": func(i int) evmc.Revision {
		return evmc.Constantinople
	},
	"ConstantinopleFix": func(i int) evmc.Revision {
		return evmc.Petersburg
	},
	"Istanbul": func(i int) evmc.Revision {
		return evmc.Istanbul
	},
	"FrontierToHomesteadAt5": func(i int) evmc.Revision {
		if i < 5 {
			return evmc.Frontier
		}
		return evmc.Homestead
	},
	"HomesteadToEIP150At5": func(i int) evmc.Revision {
		if i < 5 {
			return evmc.Homestead
		}
		return evmc.TangerineWhistle
	},
	"EIP158ToByzantiumAt5": func(i int) evmc.Revision {
		if i < 5 {
			return evmc.TangerineWhistle
		}
		return evmc.Byzantium
	},
	"ByzantiumToConstantinopleAt5": func(i int) evmc.Revision {
		if i < 5 {
			return evmc.Byzantium
		}
		return evmc.Constantinople
	},
}

func contains(l []string, name string) bool {
	for _, i := range l {
		if strings.Contains(name, i) {
			return true
		}
	}
	return false
}

func listFolders(paths ...string) ([]string, error) {
	folders := []string{}

	for _, p := range paths {
		path := filepath.Join(TESTS, p)

		files, err := ioutil.ReadDir(path)
		if err != nil {
			return nil, err
		}
		for _, i := range files {
			if i.IsDir() {
				folders = append(folders, filepath.Join(path, i.Name()))
			}
		}
	}
	return folders, nil
}

func listFiles(folder string) ([]string, error) {
	if !strings.HasPrefix(folder, filepath.Base(TESTS)) {
		folder = filepath.Join(TESTS, folder)
	}

	files := []string{}
	err := filepath.Walk(folder, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

// MarshalLogsWith marshals the logs of the receipt to RLP with a specific fastrlp.Arena
func MarshalLogsWith(logs []*state.Log) []byte {
	a := &fastrlp.Arena{}

	marshalLog := func(l *state.Log) *fastrlp.Value {
		v := a.NewArray()
		v.Set(a.NewBytes(l.Address.Bytes()))

		topics := a.NewArray()
		for _, t := range l.Topics {
			topics.Set(a.NewBytes(t.Bytes()))
		}
		v.Set(topics)
		v.Set(a.NewBytes(l.Data))
		return v
	}

	if len(logs) == 0 {
		// There are no receipts, write the RLP null array entry
		return a.NewNullArray().MarshalTo(nil)
	}
	vals := a.NewArray()
	for _, l := range logs {
		vals.Set(marshalLog(l))
	}
	return vals.MarshalTo(nil)

}

// GenesisAccount is an account in the state of the genesis block.
type GenesisAccount struct {
	Code       argBytes                  `json:"code,omitempty"`
	Storage    map[types.Hash]types.Hash `json:"storage,omitempty"`
	Balance    argBig                    `json:"balance,omitempty"`
	Nonce      argUint64                 `json:"nonce,omitempty"`
	PrivateKey *argBytes                 `json:"secretKey,omitempty"` // for tests
}

type argBig big.Int

func (a *argBig) UnmarshalText(input []byte) error {
	buf, err := decodeToHex(input)
	if err != nil {
		return err
	}
	b := new(big.Int)
	b.SetBytes(buf)
	*a = argBig(*b)
	return nil
}

func (a argBig) MarshalText() ([]byte, error) {
	b := (*big.Int)(&a)
	return []byte("0x" + b.Text(16)), nil
}

func (a argBig) Big() *big.Int {
	val := big.Int(a)
	return new(big.Int).Set(&val)
}

type argUint64 uint64

func (b argUint64) MarshalText() ([]byte, error) {
	buf := make([]byte, 2, 10)
	copy(buf, `0x`)
	buf = strconv.AppendUint(buf, uint64(b), 16)
	return buf, nil
}

func (u *argUint64) UnmarshalText(input []byte) error {
	str := strings.TrimPrefix(string(input), "0x")
	num, err := strconv.ParseUint(str, 16, 64)
	if err != nil {
		return err
	}
	*u = argUint64(num)
	return nil
}

func (u *argUint64) Uint64() uint64 {
	return uint64(*u)
}

type argBytes []byte

func (b argBytes) MarshalText() ([]byte, error) {
	return encodeToHex(b), nil
}

func (b *argBytes) UnmarshalText(input []byte) error {
	hh, err := decodeToHex(input)
	if err != nil {
		return nil
	}
	aux := make([]byte, len(hh))
	copy(aux[:], hh[:])
	*b = aux
	return nil
}

func (b *argBytes) Bytes() []byte {
	return *b
}

type argAddr types.Address

func (a *argAddr) UnmarshalText(input []byte) error {
	hh, err := decodeToHex(input)
	if err != nil {
		return nil
	}
	addr := types.BytesToAddress(hh)
	*a = argAddr(addr)
	return nil
}

type argHash types.Hash

func (a *argHash) UnmarshalText(input []byte) error {
	hh, err := decodeToHex(input)
	if err != nil {
		return nil
	}
	hash := types.BytesToHash(hh)
	*a = argHash(hash)
	return nil
}

func decodeToHex(b []byte) ([]byte, error) {
	str := string(b)
	str = strings.TrimPrefix(str, "0x")
	if len(str)%2 != 0 {
		str = "0" + str
	}
	return hex.DecodeString(str)
}

func encodeToHex(b []byte) []byte {
	str := hex.EncodeToString(b)
	if len(str)%2 != 0 {
		str = "0" + str
	}
	return []byte("0x" + str)
}
