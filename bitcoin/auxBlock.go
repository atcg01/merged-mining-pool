package bitcoin

import (
	"encoding/hex"

	"designs.capital/dogepool/utils"
)

const (
	mergedMiningHeader  = "fabe6d6d"
	mergedMiningTrailer = "010000000000000000002632"
)

type AuxBlock struct {
	Hash              string `json:"hash"`
	OtherHash         string
	ChainID           int    `json:"chainid"`
	PreviousBlockHash string `json:"previousblockhash"`
	CoinbaseHash      string `json:"coinbasehash"`
	CoinbaseValue     uint   `json:"coinbasevalue"`
	Bits              string `json:"bits"`
	Height            uint64 `json:"height"`
	Target            string `json:"target"`
	Target2           string `json:"_target"`
}

func (b *AuxBlock) GetWork() string {
	return mergedMiningHeader + b.Hash + mergedMiningTrailer
}

func ReverseBytes(data []byte) []byte {
	reversed := make([]byte, len(data))
	for i := 0; i < len(data); i++ {
		reversed[i] = data[len(data)-1-i]
	}
	return reversed
}

func (b *AuxBlock) makeAuxChainMerkleBranch(n int) AuxMerkleBranch {
	mask := "00000000"
	if n == 2 {
		mask = "00000001"
	}
	utils.LogInfo("makeAuxChainMerkleBranch", n, mask)
	bb, _ := hex.DecodeString(b.OtherHash)
	return AuxMerkleBranch{
		numberOfBranches: "01",
		mask:             mask,
		branchHashes:     utils.ReverseBytes(bb),
	}
}
