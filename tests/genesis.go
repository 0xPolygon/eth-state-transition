package tests

import (
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/0xPolygon/eth-state-transition/helper"
	"github.com/0xPolygon/eth-state-transition/types"
)

// GenesisAccount is an account in the state of the genesis block.
type GenesisAccount struct {
	Code       []byte                    `json:"code,omitempty"`
	Storage    map[types.Hash]types.Hash `json:"storage,omitempty"`
	Balance    *big.Int                  `json:"balance,omitempty"`
	Nonce      uint64                    `json:"nonce,omitempty"`
	PrivateKey []byte                    `json:"secretKey,omitempty"` // for tests
}

func (g *GenesisAccount) UnmarshalJSON(data []byte) error {
	type GenesisAccount struct {
		Code       *string                   `json:"code,omitempty"`
		Storage    map[types.Hash]types.Hash `json:"storage,omitempty"`
		Balance    *string                   `json:"balance"`
		Nonce      *string                   `json:"nonce,omitempty"`
		PrivateKey *string                   `json:"secretKey,omitempty"`
	}

	var dec GenesisAccount
	if err := json.Unmarshal(data, &dec); err != nil {
		return err
	}

	parseError := func(field string, err error) error {
		return fmt.Errorf("failed to decode field '%s': %v", field, err)
	}

	var err error
	if dec.Code != nil {
		g.Code, err = helper.ParseBytes(dec.Code)
		if err != nil {
			return parseError("code", err)
		}
	}

	if dec.Storage != nil {
		g.Storage = dec.Storage
	}

	g.Balance, err = helper.ParseUint256orHex(dec.Balance)
	if err != nil {
		return parseError("balance", err)
	}
	g.Nonce, err = helper.ParseUint64orHex(dec.Nonce)
	if err != nil {
		return parseError("nonce", err)
	}

	if dec.PrivateKey != nil {
		g.PrivateKey, err = helper.ParseBytes(dec.PrivateKey)
		if err != nil {
			return parseError("privatekey", err)
		}
	}

	return err
}
