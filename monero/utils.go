package monero

import (
	"context"
	"fmt"
	"time"

	logging "github.com/ipfs/go-log"

	"github.com/athanorlabs/atomic-swap/common"
)

var (
	// blockSleepDuration is the duration that we sleep between checks for new blocks. We
	// lower it in dev environments if fast background mining is started.
	blockSleepDuration = time.Second * 10

	log = logging.Logger("monero")
)

// WaitForBlocks waits for `count` new blocks to arrive.
// It returns the height of the chain.
func WaitForBlocks(ctx context.Context, client WalletClient, count int) (uint64, error) {
	c := client.(*walletClient)
	startHeight, err := c.getChainHeight()
	if err != nil {
		return 0, fmt.Errorf("failed to get height: %w", err)
	}
	prevHeight := startHeight - 1 // prevHeight is only for logging
	endHeight := startHeight + uint64(count)

	for {
		height, err := c.getChainHeight()
		if err != nil {
			return 0, err
		}

		if height >= endHeight {
			// ensure wallet height is refreshed to the chain height
			if err = c.refresh(); err != nil {
				return 0, err
			}
			return height, nil
		}

		if height > prevHeight {
			log.Debugf("Waiting for next block, current height %d (target height %d)", height, endHeight)
			prevHeight = height
		}

		if err = common.SleepWithContext(ctx, blockSleepDuration); err != nil {
			return 0, err
		}
	}
}
