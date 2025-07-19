package bitcoin

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"

	"designs.capital/dogepool/utils"
	"golang.org/x/crypto/ripemd160"
)

// https://developer.bitcoin.org/reference/transactions.html#coinbase-input-the-input-of-the-first-transaction-in-a-block

type CoinbaseInital struct {
	Version                     string
	NumberOfInputs              string
	PreviousOutputTransactionID string
	PreviousOutputIndex         string
	BytesInArbitrary            uint
	BytesInHeight               uint
	HeightHex                   string
}

func encodeNumber(n uint) []byte {
	// Si `n` est entre 1 et 16, renvoie directement un buffer avec 0x50 + n
	if n >= 1 && n <= 16 {
		return []byte{0x50 + byte(n)}
	}

	// Initialiser le buffer et la longueur
	buff := make([]byte, 9)
	l := 1

	// Ajouter les octets de `n` dans le buffer
	for n > 0x7f {
		buff[l] = byte(n & 0xff)
		l++
		n >>= 8
	}

	// Placer la longueur au début du buffer
	buff[0] = byte(l)
	buff[l] = byte(n)
	l++

	// Retourne un sous-tableau avec seulement les bytes utilisés
	return buff[:l]
}

func (t *Template) CoinbaseInitial(arbitraryByteLength uint) CoinbaseInital {
	// heightBytes := eightLittleEndianBytes(t.Height)
	// heightBytes = removeInsignificantBytesLittleEndian(heightBytes)
	heightBytes := encodeNumber(t.Height)
	heightHex := hex.EncodeToString(heightBytes)

	heightByteLen := uint(len(heightBytes))
	// utils.LogInfo("CoinbaseInitial - heightByteLen", heightByteLen, len(other))
	arbitraryByteLength = arbitraryByteLength + heightByteLen // + 1 // 1 is for the heightByteLen byte

	if arbitraryByteLength > 100 {
		log.Printf("!!WARNING!! - Coinbase length too long - !!WARNING!! %v\n", arbitraryByteLength)
	}

	return CoinbaseInital{
		Version:                     "01000000", // Different from template version
		NumberOfInputs:              "01",
		PreviousOutputTransactionID: "0000000000000000000000000000000000000000000000000000000000000000",
		PreviousOutputIndex:         "ffffffff",
		BytesInArbitrary:            arbitraryByteLength,
		BytesInHeight:               heightByteLen,
		HeightHex:                   heightHex,
	}
}

func (i CoinbaseInital) Serialize() string {
	// debugCoinbaseInitialOutput(i)
	return i.Version +
		i.NumberOfInputs +
		i.PreviousOutputTransactionID +
		i.PreviousOutputIndex +
		varUint(i.BytesInArbitrary) +
		// These next two aren't arbitrary, but they are in the arbitrary section ;)
		i.HeightHex
}

type CoinbaseFinal struct {
	TransactionInSequence string
	OutputCount           uint
	TxOuts                string
	TransactionLockTime   string
}

func (t *Template) CoinbaseFinal(poolPayoutPubScriptKey string) CoinbaseFinal {
	txOutputLen, txOutput := t.coinbaseTransactionOutputs(poolPayoutPubScriptKey)
	utils.LogInfof("Nombre de txOutput coinbaseFinal %d", txOutputLen)
	return CoinbaseFinal{
		TransactionInSequence: "00000000",
		OutputCount:           txOutputLen,
		TxOuts:                txOutput,
		TransactionLockTime:   "00000000",
	}
}

func (f CoinbaseFinal) Serialize() string {
	// debugCoinbaseFinalOutput(f)
	return f.TransactionInSequence +
		varUint(f.OutputCount) +
		f.TxOuts +
		f.TransactionLockTime
}

type Coinbase struct {
	CoinbaseInital string
	Arbitrary      string
	CoinbaseFinal  string
}

func (cb *Coinbase) Serialize() string {
	// debugCoinbaseOutput(cb)
	return cb.CoinbaseInital + cb.Arbitrary + cb.CoinbaseFinal
}

