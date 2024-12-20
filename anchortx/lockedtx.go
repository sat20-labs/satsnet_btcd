// Copyright (c) 2024 The sats20 developers
// All Locked tx info from BTC chain, not satsnet

package anchortx

import (
	"encoding/hex"
	"fmt"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
)

var NetParams = chaincfg.TestNet3Params

type LockedInfoInBTCChain struct {
	Utxo string // the txid with locked in lnd
	//LockedAddresses []string // the locked addresses, it should be multi sig address
	pkScript  []byte // the pkscript
	Amount    int64  // the amount with locked in lnd
	AssetInfo []*UtxoAssetInfo
}

func IsCheckLockedTx() bool {
	host := anchorManager.anchorConfig.IndexerHost
	net := anchorManager.anchorConfig.IndexerNet
	if host == "" || net == "" {
		return false
	}
	return true
}

func GetLockedInfo(txid string) (*LockedInfoInBTCChain, error) {
	lockedInfo := &LockedInfoInBTCChain{}
	host := anchorManager.anchorConfig.IndexerHost
	net := anchorManager.anchorConfig.IndexerNet

	// Get TxInfo from BTC chain (Layer 1 chain)
	indexerClient := NewIndexerClient(host, net)
	rawTx, err := indexerClient.GetRawTx(txid)
	if err != nil {
		fmt.Printf("GetRawTx failed: %s\n", err.Error())
		return nil, err
	}

	dataTx, err := hex.DecodeString(rawTx)
	if err != nil {
		fmt.Printf("DecodeString raw data failed: %s\n", err.Error())
		return nil, err
	}
	tx, err := btcutil.NewTxFromBytes(dataTx)
	if err != nil {
		fmt.Printf("btcutil.NewTxFromBytes failed: %s\n", err.Error())
		return nil, err
	}
	logMsgTx(tx.MsgTx())

	// Only get locked info with first txout
	out := tx.MsgTx().TxOut[0]
	lockedInfo.Utxo = fmt.Sprintf("%s:%d", txid, 0)
	lockedInfo.pkScript = out.PkScript
	lockedInfo.Amount = out.Value

	return lockedInfo, nil
}
func GetLockedUtxoInfo(utxo string) (*LockedInfoInBTCChain, error) {
	lockedInfo := &LockedInfoInBTCChain{}
	host := anchorManager.anchorConfig.IndexerHost
	net := anchorManager.anchorConfig.IndexerNet

	// Get TxInfo from BTC chain (Layer 1 chain)
	indexerClient := NewIndexerClient(host, net)
	utxoAssetsInfo, err := indexerClient.GetTxUtxoAssets(utxo)
	if err != nil {
		fmt.Printf("GetRawTx failed: %s\n", err.Error())
		return nil, err
	}

	lockedInfo.Utxo = utxo
	lockedInfo.pkScript = utxoAssetsInfo.OutValue.PkScript
	lockedInfo.Amount = utxoAssetsInfo.OutValue.Value
	lockedInfo.AssetInfo = utxoAssetsInfo.AssetInfo

	return lockedInfo, nil
}

func logMsgTx(tx *wire.MsgTx) {
	logLine("tx:%s", tx.TxHash().String())
	logLine("txin: %d", len(tx.TxIn))
	for index, txin := range tx.TxIn {
		logLine("		txin index: %d", index)
		logLine("		txin utxo txid: %s", txin.PreviousOutPoint.Hash.String())
		logLine("		txin utxo index: %d", txin.PreviousOutPoint.Index)
		logLine("		txin utxo Wintness: ")
		logLine("		{")
		for _, witness := range txin.Witness {
			logLine("		%x", witness)
		}
		logLine("		}")
		logLine("		txin SignatureScript: %x", txin.SignatureScript)
		logLine("		---------------------------------")
	}

	logLine("txout: %d", len(tx.TxOut))
	for index, txout := range tx.TxOut {
		logLine("		txout index: %d", index)
		logLine("		txout pkscript: %x", txout.PkScript)

		if txscript.IsNullData(txout.PkScript) {
			logLine("		txout pkscript is an OP_RETURN script")
		} else {
			addr, err := PkScriptToAddr(txout.PkScript)
			if err != nil {
				logLine("		txout pkscript is an invalidaddress: %s", err)
			} else {
				logLine("		txout address: %s", addr)
			}
		}
		logLine("		txout value: %d", txout.Value)
		logLine("		---------------------------------")
	}

}

func logLine(format string, a ...any) {
	logStr := fmt.Sprintf(format, a...)
	fmt.Println(logStr)
}

func PkScriptToAddr(pkScript []byte) (string, error) {

	if len(pkScript) > 0 && pkScript[0] == txscript.OP_RETURN {
		err := fmt.Errorf("pkscript is OP_RETURN Script")
		return "", err
	}

	_, addrs, _, err := txscript.ExtractPkScriptAddrs(pkScript, &NetParams)
	if err != nil {
		return "", err
	}

	if len(addrs) == 0 {
		err := fmt.Errorf("failed to get addr with pkscript[%v]", pkScript)
		return "", err
	}

	return addrs[0].EncodeAddress(), nil

}
