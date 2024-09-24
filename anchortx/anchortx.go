// Copyright (c) 2024 The sats20 developers

package anchortx

import (
	"fmt"

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
)

// txscript.NewScriptBuilder().AddData(txid).AddData(pkScript).
// AddInt64(int64(amount)).AddInt64(int64(extraNonce)).Script()
type LockedTxInfo struct {
	TxId     string // the txid with locked in lnd
	PkScript []byte // pkScript for locked in lnd
	Amount   int64  // the amount with locked in lnd
}

func CheckAnchorTxValid(tx *wire.MsgTx) error {

	txInfo, err := GetLockedTxInfo(tx)
	if err != nil {
		return err
	}
	fmt.Printf("txInfo: %v\n", txInfo)

	// Check the Anchor tx has completed, all the assets is locked in lnd will be mapped to sats net only one times

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
	lenAnchorScript := len(AnchorScript)
	if lenAnchorScript < MIN_LEN_ANCHORTX_SCRIPT || lenAnchorScript > MAX_LEN_ANCHORTX_SCRIPT {
		err := fmt.Errorf("invalid Anchor tx script: %s", tx.TxHash().String())
		return nil, err
	}

	const scriptVersion = 0
	tokenizer := txscript.MakeScriptTokenizer(scriptVersion, AnchorScript)

	// The First opcode must be a canonical data push, the length of the
	// data push is bounded to 40 by the initial check on overall script
	// length.
	if !tokenizer.Next() ||
		!isCanonicalPush(tokenizer.Opcode(), tokenizer.Data()) {
		err := fmt.Errorf("invalid Anchor tx script: %s", tx.TxHash().String())
		return nil, err
	}
	txid := tokenizer.Data()

	// The Second opcode must be a canonical data push, the length of the
	// data push is bounded to 40 by the initial check on overall script
	// length.
	if !tokenizer.Next() ||
		!isCanonicalPush(tokenizer.Opcode(), tokenizer.Data()) {
		err := fmt.Errorf("invalid Anchor tx script: %s", tx.TxHash().String())
		return nil, err
	}
	pkScript := tokenizer.Data()

	// The third opcode must be a int64 ScriptNum.
	if !tokenizer.Next() {
		err := fmt.Errorf("invalid Anchor tx script: %s", tx.TxHash().String())
		return nil, err
	}
	data := tokenizer.Data()
	scriptNum, err := txscript.MakeScriptNum(data, false, len(data))
	if err != nil {
		err := fmt.Errorf("invalid Anchor tx script: %s", tx.TxHash().String())
		return nil, err
	}
	amount := scriptNum.Int32()
	// The witness program is valid if there are no more opcodes, and we
	// terminated without a parsing error.
	//valid := tokenizer.Done() && tokenizer.Err() == nil

	// txid := string(AnchorScript[0:32])
	// outputScript := AnchorScript[32:65]
	// amount := binary.LittleEndian.Uint64(AnchorScript[65:73])
	//extraNonce := int64(AnchorScript[73:81])

	info := &LockedTxInfo{
		TxId:     string(txid), // txid,
		PkScript: pkScript,
		Amount:   int64(amount),
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
