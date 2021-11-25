package types

import (
	"encoding/hex"
	"fmt"
	"strings"
)

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

var ZeroAddress = Address{}
var ZeroHash = Hash{}

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
