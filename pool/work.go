package pool

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"designs.capital/dogepool/bitcoin"
	"designs.capital/dogepool/persistence"
	"designs.capital/dogepool/utils"
)

// Constants for the magic header
const magic = "\xfa\xbe\x6d\x6d"

// Function to create the coinbase transaction for merged mining
func createMergedMiningCoinbase(auxblocks []*bitcoin.AuxBlock, merkleNonce int32) (string, error) {
	merkleSize := 16
	// merkleSize := len(auxblocks)
	// if merkleSize%2 == 1 { // Assume merkle_size is a power of 2
	// 	merkleSize = merkleSize + 1
	// }

	// Build the Merkle Tree
	merkleRoot, _ := buildMerkleRoot(merkleSize, auxblocks)

	// Create coinbase scriptSig
	scriptSig := createScriptSig(merkleNonce, int32(merkleSize), merkleRoot)

	return hex.EncodeToString(scriptSig), nil
}

// Build the Merkle Root from the slots
func buildMerkleRoot(merkleSize int, auxblocks []*bitcoin.AuxBlock) ([]byte, error) {

	slots, _ := bitcoin.BuildMerkleLeaf(merkleSize, auxblocks)

	var currentLevel [][]byte

	// Add non-nil hashes to the current level
	for _, hash := range slots {
		if hash != nil {
			currentLevel = append(currentLevel, hash)
		}
	}

	// Build the Merkle tree
	for len(currentLevel) > 1 {
		var newLevel [][]byte
		for i := 0; i < len(currentLevel); i += 2 {
			var left, right []byte
			left = currentLevel[i]
			if i+1 < len(currentLevel) {
				right = currentLevel[i+1]
			} else {
				right = left // Duplicate if odd number of hashes
			}
			newHash := utils.DoubleSHA256(append(left, right...))
			newLevel = append(newLevel, newHash)
		}
		currentLevel = newLevel
	}
	return currentLevel[0], nil
}

// Create the coinbase scriptSig
func createScriptSig(merkleNonce, merkleSize int32, merkleRoot []byte) []byte {
	scriptSig := make([]byte, 0)
	scriptSig = append(scriptSig, magic...)
	reversedMerkleRoot := utils.ReverseBytes(merkleRoot)
	scriptSig = append(scriptSig, reversedMerkleRoot...)
	scriptSig = append(scriptSig, int32ToBytes(merkleSize)...)
	scriptSig = append(scriptSig, int32ToBytes(merkleNonce)...)
	// parentChainId, _ := hex.DecodeString("00002632")
	// scriptSig = append(scriptSig, parentChainId...)
	return scriptSig
}

// Convert int32 to byte array
func int32ToBytes(i int32) []byte {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, uint32(i))
	return b
}

// Main INPUT
func (p *PoolServer) fetchRpcBlockTemplatesAndCacheWork() error {
	var block *bitcoin.BitcoinBlock
	var err error
	template, auxblocks, err := p.fetchAllBlockTemplatesFromRPC()
	if err != nil {
		// Switch nodes if we fail to get work
		err = p.CheckAndRecoverRPCs()
		if err != nil {
			return err
		}
		template, auxblocks, err = p.fetchAllBlockTemplatesFromRPC()
		if err != nil {
			return err
		}
	}
	// utils.LogInfof("%+v, %+v, %+v", template, auxblock, aux2block)
	p.templates.AuxBlocks = auxblocks
	auxillary := p.config.BlockSignature
	template.AuxBlocks = auxblocks
	scriptSig, err := createMergedMiningCoinbase(auxblocks, 0)
	if err != nil {
		return err
	}
	auxillary = auxillary + hexStringToByteString(scriptSig)

	primaryName := p.config.GetPrimary()
	// TODO this is chain/bitcoin specific
	rewardPubScriptKey := p.GetPrimaryNode().RewardPubScriptKey
	extranonceByteReservationLength := 8

	block, p.workCache, err = bitcoin.GenerateWork(&template, primaryName, auxillary, rewardPubScriptKey, extranonceByteReservationLength)
	if err != nil {
		log.Print(err)
	}

	p.templates.BitcoinBlock = *block

	return nil
}

