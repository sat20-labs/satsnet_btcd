package common

import (
	"bytes"
	"fmt"

	"github.com/sat20-labs/satsnet_btcd/btcutil"
	"github.com/sat20-labs/satsnet_btcd/chaincfg"
	"github.com/sat20-labs/satsnet_btcd/txscript"
	"github.com/sat20-labs/satsnet_btcd/wire"
)

var (
	ChainTestnet = chaincfg.SatsTestNetParams.Name
	ChainMainnet = chaincfg.SatsMainNetParams.Name
)

func PkScriptToAddr(pkScript []byte, chain string) (string, error) {
	chainParams := &chaincfg.SatsTestNetParams
	switch chain {
	case ChainTestnet:
		chainParams = &chaincfg.SatsTestNetParams
	case ChainMainnet:
		chainParams = &chaincfg.SatsMainNetParams
	}
	_, addrs, _, err := txscript.ExtractPkScriptAddrs(pkScript, chainParams)
	if err != nil {
		return "", err
	}
	if len(addrs) == 0 {
		return "", fmt.Errorf("no address")
	}
	return addrs[0].EncodeAddress(), nil
}

func IsValidAddr(addr string, chain string) (bool, error) {
	chainParams := &chaincfg.SatsTestNetParams
	switch chain {
	case ChainTestnet:
		chainParams = &chaincfg.SatsTestNetParams
	case ChainMainnet:
		chainParams = &chaincfg.SatsMainNetParams
	default:
		return false, nil
	}
	_, err := btcutil.DecodeAddress(addr, chainParams)
	if err != nil {
		return false, err
	}
	return true, nil
}

func AddrToPkScript(addr string, chain string) ([]byte, error) {
	chainParams := &chaincfg.SatsMainNetParams
	switch chain {
	case ChainTestnet:
		chainParams = &chaincfg.SatsTestNetParams
	case ChainMainnet:
		chainParams = &chaincfg.SatsMainNetParams
	default:
		return nil, fmt.Errorf("invalid chain: %s", chain)
	}
	address, err := btcutil.DecodeAddress(addr, chainParams)
	if err != nil {
		return nil, err
	}
	return txscript.PayToAddrScript(address)
}

func IsOpReturn(pkScript []byte, param *chaincfg.Params) bool {
	// 解析输出脚本
	scriptClass, _, _, err := txscript.ExtractPkScriptAddrs(pkScript, param)
	if err != nil {
		return false
	}

	return scriptClass == txscript.NullDataTy
}

func IsCoinbaseTx(tx *wire.MsgTx) bool {
	// A coinbase transaction must have exactly one input.
	if len(tx.TxIn) != 1 {
		return false
	}

	// Check if the input's previous outpoint hash is all zeros and index is 0xFFFFFFFF.
	prevOut := tx.TxIn[0].PreviousOutPoint
	zeroHash := [32]byte{}
	if !bytes.Equal(prevOut.Hash[:], zeroHash[:]) || prevOut.Index != wire.MaxTxInSequenceNum {
		return false
	}

	// If the above conditions are met, it's a coinbase transaction.
	return true
}
