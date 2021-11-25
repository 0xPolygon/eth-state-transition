package itrie

import (
	"github.com/0xPolygon/eth-state-transition/helper"
	"github.com/0xPolygon/eth-state-transition/types"
)

type Batch interface {
	Put(k, v []byte)
	Write()
}

// Storage stores the trie
type Storage interface {
	Put(k, v []byte)
	Get(k []byte) ([]byte, bool)
	Batch() Batch
	SetCode(hash types.Hash, code []byte)
	GetCode(hash types.Hash) ([]byte, bool)
	Close() error
}

type memStorage struct {
	db   map[string][]byte
	code map[string][]byte
}

type memBatch struct {
	db *map[string][]byte
}

// NewMemoryStorage creates an inmemory trie storage
func NewMemoryStorage() Storage {
	return &memStorage{db: map[string][]byte{}, code: map[string][]byte{}}
}

func (m *memStorage) Put(p []byte, v []byte) {
	buf := make([]byte, len(v))
	copy(buf[:], v[:])
	m.db[helper.EncodeToHex(p)] = buf
}

func (m *memStorage) Get(p []byte) ([]byte, bool) {
	v, ok := m.db[helper.EncodeToHex(p)]
	if !ok {
		return []byte{}, false
	}
	return v, true
}

func (m *memStorage) SetCode(hash types.Hash, code []byte) {
	m.code[hash.String()] = code
}

func (m *memStorage) GetCode(hash types.Hash) ([]byte, bool) {
	code, ok := m.code[hash.String()]
	return code, ok
}

func (m *memStorage) Batch() Batch {
	return &memBatch{db: &m.db}
}

func (m *memStorage) Close() error {
	return nil
}

func (m *memBatch) Put(p, v []byte) {
	buf := make([]byte, len(v))
	copy(buf[:], v[:])
	(*m.db)[helper.EncodeToHex(p)] = buf
}

func (m *memBatch) Write() {
}
