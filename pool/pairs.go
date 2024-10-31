package pool

import "designs.capital/dogepool/bitcoin"

type Pair struct {
	bitcoin.BitcoinBlock
	AuxBlocks []bitcoin.AuxBlock
}

func (p Pair) GetPrimary() bitcoin.BitcoinBlock {
	return p.BitcoinBlock
}

func (p Pair) GetAuxN(n int) *bitcoin.AuxBlock {
	return &p.AuxBlocks[n-1]
}
