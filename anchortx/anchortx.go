// Copyright (c) 2024 The sats20 developers

package anchortx

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/sat20-labs/satsnet_btcd/btcec"
	"github.com/sat20-labs/satsnet_btcd/btcec/schnorr"
	"github.com/sat20-labs/satsnet_btcd/btcutil"
	"github.com/sat20-labs/satsnet_btcd/chaincfg"
	"github.com/sat20-labs/satsnet_btcd/txscript"
	"github.com/sat20-labs/satsnet_btcd/wire"
)

const (
	// 65 txid,
	// 35 output script, it should be p2tr
	// 8 amount for Anchor
	// 8 extra nonce
	MIN_LEN_ANCHORTX_SCRIPT = 102
	MAX_LEN_ANCHORTX_SCRIPT = 116

	// P2WSHSize 34 bytes
	//	- OP_0: 1 byte
	//	- OP_DATA: 1 byte (WitnessScriptSHA256 length)
	//	- WitnessScriptSHA256: 32 bytes
	P2WSHSize = 1 + 1 + 32
)

type AnchorConfig struct {
	IndexerHost string
	IndexerNet  string
	ChainParams *chaincfg.Params
}
type AnchorManager struct {
	anchorConfig   *AnchorConfig
	superNodeList  [][]byte
	verifyAnchorTx bool
	quit           chan struct{}
}

// var anchorConfig *AnchorConfig
var anchorManager AnchorManager

// txscript.NewScriptBuilder().AddData(txid).AddData(WitnessScript).
// AddInt64(int64(amount)).AddInt64(int64(extraNonce)).Script()
type LockedTxInfo struct {
	TxId          string // the txid with locked in lnd
	Index         int32
	WitnessScript []byte         // WitnessScript for locked in lnd
	Amount        int64          // the amount with locked in lnd
	TxAssets      *wire.TxAssets // The assets locked
}

func StartAnchorManager(config *AnchorConfig) bool {
	if config == nil {
		return false
	}

	anchorManager.anchorConfig = config
	anchorManager.quit = make(chan struct{})
	log.Debugf("AnchorConfig: IndexerHost: %s", anchorManager.anchorConfig.IndexerHost)
	log.Debugf("AnchorConfig: IndexerNet: %s", anchorManager.anchorConfig.IndexerNet)
	log.Debugf("AnchorConfig: ChainParams: %s", anchorManager.anchorConfig.ChainParams.Name)

	// Default， the node will check anchor tx
	anchorManager.verifyAnchorTx = true
	if anchorManager.anchorConfig.IndexerHost == "" || anchorManager.anchorConfig.IndexerNet == "" {
		//anchorManager.verifyAnchorTx = false
		log.Debugf("The node not config indexer infomation, will not check anchor tx")
	}

	//anchorManager.updateSuperList()
	go anchorManager.syncSuperListHandler()

	return true
}

func Stop() {
	if anchorManager.quit != nil {
		close(anchorManager.quit)
	}
}

func CheckAnchorTxValid(tx *wire.MsgTx) error {
	if anchorManager.verifyAnchorTx == false {
		log.Debugf("Not check anchor tx.")
		return nil
	}

	if len(tx.TxIn) != 1 {
		err := fmt.Errorf("invalid Anchor tx, No Anchor info: %s", tx.TxHash().String())
		return err
	}
	AnchorScript := tx.TxIn[0].SignatureScript

	// Check the Anchor tx has completed, all the assets is locked in lnd will be mapped to sats net only one times
	lockedInfo, err := checkAnchorPkScript(AnchorScript)
	if err != nil {
		log.Debugf("invalid Anchor tx, invalid Anchor script: %s, err: %s", tx.TxHash().String(), err.Error())
		return err
	}

	// Check anchor amount is same with locked amount

	anchorAmount := int64(0)
	for _, out := range tx.TxOut {
		anchorAmount += out.Value
	}

	if anchorAmount > lockedInfo.Amount {
		err := fmt.Errorf("invalid Anchor tx, Anchor amount(%d) is exceed the locked amount (%d): %s", anchorAmount, lockedInfo.Amount, tx.TxHash().String())
		return err
	}

	// Check the Anchor tx is valid
	return nil
}

