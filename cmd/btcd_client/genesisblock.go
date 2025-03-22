package main

import (
	"encoding/hex"
	"fmt"
	"math/rand"
	"time"

	"github.com/sat20-labs/satoshinet/blockchain"
	"github.com/sat20-labs/satoshinet/btcutil"
	"github.com/sat20-labs/satoshinet/chaincfg"
	"github.com/sat20-labs/satoshinet/chaincfg/chainhash"
	"github.com/sat20-labs/satoshinet/cmd/btcd_client/btcwallet"
	"github.com/sat20-labs/satoshinet/wire"
)

func showGenesisBlock(chainParams *chaincfg.Params) {

	// Show Block info
	showBlock(chainParams.GenesisBlock)

}

func parseAddress(address string, chainParams *chaincfg.Params) {

	pkScript, err := btcwallet.AddrToPkScript(address)
	if err != nil {
		fmt.Println(err)
		return
	}
	log.Debugf("    pkScript: %x", pkScript)
}

func parsePkScript(pkscript string, chainParams *chaincfg.Params) {

	pkBytes, _ := hex.DecodeString(pkscript)
	address, err := btcwallet.PkScriptToAddr(pkBytes)
	log.Debugf("pkScript: %x", pkBytes)
	if err != nil {
		log.Errorf("PkScriptToAddr failed: %v ", err)
	} else {
		log.Debugf("address: %s", address)
	}
}

func GenerateGenesisBlock(chainParams *chaincfg.Params) {

	genesisTime := time.Now()
	// Create a genesis TX
	genesisTx, _ := createGenesisTx(chainParams, genesisTime)

	btcwallet.LogMsgTx(genesisTx)

	nonce := rand.Uint32()

	genesisBlock := &wire.MsgBlock{
		Header: wire.BlockHeader{
			Version:    1,
			PrevBlock:  chainhash.Hash{},
			MerkleRoot: chainhash.Hash{},
			Timestamp:  genesisTime,
			Bits:       0,
			Nonce:      nonce,
		},
		Transactions: []*wire.MsgTx{genesisTx},
	}

	genesisBlock.Header.MerkleRoot = calcMerkleRoot(genesisBlock.Transactions)
	genesisBlock.Header.MerkleRoot = genesisBlock.Transactions[0].TxHash()

	showBlock(genesisBlock)
	logHash("var genesisMerkleRoot = chainhash.Hash", genesisBlock.Header.MerkleRoot[:])
	blockHash := genesisBlock.BlockHash()
	logHash("var satsNetGenesisHash = chainhash.Hash", blockHash[:])
	logHash("var genesisCoinbaseScript = []byte", genesisTx.TxIn[0].SignatureScript)
	logHash("var genesisTxOutPkScript = []byte", genesisTx.TxOut[0].PkScript)
}

func showBlock(block *wire.MsgBlock) {
	// Show Block info
	log.Debugf("-------------------------  Block Header  --------------------------")
	log.Debugf("    Block Hash: %s", block.BlockHash().String())

	log.Debugf("    Block Version: %d", block.Header.Version)

	log.Debugf("    Prev Block Hash: %s", block.Header.PrevBlock.String())

	log.Debugf("    Block MerkleRoot Hash: %s", block.Header.MerkleRoot.String())

	log.Debugf("    Block TimeStamp Unix: %d", block.Header.Timestamp.Unix())
	log.Debugf("    Block TimeStamp: %s", block.Header.Timestamp.Format(time.DateTime))

	log.Debugf("    Block Bits: %d", block.Header.Bits)

	log.Debugf("    Block Nonce: %d", block.Header.Nonce)

	log.Debugf("-------------------------  End  --------------------------")

	log.Debugf("-------------------------  Block Transactions  --------------------------")
	transactions := block.Transactions
	for _, tx := range transactions {
		btcwallet.LogMsgTx(tx)
	}
	log.Debugf("-------------------------  End  --------------------------")
}

// createCoinbaseTx returns a coinbase transaction paying an appropriate subsidy
// based on the passed block height to the provided address.  When the address
// is nil, the coinbase transaction will instead be redeemable by anyone.
//
// See the comment for NewBlockTemplate for more information about why the nil
// address handling is useful.
func createGenesisTx(params *chaincfg.Params, timeStamp time.Time) (*wire.MsgTx, error) {

	pkScript, _ := hex.DecodeString("51201eca94fc175e45d42a907e97eabf3ec76a3237653537cc0f11faf4dfd8c0e100")
	coinbaseScript, err := StandardGenesisScript(pkScript, timeStamp.Unix())
	if err != nil {
		panic(err)
	}

	tx := wire.NewMsgTx(wire.TxVersion)
	tx.AddTxIn(&wire.TxIn{
		// Coinbase transactions have no inputs, so previous outpoint is
		// zero hash and max index.
		PreviousOutPoint: *wire.NewOutPoint(&chainhash.Hash{},
			wire.MaxPrevOutIndex),
		SignatureScript: coinbaseScript,
		Sequence:        wire.MaxTxInSequenceNum,
	})
	tx.AddTxOut(&wire.TxOut{
		Value:    0, // For satoshinet, no award.
		Assets:   wire.TxAssets{},
		PkScript: pkScript,
	})
	return tx, nil
}

func calcMerkleRoot(txns []*wire.MsgTx) chainhash.Hash {
	if len(txns) == 0 {
		return chainhash.Hash{}
	}

	utilTxns := make([]*btcutil.Tx, 0, len(txns))
	for _, tx := range txns {
		utilTxns = append(utilTxns, btcutil.NewTx(tx))
	}
	return blockchain.CalcMerkleRoot(utilTxns, false)
}

func logHash(title string, data []byte) {
	fmt.Printf("%s: {\n        ", title)
	lineCount := 0
	for i := 0; i < len(data); i++ {
		fmt.Printf("0x%02x, ", data[i])
		if lineCount == 7 {
			fmt.Printf("\n        ")
			lineCount = 0
			continue
		}
		lineCount++
	}
	fmt.Printf("\n")
	fmt.Printf("}\n")
}
