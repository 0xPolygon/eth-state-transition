package runtime

import (
	"math/big"

	"github.com/ethereum/evmc/v10/bindings/go/evmc"
)

// Params are all the set of params for the chain
type Params struct {
	Forks   *Forks `json:"forks"`
	ChainID int    `json:"chainID"`
}

// Forks specifies when each fork is activated
type Forks struct {
	Homestead      *Fork
	Byzantium      *Fork
	Constantinople *Fork
	Petersburg     *Fork
	Istanbul       *Fork
	Tangerine      *Fork
}

func (f *Forks) active(ff *Fork, block uint64) bool {
	if ff == nil {
		return false
	}
	return ff.Active(block)
}

func (f *Forks) At(block uint64) ForksInTime {
	return ForksInTime{
		Homestead:      f.active(f.Homestead, block),
		Byzantium:      f.active(f.Byzantium, block),
		Constantinople: f.active(f.Constantinople, block),
		Petersburg:     f.active(f.Petersburg, block),
		Istanbul:       f.active(f.Istanbul, block),
		Tangerine:      f.active(f.Tangerine, block),
	}
}

type Fork uint64

func NewFork(n uint64) *Fork {
	f := Fork(n)

	return &f
}

func (f Fork) Active(block uint64) bool {
	return block >= uint64(f)
}

func (f Fork) Int() *big.Int {
	return big.NewInt(int64(f))
}

type ForksInTime struct {
	Homestead,
	Byzantium,
	Constantinople,
	Petersburg,
	Istanbul,
	Tangerine bool
}

func (f ForksInTime) Revision() evmc.Revision {
	if f.Istanbul {
		return evmc.Istanbul
	}
	if f.Petersburg {
		return evmc.Petersburg
	}
	if f.Constantinople {
		return evmc.Constantinople
	}
	if f.Byzantium {
		return evmc.Byzantium
	}
	if f.Tangerine {
		return evmc.TangerineWhistle
	}
	if f.Homestead {
		return evmc.Homestead
	}
	return evmc.Frontier
}

var AllForksEnabled = &Forks{
	Homestead:      NewFork(0),
	Tangerine:      NewFork(0),
	Byzantium:      NewFork(0),
	Constantinople: NewFork(0),
	Petersburg:     NewFork(0),
	Istanbul:       NewFork(0),
}