// The Anchor tx info is record in Anchor tx input script
// return txscript.NewScriptBuilder().AddData(data).AddData(outputScript).
// AddInt64(int64(amount)).AddInt64(int64(extraNonce)).Script()
func GetLockedTxInfo(tx *wire.MsgTx) (*LockedTxInfo, error) {
	if len(tx.TxIn) != 1 {
		err := fmt.Errorf("invalid Anchor tx: %s", tx.TxHash().String())
		return nil, err
	}
	AnchorScript := tx.TxIn[0].SignatureScript
	// lenAnchorScript := len(AnchorScript)
	// if lenAnchorScript < MIN_LEN_ANCHORTX_SCRIPT || lenAnchorScript > MAX_LEN_ANCHORTX_SCRIPT {
	// 	err := fmt.Errorf("invalid Anchor tx script: %s", tx.TxHash().String())
	// 	return nil, err
	// }

	fmt.Printf("AnchorScript: %x\n", AnchorScript)

	lockedTxInfo, err := ParseAnchorScript(AnchorScript)
	if err != nil {
		err := fmt.Errorf("%s : anchortx[%s]", err.Error(), tx.TxHash().String())
		return nil, err
	}
	return lockedTxInfo, nil
}

func ParseAnchorScript(AnchorScript []byte) (*LockedTxInfo, error) {

	const scriptVersion = 0
	tokenizer := txscript.MakeScriptTokenizer(scriptVersion, AnchorScript)

	// The First opcode must be a canonical data push, the length of the
	// data push is bounded to 40 by the initial check on overall script
	// length.
	if !tokenizer.Next() ||
		!isCanonicalPush(tokenizer.Opcode(), tokenizer.Data()) {
		err := fmt.Errorf("invalid Anchor tx script for txid")
		return nil, err
	}
	utxo := tokenizer.Data()

	parts := strings.Split(string(utxo), ":")
	if len(parts) != 2 {
		return nil, errors.New("utxo should be of the form txid:index")
	}
	txid := parts[0]

	outputIndex, err := strconv.ParseUint(parts[1], 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid output index: %v", err)
	}

	// The Second opcode must be a canonical data push, the length of the
	// data push is bounded to 40 by the initial check on overall script
	// length.
	if !tokenizer.Next() ||
		!isCanonicalPush(tokenizer.Opcode(), tokenizer.Data()) {
		err := fmt.Errorf("invalid Anchor tx script for witnessScript")
		return nil, err
	}
	witnessScript := tokenizer.Data()

	// The third opcode must be a int64 ScriptNum.
	if !tokenizer.Next() {
		err := fmt.Errorf("invalid Anchor tx script for amount")
		return nil, err
	}
	data := tokenizer.Data()
	scriptNum, err := txscript.MakeScriptNum(data, false, len(data))
	if err != nil {
		err := fmt.Errorf("invalid Anchor tx script for parse amount")
		return nil, err
	}
	amount := scriptNum.Int32()

	// The Third opcode must be a canonical data push, The data is
	// serialized as assets
	if !tokenizer.Next() ||
		!isCanonicalPush(tokenizer.Opcode(), tokenizer.Data()) {
		err := fmt.Errorf("invalid Anchor tx script for witnessScript")
		return nil, err
	}
	assetsData := tokenizer.Data()

	txAssets := &wire.TxAssets{}
	if assetsData != nil {
		err = txAssets.Deserialize(assetsData)
		if err != nil {
			return nil, err
		}
	}

	// The witness program is valid if there are no more opcodes, and we
	// terminated without a parsing error.
	//valid := tokenizer.Done() && tokenizer.Err() == nil

	// txid := string(AnchorScript[0:32])
	// outputScript := AnchorScript[32:65]
	// amount := binary.LittleEndian.Uint64(AnchorScript[65:73])
	//extraNonce := int64(AnchorScript[73:81])

	info := &LockedTxInfo{
		TxId:          txid, // txid,
		Index:         int32(outputIndex),
		WitnessScript: witnessScript,
		Amount:        int64(amount),
		TxAssets:      txAssets,
	}
	return info, nil
}

