package bitcoin

type AuxPow struct {
	ParentCoinbase   string
	ParentHeaderHash string
	ParentMerkleBranch
	auxMerkleBranch      AuxMerkleBranch
	ParentHeaderUnhashed string
}

func MakeAuxPow(parentBlock BitcoinBlock, n int) AuxPow {
	if parentBlock.hash == "" {
		panic("Set parent block hash first")
	}
	// debugAuxPow(parentBlock, makeParentMerkleBranch(parentBlock.merkleSteps), makeAuxChainMerkleBranch())

	return AuxPow{
		ParentCoinbase:       parentBlock.coinbase,
		ParentHeaderHash:     parentBlock.hash,
		ParentMerkleBranch:   makeParentMerkleBranch(parentBlock.merkleSteps),
		auxMerkleBranch:      makeAuxChainMerkleBranch(parentBlock, n),
		ParentHeaderUnhashed: parentBlock.header,
	}
}

func (p *AuxPow) Serialize() string {
	return p.ParentCoinbase +
		p.ParentHeaderHash +
		p.ParentMerkleBranch.Serialize() +
		p.auxMerkleBranch.Serialize() +
		p.ParentHeaderUnhashed
}

type ParentMerkleBranch struct {
	Length uint
	Items  []string
	mask   string
}

func makeParentMerkleBranch(items []string) ParentMerkleBranch {
	length := uint(len(items))
	return ParentMerkleBranch{
		Length: length,
		Items:  items,
		mask:   "00000000",
	}
}

func (pm *ParentMerkleBranch) Serialize() string {
	items := ""
	for _, item := range pm.Items {
		items = items + item
	}
	return varUint(pm.Length) + items + pm.mask
}

// func debugAuxPow(parentBlock BitcoinBlock, parentMerkle ParentMerkleBranch, auxchainMerkle AuxMerkleBranch) {
// 	fmt.Println()
// 	fmt.Println("coinbase", parentBlock.coinbase)
// 	fmt.Println("hash", parentBlock.hash)
// 	fmt.Println("merkleSteps", parentBlock.merkleSteps)
// 	fmt.Println("merkleDigested", parentMerkle.Serialize())
// 	fmt.Println("chainmerklebranch", auxchainMerkle.Serialize())
// 	fmt.Println("header", parentBlock.header)
// 	fmt.Println()
// }
