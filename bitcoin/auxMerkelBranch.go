package bitcoin

import (
	"bytes"
	"encoding/hex"
	"fmt"

	"designs.capital/dogepool/utils"
)

type AuxMerkleBranch struct {
	numberOfBranches string
	branchHashes     []byte
	mask             string
}

func makeAuxChainMerkleBranch(b BitcoinBlock, n int) AuxMerkleBranch {
	merkleBranches, mask, err := buildMerkleBranchesAndMask(4, b.Template.AuxBlocks, n)
	if err != nil {
		utils.LogError(err)
	}
	concatMerkelBranch := make([]byte, 0)
	for _, b := range merkleBranches {
		concatMerkelBranch = append(concatMerkelBranch, b...)
	}
	return AuxMerkleBranch{
		numberOfBranches: fmt.Sprint("0", len(merkleBranches)),
		mask:             mask,
		branchHashes:     concatMerkelBranch,
	}
}

func (am *AuxMerkleBranch) Serialize() string {
	return am.numberOfBranches + hex.EncodeToString(am.branchHashes) + am.mask
}

func buildMerkleBranchesAndMask(merkleSize int, auxblocks []*AuxBlock, n int) ([][]byte, string, error) {
	slots := make([][]byte, merkleSize)

	for i, auxblock := range auxblocks {
		hash, err := hex.DecodeString(auxblock.Hash)
		if err != nil {
			return nil, "", err
		}
		slots[i+1] = utils.ReverseBytes(hash)
	}

	// Fill unused slots with arbitrary data (e.g., zeros)
	for i := range slots {
		if slots[i] == nil {
			slots[i] = make([]byte, 32) // Fill with 32 bytes of zeros
		}
	}

	var currentLevel [][]byte
	merkleBranch := make([][]byte, 0)
	var merkleMask string

	for idx, hash := range slots {
		if bytes.Equal(hash, slots[n-1]) {
			merkleMask = fmt.Sprint("0000000", idx)
		}
		currentLevel = append(currentLevel, hash)
	}

	// Build the Merkle tree
	for len(currentLevel) > 1 {
		var newLevel [][]byte
		for i := 0; i < len(currentLevel); i += 2 {
			var left, right []byte
			left = currentLevel[i]
			right = currentLevel[i+1]
			siblingIndex := i ^ 1 // XOR avec 1 pour obtenir l'index du voisin
			merkleBranch = append(merkleBranch, currentLevel[siblingIndex])
			newHash := utils.DoubleSHA256(append(left, right...))
			newLevel = append(newLevel, newHash)
		}
		currentLevel = newLevel
	}
	return merkleBranch, merkleMask, nil
}
