// Copyright (c) 2013-2016 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"fmt"

	"github.com/sat20-labs/satsnet_btcd/txscript"
	"github.com/sat20-labs/satsnet_btcd/wire"
)

func LogMsgTx(title string, msg *wire.MsgTx) {
	rpcsLog.Debugf("		---------------------------------")
	rpcsLog.Debugf("%s", title)
	rpcsLog.Debugf("tx:%s", msg.TxHash().String())
	rpcsLog.Debugf("txin: %d", len(msg.TxIn))
	for index, txin := range msg.TxIn {
		rpcsLog.Debugf("		txin index: %d", index)
		rpcsLog.Debugf("		txin utxo txid: %s", txin.PreviousOutPoint.Hash.String())
		rpcsLog.Debugf("		txin utxo index: %d", txin.PreviousOutPoint.Index)
		rpcsLog.Debugf("		txin utxo Wintness: ")
		rpcsLog.Debugf("		{")
		for _, witness := range txin.Witness {
			rpcsLog.Debugf("		%x", witness)
		}
		rpcsLog.Debugf("		}")
		rpcsLog.Debugf("		txin SignatureScript: %x", txin.SignatureScript)
		rpcsLog.Debugf("		---------------------------------")
	}

	rpcsLog.Debugf("txout: %d", len(msg.TxOut))
	for index, txout := range msg.TxOut {
		rpcsLog.Debugf("		txout index: %d", index)
		rpcsLog.Debugf("		txout pkscript: %x", txout.PkScript)

		if txscript.IsNullData(txout.PkScript) {
			rpcsLog.Debugf("		txout pkscript is an OP_RETURN script")
		} else {
			addr, err := PkScriptToAddr(txout.PkScript)
			if err != nil {
				rpcsLog.Debugf("		txout pkscript is an invalidaddress: %s", err)
			} else {
				rpcsLog.Debugf("		txout address: %s", addr)
			}
		}
		rpcsLog.Debugf("		txout value: %d", txout.Value)
		logTxRanges("", txout.SatsRanges)
		rpcsLog.Debugf("		---------------------------------")
	}
}
func logTxRanges(desc string, tr wire.TxRanges) {
	if desc != "" {
		rpcsLog.Debugf("       	SatsRangs desc: %s", desc)
	}
	rpcsLog.Debugf("       	SatsRangs count: %d", len(tr))
	for index, satsRange := range tr {
		rpcsLog.Debugf("		---------------------------------")
		rpcsLog.Debugf("			satsRange index: %d", index)
		rpcsLog.Debugf("			satsRange Start: %d", satsRange.Start)
		rpcsLog.Debugf("			satsRange Size: %d", satsRange.Size)
	}

}

func PkScriptToAddr(pkScript []byte) (string, error) {

	if len(pkScript) > 0 && pkScript[0] == txscript.OP_RETURN {
		err := fmt.Errorf("pkscript is OP_RETURN Script")
		return "", err
	}

	_, addrs, _, err := txscript.ExtractPkScriptAddrs(pkScript, activeNetParams.Params)
	if err != nil {
		return "", err
	}

	if len(addrs) == 0 {
		err := fmt.Errorf("failed to get addr with pkscript[%v]", pkScript)
		return "", err
	}

	return addrs[0].EncodeAddress(), nil

}