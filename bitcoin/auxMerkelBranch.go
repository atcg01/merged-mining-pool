package bitcoin

import "encoding/hex"

type AuxMerkleBranch struct {
	numberOfBranches string
	branchHashes     []byte
	mask             string
}

func (am *AuxMerkleBranch) Serialize() string {
	return am.numberOfBranches + hex.EncodeToString(am.branchHashes) + am.mask
}