// isCanonicalPush returns true if the opcode is either not a push instruction
// or the data associated with the push instruction uses the smallest
// instruction to do the job.  False otherwise.
//
// For example, it is possible to push a value of 1 to the stack as "OP_1",
// "OP_DATA_1 0x01", "OP_PUSHDATA1 0x01 0x01", and others, however, the first
// only takes a single byte, while the rest take more.  Only the first is
// considered canonical.
func isCanonicalPush(opcode byte, data []byte) bool {
	dataLen := len(data)
	if opcode > txscript.OP_16 {
		return true
	}

	if opcode < txscript.OP_PUSHDATA1 && opcode > txscript.OP_0 && (dataLen == 1 && data[0] <= 16) {
		return false
	}
	if opcode == txscript.OP_PUSHDATA1 && dataLen < txscript.OP_PUSHDATA1 {
		return false
	}
	if opcode == txscript.OP_PUSHDATA2 && dataLen <= 0xff {
		return false
	}
	if opcode == txscript.OP_PUSHDATA4 && dataLen <= 0xffff {
		return false
	}
	return true
}

// WitnessScriptHash generates a pay-to-witness-script-hash public key script
// paying to a version 0 witness program paying to the passed redeem script.
func WitnessScriptHash(witnessScript []byte) ([]byte, error) {
	bldr := txscript.NewScriptBuilder(
		txscript.WithScriptAllocSize(P2WSHSize),
	)

	bldr.AddOp(txscript.OP_0)
	scriptHash := sha256.Sum256(witnessScript)
	bldr.AddData(scriptHash[:])
	return bldr.Script()
}

// HexToSecp256k1PublicKey 将十六进制字符串格式的公钥转换为 secp256k1.PublicKey
func BytesToPublicKey(pubKeyBytes []byte) (*secp256k1.PublicKey, error) {
	// 检查公钥长度
	if len(pubKeyBytes) != 33 && len(pubKeyBytes) != 65 {
		return nil, fmt.Errorf("invalid public key length: %d", len(pubKeyBytes))
	}

	// 解析公钥
	pubKey, err := secp256k1.ParsePubKey(pubKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %v", err)
	}

	return pubKey, nil
}

func PublicKeyToTaprootAddress(pubKey *btcec.PublicKey) (*btcutil.AddressTaproot, error) {
	taprootPubKey := txscript.ComputeTaprootKeyNoScript(pubKey)
	return btcutil.NewAddressTaproot(schnorr.SerializePubKey(taprootPubKey), anchorManager.anchorConfig.ChainParams)
}

func checkAnchorPkScript(anchorPkScript []byte) (*LockedTxInfo, error) {
	lockedTxInfo, err := ParseAnchorScript(anchorPkScript)
	if err != nil {
		return nil, err
	}

	pkScript, err := WitnessScriptHash(lockedTxInfo.WitnessScript)
	if err != nil {
		return nil, err
	}

	addrType, addresses, _, err := txscript.ExtractPkScriptAddrs(pkScript, anchorManager.anchorConfig.ChainParams)
	if err != nil {
		return nil, err
	}
	if addrType != txscript.WitnessV0ScriptHashTy {
		return nil, fmt.Errorf("invalid addr type %d", addrType)
	}
	log.Infof("multi-signed address: %s", addresses[0].EncodeAddress()) // 多签地址，也就是L2锁定对应聪的地址

	addrType2, addresses2, _, err := txscript.ExtractPkScriptAddrs(lockedTxInfo.WitnessScript, anchorManager.anchorConfig.ChainParams)
	if err != nil {
		return nil, err
	}
	if addrType2 != txscript.MultiSigTy {
		return nil, fmt.Errorf("invalid addr type %d", addrType)
	}
	hasSuperNode := false
	for i, addr := range addresses2 {
		pubkey, err := BytesToPublicKey(addr.ScriptAddress())
		if err != nil {
			return nil, fmt.Errorf("BytesToPublicKey failed. %v", err)
		}

		p2trAddr, err := PublicKeyToTaprootAddress(pubkey)
		if err != nil {
			return nil, fmt.Errorf("PublicKeyToTaprootAddress failed. %v", err)
		}

		log.Infof("wallet %d address: %s\n", i, p2trAddr.EncodeAddress()) // 通道双方，需要检查是否存在super node的地址

		// check super node public key: pubkey
		if isSuperPublic(pubkey.SerializeCompressed()) {
			hasSuperNode = true
			break
		}
	}

	if hasSuperNode == false {
		return nil, fmt.Errorf("NO super node public key in multisig addr")
	}

	// rawTx := GetManagerInstance().l1IndexerClient.GetRawTx(txid)
	// if rawTx == "" {
	// 	// 可能还没确认，或者其他原因的延迟，需要下次再检查
	// 	return fmt.Errorf("can't find tx in mainnet: %s", txid)
	// }

	// tx, err := DecodeStringToTx(rawTx)
	// if err != nil {
	// 	return err
	// }

	// // 只检查第一个输出
	// txOut := tx.MsgTx().TxOut[0]
	// if txOut.Value != lockedTxInfo.Amount {
	// 	return fmt.Errorf("invalid value %d", lockedTxInfo.Amount)
	// }
	// if !bytes.Equal(txOut.PkScript, pkScript) {
	// 	return fmt.Errorf("invalid pkscript")
	// }

	if IsCheckLockedTx() == true {
		// 检查BTC锁定交易, 只有定义了anchorManager.anchorConfig.IndexerHost, anchorManager.anchorConfig.IndexerNet才检查
		utxoLocked := fmt.Sprintf("%s:%d", lockedTxInfo.TxId, lockedTxInfo.Index)
		lockedInfoInBTC, err := GetLockedUtxoInfo(utxoLocked)
		if err != nil {
			return nil, err
		}
		if lockedInfoInBTC.Amount != lockedTxInfo.Amount {
			return nil, fmt.Errorf("invalid value %d", lockedTxInfo.Amount)
		}
		if !bytes.Equal(lockedInfoInBTC.pkScript, pkScript) {
			return nil, fmt.Errorf("invalid pkscript")
		}

		if isSameAssets(lockedInfoInBTC.AssetInfo, lockedTxInfo.TxAssets) == false {
			return nil, fmt.Errorf("invalid assets")
		}
	}

	return lockedTxInfo, nil
}

