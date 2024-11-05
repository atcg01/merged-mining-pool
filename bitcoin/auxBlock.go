package bitcoin

const (
	mergedMiningHeader  = "fabe6d6d"
	merkleTreeSize      = "01000000" // set to 1 if count(auxChains) < 2. Needs research.
	nonceAndSubchainId  = "00000000" // set to 0 if count(auxChains) < 2. Needs research.
	parentChainId       = "00002632" // litecoin's chain ID
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
