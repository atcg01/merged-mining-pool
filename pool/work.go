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
	merkleSize := 4
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

// Calculate the slot number based on the provided nonce and chain ID
// func calculateSlotNum(merkleNonce, chainID, merkleSize int32) int32 {
// 	return chainID % merkleSize
// }

// Build the Merkle Root from the slots
func buildMerkleRoot(merkleSize int, auxblocks []*bitcoin.AuxBlock) ([]byte, error) {

	slots := make([][]byte, merkleSize)

	for i, auxblock := range auxblocks {
		hash, err := hex.DecodeString(auxblock.Hash)
		if err != nil {
			return nil, err
		}
		slots[i+1] = utils.ReverseBytes(hash)
	}

	// Fill unused slots with arbitrary data (e.g., zeros)
	for i := range slots {
		if slots[i] == nil {
			slots[i] = make([]byte, 32) // Fill with 32 bytes of zeros
		}
	}

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
	parentChainId, _ := hex.DecodeString("00002632")
	scriptSig = append(scriptSig, parentChainId...)
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
	aux1Block := p.templates.GetAuxN(1)
	aux2Block := p.templates.GetAuxN(2)

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

	shareStatus, shareDifficulty := validateAndWeighShare(&primaryBlockTemplate, aux1Block, aux2Block, p.config.PoolDifficulty)

	heightMessage := fmt.Sprintf("%s:%v", p.config.BlockChainOrder[0], primaryBlockHeight)
	if shareStatus == paux1Candidate {
		heightMessage = fmt.Sprintf("%s:%v, %s:%v", p.config.BlockChainOrder[0], primaryBlockHeight, p.config.BlockChainOrder[1], aux1Block.Height)
	} else if shareStatus == paux2Candidate {
		heightMessage = fmt.Sprintf("%s:%v, %s:%v", p.config.BlockChainOrder[0], primaryBlockHeight, p.config.BlockChainOrder[2], aux2Block.Height)
	} else if shareStatus == aux1Candidate {
		heightMessage = fmt.Sprintf("%s:%v", p.config.BlockChainOrder[1], aux1Block.Height)
	} else if shareStatus == aux2Candidate {
		heightMessage = fmt.Sprintf("%s:%v", p.config.BlockChainOrder[2], aux2Block.Height)
	} else if shareStatus == aux12Candidate {
		heightMessage = fmt.Sprintf("%s:%v, %s:%v", p.config.BlockChainOrder[1], aux1Block.Height, p.config.BlockChainOrder[2], aux2Block.Height)
	} else if shareStatus == tripleCandidate {
		heightMessage = fmt.Sprintf("----- %s:%v, %s:%v, %s:%v", p.config.BlockChainOrder[0], primaryBlockHeight, p.config.BlockChainOrder[1], aux1Block.Height, p.config.BlockChainOrder[2], aux2Block.Height)
	}

	if shareStatus == shareInvalid {
		m := "❔ Invalid share for block %v from %v [%v] [%v] [%v/%v]"
		m = fmt.Sprintf(m, heightMessage, client.ip, rigID, client.userAgent, shareDifficulty, p.config.PoolDifficulty)
		return errors.New(m)
	}

	m := "Valid share for block %v from %v [%v] [%v/%v]"
	m = fmt.Sprintf(m, heightMessage, client.ip, rigID, shareDifficulty, p.config.PoolDifficulty)
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

	statusReadable := statusMap[shareStatus]
	successStatus := 0

	m = "%v block candidate for block %v from %v [%v]"
	m = fmt.Sprintf(m, statusReadable, heightMessage, client.ip, rigID)
	utils.LogInfo(m)

	found := persistence.Found{
		PoolID:               p.config.PoolName,
		Status:               persistence.StatusPending,
		Type:                 statusReadable,
		ConfirmationProgress: 0,
		Miner:                minerAddress,
		Source:               "",
	}

	prefix := ""

	aux1Name := p.config.GetAuxN(1)
	if shareStatus == aux1Candidate || shareStatus == aux12Candidate || shareStatus == paux1Candidate || shareStatus == tripleCandidate {
		err = p.submitAuxBlock(1, primaryBlockTemplate)
		if err != nil {
			// XXX NICO TODO HANDLE
			utils.LogInfo("!!!! ERROR AUX1 submit", err)
		} else {
			// EnrichShare
			aux1Target := bitcoin.Target(reverseHexBytes(aux1Block.Target))
			aux1Difficulty, _ := aux1Target.ToDifficulty()
			aux1Difficulty = aux1Difficulty * bitcoin.GetChain(aux1Name).ShareMultiplier()

			found.Chain = aux1Name
			found.Created = time.Now()
			found.Hash = aux1Block.Hash
			found.NetworkDifficulty = aux1Difficulty
			found.BlockHeight = uint(aux1Block.Height)
			// Likely doesn't exist on your AUX coin API unless you editted the daemon source to return this
			found.TransactionConfirmationData = reverseHexBytes(aux1Block.CoinbaseHash)

			err = persistence.Blocks.Insert(found)
			if err != nil {
				utils.LogError(err)
			}
			successStatus = aux1Candidate
		}
	}

	aux2Name := p.config.GetAuxN(2)
	if shareStatus == aux2Candidate || shareStatus == aux12Candidate || shareStatus == paux2Candidate || shareStatus == tripleCandidate {
		err = p.submitAuxBlock(2, primaryBlockTemplate)
		if err != nil {
			// XXX NICO TODO HANDLE
			utils.LogInfo("!!!! ERROR AUX2 submit", err)
		} else {
			// EnrichShare
			aux2Target := bitcoin.Target(reverseHexBytes(aux2Block.Target))
			aux2Difficulty, _ := aux2Target.ToDifficulty()
			aux2Difficulty = aux2Difficulty * bitcoin.GetChain(aux2Name).ShareMultiplier()

			found.Chain = aux2Name
			found.Created = time.Now()
			found.Hash = aux2Block.Hash
			found.NetworkDifficulty = aux2Difficulty
			found.BlockHeight = uint(aux2Block.Height)
			// Likely doesn't exist on your AUX coin API unless you editted the daemon source to return this
			found.TransactionConfirmationData = reverseHexBytes(aux2Block.CoinbaseHash)
			// utils.LogInfof("%+v", found)
			err = persistence.Blocks.Insert(found)
			if err != nil {
				utils.LogError(err)
			}

			if successStatus == aux1Candidate {
				prefix = "~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~"
				successStatus = aux12Candidate
			} else {
				successStatus = aux2Candidate
			}
		}
	}

	if shareStatus == primaryCandidate || shareStatus == paux1Candidate || shareStatus == paux2Candidate || shareStatus == tripleCandidate {
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
		if successStatus == aux1Candidate {
			prefix = "####################################"
			successStatus = paux1Candidate
		} else if successStatus == aux2Candidate {
			prefix = "####################################"
			successStatus = paux2Candidate
		} else if successStatus == aux12Candidate {
			prefix = "))))))))))))))))))))))))))))!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!####################################"
			successStatus = tripleCandidate
		} else {
			successStatus = primaryCandidate
		}
	}

	statusReadable = statusMap[successStatus]

	utils.LogInfof("✅ %s Successful %v submission of block %v from: %v [%v]", prefix, statusReadable, heightMessage, client.ip, rigID)

	return nil
}

func (pool *PoolServer) generateWorkFromCache(refresh bool) (bitcoin.Work, error) {
	work := append(pool.workCache, interface{}(refresh))

	return work, nil
}
