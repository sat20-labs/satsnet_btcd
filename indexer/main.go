package indexer

import (
	"github.com/sat20-labs/indexer/config"
	"github.com/sat20-labs/satsnet_btcd/chaincfg"
	"github.com/sat20-labs/satsnet_btcd/indexer/common"
	"github.com/sat20-labs/satsnet_btcd/indexer/indexer"
	"github.com/sat20-labs/satsnet_btcd/indexer/rpcserver"
	shareIndexer "github.com/sat20-labs/satsnet_btcd/indexer/share/indexer"

)

type Config struct {
	DBDir           string
	ChainParam      *chaincfg.Params
	MaxIndexHeight  int
	PeriodFlushToDB int
}

func NewIndexerMgr(dbPath string, interrupt <-chan struct{}) (*indexer.IndexerMgr, error) {
	yamlcfg := config.InitConfig("indexer.conf")
	if dbPath != "" {
		yamlcfg.DB.Path = dbPath
	}

	// 需要等rpcserver运行
	// err := InitRpc(yamlcfg)
	// if err != nil {
	// 	common.Log.Error(err)
	// 	return nil, err
	// }

	indexerMgr := indexer.NewIndexerMgr(yamlcfg, interrupt)
	shareIndexer.InitIndexer(indexerMgr)
	indexerMgr.Init()

	_, err := InitRpcService(yamlcfg, indexerMgr)
	if err != nil {
		common.Log.Error(err)
		return nil, err
	}
	return indexerMgr, nil
}

func InitRpcService(conf *config.YamlConf, indexerMgr *indexer.IndexerMgr) (*rpcserver.Rpc, error) {
	maxIndexHeight := int64(0)
	addr := ""
	host := ""
	scheme := ""
	proxy := ""
	logPath := ""

	maxIndexHeight = conf.BasicIndex.MaxIndexHeight
	rpcService := conf.RPCService
	addr = rpcService.Addr
	host = rpcService.Swagger.Host
	for _, v := range rpcService.Swagger.Schemes {
		scheme += v + ","
	}
	proxy = rpcService.Proxy
	logPath = rpcService.LogPath

	chain := conf.Chain
	rpc := rpcserver.NewRpc(indexerMgr, chain)
	if maxIndexHeight <= 0 { // default true. set to false when compiling database.
		err := rpc.Start(addr, host, scheme,
			proxy, logPath, &rpcService.API)
		if err != nil {
			return rpc, err
		}
		common.Log.Info("rpc started")
	}
	return rpc, nil
}
