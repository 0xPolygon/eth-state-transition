package types

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	"github.com/umbracle/fastrlp"
)

type Transaction struct {
	Nonce    uint64
	GasPrice *big.Int
	Gas      uint64
	To       *Address
	Value    *big.Int
	Input    []byte
	V        []byte
	R        []byte
	S        []byte
	Hash     Hash
	From     Address
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

const HashLength = 32

type Hash [32]byte

func (h Hash) Bytes() []byte {
	return h[:]
}

func BytesToHash(b []byte) Hash {
	var h Hash

	size := len(b)
	min := min(size, HashLength)

	copy(h[HashLength-min:], b[len(b)-min:])
	return h
}

func StringToHash(str string) Hash {
	return BytesToHash(stringToBytes(str))
}

func (h Hash) String() string {
	return "0x" + hex.EncodeToString(h[:])
}

const AddressLength = 20

type Address [AddressLength]byte

func (a Address) Bytes() []byte {
	return a[:]
}

func BytesToAddress(b []byte) Address {
	var a Address

	size := len(b)
	min := min(size, AddressLength)

	copy(a[AddressLength-min:], b[len(b)-min:])
	return a
}

func StringToAddress(str string) Address {
	return BytesToAddress(stringToBytes(str))
}

func (a Address) String() string {
	return "0x" + hex.EncodeToString(a[:])
}

type ReceiptStatus uint64

const (
	ReceiptFailed ReceiptStatus = iota
	ReceiptSuccess
)

type Receipt struct {
	// consensus fields
	Root              Hash
	CumulativeGasUsed uint64
	LogsBloom         Bloom
	Logs              []*Log
	Status            *ReceiptStatus

	// context fields
	GasUsed         uint64
	ContractAddress Address
	TxHash          Hash
}

func (r *Receipt) SetStatus(s ReceiptStatus) {
	r.Status = &s
}

type Log struct {
	Address Address
	Topics  []Hash
	Data    []byte
}

const BloomByteLength = 256

type Bloom [BloomByteLength]byte

func min(i, j int) int {
	if i < j {
		return i
	}
	return j
}

func stringToBytes(str string) []byte {
	str = strings.TrimPrefix(str, "0x")
	if len(str)%2 == 1 {
		str = "0" + str
	}
	b, _ := hex.DecodeString(str)
	return b
}

var (
	// EmptyRootHash is the root when there are no transactions
	EmptyRootHash = StringToHash("0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421")
)

var ZeroAddress = Address{}
var ZeroHash = Hash{}

// MarshalLogsWith marshals the logs of the receipt to RLP with a specific fastrlp.Arena
func (r *Receipt) MarshalLogsWith(a *fastrlp.Arena) *fastrlp.Value {
	if len(r.Logs) == 0 {
		// There are no receipts, write the RLP null array entry
		return a.NewNullArray()
	}
	logs := a.NewArray()
	for _, l := range r.Logs {
		logs.Set(l.MarshalRLPWith(a))
	}
	return logs
}

func (l *Log) MarshalRLPWith(a *fastrlp.Arena) *fastrlp.Value {
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

// UnmarshalText parses a hash in hex syntax.
func (h *Hash) UnmarshalText(input []byte) error {
	*h = BytesToHash(stringToBytes(string(input)))
	return nil
}

// UnmarshalText parses an address in hex syntax.
func (a *Address) UnmarshalText(input []byte) error {
	buf := stringToBytes(string(input))
	if len(buf) != AddressLength {
		return fmt.Errorf("incorrect length")
	}
	*a = BytesToAddress(buf)
	return nil
}

func (h Hash) MarshalText() ([]byte, error) {
	return []byte(h.String()), nil
}

func (a Address) MarshalText() ([]byte, error) {
	return []byte(a.String()), nil
}
