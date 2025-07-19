package bitcoin

import (
	"regexp"
)

type Pepecoin struct{}

func (Pepecoin) ChainName() string {
	return "dogecoin"
}

func (Pepecoin) CoinbaseDigest(coinbase string) (string, error) {
	return DoubleSha256(coinbase)
}

func (Pepecoin) HeaderDigest(header string) (string, error) {
	return ScryptDigest(header)
}

func (Pepecoin) ShareMultiplier() float64 {
	return 65536
}

func (Pepecoin) ValidMainnetAddress(address string) bool {
	// Apparently a base58 decode is the best way to validate.. TODO.
	return regexp.MustCompile("^(D|A|9)[a-km-zA-HJ-NP-Z1-9]{33,34}$").MatchString(address)
}

func (Pepecoin) ValidTestnetAddress(address string) bool {
	return regexp.MustCompile("^(K|2)[a-km-zA-HJ-NP-Z1-9]{33}$").MatchString(address)
}

func (Pepecoin) MinimumConfirmations() uint {
	return uint(251)
}
