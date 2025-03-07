package indexer

import (
	"strconv"

	"github.com/sat20-labs/satsnet_btcd/indexer/common"
	"github.com/sat20-labs/satsnet_btcd/indexer/indexer"
	"github.com/sat20-labs/satsnet_btcd/indexer/rpcserver"
	shareIndexer "github.com/sat20-labs/satsnet_btcd/indexer/share/indexer"
)


func NewIndexerMgr(dbPath, port, user, ps string, bTestNet bool, interrupt <-chan struct{}) (*indexer.IndexerMgr, error) {
	
	p, err := strconv.Atoi(port)
	if err != nil {
		return nil, err
	}

	cfg := indexer.Config{
		DataPath: dbPath,
		RPCCfg: &indexer.RPCConfig{
			Host: "127.0.0.1",
			Port: p,
			User: user,
			Password: ps,
		},
	}

	indexerMgr := indexer.NewIndexerMgr(&cfg, bTestNet, interrupt)
	shareIndexer.InitIndexer(indexerMgr)
	indexerMgr.Init()

	_, err = InitRpcService(dbPath, bTestNet, indexerMgr)
	if err != nil {
		common.Log.Error(err)
		return nil, err
	}
	return indexerMgr, nil
}

func InitRpcService(dbPath string, bTestNet bool, indexerMgr *indexer.IndexerMgr) (*rpcserver.Rpc, error) {
	
	addr := "https://0.0.0.0:8005"
	proxy := "mainnet"
	if bTestNet {
		addr = "http://0.0.0.0:8009"
		proxy = "testnet"
	} 
	
	rpc := rpcserver.NewRpc(indexerMgr)
	err := rpc.Start(addr, proxy, dbPath+"/logs/"+indexerMgr.GetChainParam().Name)
	if err != nil {
		return rpc, err
	}
	common.Log.Info("indexer rpc service started")

	return rpc, nil
}
