package pool

import (
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

func cbi(firstHash, secondHash string) string {

	firstBytes, _ := hex.DecodeString(firstHash)
	secondBytes, _ := hex.DecodeString(secondHash)

	coinbaseInitial := fmt.Sprintf("fabe6d6d%s010000000000000000002632%s010000000000", hex.EncodeToString(firstBytes[:32]), hex.EncodeToString(secondBytes))
	// coinbaseInitial := fmt.Sprintf("fabe6d6d%s010000000000000000002632", dogecoinHash)

	return coinbaseInitial
}

// Main INPUT
func (p *PoolServer) fetchRpcBlockTemplatesAndCacheWork() error {
	var block *bitcoin.BitcoinBlock
	var err error
	template, aux1block, aux2block, err := p.fetchAllBlockTemplatesFromRPC()
	if err != nil {
		// Switch nodes if we fail to get work
		err = p.CheckAndRecoverRPCs()
		if err != nil {
			return err
		}
		template, aux1block, aux2block, err = p.fetchAllBlockTemplatesFromRPC()
		if err != nil {
			return err
		}
	}
	// utils.LogInfof("%+v, %+v, %+v", template, auxblock, aux2block)
	p.templates.AuxBlocks = make([]bitcoin.AuxBlock, 0)

	auxillary := p.config.BlockSignature
	if aux1block != nil {
		// mergedPOW := aux1block.GetWork()
		// auxillary = auxillary + hexStringToByteString(mergedPOW)

		p.templates.AuxBlocks = append(p.templates.AuxBlocks, *aux1block)
	}

	if aux2block != nil {
		// mergedPOW := aux2block.GetWork()
		// auxillary = hexStringToByteString(mergedPOW)

		p.templates.AuxBlocks = append(p.templates.AuxBlocks, *aux2block)
	}
	mergedPOW := cbi(aux1block.Hash, aux2block.Hash)
	auxillary = auxillary + hexStringToByteString(mergedPOW)

	primaryName := p.config.GetPrimary()
	// TODO this is chain/bitcoin specific
	rewardPubScriptKey := p.GetPrimaryNode().RewardPubScriptKey
	extranonceByteReservationLength := 8

	block, p.workCache, err = bitcoin.GenerateWork(&template, aux1block, aux2block, primaryName, auxillary, rewardPubScriptKey, extranonceByteReservationLength)
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
		err = p.submitAuxBlock(1, primaryBlockTemplate, *aux1Block)
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
		err = p.submitAuxBlock(2, primaryBlockTemplate, *aux2Block)
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
