package main

import (
	"fmt"

	"github.com/sat20-labs/satsnet_btcd/anchortx"
)

func testGetLockedTx(txid string) {
	fmt.Printf("test GetLockedInfo...\n")

	// SendRawTransaction(raw)
	result, err := anchortx.GetLockedInfo(txid)
	if err != nil {
		fmt.Printf("GetLockedInfo error: %s\n", err)
		return
	}

	fmt.Printf("GetLockedInfo success,result: %v\n", result)

}

func testGetLockedUtxoInfo(utxo string) {
	//lockedInfo := &anchortx.LockedInfoInBTCChain{}
	host := "192.168.10.104:8009"
	net := "testnet"

	// Get TxInfo from BTC chain (Layer 1 chain)
	indexerClient := anchortx.NewIndexerClient(host, net)
	utxoAssetsInfo, err := indexerClient.GetTxUtxoAssets(utxo)
	if err != nil {
		fmt.Printf("GetTxUtxoAssets failed: %s\n", err.Error())
		return
	}

	log.Debugf("utxoAssetsInfo: %v", utxoAssetsInfo)
}
