package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"

	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
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
func CreateAnchorTx(txid string, addr string, amount int64) *wire.MsgTx {
	extraNonce := rand.Uint64()
	pkScript, err := AddrToPkScript(addr, currentNetwork)
	anchorScript, err := StandardAnchorScript(txid, pkScript, amount, extraNonce)
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
		PkScript: anchorScript,
		Value:    amount,
	})
	return tx
}

// StandardCoinbaseScript returns a standard script suitable for use as the
// signature script of the coinbase transaction of a new block.  In particular,
// it starts with the block height that is required by version 2 blocks.
func StandardCoinbaseScript(blockHeight int32, extraNonce uint64) ([]byte, error) {
	return txscript.NewScriptBuilder().AddInt64(int64(blockHeight)).
		AddInt64(int64(extraNonce)).Script()
}

// StandardCoinbaseScript returns a standard script suitable for use as the
// signature script of the coinbase transaction of a new block.  In particular,
// it starts with the block height that is required by version 2 blocks.
func StandardAnchorScript(txid string, pkScript []byte, amount int64, extraNonce uint64) ([]byte, error) {
	return txscript.NewScriptBuilder().AddData([]byte(txid)).AddData(pkScript).
		AddInt64(int64(amount)).AddInt64(int64(extraNonce)).Script()
}

func AddrToPkScript(addr string, netParams *chaincfg.Params) ([]byte, error) {
	address, err := btcutil.DecodeAddress(addr, netParams)
	if err != nil {
		return nil, err
	}

	return txscript.PayToAddrScript(address)
}

func testAnchorTx() {
	fmt.Printf("testAnchorTx...\n")
	Txid := "b274b49e885fdd87ea2870930297d2c6ecee7cc62fe8e67b21b452fb348c441e"
	address := "tb1prm9fflqhtezag25s06t740e7ca4rydm9x5mucrc3lt6dlkxquyqq02k2cf"
	amount := int64(1000000)

	anchorTx := CreateAnchorTx(Txid, address, amount)

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

// messageToHex serializes a message to the wire protocol encoding using the
// latest protocol version and returns a hex-encoded string of the result.
func messageToHex(msg wire.Message) (string, error) {
	var buf bytes.Buffer
	maxProtocolVersion := uint32(70002)
	if err := msg.BtcEncode(&buf, maxProtocolVersion, wire.WitnessEncoding); err != nil {
		fmt.Printf("Failed to encode msg of type %T", msg)
		return "", err
	}

	return hex.EncodeToString(buf.Bytes()), nil
}

func SendRawTransaction(raw string) {
	// Attempt to create the appropriate command using the arguments
	// provided by the user.
	cmd, err := btcjson.NewCmd("sendrawtransaction", raw, &btcjson.AllowHighFeesOrMaxFeeRate{
		Value: btcjson.Bool(false),
	})
	if err != nil {
		fmt.Printf("Create cmd failed: error: %s\n",
			err)
		return
	}

	// Marshal the command into a JSON-RPC byte slice in preparation for
	// sending it to the RPC server.
	marshalledJSON, err := btcjson.MarshalCmd(btcjson.RpcVersion1, 1, cmd)
	if err != nil {
		fmt.Printf("MarshalCmd failed: error: %s\n",
			err)
		return
	}

	// Send the JSON-RPC request to the server using the user-specified
	// connection configuration.
	result, err := sendPostRequest(marshalledJSON, currentCfg)
	if err != nil {
		fmt.Printf("sendPostRequest failed: error: %s\n",
			err)
		return
	}

	// Choose how to display the result based on its type.
	strResult := string(result)
	if strings.HasPrefix(strResult, "{") || strings.HasPrefix(strResult, "[") {
		var dst bytes.Buffer
		if err := json.Indent(&dst, result, "", "  "); err != nil {
			fmt.Printf("Indent failed: error: %s\n", err)
			return
		}
		fmt.Println(dst.String())

	} else if strings.HasPrefix(strResult, `"`) {
		var str string
		if err := json.Unmarshal(result, &str); err != nil {
			fmt.Printf("Unmarshal result failed: error: %s\n", err)
			return
		}
		fmt.Println(str)

	} else if strResult != "null" {
		fmt.Println(strResult)
	}
}
