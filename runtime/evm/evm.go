package evm

import (
	"github.com/0xPolygon/eth-state-transition/runtime"
	"github.com/ethereum/evmc/v10/bindings/go/evmc"
)

// Run implements the runtime interface
func Run(c *runtime.Contract, host runtime.Host, rev evmc.Revision) *runtime.ExecutionResult {

	s := acquireState()
	s.resetReturnData()

	//contract.msg = c
	s.Address = c.Address
	s.Caller = c.Caller
	s.Depth = c.Depth
	s.Value = c.Value
	s.Input = c.Input
	s.Static = c.Static

	s.code = c.Code
	s.gas = c.Gas
	s.host = host
	s.rev = rev
	s.bitmap.setCode(c.Code)

	ret, err := s.Run()

	// We are probably doing this append magic to make sure that the slice doesn't have more capacity than it needs
	var returnValue []byte
	returnValue = append(returnValue[:0], ret...)

	gasLeft := s.gas

	releaseState(s)

	if err != nil && err != errRevert {
		gasLeft = 0
	}

	return &runtime.ExecutionResult{
		ReturnValue: returnValue,
		GasLeft:     gasLeft,
		Err:         err,
	}
}
