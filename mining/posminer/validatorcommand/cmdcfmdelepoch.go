// Copyright (c) 2013-2016 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package validatorcommand

import (
	"bytes"
	"fmt"
	"io"

	"github.com/sat20-labs/satsnet_btcd/mining/posminer/epoch"
	"github.com/sat20-labs/satsnet_btcd/mining/posminer/utils"
)

// MsgConfirmDelEpoch implements the Message interface and get current generator
// message. The remote peer must then respond with current generator
// message of its own containing the negotiated values followed by a verack
// message (MsgGenerator).
type MsgConfirmDelEpoch struct {
	DelEpochMember *epoch.DelEpochMember
}

// BtcDecode decodes r using the bitcoin protocol encoding into the receiver.
// The version message is special in that the protocol version hasn't been
// negotiated yet.  As a result, the pver field is ignored and any fields which
// are added in new versions are optional.  This also mean that r must be a
// *bytes.Buffer so the number of remaining bytes can be ascertained.
//
// This is part of the Message interface implementation.
func (msg *MsgConfirmDelEpoch) BtcDecode(r io.Reader, pver uint32) error {
	buf, ok := r.(*bytes.Buffer)
	if !ok {
		return fmt.Errorf("MsgConfirmDelEpoch.BtcDecode reader is not a " +
			"*bytes.Buffer")
	}

	msg.DelEpochMember = &epoch.DelEpochMember{}

	err := utils.ReadElements(buf, &msg.DelEpochMember.ValidatorId, &msg.DelEpochMember.DelValidatorId, &msg.DelEpochMember.DelCode, &msg.DelEpochMember.EpochIndex, &msg.DelEpochMember.Result, &msg.DelEpochMember.Token)
	if err != nil {
		return err
	}

	return nil
}

// BtcEncode encodes the receiver to w using the bitcoin protocol encoding.
// This is part of the Message interface implementation.
func (msg *MsgConfirmDelEpoch) BtcEncode(w io.Writer, pver uint32) error {
	err := utils.WriteElements(w, msg.DelEpochMember.ValidatorId, msg.DelEpochMember.DelValidatorId, msg.DelEpochMember.DelCode, msg.DelEpochMember.EpochIndex, msg.DelEpochMember.Result, msg.DelEpochMember.Token)
	if err != nil {
		return err
	}

	return nil
}

// Command returns the protocol command string for the message.  This is part
// of the Message interface implementation.
func (msg *MsgConfirmDelEpoch) Command() string {
	return CmdConfirmDelEpoch
}

// MaxPayloadLength returns the maximum length the payload can be for the
// receiver.  This is part of the Message interface implementation.
func (msg *MsgConfirmDelEpoch) MaxPayloadLength(pver uint32) uint32 {
	// validatorId 8 bytes + DelValidatorId 8 bytes + DelCode 4 bytes + EpochIndex 8 bytes + Result 4 bytes + + token 256 bytes
	return 32 + 256
}

func (msg *MsgConfirmDelEpoch) LogCommandInfo() {
	log.Debugf("Command MsgConfirmDelEpoch:")
	log.Debugf("ValidatorId: %d", msg.DelEpochMember.ValidatorId)
	log.Debugf("DelValidatorId: %d", msg.DelEpochMember.DelValidatorId)
	log.Debugf("DelCode: %d", msg.DelEpochMember.DelCode)
	log.Debugf("EpochIndex: %d", msg.DelEpochMember.EpochIndex)
	log.Debugf("Result: %d", msg.DelEpochMember.Result)
	log.Debugf("Token: %s", msg.DelEpochMember.Token)
}

// NewMsgConfirmDelEpoch returns a new bitcoin version message that conforms to the
// Message interface using the passed parameters and defaults for the remaining
// fields.
func NewMsgConfirmDelEpoch(delEpochMember *epoch.DelEpochMember) *MsgConfirmDelEpoch {

	// Limit the timestamp to one second precision since the protocol
	// doesn't support better.
	return &MsgConfirmDelEpoch{
		DelEpochMember: delEpochMember,
	}
}
