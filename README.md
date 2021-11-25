
# Eth-state-transition

Ethereum state transition function.

## Usage

```golang

import (
    itrie "github.com/0xPolygon/eth-state-transition/immutable-trie"
    "github.com/0xPolygon/eth-state-transition/runtime"
    state "github.com/0xPolygon/eth-state-transition"
)

func main() {
    // get a reference for the state
    state := itrie.NewArchiveState(itrie.NewMemoryStorage())
    snap := s.NewSnapshot()

    // create a transition object
    forks := runtime.ForksInTime{}
    config := runtime.TxContext{}
    transition := state.NewTransition(forks, config, snap)

    // process a transaction
    result, err := transition.Write(&state.Transaction{})
    if err != nil {
        panic(err)
    }

    fmt.Printf("Logs: %v\n", result.Logs)
    fmt.Printf("Gas used: %d\n", result.GasUsed)

    // retrieve the state data changed
    objs := transition.Commit()

    // commit the data to the state
    if _, err := snap.Commit(objs); err != nil {
        panic(err)
    }
}
```