func isSameAssets(utxoAssetInfo []*UtxoAssetInfo, txAssets *wire.TxAssets) bool {
	if utxoAssetInfo == nil && txAssets == nil {
		return true
	}

	if utxoAssetInfo == nil || txAssets == nil {
		log.Errorf("isSameAssets failed, utxoAssetInfo: %v, txAssets: %v", utxoAssetInfo, txAssets)
		return false
	}

	if len(utxoAssetInfo) != len(*txAssets) {
		log.Errorf("isSameAssets failed, utxoAssetInfo: %v, txAssets: %v", utxoAssetInfo, txAssets)
		return false
	}

	for i := 0; i < len(utxoAssetInfo); i++ {
		if utxoAssetInfo[i].Asset.Name.Protocol != (*txAssets)[i].Name.Protocol {
			log.Errorf("isSameAssets failed, utxoAssetInfo: %v, txAssets: %v", utxoAssetInfo, txAssets)
			return false
		}
		if utxoAssetInfo[i].Asset.Name.Type != (*txAssets)[i].Name.Type {
			log.Errorf("isSameAssets failed, utxoAssetInfo: %v, txAssets: %v", utxoAssetInfo, txAssets)
			return false
		}
		if utxoAssetInfo[i].Asset.Name.Ticker != (*txAssets)[i].Name.Ticker {
			log.Errorf("isSameAssets failed, utxoAssetInfo: %v, txAssets: %v", utxoAssetInfo, txAssets)
			return false
		}
		if utxoAssetInfo[i].Asset.Amount != (*txAssets)[i].Amount {
			log.Errorf("isSameAssets failed, utxoAssetInfo: %v, txAssets: %v", utxoAssetInfo, txAssets)
			return false
		}
		if utxoAssetInfo[i].Asset.BindingSat != (*txAssets)[i].BindingSat {
			log.Errorf("isSameAssets failed, utxoAssetInfo: %v, txAssets: %v", utxoAssetInfo, txAssets)
			return false
		}
	}

	return true
}

func isSuperPublic(publicKey []byte) bool {
	return true
}

// updateSuperList for sync super list from indexer
func (m *AnchorManager) updateSuperList() {
	m.superNodeList = make([][]byte, 0)

	publicKey, _ := hex.DecodeString("027e4e20121cd42053d971944ddd24d8442707062e4815d2b4090b7a62a18c411a")

	m.superNodeList = append(m.superNodeList, publicKey)
}

// syncSuperListHandler for sync super list from indexer on a timer
func (m *AnchorManager) syncSuperListHandler() {
	syncInterval := time.Second * 60
	syncTicker := time.NewTicker(syncInterval)
	defer syncTicker.Stop()

out:
	for {
		log.Debugf("Waiting next timer for syncing super node list...")
		select {
		case <-syncTicker.C:
			m.updateSuperList()
		case <-m.quit:
			break out
		}
	}

	log.Debugf("syncSuperListHandler done.")
}
