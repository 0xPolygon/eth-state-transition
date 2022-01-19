package evm

import (
	"github.com/0xPolygon/eth-state-transition/runtime"
	"github.com/ethereum/evmc/v10/bindings/go/evmc"
)

// Run implements the runtime interface
func Run(c *runtime.Contract, host runtime.Host, rev evmc.Revision) *runtime.ExecutionResult {

	contract := acquireState()
	contract.resetReturnData()

	contract.msg = c
	contract.code = c.Code
	contract.gas = c.Gas
	contract.host = host
	contract.rev = rev
	contract.bitmap.setCode(c.Code)

	ret, err := contract.Run()

	// We are probably doing this append magic to make sure that the slice doesn't have more capacity than it needs
	var returnValue []byte
	returnValue = append(returnValue[:0], ret...)

	gasLeft := contract.gas

	releaseState(contract)

	if err != nil && err != errRevert {
		gasLeft = 0
	}

	return &runtime.ExecutionResult{
		ReturnValue: returnValue,
		GasLeft:     gasLeft,
		Err:         err,
	}
}
