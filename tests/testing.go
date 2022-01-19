package tests

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"

	state "github.com/0xPolygon/eth-state-transition"
	"github.com/0xPolygon/eth-state-transition/helper"
	itrie "github.com/0xPolygon/eth-state-transition/immutable-trie"
	"github.com/0xPolygon/eth-state-transition/runtime"
	"github.com/0xPolygon/eth-state-transition/types"
	"github.com/ethereum/evmc/v10/bindings/go/evmc"
	"github.com/stretchr/testify/assert"
	"github.com/umbracle/fastrlp"
)

// TESTS is the default location of the tests folder
const TESTS = "./tests"

type info struct {
	Comment     string `json:"comment"`
	FilledWith  string `json:"filledwith"`
	LllcVersion string `json:"lllcversion"`
	Source      string `json:"source"`
	SourceHash  string `json:"sourcehash"`
}

type env struct {
	Coinbase   string `json:"currentCoinbase"`
	Difficulty string `json:"currentDifficulty"`
	GasLimit   string `json:"currentGasLimit"`
	Number     string `json:"currentNumber"`
	Timestamp  string `json:"currentTimestamp"`
}

func remove0xPrefix(str string) string {
	if strings.HasPrefix(str, "0x") {
		return strings.Replace(str, "0x", "", -1)
	}
	return str
}

func stringToAddress(str string) (types.Address, error) {
	if str == "" {
		return types.Address{}, fmt.Errorf("value not found")
	}
	return types.StringToAddress(str), nil
}

func stringToHash(str string) (types.Hash, error) {
	if str == "" {
		return types.Hash{}, fmt.Errorf("value not found")
	}
	return types.StringToHash(str), nil
}

func stringToBigInt(str string) (*big.Int, error) {
	if str == "" {
		return nil, fmt.Errorf("value not found")
	}
	base := 10
	if strings.HasPrefix(str, "0x") {
		str, base = remove0xPrefix(str), 16
	}
	n, ok := big.NewInt(1).SetString(str, base)
	if !ok {
		return nil, fmt.Errorf("failed to convert %s to big.Int with base %d", str, base)
	}
	return n, nil
}

func stringToAddressT(t *testing.T, str string) types.Address {
	address, err := stringToAddress(str)
	if err != nil {
		t.Fatal(err)
	}
	return address
}

func stringToHashT(t *testing.T, str string) types.Hash {
	address, err := stringToHash(str)
	if err != nil {
		t.Fatal(err)
	}
	return address
}

func stringToUint64(str string) (uint64, error) {
	n, err := stringToBigInt(str)
	if err != nil {
		return 0, err
	}
	return n.Uint64(), nil
}

