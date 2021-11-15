package itrie

import (
	"fmt"

	"github.com/umbracle/fastrlp"
)

// Node represents a node reference
type Node interface {
	Hash() ([]byte, bool)
	SetHash(b []byte) []byte
}

// ValueNode is a leaf on the merkle-trie
type ValueNode struct {
	// hash marks if this value node represents a stored node
	hash bool
	buf  []byte
}

// Hash implements the node interface
func (v *ValueNode) Hash() ([]byte, bool) {
	return v.buf, v.hash
}

// SetHash implements the node interface
func (v *ValueNode) SetHash(b []byte) []byte {
	panic("We cannot set hash on value node")
}

type common struct {
	hash []byte
}

// Hash implements the node interface
func (c *common) Hash() ([]byte, bool) {
	return c.hash, len(c.hash) != 0
}

// SetHash implements the node interface
func (c *common) SetHash(b []byte) []byte {
	c.hash = extendByteSlice(c.hash, len(b))
	copy(c.hash, b)
	return c.hash
}

// ShortNode is an extension or short node
type ShortNode struct {
	common
	key   []byte
	child Node
}

// FullNode is a node with several children
type FullNode struct {
	common
	epoch    uint32
	value    Node
	children [16]Node
}

func (f *FullNode) copy() *FullNode {
	nc := &FullNode{}
	nc.value = f.value
	copy(nc.children[:], f.children[:])
	return nc
}

func (f *FullNode) replaceEdge(idx byte, e Node) {
	if idx == 16 {
		f.value = e
	} else {
		f.children[idx] = e
	}
}

func (f *FullNode) setEdge(idx byte, e Node) {
	if idx == 16 {
		f.value = e
	} else {
		f.children[idx] = e
	}
}

func (f *FullNode) getEdge(idx byte) Node {
	if idx == 16 {
		return f.value
	} else {
		return f.children[idx]
	}
}

var parserPool fastrlp.ParserPool

func DecodeNode(data []byte) (Node, error) {
	// NOTE. We dont need to make copies of the bytes because the nodes
	// take the reference from data itself which is a safe copy.
	p := parserPool.Get()
	defer parserPool.Put(p)

	v, err := p.Parse(data)
	if err != nil {
		return nil, err
	}

	if v.Type() != fastrlp.TypeArray {
		return nil, fmt.Errorf("storage item should be an array")
	}

	if v.Type() == fastrlp.TypeBytes {
		vv := &ValueNode{
			hash: true,
		}
		vv.buf = append(vv.buf[:0], v.Raw()...)
		return vv, nil
	}

	return decodeNodeImpl(v)
}

func decodeNodeImpl(v *fastrlp.Value) (Node, error) {
	var err error

	ll := v.Elems()
	if ll == 2 {
		key := v.Get(0)
		if key.Type() != fastrlp.TypeBytes {
			return nil, fmt.Errorf("short key expected to be bytes")
		}

		// this can be either an array (extension node)
		// or bytes (leaf node)
		nc := &ShortNode{}
		nc.key = compactToHex(key.Raw())
		if hasTerm(nc.key) {
			// value node
			if v.Get(1).Type() != fastrlp.TypeBytes {
				return nil, fmt.Errorf("short leaf value expected to be bytes")
			}
			vv := &ValueNode{}
			vv.buf = append(vv.buf, v.Get(1).Raw()...)
			nc.child = vv
		} else {
			nc.child, err = decodeNodeImpl(v.Get(1))
			if err != nil {
				return nil, err
			}
		}
		return nc, nil
	} else if ll == 17 {
		// full node
		nc := &FullNode{}
		for i := 0; i < 16; i++ {
			if v.Get(i).Type() == fastrlp.TypeBytes && len(v.Get(i).Raw()) == 0 {
				// empty
				continue
			}
			nc.children[i], err = decodeNodeImpl(v.Get(i))
			if err != nil {
				return nil, err
			}
		}

		if v.Get(16).Type() != fastrlp.TypeBytes {
			return nil, fmt.Errorf("full node value expected to be bytes")
		}
		if len(v.Get(16).Raw()) != 0 {
			vv := &ValueNode{}
			vv.buf = append(vv.buf[:0], v.Get(16).Raw()...)
			nc.value = vv
		}
		return nc, nil
	}
	return nil, fmt.Errorf("node has incorrect number of leafs")
}
