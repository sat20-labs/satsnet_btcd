package satsnet_rpc

import (
	"os"
	"path/filepath"
	"strconv"

	"github.com/sat20-labs/satsnet_btcd/btcutil"
	"github.com/sat20-labs/satsnet_btcd/indexer/common"
	"github.com/sat20-labs/satsnet_btcd/rpcclient"
	"github.com/sat20-labs/satsnet_btcd/wire"
)

type BlockOnConnected func(height int32, header *wire.BlockHeader, txns []*btcutil.Tx)
type BlockOnDisconneted func(height int32, header *wire.BlockHeader)

type BtcdClient struct {
	client         *rpcclient.Client
	OnConnected    BlockOnConnected
	OnDisConnected BlockOnDisconneted
}

var _client BtcdClient


func RegisterOnConnected(cb BlockOnConnected) {
	_client.OnConnected = cb
}

func RpcClientReady() bool {
	return _client.client != nil
}

func InitSatsNetClient(host string, port int, user, passwd, dataPath string) error {
	ntfnHandlers := rpcclient.NotificationHandlers{
		OnFilteredBlockConnected: func(height int32, header *wire.BlockHeader, txns []*btcutil.Tx) {
			common.Log.Infof("Block connected: %v (%d) %v",
				header.BlockHash(), height, header.Timestamp)
			if _client.OnConnected != nil {
				_client.OnConnected(height, header, txns)
			}
		},
		OnFilteredBlockDisconnected: func(height int32, header *wire.BlockHeader) {
			common.Log.Infof("Block disconnected: %v (%d) %v",
				header.BlockHash(), height, header.Timestamp)
			if _client.OnDisConnected != nil {
				_client.OnDisConnected(height, header)
			}
		},
	}

	// Connect to local btcd RPC server using websockets.
	certFile := filepath.Join(dataPath, "rpc.cert")
	common.Log.Infof("cert file: %s", certFile)
	certs, err := os.ReadFile(certFile)
	if err != nil {
		common.Log.Errorf("ReadFile %s failed, %v", certFile, err)
		return err
	}

	connCfg := &rpcclient.ConnConfig{
		Host:     host + ":" + strconv.Itoa(port),
		User:     user,
		Endpoint: "ws",
		Pass:     passwd,
		//HTTPPostMode: true,
		Certificates: certs,
	}
	client, err := rpcclient.New(connCfg, &ntfnHandlers)
	if err != nil {
		common.Log.Errorf("rpcclient.New failed. %v", err)
		return err
	}

	// Register for block connect and disconnect notifications.
	if err := client.NotifyBlocks(); err != nil {
		common.Log.Errorf("client.NotifyBlocks failed. %v", err)
		return err
	}
	common.Log.Infof("NotifyBlocks: Registration Complete")

	// Get the current block count.
	blockCount, err := client.GetBlockCount()
	if err != nil {
		common.Log.Errorf("client.GetBlockCount failed. %v", err)
		return err
	}
	common.Log.Infof("Block count: %d", blockCount)

	common.Log.Infof("rpc client connected")
	_client.client = client

	return nil
}

func ShutdownSatsNetClient() {
	_client.client.Shutdown()
}