func stringToUint64T(t *testing.T, str string) uint64 {
	n, err := stringToUint64(str)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func stringToInt64T(t *testing.T, str string) int64 {
	n, err := stringToUint64(str)
	if err != nil {
		t.Fatal(err)
	}
	return int64(n)
}

func (e *env) ToHeader(t *testing.T) runtime.TxContext {
	return runtime.TxContext{
		Coinbase:   stringToAddressT(t, e.Coinbase),
		Difficulty: stringToHashT(t, e.Difficulty),
		GasLimit:   stringToInt64T(t, e.GasLimit),
		Number:     stringToInt64T(t, e.Number),
		Timestamp:  stringToInt64T(t, e.Timestamp),
	}
}

func (e *env) ToEnv(t *testing.T) runtime.TxContext {
	return runtime.TxContext{
		Coinbase:   stringToAddressT(t, e.Coinbase),
		Difficulty: stringToHashT(t, e.Difficulty),
		GasLimit:   stringToInt64T(t, e.GasLimit),
		Number:     stringToInt64T(t, e.Number),
		Timestamp:  stringToInt64T(t, e.Timestamp),
	}
}

type exec struct {
	Address  types.Address
	Caller   types.Address
	Origin   types.Address
	Code     []byte
	Data     []byte
	Value    *big.Int
	GasLimit uint64
	GasPrice *big.Int
}

func (e *exec) UnmarshalJSON(input []byte) error {
	type execUnmarshall struct {
		Address  types.Address `json:"address"`
		Caller   types.Address `json:"caller"`
		Origin   types.Address `json:"origin"`
		Code     string        `json:"code"`
		Data     string        `json:"data"`
		Value    string        `json:"value"`
		Gas      string        `json:"gas"`
		GasPrice string        `json:"gasPrice"`
	}

	var dec execUnmarshall
	err := json.Unmarshal(input, &dec)
	if err != nil {
		return err
	}

	e.Address = dec.Address
	e.Caller = dec.Caller
	e.Origin = dec.Origin

	e.Code, err = ParseBytes(&dec.Code)
	if err != nil {
		return err
	}
	e.Data, err = ParseBytes(&dec.Data)
	if err != nil {
		return err
	}

	e.Value, err = ParseUint256orHex(&dec.Value)
	if err != nil {
		return err
	}
	e.GasLimit, err = ParseUint64orHex(&dec.Gas)
	if err != nil {
		return err
	}
	e.GasPrice, err = ParseUint256orHex(&dec.GasPrice)
	if err != nil {
		return err
	}

	return nil
}

func buildState(t *testing.T, allocs map[types.Address]*GenesisAccount) (state.SnapshotWriter, types.Hash) {
	s := itrie.NewArchiveState(itrie.NewMemoryStorage())
	snap := s.NewSnapshot()

	txn := state.NewTxn(snap)

	for addr, alloc := range allocs {
		txn.CreateAccount(addr)
		txn.SetNonce(addr, alloc.Nonce)
		txn.SetBalance(addr, alloc.Balance)

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
	Root    types.Hash
	Logs    types.Hash
	Indexes indexes
}

type postState []postEntry

func (p *postEntry) UnmarshalJSON(input []byte) error {
	type stateUnmarshall struct {
		Root    string  `json:"hash"`
		Logs    string  `json:"logs"`
		Indexes indexes `json:"indexes"`
	}

	var dec stateUnmarshall
	if err := json.Unmarshal(input, &dec); err != nil {
		return err
	}

	p.Root = types.StringToHash(dec.Root)
	p.Logs = types.StringToHash(dec.Logs)
	p.Indexes = dec.Indexes

	return nil
}

type stTransaction struct {
	Data     []string       `json:"data"`
	GasLimit []uint64       `json:"gasLimit"`
	Value    []*big.Int     `json:"value"`
	GasPrice *big.Int       `json:"gasPrice"`
	Nonce    uint64         `json:"nonce"`
	From     types.Address  `json:"secretKey"`
	To       *types.Address `json:"to"`
}

func (t *stTransaction) At(i indexes) (*state.Transaction, error) {
	if i.Data > len(t.Data) {
		return nil, fmt.Errorf("data index %d out of bounds (%d)", i.Data, len(t.Data))
	}
	if i.Gas > len(t.GasLimit) {
		return nil, fmt.Errorf("gas index %d out of bounds (%d)", i.Gas, len(t.GasLimit))
	}
	if i.Value > len(t.Value) {
		return nil, fmt.Errorf("value index %d out of bounds (%d)", i.Value, len(t.Value))
	}

	msg := &state.Transaction{
		To:       t.To,
		Nonce:    t.Nonce,
		Value:    new(big.Int).Set(t.Value[i.Value]),
		Gas:      t.GasLimit[i.Gas],
		GasPrice: new(big.Int).Set(t.GasPrice),
		Input:    helper.MustDecodeHex(t.Data[i.Data]),
	}

	msg.From = t.From
	return msg, nil
}

func (t *stTransaction) UnmarshalJSON(input []byte) error {
	type txUnmarshall struct {
		Data      []string `json:"data"`
		GasLimit  []string `json:"gasLimit"`
		Value     []string `json:"value"`
		GasPrice  string   `json:"gasPrice"`
		Nonce     string   `json:"nonce"`
		SecretKey string   `json:"secretKey"`
		To        string   `json:"to"`
	}

	var dec txUnmarshall
	err := json.Unmarshal(input, &dec)
	if err != nil {
		return err
	}

	t.Data = dec.Data
	for _, i := range dec.GasLimit {
		if j, err := stringToUint64(i); err != nil {
			return err
		} else {
			t.GasLimit = append(t.GasLimit, j)
		}
	}

	for _, i := range dec.Value {
		value := new(big.Int)
		if i != "0x" {
			v, err := ParseUint256orHex(&i)
			if err != nil {
				return err
			}
			/*
				v, ok := math.ParseBig256(i)
				if !ok {
					return fmt.Errorf("invalid tx value %q", i)
				}
			*/
			value = v
		}
		t.Value = append(t.Value, value)
	}

	t.GasPrice, err = stringToBigInt(dec.GasPrice)
	if err != nil {
		return err
	}

	t.Nonce, err = stringToUint64(dec.Nonce)
	if err != nil {
		return err
	}

	t.From = types.Address{}
	if len(dec.SecretKey) > 0 {
		secretKey, err := ParseBytes(&dec.SecretKey)
		if err != nil {
			return err
		}
		key, err := helper.ParsePrivateKey(secretKey)
		if err != nil {
			return fmt.Errorf("invalid private key: %v", err)
		}
		t.From = helper.PubKeyToAddress(&key.PublicKey)
	}

	if dec.To != "" {
		address := types.StringToAddress(dec.To)
		t.To = &address
	}
	return nil
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
