package main

import (
	"fmt"

	"github.com/sat20-labs/satoshinet/httpclient"
)


func testGetLockedUtxoInfo(utxo string) {
	//lockedInfo := &anchortx.LockedInfoInBTCChain{}
	host := "192.168.10.104:8009"
	net := "testnet"

	// Get TxInfo from BTC chain (Layer 1 chain)
	indexerClient := httpclient.NewIndexerClient("", host, net)
	utxoAssetsInfo, err := indexerClient.GetTxUtxoAssets(utxo)
	if err != nil {
		fmt.Printf("GetTxUtxoAssets failed: %s\n", err.Error())
		return
	}

	log.Debugf("utxoAssetsInfo: %v", utxoAssetsInfo)
}
