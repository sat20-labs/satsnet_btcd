package indexer

import (
	"strconv"

	"github.com/sat20-labs/satoshinet/indexer/common"
	"github.com/sat20-labs/satoshinet/indexer/indexer"
	"github.com/sat20-labs/satoshinet/indexer/rpcserver"
	shareIndexer "github.com/sat20-labs/satoshinet/indexer/share/indexer"
)


func NewIndexerMgr(dbPath, port, user, ps string, enableTls, bTestNet bool, interrupt <-chan struct{}) (*indexer.IndexerMgr, error) {
	
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
			EnableTls: enableTls,
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
	
	addr := "0.0.0.0:9528"
	proxy := "mainnet"
	if bTestNet {
		addr = "0.0.0.0:19528"
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
