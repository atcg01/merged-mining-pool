package pool

import (
	"log"
	"time"

	"designs.capital/dogepool/persistence"
	"designs.capital/dogepool/utils"
)

func (pool *PoolServer) startBufferManager() error {
	interval := mustParseDuration(pool.config.ShareFlushInterval)
	log.Printf("Share buffer flushes every %v\n", pool.config.ShareFlushInterval)
	go pool.flushShareBufferAtInterval(interval)

	return nil
}

func (pool *PoolServer) flushShareBufferAtInterval(interval time.Duration) {
	for {
		time.Sleep(interval)

		pool.Lock()
		sharesToWrite := pool.shareBuffer
		pool.shareBuffer = nil
		pool.Unlock()

		err := persistence.Shares.InsertBatch(sharesToWrite)
		if err != nil {
			utils.LogInfof("%s %+v", err, sharesToWrite)
			pool.Lock()
			pool.shareBuffer = append(pool.shareBuffer, sharesToWrite...)
			pool.Unlock()
		}
	}
}