func GeneratePubKeyScript(pubKey string) (string, error) {
	pubKeyBytes, err := hex.DecodeString(pubKey)
	if err != nil {
		return "", err
	}

	// Étape 1 : Hachage SHA-256
	shaHash := sha256.Sum256(pubKeyBytes)

	// Étape 2 : Hachage RIPEMD-160
	ripemdHasher := ripemd160.New()
	_, err = ripemdHasher.Write(shaHash[:])
	if err != nil {
		return "", err
	}
	pubKeyHash := ripemdHasher.Sum(nil)

	// Construire le script
	script := "76a914" + hex.EncodeToString(pubKeyHash) + "88ac" // Script P2PKH
	return script, nil
}

func (t *Template) coinbaseTransactionOutputs(poolPubScriptKey string) (uint, string) {
	outputsCount := uint(0)
	outputs := ""

	if t.DefaultWitnessCommitment != "" {
		outAmount := "0000000000000000"
		outputs = outputs + TransactionOut(outAmount, t.DefaultWitnessCommitment)
		outputsCount++
	}

	// Some alt coins may have additional outputs..

	// Pool reward output
	rewardAmount := fmt.Sprintf("%016x", t.CoinBaseValue)
	rewardAmount, _ = reverseHexBytes(rewardAmount)
	pubScriptKey, err := GeneratePubKeyScript(poolPubScriptKey)
	if err != nil {
		panic("Erreur lors de la génération du script de clé publique : " + err.Error())
	}

	outputs = outputs + TransactionOut(rewardAmount, pubScriptKey)
	outputsCount++
	outputs = outputs + TransactionOut(rewardAmount, poolPubScriptKey)
	outputsCount++
	outputs = outputs + TransactionOut(rewardAmount, "AAAA")
	outputsCount++

	return outputsCount, outputs
}

// func debugCoinbaseOutput(cb *Coinbase) {
// 	fmt.Println()
// 	fmt.Println("**Coinbase Parts**")
// 	fmt.Println()
// 	fmt.Println("Initial", cb.CoinbaseInital)
// 	fmt.Println("Arbitrary", cb.Arbitrary)
// 	fmt.Println("Final", cb.CoinbaseFinal)
// 	fmt.Println()
// 	fmt.Println("Coinbase", cb.CoinbaseInital+cb.Arbitrary+cb.CoinbaseFinal)
// 	fmt.Println()
// }

// func debugCoinbaseInitialOutput(i CoinbaseInital) {
// 	fmt.Println()
// 	fmt.Println("🧐 Coinbase Initial Parts ➔ ➔ ➔ ➔")
// 	fmt.Println()
// 	fmt.Println("Version", i.Version)
// 	fmt.Println("NumberOfInputs", i.NumberOfInputs)
// 	fmt.Println("PreviousOutputTransactionID", i.PreviousOutputTransactionID)
// 	fmt.Println("PreviousOutputIndex", i.PreviousOutputIndex)
// 	fmt.Println("BytesInArbitrary", i.BytesInArbitrary)
// 	fmt.Println("BytesInHeight", i.BytesInHeight)
// 	fmt.Println("HeightHex", i.HeightHex)
// 	fmt.Println()
// 	cbI := i.Version +
// 		i.NumberOfInputs +
// 		i.PreviousOutputTransactionID +
// 		i.PreviousOutputIndex +
// 		varUint(i.BytesInArbitrary) +
// 		varUint(i.BytesInHeight) +
// 		i.HeightHex
// 	fmt.Println("Coinbase Initial", cbI)
// 	fmt.Println()
// }

// func debugCoinbaseFinalOutput(f CoinbaseFinal) {
// 	fmt.Println()
// 	fmt.Println("➔ ➔ ➔ ➔ Coinbase Final Parts**")
// 	fmt.Println()
// 	fmt.Println("TransactionInSequence", f.TransactionInSequence)
// 	fmt.Println("OutputCount", f.OutputCount)
// 	fmt.Println("TxOuts", f.TxOuts)
// 	fmt.Println("TransactionLockTime", f.TransactionLockTime)
// 	fmt.Println()
// 	cbf := f.TransactionInSequence +
// 		varUint(f.OutputCount) +
// 		f.TxOuts +
// 		f.TransactionLockTime
// 	fmt.Println("Coinbase Final", cbf)
// 	fmt.Println()
// }
