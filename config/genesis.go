package config

import (
	"time"

	"github.com/tendermint/tendermint/types"
)

type Genesis struct {
	ConsulsCount              string
	BftOracleInNebulaCount    string
	GenesisTime               time.Time
	ChainID                   string
	Block                     types.BlockParams
	Evidence                  types.EvidenceParams
	InitScore                 map[string]uint64
	OraclesAddressByValidator map[string]map[string]string
}
