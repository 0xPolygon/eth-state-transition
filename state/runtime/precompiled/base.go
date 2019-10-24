package precompiled

import (
	"crypto/sha256"

	"golang.org/x/crypto/ripemd160"

	"github.com/umbracle/minimal/crypto"
	"github.com/umbracle/minimal/helper/keccak"
)

type ecrecover struct {
	p *Precompiled
}

func (e *ecrecover) gas(input []byte) uint64 {
	return 3000
}

func (e *ecrecover) run(input []byte) ([]byte, error) {
	input, _ = e.p.get(input, 128)

	// recover the value v. Expect all zeros except the last byte
	for i := 32; i < 63; i++ {
		if input[i] != 0 {
			return nil, nil
		}
	}
	v := input[63] - 27
	if !crypto.ValidateSignatureValues(v, input[64:96], input[96:128]) {
		return nil, nil
	}

	pubKey, err := crypto.Ecrecover(input[:32], append(input[64:128], v))
	if err != nil {
		return nil, nil
	}

	h := keccak.DefaultKeccakPool.Get()
	h.Write(pubKey[1:])
	dst := h.Sum(nil)
	dst = e.p.leftPad(dst[12:], 32)
	keccak.DefaultKeccakPool.Put(h)

	return dst, nil
}

type identity struct {
}

func (i *identity) gas(input []byte) uint64 {
	return baseGasCalc(input, 15, 3)
}

func (i *identity) run(in []byte) ([]byte, error) {
	return in, nil
}

type sha256h struct {
}

func (s *sha256h) gas(input []byte) uint64 {
	return baseGasCalc(input, 60, 12)
}

func (s *sha256h) run(input []byte) ([]byte, error) {
	h := sha256.Sum256(input)
	return h[:], nil
}

type ripemd160h struct {
	p *Precompiled
}

func (r *ripemd160h) gas(input []byte) uint64 {
	return baseGasCalc(input, 600, 120)
}

func (r *ripemd160h) run(input []byte) ([]byte, error) {
	ripemd := ripemd160.New()
	ripemd.Write(input)
	res := ripemd.Sum(nil)
	return r.p.leftPad(res, 32), nil
}

func baseGasCalc(input []byte, base, word uint64) uint64 {
	return base + uint64(len(input)+31)/32*word
}
