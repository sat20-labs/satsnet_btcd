package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"

	"github.com/sat20-labs/satsnet_btcd/btcjson"
	"github.com/sat20-labs/satsnet_btcd/btcutil"
	"github.com/sat20-labs/satsnet_btcd/chaincfg/chainhash"
	"github.com/sat20-labs/satsnet_btcd/cmd/btcd_client/btcwallet"
	"github.com/sat20-labs/satsnet_btcd/cmd/btcd_client/satsnet_rpc"
	"github.com/sat20-labs/satsnet_btcd/wire"
)

// messageToHex serializes a message to the wire protocol encoding using the
// latest protocol version and returns a hex-encoded string of the result.
func messageToHex(msg *wire.MsgTx) (string, error) {
	// var buf bytes.Buffer
	// maxProtocolVersion := uint32(70002)
	// if err := msg.BtcEncode(&buf, maxProtocolVersion, wire.WitnessEncoding); err != nil {
	// 	fmt.Printf("Failed to encode msg of type %T", msg)
	// 	return "", err
	// }

	var buf bytes.Buffer

	// Serialize the MsgTx into a byte buffer
	err := msg.Serialize(&buf)
	if err != nil {
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
	result, err := sendPostRequest(marshalledJSON, currentCfg, RPC_BTCD)
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

func testImportWallet(walletName string) {
	mnemonic := "mnemonic"
	// walletManager := btcwallet.GetWalletInst()
	// if walletManager == nil {
	// 	fmt.Printf("GetWalletInst failed.\n")
	// 	return
	// }

	wallet, err := btcwallet.Import(walletName, mnemonic)
	if err != nil {
		fmt.Printf("Import wallet failed: error: %s\n",
			err)
		return
	}

	fmt.Printf("wallet: %v\n", wallet)

	//walletManager.AddBTCWallet(wallet)
}

func testCreateWallet(walletName string) {
	// walletManager := btcwallet.GetWalletInst()
	// if walletManager == nil {
	// 	fmt.Printf("GetWalletInst failed.\n")
	// 	return
	// }

	wallet, err := btcwallet.CreateBTCWallet(walletName)
	if err != nil {
		fmt.Printf("Create wallet failed: error: %s\n",
			err)
		return
	}

	fmt.Printf("wallet: %v\n", wallet)

	//walletManager.AddBTCWallet(wallet)
}

func testShowAddress() {
	walletManager := btcwallet.GetWalletInst()
	if walletManager == nil {
		fmt.Printf("GetWalletInst failed.\n")
		return
	}
	address, err := walletManager.GetDefaultAddress()

	if err != nil {
		fmt.Printf("GetDefaultAddress failed: error: %s\n",
			err)
		return
	}
	fmt.Printf("address: %v\n", address)
}

func testTransfer(address string, amount int64) {
	fmt.Printf("Will transfer %d sats to %s\n", amount, address)
	walletManager := btcwallet.GetWalletInst()
	wallet, err := walletManager.GetDefaultWallet()
	if err != nil {
		fmt.Printf("GetDefaultWallet failed: error: %s\n",
			err)
		return
	}

	addr, err := wallet.GetDefaultAddress()
	if err != nil {
		fmt.Printf("GetDefaultAddress failed: error: %s\n",
			err)
		return
	}

	fmt.Printf("Transfer addr: %v\n", addr)

	btcAmount := btcutil.Amount(amount)
	amountTx := big.NewFloat(btcAmount.ToBTC())

	raw, err := wallet.Transaction(addr, address, amountTx, 5, "")
	if err != nil {
		fmt.Printf("GetDefaultAddress failed: error: %s\n",
			err)
		return
	}

	hash, err := satsnet_rpc.SendRawTransaction(raw, true)
	if err != nil {
		fmt.Printf("rpc.SendRawTransaction failed: error: %s\n",
			err)
		wallet.RefreshAccountDetails()

		return
	}
	txid := hash.String()
	fmt.Printf("Transfer txid: %v\n", txid)
	//SendRawTransaction(raw)

	//fmt.Printf("Transfer txid: %v\n", txid)
}

func testUnanchorTransfer(address string, amount int64) {
	fmt.Printf("Will unanchor %d sats to address %s\n", amount, address)
	walletManager := btcwallet.GetWalletInst()
	wallet, err := walletManager.GetDefaultWallet()
	if err != nil {
		fmt.Printf("GetDefaultWallet failed: error: %s\n",
			err)
		return
	}

	addr, err := wallet.GetDefaultAddress()
	if err != nil {
		fmt.Printf("GetDefaultAddress failed: error: %s\n",
			err)
		return
	}

	fmt.Printf("Transfer addr: %v\n", addr)

	btcAmount := btcutil.Amount(amount)
	amountTx := big.NewFloat(btcAmount.ToBTC())

	unanchorTargetScript, err := btcwallet.AddrToPkScript(address)
	if err != nil {
		fmt.Printf("Invalid address failed: error: %s\n", err)
		return
	}

	unanchorScript, err := StandardUnanchorScript(unanchorTargetScript, amount)
	if err != nil {
		fmt.Printf("Generate unanchor script failed: error: %s\n", err)
		return
	}
	raw, err := wallet.UnanchorTransaction(addr, unanchorScript, amountTx, 5)
	if err != nil {
		fmt.Printf("GetDefaultAddress failed: error: %s\n",
			err)
		return
	}
	SendRawTransaction(raw)

	//fmt.Printf("Transfer txid: %v\n", txid)
}

func testSampleTx() {
	fmt.Printf("testSampleTx\n")
	tx := wire.NewMsgTx(1)

	txid := "445f6422d72e64a2e0b808c3dadcc9131403bc1e43d47e2e515cd6db90aa6f38"
	index := uint32(0)
	//hashCode, _ := hex.DecodeString("445f6422d72e64a2e0b808c3dadcc9131403bc1e43d47e2e515cd6db90aa6f38")
	//hash := chainhash.Hash(hashCode)

	hashTx, _ := chainhash.NewHashFromStr(txid)

	Witness := wire.TxWitness{}
	code, _ := hex.DecodeString("304402202d0f3635b4d9efd8b17d63b6337600332802d0f21204c302344c882b090635bc022063e2f28707c5372312371acf5031bf05fe0b53c3302ad7a6dbe9e40cfbe8d0a501")
	Witness = append(Witness, code)

	code, _ = hex.DecodeString("304402204070bb47d42e5ca5ce2cc25baf65abb06f5d7cc874ac1d2111c6a3021be7547f0220552cd860c53be5fd20ab7950485cb50bf80192bda7fd71152df3d3ee7d1a716001")
	Witness = append(Witness, code)

	code, _ = hex.DecodeString("52210304374824804565034f8a7ffb680f05a20db8ef6888d5621d8577f6f412596f9e21036fa703396bdcbf4b3466c62079df08366b080e9832f28fd246c0eeea90be40d052ae")
	Witness = append(Witness, code)

	// []byte("304402202d0f3635b4d9efd8b17d63b6337600332802d0f21204c302344c882b090635bc022063e2f28707c5372312371acf5031bf05fe0b53c3302ad7a6dbe9e40cfbe8d0a501"),
	// []byte("304402204070bb47d42e5ca5ce2cc25baf65abb06f5d7cc874ac1d2111c6a3021be7547f0220552cd860c53be5fd20ab7950485cb50bf80192bda7fd71152df3d3ee7d1a716001"),
	// []byte("52210304374824804565034f8a7ffb680f05a20db8ef6888d5621d8577f6f412596f9e21036fa703396bdcbf4b3466c62079df08366b080e9832f28fd246c0eeea90be40d052ae")}

	tx.AddTxIn(&wire.TxIn{
		// Anchor transactions have no inputs, so previous outpoint is
		// zero hash and anchor tx index.
		PreviousOutPoint: *wire.NewOutPoint(hashTx,
			index),
		Sequence: 0,
		//SignatureScript: anchorScript,
		Witness: Witness,
	})

	satsRanges0 := wire.TxRanges{
		{Start: 133474737439951, Size: 18797},
		{Start: 133474737458768, Size: 1183},
	}
	pkScript1, _ := hex.DecodeString("0020650c7b012cd5aa9201251bb1bedefc62aa84548a73bb259595959029e2011616")
	tx.AddTxOut(&wire.TxOut{
		PkScript:   pkScript1, // output to specified address
		Value:      19980,
		SatsRanges: satsRanges0,
	})
	satsRanges1 := wire.TxRanges{
		{Start: 133474737458748, Size: 10},
	}
	pkScript2, _ := hex.DecodeString("51208be89118321c458463a3f3404d626559dc4adeccb4017846e920b32d32e374d9")
	tx.AddTxOut(&wire.TxOut{
		PkScript:   pkScript2, // output to specified address
		Value:      10,
		SatsRanges: satsRanges1,
	})

	fmt.Printf("Sample tx is %v.\n", tx)
	raw, err := messageToHex(tx)
	if err != nil {
		fmt.Printf("Error message tx: %v, error: %s\n",
			tx, err)
		return
	}

	SendRawTransaction(raw)
}
