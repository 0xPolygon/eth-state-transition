package tests

import (
	"encoding/json"
	"io/ioutil"
	"strings"
	"testing"

	state "github.com/0xPolygon/eth-state-transition"
	"github.com/0xPolygon/eth-state-transition/helper"
	"github.com/0xPolygon/eth-state-transition/runtime"
	"github.com/0xPolygon/eth-state-transition/runtime/evm"
	"github.com/0xPolygon/eth-state-transition/types"
)

var mainnetChainConfig = runtime.Params{
	Forks: &runtime.Forks{
		Homestead: runtime.NewFork(1150000),
		EIP150:    runtime.NewFork(2463000),
		EIP158:    runtime.NewFork(2675000),
		Byzantium: runtime.NewFork(4370000),
	},
}

var vmTests = "VMTests"

type VMCase struct {
	Info *info `json:"_info"`
	Env  *env  `json:"env"`
	Exec *exec `json:"exec"`

	Gas  string `json:"gas"`
	Logs string `json:"logs"`
	Out  string `json:"out"`

	Post map[types.Address]*GenesisAccount `json:"post"`
	Pre  map[types.Address]*GenesisAccount `json:"pre"`
}

func testVMCase(t *testing.T, name string, c *VMCase) {
	env := c.Env.ToEnv(t)
	env.GasPrice = types.BytesToHash(c.Exec.GasPrice.Bytes())
	env.Origin = c.Exec.Origin

	snap, _ := buildState(t, c.Pre)

	config := mainnetChainConfig.Forks.At(uint64(env.Number))

	runtimeCtx := c.Env.ToHeader(t)
	runtimeCtx.ChainID = int64(mainnetChainConfig.ChainID)

	forks := mainnetChainConfig.Forks.At(uint64(runtimeCtx.Number))
	transition := state.NewTransition(forks, runtimeCtx, snap)

	evmR := evm.NewEVM()

	code := transition.GetCode(c.Exec.Address)
	contract := runtime.NewContractCall(1, c.Exec.Caller, c.Exec.Caller, c.Exec.Address, c.Exec.Value, c.Exec.GasLimit, code, c.Exec.Data)

	result := evmR.Run(contract, transition, &config)

	if c.Gas == "" {
		if result.Succeeded() {
			t.Fatalf("gas unspecified (indicating an error), but VM returned no error")
		}
		if result.GasLeft > 0 {
			t.Fatalf("gas unspecified (indicating an error), but VM returned gas remaining > 0")
		}
		return
	}

	// check return
	if c.Out == "" {
		c.Out = "0x"
	}
	if ret := helper.EncodeToHex(result.ReturnValue); ret != c.Out {
		t.Fatalf("return mismatch: got %s, want %s", ret, c.Out)
	}

	txn := transition.Txn()

	// check logs
	if logs := rlpHashLogs(txn.Logs()); logs != types.StringToHash(c.Logs) {
		t.Fatalf("logs hash mismatch: got %x, want %x", logs, c.Logs)
	}

	// check state
	for addr, alloc := range c.Post {
		for key, val := range alloc.Storage {
			if have := txn.GetState(addr, types.BytesToHash(key[:])); have != types.BytesToHash(val[:]) {
				t.Fatalf("wrong storage value at %s:\n  got  %s\n  want %s\n at address %s", key, have, val, addr)
			}
		}
	}

	// check remaining gas
	if expected := stringToUint64T(t, c.Gas); result.GasLeft != expected {
		t.Fatalf("gas left mismatch: got %d want %d", result.GasLeft, expected)
	}
}

func rlpHashLogs(logs []*state.Log) (res types.Hash) {
	dst := helper.Keccak256(MarshalLogsWith(logs))
	return types.BytesToHash(dst)
}

func TestEVM(t *testing.T) {
	folders, err := listFolders(vmTests)
	if err != nil {
		t.Fatal(err)
	}

	long := []string{
		"loop-",
		"gasprice",
		"origin",
	}

	for _, folder := range folders {
		files, err := listFiles(folder)
		if err != nil {
			t.Fatal(err)
		}

		for _, file := range files {
			t.Run(file, func(t *testing.T) {
				if !strings.HasSuffix(file, ".json") {
					return
				}

				data, err := ioutil.ReadFile(file)
				if err != nil {
					t.Fatal(err)
				}

				var vmcases map[string]*VMCase
				if err := json.Unmarshal(data, &vmcases); err != nil {
					t.Fatal(err)
				}

				for name, cc := range vmcases {
					if contains(long, name) && testing.Short() {
						t.Skip()
						continue
					}
					testVMCase(t, name, cc)
				}
			})
		}
	}
}
