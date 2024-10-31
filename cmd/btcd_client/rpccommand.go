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

func testSendRawTransaction(rawSend string) {
	fmt.Printf("test sendrawtransaction...\n")

	raw := rawSend
	if raw == "" {
		raw = "01000000000102553ce5be0f7e3c2971a4e14d76fd15101760f9cce4122a2ae22f4c0d73a98a620000000000ffffffff553ce5be0f7e3c2971a4e14d76fd15101760f9cce4122a2ae22f4c0d73a98a620100000000ffffffff02194800000000000001ff83134c0265790000fd1948220020650c7b012cd5aa9201251bb1bedefc62aa84548a73bb259595959029e20116160a0000000000000001ff9c5b4c02657900000a2251208be89118321c458463a3f3404d626559dc4adeccb4017846e920b32d32e374d9040048304502210088895b506a9cee8b90180af66007b196c77b4ae00db3c2b4725b4706d662755302207843bab10206371d69780a18dbad9bfe883276c2b9f4d28d937926096303b16d01483045022100fae920f172165d503bcd99cbda817ea0a433409cbae77c097c2e51a04b7989da02204f06e0fb15a275355fc4a4d31583187d158a4f8a34dd8c57f73687004dff40eb014752210304374824804565034f8a7ffb680f05a20db8ef6888d5621d8577f6f412596f9e21036fa703396bdcbf4b3466c62079df08366b080e9832f28fd246c0eeea90be40d052ae01405f989f76dafc2b53ddd239a35ff52f133a421a35a648fc747b05a819c6ddcfee40bf746d871bdd1cea5dc10aa088b65055b3fcf43f62884851a514bee53a12de00000000"
	}

	// SendRawTransaction(raw)
	result, err := satsnet_rpc.SendRawTransaction(raw, true)
	if err != nil {
		fmt.Printf("sendrawtransaction error: %s\n", err)
		return
	}

	//btcwallet.LogMsgTx(result.MsgTx())
	fmt.Printf("sendrawtransaction success,result: %v\n", result)

}
