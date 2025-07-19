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
	merkleBranches, mask, err := buildMerkleBranchesAndMask(8, b.Template.AuxBlocks, n)
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

func getExpectedIndex(nChainId int, h int) uint32 {
	// Calcul pseudo-aléatoire pour déterminer l'index dans l'arbre de Merkle
	rand := uint32(0)
	rand = rand*1103515245 + 12345
	rand += uint32(nChainId)
	rand = rand*1103515245 + 12345

	return rand % (1 << h)
}

func BuildMerkleLeaf(merkleSize int, auxblocks []*AuxBlock) ([][]byte, error) {
	slots := make([][]byte, merkleSize)

	for i, auxblock := range auxblocks {
		hash, err := hex.DecodeString(auxblock.Hash)
		if err != nil {
			return nil, err
		}
		if slots[i] != nil {
			utils.LogWarning("Conflit slot when building merkle tree", i, auxblock.ChainID)
		}
		slots[getExpectedIndex(auxblock.ChainID, 4)] = utils.ReverseBytes(hash)
	}

	// Fill unused slots with arbitrary data (e.g., zeros)
	for i := range slots {
		if slots[i] == nil {
			slots[i] = make([]byte, 32) // Fill with 32 bytes of zeros
		}
	}
	return slots, nil
}

func buildMerkleBranchesAndMask(merkleSize int, auxblocks []*AuxBlock, n int) ([][]byte, string, error) {
	slots, _ := BuildMerkleLeaf(merkleSize, auxblocks)

	var currentLevel [][]byte
	merkleBranch := make([][]byte, 0)
	var merkleMask string

	searchedHash, _ := hex.DecodeString(auxblocks[n-1].Hash)
	searchedHash = utils.ReverseBytes(searchedHash)
	searchedIndex := 0
	for idx, hash := range slots {
		if bytes.Equal(hash, searchedHash) {
			searchedIndex = idx
			merkleMask = fmt.Sprintf("0%d000000", idx)
		}
		currentLevel = append(currentLevel, hash)
	}

	// Build the Merkle tree
	for len(currentLevel) > 1 {
		var newLevel [][]byte
		siblingIndex := searchedIndex ^ 1 // XOR avec 1 pour obtenir l'index du voisin
		merkleBranch = append(merkleBranch, currentLevel[siblingIndex])
		for i := 0; i < len(currentLevel); i += 2 {
			newHash := utils.DoubleSHA256(append(currentLevel[i], currentLevel[i+1]...))
			newLevel = append(newLevel, newHash)
		}
		searchedIndex /= 2
		currentLevel = newLevel
	}
	return merkleBranch, merkleMask, nil
}
