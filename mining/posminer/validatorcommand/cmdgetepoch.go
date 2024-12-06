// Copyright (c) 2013-2016 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package validatorcommand

import (
	"bytes"
	"fmt"
	"io"

	"github.com/btcsuite/btclog"
	"github.com/sat20-labs/satsnet_btcd/mining/posminer/utils"
)

// MsgGetEpoch implements the Message interface and get current generator
// message. The remote peer must then respond with current generator
// message of its own containing the negotiated values followed by a verack
// message (MsgGenerator).
type MsgGetEpoch struct {
	// Request validator id
	ValidatorId uint64
}

// BtcDecode decodes r using the bitcoin protocol encoding into the receiver.
// The version message is special in that the protocol version hasn't been
// negotiated yet.  As a result, the pver field is ignored and any fields which
// are added in new versions are optional.  This also mean that r must be a
// *bytes.Buffer so the number of remaining bytes can be ascertained.
//
// This is part of the Message interface implementation.
func (msg *MsgGetEpoch) BtcDecode(r io.Reader, pver uint32) error {
	buf, ok := r.(*bytes.Buffer)
	if !ok {
		return fmt.Errorf("MsgGetEpoch.BtcDecode reader is not a " +
			"*bytes.Buffer")
	}

	err := utils.ReadElements(buf, &msg.ValidatorId)
	if err != nil {
		return err
	}

	return nil
}

// BtcEncode encodes the receiver to w using the bitcoin protocol encoding.
// This is part of the Message interface implementation.
func (msg *MsgGetEpoch) BtcEncode(w io.Writer, pver uint32) error {
	err := utils.WriteElements(w, msg.ValidatorId)
	if err != nil {
		return err
	}

	return nil
}

// Command returns the protocol command string for the message.  This is part
// of the Message interface implementation.
func (msg *MsgGetEpoch) Command() string {
	return CmdGetEpoch
}

// MaxPayloadLength returns the maximum length the payload can be for the
// receiver.  This is part of the Message interface implementation.
func (msg *MsgGetEpoch) MaxPayloadLength(pver uint32) uint32 {
	// validatorId 8 bytes
	return 8
}

func (msg *MsgGetEpoch) LogCommandInfo(log btclog.Logger) {
	log.Debugf("Command MsgGetEpoch:")
	log.Debugf("ValidatorId: %d", msg.ValidatorId)
}

// NewMsgGetEpoch returns a new bitcoin version message that conforms to the
// Message interface using the passed parameters and defaults for the remaining
// fields.
func NewMsgGetEpoch(validatorId uint64) *MsgGetEpoch {

	// Limit the timestamp to one second precision since the protocol
	// doesn't support better.
	return &MsgGetEpoch{
		ValidatorId: validatorId,
	}

}
