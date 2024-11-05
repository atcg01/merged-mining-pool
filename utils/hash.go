package utils

import "crypto/sha256"

// Double SHA-256 hash function
func DoubleSHA256(data []byte) []byte {
	hash1 := sha256.Sum256(data)
	hash2 := sha256.Sum256(hash1[:])
	return hash2[:]
}
