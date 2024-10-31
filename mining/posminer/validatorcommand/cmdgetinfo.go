// Copyright (c) 2013-2016 The btcsuite developers
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

// MaxUserAgentLen is the maximum allowed length for the user agent field in a
// version message (MsgGetInfo).
const MaxUserAgentLen = 256

// DefaultUserAgent for wire in the stack
const DefaultUserAgent = "/btcwire:0.5.0/"

// MsgGetInfo implements the Message interface and represents a bitcoin version
// message.  It is used for a peer to advertise itself as soon as an outbound
// connection is made.  The remote peer then uses this information along with
// its own to negotiate.  The remote peer must then respond with a version
// message of its own containing the negotiated values followed by a verack
// message (MsgVerAck).  This exchange must take place before any further
// communication is allowed to proceed.
type MsgGetInfo struct {
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
// The version message is special in that the protocol version hasn't been
// negotiated yet.  As a result, the pver field is ignored and any fields which
// are added in new versions are optional.  This also mean that r must be a
// *bytes.Buffer so the number of remaining bytes can be ascertained.
//
// This is part of the Message interface implementation.
func (msg *MsgGetInfo) BtcDecode(r io.Reader, pver uint32) error {
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
func (msg *MsgGetInfo) BtcEncode(w io.Writer, pver uint32) error {
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
func (msg *MsgGetInfo) Command() string {
	return CmdGetInfo
}

// MaxPayloadLength returns the maximum length the payload can be for the
// receiver.  This is part of the Message interface implementation.
func (msg *MsgGetInfo) MaxPayloadLength(pver uint32) uint32 {
	// XXX: <= 106 different

	// Protocol version 4 bytes + validatorId 8 bytes  +timestamp 8 bytes + nonce 8 bytes
	return 28
}

func (msg *MsgGetInfo) LogCommandInfo(log btclog.Logger) {
	log.Debugf("Command MsgGetInfo:")
	log.Debugf("ProtocolVersion: %d", msg.ProtocolVersion)
	log.Debugf("ValidatorId: %d", msg.ValidatorId)
	log.Debugf("Timestamp: %s", msg.Timestamp.Format(time.DateTime))
	log.Debugf("Nonce: %d", msg.Nonce)
}

// NewMsgGetInfo returns a new bitcoin version message that conforms to the
// Message interface using the passed parameters and defaults for the remaining
// fields.
func NewMsgGetInfo(validatorId uint64, nonce uint64) *MsgGetInfo {

	// Limit the timestamp to one second precision since the protocol
	// doesn't support better.
	return &MsgGetInfo{
		ProtocolVersion: int32(VALIDATOR_VERION),
		ValidatorId:     validatorId,
		Timestamp:       time.Unix(time.Now().Unix(), 0),
		Nonce:           nonce,
	}

}
