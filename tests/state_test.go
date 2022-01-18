package tests

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"strings"
	"testing"

	state "github.com/0xPolygon/eth-state-transition"
	"github.com/0xPolygon/eth-state-transition/helper"
	"github.com/0xPolygon/eth-state-transition/types"
	"github.com/stretchr/testify/assert"
)

var (
	stateTests       = "GeneralStateTests"
	legacyStateTests = "LegacyTests/Constantinople/GeneralStateTests"
)

type stateCase struct {
	Info        *info                             `json:"_info"`
	Env         *env                              `json:"env"`
	Pre         map[types.Address]*GenesisAccount `json:"pre"`
	Post        map[string]postState              `json:"post"`
	Transaction *stTransaction                    `json:"transaction"`
}

var ripemd = types.StringToAddress("0000000000000000000000000000000000000003")

func RunSpecificTest(file string, t *testing.T, c stateCase, name, fork string, index int, p postEntry) {
	config, ok := Forks[fork]
	if !ok {
		t.Fatalf("config %s not found", fork)
	}

	env := c.Env.ToEnv(t)

	msg, err := c.Transaction.At(p.Indexes)
	if err != nil {
		t.Fatal(err)
	}

	snap, _ := buildState(t, c.Pre)
	forks := config.At(uint64(env.Number))

	runtimeCtx := c.Env.ToHeader(t)
	runtimeCtx.ChainID = 1

	transition := state.NewTransition(forks, runtimeCtx, snap)

	result, err := transition.Write(msg)
	assert.NoError(t, err)

	// txn.CleanDeleteObjects(forks.EIP158)
	_, root := snap.Commit(transition.Commit())
	if !bytes.Equal(root, p.Root.Bytes()) {
		t.Fatalf("root mismatch (%s %s %s %d): expected %s but found %s", file, name, fork, index, p.Root.String(), helper.EncodeToHex(root))
	}

	if logs := rlpHashLogs(result.Logs); logs != p.Logs {
		t.Fatalf("logs mismatch (%s, %s %d): expected %s but found %s", name, fork, index, p.Logs.String(), logs.String())
	}
}

func TestState(t *testing.T) {
	long := []string{
		"static_Call50000",
		"static_Return50000",
		"static_Call1MB",
		"stQuadraticComplexityTest",
		"stTimeConsuming",
	}

	skip := []string{
		"RevertPrecompiledTouch",
		"failed_tx_xcf416c53",
	}

	// There are two folders in spec tests, one for the current tests for the Istanbul fork
	// and one for the legacy tests for the other forks
	folders, err := listFolders(stateTests, legacyStateTests)
	if err != nil {
		t.Fatal(err)
	}

	for _, folder := range folders {
		t.Run(folder, func(t *testing.T) {
			files, err := listFiles(folder)
			if err != nil {
				t.Fatal(err)
			}

			for _, file := range files {
				if !strings.HasSuffix(file, ".json") {
					continue
				}

				if contains(long, file) && testing.Short() {
					t.Log("Long tests are skipped in short mode")
					continue
				}

				if contains(skip, file) {
					t.Log("Skip test")
					continue
				}

				data, err := ioutil.ReadFile(file)
				if err != nil {
					t.Fatal(err)
				}

				var c map[string]stateCase
				if err := json.Unmarshal(data, &c); err != nil {
					t.Fatal(err)
				}

				for name, i := range c {
					for fork, f := range i.Post {
						for indx, e := range f {
							RunSpecificTest(file, t, i, name, fork, indx, e)
						}
					}
				}
			}
		})
	}
}

func rlpHashLogs(logs []*state.Log) (res types.Hash) {
	dst := helper.Keccak256(MarshalLogsWith(logs))
	return types.BytesToHash(dst)
}
