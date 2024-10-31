package bitcoin

import (
	"regexp"
)

type Bellscoin struct{}

func (Bellscoin) ChainName() string {
	return "bellscoin"
}

func (Bellscoin) CoinbaseDigest(coinbase string) (string, error) {
	return DoubleSha256(coinbase)
}

func (Bellscoin) HeaderDigest(header string) (string, error) {
	return ScryptDigest(header)
}

func (Bellscoin) ShareMultiplier() float64 {
	return 65536
}

func (Bellscoin) ValidMainnetAddress(address string) bool {
	return regexp.MustCompile("^(L|M)[A-Za-z0-9]{33}$|^(ltc1)[0-9A-Za-z]{39}$").MatchString(address)
}

func (Bellscoin) ValidTestnetAddress(address string) bool {
	// utils.LogInfo(address)
	return regexp.MustCompile("[a-zA-Z0-9]{34}").MatchString(address)
}

func (Bellscoin) MinimumConfirmations() uint {
	return uint(BitcoinMinConfirmations)
}
