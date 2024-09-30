package main

import (
	"fmt"
	"math/rand"

	"github.com/sat20-labs/satsnet_btcd/btcutil"
	"github.com/sat20-labs/satsnet_btcd/chaincfg"
	"github.com/sat20-labs/satsnet_btcd/chaincfg/chainhash"
	"github.com/sat20-labs/satsnet_btcd/cmd/btcd_client/btcwallet"
	"github.com/sat20-labs/satsnet_btcd/txscript"
	"github.com/sat20-labs/satsnet_btcd/wire"
)

// CreateCoinbaseTx returns a coinbase transaction paying an appropriate
// subsidy based on the passed block height and the block subsidy.  The
// coinbase signature script conforms to the requirements of version 2 blocks.
func CreateCoinbaseTx(blockHeight int32, miningAddr string, feeAmount int64) *wire.MsgTx {
	extraNonce := uint64(0)
	coinbaseScript, err := StandardCoinbaseScript(blockHeight, extraNonce)
	if err != nil {
		panic(err)
	}

	tx := wire.NewMsgTx(1)
	tx.AddTxIn(&wire.TxIn{
		// Coinbase transactions have no inputs, so previous outpoint is
		// zero hash and max index.
		PreviousOutPoint: *wire.NewOutPoint(&chainhash.Hash{},
			wire.MaxPrevOutIndex),
		Sequence:        wire.MaxTxInSequenceNum,
		SignatureScript: coinbaseScript,
	})

	outputScript, err := AddrToPkScript(miningAddr, currentNetwork)
	tx.AddTxOut(&wire.TxOut{
		Value:    feeAmount,
		PkScript: outputScript,
	})
	return tx
}

// CreateAnchorTx
func CreateAnchorTx(txid string, addr string, amount int64, satsRanges []wire.SatsRange) *wire.MsgTx {
	pkScript, err := AddrToPkScript(addr, currentNetwork)
	anchorScript, err := StandardAnchorScript(txid, pkScript, amount)
	if err != nil {
		panic(err)
	}

	tx := wire.NewMsgTx(1)
	tx.AddTxIn(&wire.TxIn{
		// Anchor transactions have no inputs, so previous outpoint is
		// zero hash and anchor tx index.
		PreviousOutPoint: *wire.NewOutPoint(&chainhash.Hash{},
			wire.AnchorTxOutIndex),
		Sequence:        wire.MaxTxInSequenceNum,
		SignatureScript: anchorScript,
	})
	tx.AddTxOut(&wire.TxOut{
		PkScript:   pkScript, // output to specified address
		Value:      amount,
		SatsRanges: satsRanges,
	})
	return tx
}

// // CreateAnchorTx
// // utxo : 需要
// func CreateUnanchorTx(utxo []string, addr string, amount int64, satsRanges []wire.SatsRange) *wire.MsgTx {
// 	extraNonce := rand.Uint64()
// 	pkScript, err := AddrToPkScript(addr, currentNetwork)
// 	unanchorScript, err := StandardUnanchorScript(txid, pkScript, amount, extraNonce)
// 	if err != nil {
// 		panic(err)
// 	}

// 	tx := wire.NewMsgTx(1)
// 	tx.AddTxIn(&wire.TxIn{
// 		// Anchor transactions have no inputs, so previous outpoint is
// 		// zero hash and anchor tx index.
// 		PreviousOutPoint: *wire.NewOutPoint(&chainhash.Hash{},
// 			wire.AnchorTxOutIndex),
// 		Sequence:        wire.MaxTxInSequenceNum,
// 		SignatureScript: anchorScript,
// 	})
// 	tx.AddTxOut(&wire.TxOut{
// 		PkScript:   pkScript, // output to specified address
// 		Value:      amount,
// 		SatsRanges: satsRanges,
// 	})
// 	return tx
// }

// StandardCoinbaseScript returns a standard script suitable for use as the
// signature script of the coinbase transaction of a new block.  In particular,
// it starts with the block height that is required by version 2 blocks.
func StandardCoinbaseScript(blockHeight int32, extraNonce uint64) ([]byte, error) {
	return txscript.NewScriptBuilder().AddInt64(int64(blockHeight)).
		AddInt64(int64(extraNonce)).Script()
}

// StandardAnchorScript returns a standard script suitable for use as the
// signature script of the coinbase transaction of a new block.  In particular,
// it starts with the block height that is required by version 2 blocks.
func StandardAnchorScript(txid string, pkScript []byte, amount int64) ([]byte, error) {
	extraNonce := rand.Uint64()
	return txscript.NewScriptBuilder().AddData([]byte(txid)).AddData(pkScript).
		AddInt64(int64(amount)).AddInt64(int64(extraNonce)).Script()
}

// StandardUnnchorScript returns a standard script suitable for use as the
// signature script of the coinbase transaction of a new block.  In particular,
// it starts with the block height that is required by version 2 blocks.
func StandardUnanchorScript(pkScript []byte, amount int64) ([]byte, error) {
	extraNonce := rand.Uint64()
	return txscript.NewScriptBuilder().AddOp(txscript.OP_RETURN).AddData(pkScript).
		AddInt64(int64(amount)).AddInt64(int64(extraNonce)).Script()
}

func AddrToPkScript(addr string, netParams *chaincfg.Params) ([]byte, error) {
	address, err := btcutil.DecodeAddress(addr, netParams)
	if err != nil {
		return nil, err
	}

	return txscript.PayToAddrScript(address)
}

func testAnchorTx(lockedTxid string, address string) {
	fmt.Printf("testAnchorTx...\n")
	TxidDefault := "b274b49e885fdd87ea2870930297d2c6ecee7cc62fe8e67b21b452fb348c441e"
	//address := "tb1prm9fflqhtezag25s06t740e7ca4rydm9x5mucrc3lt6dlkxquyqq02k2cf"
	amount := int64(1000000)
	satsRanges := []wire.SatsRange{{Start: 2000000, Size: 500000}, {Start: 5000000, Size: 500000}}

	walletManager := btcwallet.GetWalletInst()
	if walletManager == nil {
		fmt.Printf("GetWalletInst failed.\n")
		return
	}

	if address == "" {
		var err error
		address, err = walletManager.GetDefaultAddress()
		if err != nil {
			fmt.Printf("GetDefaultAddress failed: %v.\n", err)
			return
		}
	}
	if lockedTxid == "" {
		lockedTxid = TxidDefault
	}

	fmt.Printf("Anchor tx address is : %s.\n", address)

	anchorTx := CreateAnchorTx(lockedTxid, address, amount, satsRanges)

	btcwallet.LogMsgTx(anchorTx)

	fmt.Printf("Anchor tx is %v.\n", anchorTx)
	raw, err := messageToHex(anchorTx)
	if err != nil {
		fmt.Printf("Error message tx: %v, error: %s\n",
			anchorTx, err)
		return
	}

	SendRawTransaction(raw)

	fmt.Printf("testAnchorTx done.\n")
}