// Main OUTPUT
func (p *PoolServer) recieveWorkFromClient(share bitcoin.Work, client *stratumClient) error {
	primaryBlockTemplate := p.templates.GetPrimary()
	if primaryBlockTemplate.Template == nil {
		return errors.New("primary block template not yet set")
	}

	var err error

	// TODO - this key and interface isn't very invertable..
	workerString := share[0].(string)
	workerStringParts := strings.Split(workerString, ".")
	if len(workerStringParts) < 2 {
		return errors.New("invalid miner address")
	}
	minerAddress := workerStringParts[0]
	rigID := workerStringParts[1]

	primaryBlockHeight := primaryBlockTemplate.Template.Height
	nonce := share[primaryBlockTemplate.NonceSubmissionSlot()].(string)
	extranonce2Slot, _ := primaryBlockTemplate.Extranonce2SubmissionSlot()
	extranonce2 := share[extranonce2Slot].(string)
	nonceTime := share[primaryBlockTemplate.NonceTimeSubmissionSlot()].(string)

	// TODO - validate input

	extranonce := client.extranonce1 + extranonce2

	_, err = primaryBlockTemplate.MakeHeader(extranonce, nonce, nonceTime)

	if err != nil {
		return err
	}

	shareStatus, candidate, shareDifficulty := validateAndWeighShare(&primaryBlockTemplate, p.config.PoolDifficulty)

	if shareStatus == shareInvalid {
		m := "❔ Invalid share for block %v from %v [%v] [%v] [%v/%v]"
		m = fmt.Sprintf(m, primaryBlockHeight, client.ip, rigID, client.userAgent, shareDifficulty, p.config.PoolDifficulty)
		return errors.New(m)
	}

	m := "Valid share for block %v from %v [%v] [%v/%v]"
	m = fmt.Sprintf(m, primaryBlockHeight, client.ip, rigID, shareDifficulty, p.config.PoolDifficulty)
	utils.LogInfo(m)

	blockTarget := bitcoin.Target(primaryBlockTemplate.Template.Target)
	blockDifficulty, _ := blockTarget.ToDifficulty()
	blockDifficulty = blockDifficulty * primaryBlockTemplate.ShareMultiplier()

	p.Lock()
	p.shareBuffer = append(p.shareBuffer, persistence.Share{
		PoolID:            p.config.PoolName,
		BlockHeight:       primaryBlockHeight,
		Miner:             minerAddress,
		Worker:            rigID,
		UserAgent:         client.userAgent,
		Difficulty:        shareDifficulty,
		NetworkDifficulty: blockDifficulty,
		IpAddress:         client.ip,
		Created:           time.Now(),
	})
	p.Unlock()

	if shareStatus == shareValid {
		return nil
	}
	nbCandidate := 0
	for _, value := range candidate {
		if value {
			nbCandidate++
		}
	}
	statusReadable := fmt.Sprintf("%d candidates", nbCandidate)

	// m = "%v block candidate for block %v from %v [%v]"
	// m = fmt.Sprintf(m, statusReadable, primaryBlockHeight, client.ip, rigID)
	// utils.LogInfo(m)

	found := persistence.Found{
		PoolID:               p.config.PoolName,
		Status:               persistence.StatusPending,
		Type:                 statusReadable,
		ConfirmationProgress: 0,
		Miner:                minerAddress,
		Source:               "",
	}

	successSubmit := make([]bool, len(p.config.BlockChainOrder))
	nbSuccess := 0
	blockSubmitted := ""

	if candidate[0] {
		err = p.submitBlockToChain(primaryBlockTemplate)
		// if err != nil {
		// Try to submit on different node
		// err = p.rpcManagers[p.config.GetPrimary()].CheckAndRecoverRPCs()
		if err != nil {
			return err
		}
		// err = p.submitBlockToChain(primaryBlockTemplate)
		// }

		found.Chain = p.config.GetPrimary()
		found.Created = time.Now()
		found.Hash, err = primaryBlockTemplate.HeaderHashed()
		if err != nil {
			utils.LogError(err)
		}
		found.NetworkDifficulty = blockDifficulty
		found.BlockHeight = primaryBlockHeight
		found.TransactionConfirmationData, err = primaryBlockTemplate.CoinbaseHashed()
		if err != nil {
			utils.LogError(err)
		}

		err = persistence.Blocks.Insert(found)
		if err != nil {
			utils.LogError(err)
		}
		found.Chain = ""
		nbSuccess++
		successSubmit[0] = true
		blockSubmitted += fmt.Sprintf("%s - %d, ", p.config.GetPrimary(), primaryBlockHeight)
	}

	for i, chainName := range p.config.BlockChainOrder {
		if i == 0 {
			continue
		}
		auxBlock := p.templates.GetAuxN(i)
		if candidate[i] {
			err = p.submitAuxBlock(i, primaryBlockTemplate)
			if err != nil {
				// XXX NICO TODO HANDLE
				utils.LogErrorf("!!!! ERROR AUX %d submit %s", i, err)
			} else {
				// EnrichShare
				aux1Target := bitcoin.Target(reverseHexBytes(auxBlock.Target))
				auxDifficulty, _ := aux1Target.ToDifficulty()
				auxDifficulty = auxDifficulty * bitcoin.GetChain(chainName).ShareMultiplier()

				found.Chain = chainName
				found.Created = time.Now()
				found.Hash = auxBlock.Hash
				found.NetworkDifficulty = auxDifficulty
				found.BlockHeight = uint(auxBlock.Height)
				// Likely doesn't exist on your AUX coin API unless you editted the daemon source to return this
				found.TransactionConfirmationData = reverseHexBytes(auxBlock.CoinbaseHash)

				err = persistence.Blocks.Insert(found)
				if err != nil {
					utils.LogError(err)
				}
				successSubmit[i] = true
				nbSuccess++
				blockSubmitted += fmt.Sprintf("%s - %d, ", chainName, auxBlock.Height)
			}
		}
	}
	icon := ""
	for i := 0; i < nbSuccess; i++ {
		icon = icon + "✅ "
	}
	utils.LogInfof("%sSuccessful %d/%d submission of block %v from: %v [%v]", icon, nbSuccess, nbCandidate, blockSubmitted, client.ip, rigID)

	return nil
}

func (pool *PoolServer) generateWorkFromCache(refresh bool) (bitcoin.Work, error) {
	work := append(pool.workCache, interface{}(refresh))

	return work, nil
}
