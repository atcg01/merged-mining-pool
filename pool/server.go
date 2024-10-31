package pool

import (
	"encoding/json"
	"errors"
	"log"
	"sync"
	"time"

	"designs.capital/dogepool/bitcoin"
	"designs.capital/dogepool/config"
	"designs.capital/dogepool/persistence"
	"designs.capital/dogepool/rpc"
	"designs.capital/dogepool/utils"
)

type PoolServer struct {
	sync.RWMutex
	config            *config.Config
	activeNodes       BlockChainNodesMap
	rpcManagers       map[string]*rpc.Manager
	connectionTimeout time.Duration
	templates         Pair
	workCache         bitcoin.Work
	shareBuffer       []persistence.Share
}

func NewServer(cfg *config.Config, rpcManagers map[string]*rpc.Manager) *PoolServer {
	if len(cfg.PoolName) < 1 {
		log.Println("Pool must have a name")
	}
	if len(cfg.BlockchainNodes) < 1 {
		log.Println("Pool must have at least 1 blockchain node to work from")
	}
	if len(cfg.BlockChainOrder) < 1 {
		log.Println("Pool must have a blockchain order to tell primary vs aux")
	}

	pool := &PoolServer{
		config:      cfg,
		rpcManagers: rpcManagers,
	}

	return pool
}

func (pool *PoolServer) Start() {
	initiateSessions()
	pool.loadBlockchainNodes()
	pool.startBufferManager()

	amountOfChains := len(pool.config.BlockChainOrder) - 1
	utils.LogInfo("Nb aux chain", amountOfChains)
	pool.templates.AuxBlocks = make([]bitcoin.AuxBlock, amountOfChains)

	// Initial work creation
	panicOnError(pool.fetchRpcBlockTemplatesAndCacheWork())
	work, err := pool.generateWorkFromCache(false)
	panicOnError(err)

	go pool.listenForConnections()
	pool.broadcastWork(work)

	// There after..
	panicOnError(pool.listenForBlockNotifications())
}

func (pool *PoolServer) broadcastWork(work bitcoin.Work) {
	request := miningNotify(work)
	err := notifyAllSessions(request)
	logOnError(err)
}

func (p *PoolServer) fetchAllBlockTemplatesFromRPC() (bitcoin.Template, *bitcoin.AuxBlock, *bitcoin.AuxBlock, error) {
	var template bitcoin.Template
	var err error
	response, err := p.GetPrimaryNode().RPC.GetBlockTemplate()
	if err != nil {
		return template, nil, nil, errors.New("RPC error: " + err.Error())
	}

	err = json.Unmarshal(response, &template)
	if err != nil {
		return template, nil, nil, err
	}
	// utils.LogInfof("Fetch primary block %+v", template)

	var aux1Block bitcoin.AuxBlock
	if p.config.GetAuxN(1) != "" {
		response, err = p.GetAuxNNode(1).RPC.CreateAuxBlock(p.GetAuxNNode(1).RewardTo)
		if err != nil {
			log.Println("No aux1 block found: " + err.Error())
			return template, nil, nil, nil
		}

		err = json.Unmarshal(response, &aux1Block)
		if err != nil {
			return template, nil, nil, err
		}
		if len(aux1Block.Target2) > 0 {
			aux1Block.Target = aux1Block.Target2
		}
		// utils.LogInfof("Aux1 %+v", auxBlock)
	}

	var aux2Block bitcoin.AuxBlock
	if p.config.GetAuxN(2) != "" {
		response, err = p.GetAuxNNode(2).RPC.CreateAuxBlock(p.GetAuxNNode(2).RewardTo)
		if err != nil {
			log.Println("No aux block2 found: " + err.Error())
			return template, nil, nil, nil
		}

		err = json.Unmarshal(response, &aux2Block)
		if err != nil {
			return template, nil, nil, err
		}
		if len(aux2Block.Target2) > 0 {
			aux2Block.Target = aux2Block.Target2
		}
		// utils.LogInfof("Aux2 %+v", aux2Block)
	}
	utils.LogInfof("priH:%d, aux1H %s:%d, aux2H %s:%d", template.Height, p.GetAuxNNode(1).ChainName, aux1Block.Height, p.GetAuxNNode(2).ChainName, aux2Block.Height)
	// utils.LogInfof("%+v", aux1Block)
	// utils.LogInfof("%+v", aux2Block)
	return template, &aux1Block, &aux2Block, nil
}

func notifyAllSessions(request stratumRequest) error {
	for _, client := range sessions {
		err := sendPacket(request, client)
		logOnError(err)
	}
	// log.Printf("Sent work to %v client(s)", len(sessions))
	return nil
}

func panicOnError(e error) {
	if e != nil {
		panic(e)
	}
}

func logOnError(e error) {
	if e != nil {
		log.Println(e)
	}
}

func logFatalOnError(e error) {
	if e != nil {
		log.Fatal(e)
	}
}
