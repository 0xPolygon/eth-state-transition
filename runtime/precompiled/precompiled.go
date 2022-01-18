package precompiled

import (
	"encoding/binary"

	"github.com/0xPolygon/eth-state-transition/runtime"
	"github.com/0xPolygon/eth-state-transition/types"
)

var p = &Precompiled{}

var Contracts map[types.Address]contract

func register(addrStr string, b contract) {
	if len(Contracts) == 0 {
		Contracts = map[types.Address]contract{}
	}
	Contracts[types.StringToAddress(addrStr)] = b
}

func init() {
	register("1", &ecrecover{p})
	register("2", &sha256h{})
	register("3", &ripemd160h{p})
	register("4", &identity{})

	// Byzantium fork
	register("5", &modExp{p})
	register("6", &bn256Add{p})
	register("7", &bn256Mul{p})
	register("8", &bn256Pairing{p})

	// Istanbul fork
	register("9", &blake2f{p})
}

type contract interface {
	gas(input []byte, config *runtime.ForksInTime) uint64
	run(input []byte) ([]byte, error)
}

// Precompiled is the runtime for the precompiled contracts
type Precompiled struct {
	buf []byte
}

// Run runs an execution
func Run(codeAddress types.Address, input []byte, gas uint64, config *runtime.ForksInTime) *runtime.ExecutionResult {
	contract := Contracts[codeAddress]
	gasCost := contract.gas(input, config)

	// In the case of not enough gas for precompiled execution we return ErrOutOfGas
	if gas < gasCost {
		return &runtime.ExecutionResult{
			GasLeft: 0,
			Err:     runtime.ErrOutOfGas,
		}
	}

	gas = gas - gasCost
	returnValue, err := contract.run(input)

	result := &runtime.ExecutionResult{
		ReturnValue: returnValue,
		GasLeft:     gas,
		Err:         err,
	}

	if result.Failed() {
		result.GasLeft = 0
		result.ReturnValue = nil
	}

	return result
}

var zeroPadding = make([]byte, 64)

func (p *Precompiled) leftPad(buf []byte, n int) []byte {
	// TODO, avoid buffer allocation
	l := len(buf)
	if l > n {
		return buf
	}

	tmp := make([]byte, n)
	copy(tmp[n-l:], buf)
	return tmp
}

func (p *Precompiled) get(input []byte, size int) ([]byte, []byte) {
	p.buf = extendByteSlice(p.buf, size)
	n := size
	if len(input) < n {
		n = len(input)
	}

	// copy the part from the input
	copy(p.buf[0:], input[:n])

	// copy empty values
	if n < size {
		rest := size - n
		if rest < 64 {
			copy(p.buf[n:], zeroPadding[0:size-n])
		} else {
			copy(p.buf[n:], make([]byte, rest))
		}
	}
	return p.buf, input[n:]
}

func (p *Precompiled) getUint64(input []byte) (uint64, []byte) {
	p.buf, input = p.get(input, 32)
	num := binary.BigEndian.Uint64(p.buf[24:32])
	return num, input
}

func extendByteSlice(b []byte, needLen int) []byte {
	b = b[:cap(b)]
	if n := needLen - cap(b); n > 0 {
		b = append(b, make([]byte, n)...)
	}
	return b[:needLen]
}
