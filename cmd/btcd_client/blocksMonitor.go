package main

import (
	"fmt"
	"time"

	"github.com/sat20-labs/satsnet_btcd/cmd/btcd_client/btcwallet"
	"github.com/sat20-labs/satsnet_btcd/cmd/btcd_client/satsnet_rpc"
	"github.com/sat20-labs/satsnet_btcd/wire"
)

const (
	DAEMON_INTERVAL = 30 * time.Second
	MAX_TIMES       = 3000 // MAX sync blocks number in one interval
)

type UtxoAssets struct {
	Utxo       string
	Value      int64
	SatsRanges wire.TxRanges
}

var (
	syncedBlock   int64
	AddressAssets map[string]string
)

func startDaemon() {
	syncedBlock = -1
	AddressAssets = make(map[string]string)

	go func() {
		BlockMoniterThread()
	}()

	//syncBlock(31997)
}

func parseBlock(height int64) error {
	log.Debugf("Block: %d", height)
	hash, err := satsnet_rpc.GetBlockHash(height)
	if err != nil {
		log.Error(err)
		return err
	}

	log.Debugf("Block Hash: %s", hash.String())
	block, err := satsnet_rpc.GetRawBlock(hash)
	if err != nil {
		log.Error(err)
		return err
	}
	// blockData, err := hex.DecodeString(rawBlock)
	// if err != nil {
	// 	log.Errorf("syncBlock-> Failed to decode block: %v", err)
	// 	return err
	// }

	// Deserialize the bytes into a btcutil.Block.
	// block, err := btcutil.NewBlockFromBytes(blockData)
	// if err != nil {
	// 	//log.Panicf("syncBlock-> Failed to parse block: %v", err)
	// 	log.Error(err)
	// 	return err
	// }

	transactions := block.Transactions
	for index, tx := range transactions {

		log.Debugf("tx: %d", index)
		txid := tx.TxHash().String()
		log.Debugf("txid: %s ", txid)

		//txInBlock(tx)
		log.Debugf("------------TxIn-------------")
		for _, in := range tx.TxIn {
			txid := in.PreviousOutPoint.Hash.String()
			vout := in.PreviousOutPoint.Index
			utxo := fmt.Sprintf("%s:%d", txid, vout)
			SpentUtxo(utxo)
		}

		log.Debugf("------------TxOut-------------")

		for index, out := range tx.TxOut {
			utxo := fmt.Sprintf("%s:%d", txid, index)

			assets := UtxoAssets{
				Utxo:       utxo,
				Value:      out.Value,
				SatsRanges: out.SatsRanges}

			NewUtxo(out.PkScript, assets)
		}
	}

	log.Debugf("Parse Block done.")
	return nil
}

func BlockMoniterThread() {
	// Create a ticker that ticks every n seconds
	ticker := time.NewTicker(DAEMON_INTERVAL)
	updatedCurrentHeight := int64(0)
	for {
		select {
		case <-ticker.C:
			log.Debugf("Moniter timeup...")

			// Check the day is changed
			// today := time.Now().Format(time.DateOnly)

			// recordToday = moniterInstance.marketPlaceInst.GetRecordToday()

			// if recordToday != today {
			// 	moniterInstance.marketPlaceInst.TodayPast()
			// }

			// moniterInstance.marketPlaceInst.UpdateBTCPrice()

			currentHeight, err := satsnet_rpc.GetBlockCount()
			if err != nil {
				continue
			}
			// if syncedBlock == 0 {
			// 	syncedBlock = currentHeight
			// }

			if updatedCurrentHeight != currentHeight {
				// The current height is updated
				//moniterInstance.marketPlaceInst.CurrentHeightUpdated(currentHeight)
				updatedCurrentHeight = currentHeight

			}
			blocks := 0

			for {
				blocks++
				// Check the block height of btc is changed
				log.Debugf("current Block: %d, Synced Block: %d", currentHeight, syncedBlock)
				if currentHeight <= syncedBlock {
					// no new block
					break
				}
				syncingBlock := syncedBlock + 1
				err = parseBlock(syncingBlock)
				if err != nil {
					log.Errorf("UtxoMoniterThread->syncBlock failed: %s", err)
					break
				}
				syncedBlock = syncingBlock
				log.Debugf("Has Synced Block: %d", syncedBlock)
				if blocks > MAX_TIMES {
					// max check 10 blocks in one time
					break
				}
			}
		}
	}
}

func SpentUtxo(utxo string) {
	log.Debugf("utxo: %s", utxo)
	log.Debugf("----------------------------------")
}

func NewUtxo(pkScript []byte, assets UtxoAssets) {
	address, err := btcwallet.PkScriptToAddr(pkScript)
	if err != nil {
		log.Errorf("NewUtxo->PkScriptToAddr failed: %x", pkScript)
		return
	}
	log.Debugf("address: %s", address)
	log.Debugf("utxo:%s, Value:%d", assets.Utxo, assets.Value)
	for _, satsRange := range assets.SatsRanges {
		log.Debugf("    Sats Range: [%d-%d]", satsRange.Start, satsRange.Start+satsRange.Size-1)
	}
	log.Debugf("----------------------------------")
}

func ShowBlocks(start, end int64) {
	currentHeight, err := satsnet_rpc.GetBlockCount()
	if err != nil {
		return
	}

	if start < 0 {
		start = 0
	}

	if end == -1 {
		end = currentHeight
	}

	for block := start; block <= end; block++ {
		// Check the block height of btc is changed
		log.Debugf("current Block: %d, Shows Block: %d", currentHeight, block)
		if block > currentHeight {
			// no more block
			break
		}
		err = parseBlock(block)
		if err != nil {
			log.Errorf("parseBlock failed: %s", err)
			break
		}
		log.Debugf("Show Block: %d completed.", block)
		log.Debugf(" -----------------------------------------------------------------")
	}
}

func TestRPCGetBlocks() {

	times := 10000
	failedTimes := 0
	for i := 0; i <= times; i++ {
		currentHeight, err := satsnet_rpc.GetBlockCount()
		if err != nil {
			log.Error("GetBlockCount failed: ", err)
			failedTimes++
			continue
		}

		_, err = satsnet_rpc.GetBlockHash(currentHeight)
		if err != nil {
			log.Error("GetBlockHash failed: ", err)
			failedTimes++
			continue
		}
	}

	log.Debugf("TestBlocks times: %d, failed times: %d", times, failedTimes)

}
