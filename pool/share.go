package pool

import (
	"designs.capital/dogepool/bitcoin"
)

const (
	shareInvalid = iota
	shareValid
	primaryCandidate
	aux1Candidate
	aux2Candidate
	aux12Candidate
	paux1Candidate
	paux2Candidate
	tripleCandidate
)

var statusMap = map[int]string{
	2: "Primary",
	3: "Aux1",
	4: "Aux2",
	5: "Aux12",
	6: "PAux1",
	7: "PAux2",
	8: "@@@@@@@@@@@@@@@@@@@@@@@@@@ TRIPLE",
}

func validateAndWeighShare(primary *bitcoin.BitcoinBlock, aux1 *bitcoin.AuxBlock, aux2 *bitcoin.AuxBlock, poolDifficulty float64) (int, float64) {
	primarySum, err := primary.Sum()
	logOnError(err)

	primaryTarget := bitcoin.Target(primary.Template.Target)
	primaryTargetBig, _ := primaryTarget.ToBig()

	poolTarget, _ := bitcoin.TargetFromDifficulty(poolDifficulty / primary.ShareMultiplier())
	shareDifficulty, _ := poolTarget.ToDifficulty()

	status := shareInvalid

	// s, _ := primarySum.Float64()
	// t, _ := primaryTargetBig.Float64()
	// utils.LogInfof("share: %e ---- target: %e", s, t)

	if primarySum.Cmp(primaryTargetBig) <= 0 {
		status = primaryCandidate
	}

	if aux1.Hash != "" {
		aux1Target := bitcoin.Target(reverseHexBytes(aux1.Target))
		aux1TargetBig, _ := aux1Target.ToBig()

		if primarySum.Cmp(aux1TargetBig) <= 0 {
			if status == primaryCandidate {
				status = paux1Candidate
			} else {
				status = aux1Candidate
			}
		}
	}

	if aux2.Hash != "" {
		aux2Target := bitcoin.Target(reverseHexBytes(aux2.Target))
		aux2TargetBig, _ := aux2Target.ToBig()
		// utils.LogInfof("%+v, %+v", aux2, auxTarget)
		if primarySum.Cmp(aux2TargetBig) <= 0 {
			if status == paux1Candidate {
				status = tripleCandidate
			} else if status == primaryCandidate {
				status = paux2Candidate
			} else if status == aux1Candidate {
				status = aux12Candidate
			} else {
				status = aux2Candidate
			}
		}
	}

	if status > shareInvalid {
		return status, shareDifficulty
	}

	poolTargettBig, _ := poolTarget.ToBig()
	if primarySum.Cmp(poolTargettBig) <= 0 {
		// sd, _ := primarySum.Div(poolTargettBig, primarySum).Float64()
		return shareValid, shareDifficulty
	}

	return shareInvalid, shareDifficulty
}
