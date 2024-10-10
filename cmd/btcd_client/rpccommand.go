package main

import (
	"fmt"

	"github.com/sat20-labs/satsnet_btcd/chaincfg/chainhash"
	"github.com/sat20-labs/satsnet_btcd/cmd/btcd_client/btcwallet"
	"github.com/sat20-labs/satsnet_btcd/cmd/btcd_client/satsnet_rpc"
)

func testGetBlockStats(height int, stats []string) {
	fmt.Printf("test GetBlockStats...\n")

	// SendRawTransaction(raw)
	result, err := satsnet_rpc.GetBlockStats(height, &stats)
	if err != nil {
		fmt.Printf("GetBlockStats error: %s\n", err)
		return
	}
	fmt.Printf("GetBlockStats success,result: %v\n", result)

}

func testGetMempoolEntry(txid string) {
	fmt.Printf("test GetMempoolEntry...\n")

	// SendRawTransaction(raw)
	result, err := satsnet_rpc.GetMempoolEntry(txid)
	if err != nil {
		fmt.Printf("GetMempoolEntry error: %s\n", err)
		return
	}
	fmt.Printf("GetMempoolEntry success,result: %v\n", result)

}

func testGetRawTransaction(txid string) {
	fmt.Printf("test getrawtransaction...\n")

	txHash, err := chainhash.NewHashFromStr(txid)
	if err != nil {
		log.Errorf("Invalid Txid : %s", txid)
	}

	// SendRawTransaction(raw)
	result, err := satsnet_rpc.GetRawTransaction(txHash)
	if err != nil {
		fmt.Printf("getrawtransaction error: %s\n", err)
		return
	}

	btcwallet.LogMsgTx(result.MsgTx())
	fmt.Printf("getrawtransaction success,result: %v\n", result)

}
