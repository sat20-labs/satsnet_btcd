// Copyright (c) 2013-2016 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package validatorcommand

import (
	"bytes"
	"fmt"
	"io"

	"github.com/btcsuite/btclog"
)

const (
	MaxVoteCount = 1024
)

type VoteItem struct {
	ValidatorId uint64 // The validator id of Vote
	Pass        uint32 // Pass or fail, 0 fail -- against this vote, 1 pass  -- agree this vote
	GeneratorId uint64 // The generator id of Vote result for vote a new generator, if the vote is a new epoch, the generator id is 0
	Token       string
}

type VoteResult struct {
	ValidatorId uint64     // The validator id of request vote
	VoteType    uint32     // Vote type
	VoteId      uint32     // The vote id
	EpochIndex  uint32     // The epoch index
	VoteCount   uint32     // The vote count
	Pass        uint32     // Pass or fail
	VoteList    []VoteItem // The vote list
}

// MsgVoteResult implements the Message interface and get current generator
// message. The remote peer must then respond with current generator
// message of its own containing the negotiated values followed by a verack
// message (MsgVoteResult).
type MsgVoteResult struct {
	VoteResultInfo VoteResult
}

// BtcDecode decodes r using the bitcoin protocol encoding into the receiver.
// The version message is special in that the protocol version hasn't been
// negotiated yet.  As a result, the pver field is ignored and any fields which
// are added in new versions are optional.  This also mean that r must be a
// *bytes.Buffer so the number of remaining bytes can be ascertained.
//
// This is part of the Message interface implementation.
func (msg *MsgVoteResult) BtcDecode(r io.Reader, pver uint32) error {
	buf, ok := r.(*bytes.Buffer)
	if !ok {
		return fmt.Errorf("MsgVoteResult.BtcDecode reader is not a " +
			"*bytes.Buffer")
	}

	err := readElements(buf,
		&msg.VoteResultInfo.ValidatorId,
		&msg.VoteResultInfo.VoteType,
		&msg.VoteResultInfo.VoteId,
		&msg.VoteResultInfo.EpochIndex,
		&msg.VoteResultInfo.VoteCount,
		&msg.VoteResultInfo.Pass)
	if err != nil {
		return err
	}

	for i := 0; i < int(msg.VoteResultInfo.VoteCount); i++ {
		var voteItem VoteItem
		err = readElements(buf, &voteItem.ValidatorId, &voteItem.Pass, &voteItem.GeneratorId, &voteItem.Token)
		if err != nil {

		}
		msg.VoteResultInfo.VoteList = append(msg.VoteResultInfo.VoteList, voteItem)
	}
	return nil
}

// BtcEncode encodes the receiver to w using the bitcoin protocol encoding.
// This is part of the Message interface implementation.
func (msg *MsgVoteResult) BtcEncode(w io.Writer, pver uint32) error {
	if len(msg.VoteResultInfo.VoteList) != int(msg.VoteResultInfo.VoteCount) {
		return fmt.Errorf("msg.VoteResultInfo.VoteList len is not equal to msg.VoteResultInfo.VoteCount")
	}

	err := writeElements(w,
		msg.VoteResultInfo.ValidatorId,
		msg.VoteResultInfo.VoteType,
		msg.VoteResultInfo.VoteId,
		msg.VoteResultInfo.EpochIndex,
		msg.VoteResultInfo.VoteCount,
		msg.VoteResultInfo.Pass)
	if err != nil {
		return err
	}

	for i := 0; i < int(msg.VoteResultInfo.VoteCount); i++ {
		voteItem := msg.VoteResultInfo.VoteList[i]
		err = writeElements(w,
			voteItem.ValidatorId,
			voteItem.Pass,
			voteItem.GeneratorId,
			voteItem.Token)
		if err != nil {
			return err
		}
	}
	return nil
}

// Command returns the protocol command string for the message.  This is part
// of the Message interface implementation.
func (msg *MsgVoteResult) Command() string {
	return CmdVoteResp
}

// MaxPayloadLength returns the maximum length the payload can be for the
// receiver.  This is part of the Message interface implementation.
func (msg *MsgVoteResult) MaxPayloadLength(pver uint32) uint32 {
	// ValidatorId 8 bytes + VoteType 4 bytes + VoteId 4 bytes  + EpochIndex 4 bytes + VoteCount 4 bytes + Pass 4 bytes +
	// (ValidatorId 8 bytes + Pass 4 bytes + GeneratorId 8 bytes + Token 20 bytes) * MaxVoteCount
	return 36 + (20+MaxVoteTokenSize)*MaxVoteCount
}

func (msg *MsgVoteResult) LogCommandInfo(log btclog.Logger) {
	log.Debugf("Command MsgVoteResult:")
	log.Debugf("ValidatorId: %d", msg.VoteResultInfo.ValidatorId)
	voteType := "Unknown type"
	if msg.VoteResultInfo.VoteType == VoteType_NewGenerator {
		voteType = "New Generator"
	} else if msg.VoteResultInfo.VoteType == VoteType_NewEpoch {
		voteType = "New Epoch"
	}

	log.Debugf("VoteType: %s", voteType)
	log.Debugf("VoteId: %d", msg.VoteResultInfo.VoteId)
	log.Debugf("EpochIndex: %d", msg.VoteResultInfo.EpochIndex)
	log.Debugf("VoteCount: %d", msg.VoteResultInfo.VoteCount)
	log.Debugf("Pass: %d", msg.VoteResultInfo.Pass)
	for index, voteItem := range msg.VoteResultInfo.VoteList {
		log.Debugf("------------------------------------")
		log.Debugf("	Vote Index: %d", index)
		log.Debugf("	GeneratorId: %d", voteItem.GeneratorId)
		log.Debugf("	Pass: %d", voteItem.Pass)
		log.Debugf("	Token: %s", voteItem.Token)
	}
}

// NewMsgVoteResult returns a new bitcoin version message that conforms to the
// Message interface using the passed parameters and defaults for the remaining
// fields.
func NewMsgVoteResult(voteResult *VoteResult) *MsgVoteResult {

	// Limit the timestamp to one second precision since the protocol
	// doesn't support better.
	return &MsgVoteResult{
		VoteResultInfo: *voteResult,
	}

}
