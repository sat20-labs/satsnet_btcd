package satsnet_rpc

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/sat20-labs/satsnet_btcd/btcutil"
	"github.com/sat20-labs/satsnet_btcd/rpcclient"
	"github.com/sat20-labs/satsnet_btcd/wire"
)

var client *rpcclient.Client

func InitSatsNetClient(host string, port int, user, passwd string, homedir string) error {
	ntfnHandlers := rpcclient.NotificationHandlers{
		OnFilteredBlockConnected: func(height int32, header *wire.BlockHeader, txns []*btcutil.Tx) {
			fmt.Printf("Block connected: %v (%d) %v",
				header.BlockHash(), height, header.Timestamp)
		},
		OnFilteredBlockDisconnected: func(height int32, header *wire.BlockHeader) {
			fmt.Printf("Block disconnected: %v (%d) %v",
				header.BlockHash(), height, header.Timestamp)
		},
	}

	// Connect to local btcd RPC server using websockets.
	//btcdHomeDir := btcutil.AppDataDir("btcd", false)
	//btcdHomeDir := "D:\\data\\satsnet_btcd\\btcd" // TODO
	btcdHomeDir := filepath.Join(homedir, "btcd")
	certs, err := os.ReadFile(filepath.Join(btcdHomeDir, "rpc.cert"))
	if err != nil {
		// try to read in current dir
		certs, err = os.ReadFile(filepath.Join("./", "rpc.cert"))
		if err != nil {
			return err
		}
	}

	connCfg := &rpcclient.ConnConfig{
		Host:     host + ":" + strconv.Itoa(port),
		User:     user,
		Endpoint: "ws",
		Pass:     passwd,
		//HTTPPostMode: true,
		Certificates: certs,
	}
	client, err = rpcclient.New(connCfg, &ntfnHandlers)
	if err != nil {
		fmt.Printf("rpcclient.New failed. %v", err)
		return err
	}

	// Register for block connect and disconnect notifications.
	if err := client.NotifyBlocks(); err != nil {
		fmt.Printf("client.NotifyBlocks failed. %v", err)
		return err
	}
	fmt.Printf("NotifyBlocks: Registration Complete")

	// Get the current block count.
	blockCount, err := client.GetBlockCount()
	if err != nil {
		fmt.Printf("client.GetBlockCount failed. %v", err)
		return err
	}
	fmt.Printf("Block count: %d", blockCount)

	return nil
}

func ShutdownSatsNetClient() {
	client.Shutdown()
}
