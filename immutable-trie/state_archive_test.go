package itrie

import (
	"testing"

	state "github.com/0xPolygon/eth-state-transition"
)

func TestState(t *testing.T) {
	state.TestState(t, buildPreState)
}

func buildPreState(pre state.PreStates) state.SnapshotWriter {
	storage := NewMemoryStorage()
	st := NewArchiveState(storage)
	snap := st.NewSnapshot()

	return snap
}
