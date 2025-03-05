package base

import (
	"time"

	"github.com/sat20-labs/satsnet_btcd/chaincfg/chainhash"
	"github.com/sat20-labs/satsnet_btcd/indexer/common"
	"github.com/sat20-labs/satsnet_btcd/indexer/share/satsnet_rpc"
	"github.com/sat20-labs/satsnet_btcd/wire"
)

// 带了延时，仅用于跑数据

func getBlockCount() (int64, error) {
	h, err := satsnet_rpc.GetBlockCount()
	if err != nil {
		n := 1
		for n < 10 {
			common.Log.Infof("GetBlockCount failed. try again ...")
			time.Sleep(time.Duration(n) * time.Second)
			n++
			h, err = satsnet_rpc.GetBlockCount()
			if err == nil {
				break
			}
		}
	}

	return h, err
}

func getBlockHash(height uint64) (*chainhash.Hash, error) {
	h, err := satsnet_rpc.GetBlockHash(int64(height))
	if err != nil {
		n := 1
		for n < 10 {
			common.Log.Infof("GetBlockHash failed. try again ...")
			time.Sleep(time.Duration(n) * time.Second)
			n++
			h, err = satsnet_rpc.GetBlockHash(int64(height))
			if err == nil {
				break
			}
		}
	}
	return h, err
}

func getRawBlock(blockHash *chainhash.Hash) (*wire.MsgBlock, error) {
	h, err := satsnet_rpc.GetRawBlock(blockHash)
	if err != nil {
		n := 1
		for n < 10 {
			common.Log.Infof("GetRawBlock failed %s. try again ...", err)
			time.Sleep(time.Duration(n) * time.Second)
			n++
			h, err = satsnet_rpc.GetRawBlock(blockHash)
			if err == nil {
				break
			}
		}
	}
	return h, err
}
