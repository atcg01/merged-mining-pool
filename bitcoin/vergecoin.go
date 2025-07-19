package bitcoin

import (
	"regexp"
)

type Vergecoin struct{}

func (Vergecoin) ChainName() string {
	return "vergecoin"
}

func (Vergecoin) CoinbaseDigest(coinbase string) (string, error) {
	return DoubleSha256(coinbase)
}

func (Vergecoin) HeaderDigest(header string) (string, error) {
	return ScryptDigest(header)
}

func (Vergecoin) ShareMultiplier() float64 {
	return 65536
}

func (Vergecoin) ValidMainnetAddress(address string) bool {
	// Apparently a base58 decode is the best way to validate.. TODO.
	return regexp.MustCompile("^(D|A|9)[a-km-zA-HJ-NP-Z1-9]{33,34}$").MatchString(address)
}

func (Vergecoin) ValidTestnetAddress(address string) bool {
	return regexp.MustCompile("^(o|2)[a-km-zA-HJ-NP-Z1-9]{33}$").MatchString(address)
}

func (Vergecoin) MinimumConfirmations() uint {
	return uint(251)
}
