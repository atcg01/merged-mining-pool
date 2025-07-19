package bitcoin

import (
	"regexp"
)

type Luckycoin struct{}

func (Luckycoin) ChainName() string {
	return "dogecoin"
}

func (Luckycoin) CoinbaseDigest(coinbase string) (string, error) {
	return DoubleSha256(coinbase)
}

func (Luckycoin) HeaderDigest(header string) (string, error) {
	return ScryptDigest(header)
}

func (Luckycoin) ShareMultiplier() float64 {
	return 65536
}

func (Luckycoin) ValidMainnetAddress(address string) bool {
	// Apparently a base58 decode is the best way to validate.. TODO.
	return regexp.MustCompile("^(D|A|9)[a-km-zA-HJ-NP-Z1-9]{33,34}$").MatchString(address)
}

func (Luckycoin) ValidTestnetAddress(address string) bool {
	return regexp.MustCompile("^(K|2)[a-km-zA-HJ-NP-Z1-9]{33}$").MatchString(address)
}

func (Luckycoin) MinimumConfirmations() uint {
	return uint(251)
}
