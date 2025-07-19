package pool

import (
	"designs.capital/dogepool/bitcoin"
)

const (
	shareInvalid = iota
	shareValid
	shareCandidate
)

func validateAndWeighShare(primary *bitcoin.BitcoinBlock, poolDifficulty float64) (int, []bool, float64) {
	primarySum, err := primary.Sum()
	logOnError(err)

	primaryTarget := bitcoin.Target(primary.Template.Target)
	primaryTargetBig, _ := primaryTarget.ToBig()

	poolTarget, _ := bitcoin.TargetFromDifficulty(poolDifficulty / primary.ShareMultiplier())
	shareDifficulty, _ := poolTarget.ToDifficulty()

	candidate := make([]bool, len(primary.Template.AuxBlocks)+1)

	// s, _ := primarySum.Float64()
	// t, _ := primaryTargetBig.Float64()
	// utils.LogInfof("share: %e ---- target: %e", s, t)

	validShare := false
	if primarySum.Cmp(primaryTargetBig) <= 0 {
		validShare = true
		candidate[0] = true
	}

	for i, auxBlock := range primary.Template.AuxBlocks {
		aux1Target := bitcoin.Target(reverseHexBytes(auxBlock.Target))
		aux1TargetBig, _ := aux1Target.ToBig()

		if primarySum.Cmp(aux1TargetBig) <= 0 {
			validShare = true
			candidate[i+1] = true
		}
	}

	if validShare {
		return shareCandidate, candidate, shareDifficulty
	}

	poolTargettBig, _ := poolTarget.ToBig()
	if primarySum.Cmp(poolTargettBig) <= 0 {
		return shareValid, candidate, shareDifficulty
	}

	return shareInvalid, candidate, shareDifficulty
}
