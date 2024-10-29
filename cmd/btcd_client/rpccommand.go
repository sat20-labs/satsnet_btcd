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

func testSendRawTransaction() {
	fmt.Printf("test sendrawtransaction...\n")

	raw := "010000000001026adee5a96509fefbcb0bd1eefbc98c1d290340cd45d696c1af688cc3e7bc2dfa0000000000ffffffff6adee5a96509fefbcb0bd1eefbc98c1d290340cd45d696c1af688cc3e7bc2dfa0100000000ffffffff01774900000000000001ff73694c0265790000fd7749220020650c7b012cd5aa9201251bb1bedefc62aa84548a73bb259595959029e2011616040047304402203e9ff884120b5908e48f8cbb78b764099ebb20b069f6ceda1a8b8ad3b30c3928022064e2c11d49e5517a25cbeb2c6c457c18a33a599e0bd8ed2117b1dc8e8a8ddd550147304402203e25e733efcbb0345588b1f8a962d51dec8d78286b5b524c3c9146ce4e2e2a820220452e57b506f39169b2a57d8a285524fad62dd335b744143e6ba7a6a62e6b5911014752210304374824804565034f8a7ffb680f05a20db8ef6888d5621d8577f6f412596f9e21036fa703396bdcbf4b3466c62079df08366b080e9832f28fd246c0eeea90be40d052ae0140ebd88311d49718904d232ba40c7c59d498db76f727e7150398ccdee233daf22ca73feadb84d50a2df23ca20ea811bdee67a2250f7c5badf40e32cee75ac77a6a00000000"

	// SendRawTransaction(raw)
	result, err := satsnet_rpc.SendRawTransaction(raw, true)
	if err != nil {
		fmt.Printf("sendrawtransaction error: %s\n", err)
		return
	}

	//btcwallet.LogMsgTx(result.MsgTx())
	fmt.Printf("sendrawtransaction success,result: %v\n", result)

}
