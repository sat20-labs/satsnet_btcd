// Copyright (c) 2013-2015 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package validatorcommand

import (
	"bytes"
	"fmt"
	"io"
	"time"

	"github.com/btcsuite/btclog"
)

// MsgPeerInfo defines a bitcoin verack message which is used for a peer to
// acknowledge a version message (MsgVersion) after it has used the information
// to negotiate parameters.  It implements the Message interface.
//
// This message has no payload.
type MsgPeerInfo struct {
	// Version of the protocol the node is using.
	ProtocolVersion int32

	// Current validator id
	ValidatorId uint64

	// Time the validator connected to Validator network.  This is encoded as an int64 on the wire.
	Timestamp time.Time

	// Unique value associated with message that is used to detect self
	// connections.
	Nonce uint64
}

// BtcDecode decodes r using the bitcoin protocol encoding into the receiver.
// This is part of the Message interface implementation.
func (msg *MsgPeerInfo) BtcDecode(r io.Reader, pver uint32) error {
	buf, ok := r.(*bytes.Buffer)
	if !ok {
		return fmt.Errorf("MsgGetInfo.BtcDecode reader is not a " +
			"*bytes.Buffer")
	}

	err := readElements(buf, &msg.ProtocolVersion, &msg.ValidatorId,
		(*int64Time)(&msg.Timestamp))
	if err != nil {
		return err
	}

	if buf.Len() > 0 {
		err = readElement(buf, &msg.Nonce)
		if err != nil {
			return err
		}
	}
	return nil
}

// BtcEncode encodes the receiver to w using the bitcoin protocol encoding.
// This is part of the Message interface implementation.
func (msg *MsgPeerInfo) BtcEncode(w io.Writer, pver uint32) error {
	err := writeElements(w, msg.ProtocolVersion, msg.ValidatorId,
		msg.Timestamp.Unix())
	if err != nil {
		return err
	}

	err = writeElement(w, msg.Nonce)
	if err != nil {
		return err
	}

	return nil
}

// Command returns the protocol command string for the message.  This is part
// of the Message interface implementation.
func (msg *MsgPeerInfo) Command() string {
	return CmdPeerInfo
}

// MaxPayloadLength returns the maximum length the payload can be for the
// receiver.  This is part of the Message interface implementation.
func (msg *MsgPeerInfo) MaxPayloadLength(pver uint32) uint32 {
	return 28
}

func (msg *MsgPeerInfo) LogCommandInfo(log btclog.Logger) {
	log.Debugf("Command MsgPeerInfo:")
	log.Debugf("ProtocolVersion: %d", msg.ProtocolVersion)
	log.Debugf("ValidatorId: %d", msg.ValidatorId)
	log.Debugf("Timestamp: %s", msg.Timestamp.Format(time.DateTime))
	log.Debugf("Nonce: %d", msg.Nonce)
}

// NewMsgPeerInfo returns a new bitcoin verack message that conforms to the
// Message interface.
func NewMsgPeerInfo(validatorId uint64, nonce uint64) *MsgPeerInfo {

	// Limit the timestamp to one second precision since the protocol
	// doesn't support better.
	return &MsgPeerInfo{
		ProtocolVersion: int32(VALIDATOR_VERION),
		ValidatorId:     validatorId,
		Timestamp:       time.Unix(time.Now().Unix(), 0),
		Nonce:           nonce,
	}

}
