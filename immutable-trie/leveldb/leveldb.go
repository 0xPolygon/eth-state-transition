package itrie

import (
	itrie "github.com/0xPolygon/eth-state-transition/immutable-trie"
	"github.com/0xPolygon/polygon-sdk/types"
	"github.com/syndtr/goleveldb/leveldb"
)

var (
	// codePrefix is the code prefix for leveldb
	codePrefix = []byte("code")
)

// KVStorage is a k/v storage on memory using leveldb
type KVStorage struct {
	db *leveldb.DB
}

// KVBatch is a batch write for leveldb
type KVBatch struct {
	db    *leveldb.DB
	batch *leveldb.Batch
}

func (b *KVBatch) Put(k, v []byte) {
	b.batch.Put(k, v)
}

func (b *KVBatch) Write() {
	b.db.Write(b.batch, nil)
}

func (kv *KVStorage) SetCode(hash types.Hash, code []byte) {
	kv.Put(append(codePrefix, hash.Bytes()...), code)
}

func (kv *KVStorage) GetCode(hash types.Hash) ([]byte, bool) {
	return kv.Get(append(codePrefix, hash.Bytes()...))
}

func (kv *KVStorage) Batch() itrie.Batch {
	return &KVBatch{db: kv.db, batch: &leveldb.Batch{}}
}

func (kv *KVStorage) Put(k, v []byte) {
	kv.db.Put(k, v, nil)
}

func (kv *KVStorage) Get(k []byte) ([]byte, bool) {
	data, err := kv.db.Get(k, nil)
	if err != nil {
		if err.Error() == "leveldb: not found" {
			return nil, false
		} else {
			panic(err)
		}
	}
	return data, true
}

func (kv *KVStorage) Close() error {
	return kv.db.Close()
}

func NewLevelDBStorage(path string) (itrie.Storage, error) {
	db, err := leveldb.OpenFile(path, nil)
	if err != nil {
		return nil, err
	}
	return &KVStorage{db}, nil
}
