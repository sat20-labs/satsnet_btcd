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
	"github.com/sat20-labs/satsnet_btcd/cmd/btcd_client/btcwallet"
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
	SendRawTransaction(raw)

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
