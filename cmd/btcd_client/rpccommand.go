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

	raw := "010000000001016505940676b97e67937efb67d9004d56bd07e708e84afb4583f825200c1abe3a0000000000ffffffff01fe5100000000000001ffdd92920665790000fdfe510e6a0c5354503a4465416e63686f720400483045022100d03913b57367b7ed18f974bb8d8207fa0dff799fbcfd3badc22e07f515e49e810220618b6ba4c29a10f352f0a2b5ef399961cb3ef0edd1dacb96543b8c638811e8fd01483045022100f02f334b59d3ab51cc735a9e12ab9c9801024466ad6fe8e20046a36ca09c36e402203c5c6b9d183cdb744ae00ab59529f442355927f785185747e3629f1f943b6a600147522103e87d073d7a2e4c7133501d652593a884a7e624e2327c3b0e2b025154f72aebac2103ea4d2bf0c98d738b4adb0e76ec9db2385ac02adb573ea646ed8cc18fbb0c65b752ae00000000"

	// SendRawTransaction(raw)
	result, err := satsnet_rpc.SendRawTransaction(raw, true)
	if err != nil {
		fmt.Printf("sendrawtransaction error: %s\n", err)
		return
	}

	//btcwallet.LogMsgTx(result.MsgTx())
	fmt.Printf("sendrawtransaction success,result: %v\n", result)

}
