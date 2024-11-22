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
