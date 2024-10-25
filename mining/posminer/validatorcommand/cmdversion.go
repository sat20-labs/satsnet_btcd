// Copyright (c) 2013-2016 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package validatorcommand

import (
	"bytes"
	"fmt"
	"io"
	"time"
)

// MaxUserAgentLen is the maximum allowed length for the user agent field in a
// version message (MsgVersion).
const MaxUserAgentLen = 256

// DefaultUserAgent for wire in the stack
const DefaultUserAgent = "/btcwire:0.5.0/"

// MsgVersion implements the Message interface and represents a bitcoin version
// message.  It is used for a peer to advertise itself as soon as an outbound
// connection is made.  The remote peer then uses this information along with
// its own to negotiate.  The remote peer must then respond with a version
// message of its own containing the negotiated values followed by a verack
// message (MsgVerAck).  This exchange must take place before any further
// communication is allowed to proceed.
type MsgVersion struct {
	// Version of the protocol the node is using.
	ValidatorVersion int32

	// Time the message was generated.  This is encoded as an int64 on the wire.
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
func (msg *MsgVersion) BtcDecode(r io.Reader, pver uint32) error {
	buf, ok := r.(*bytes.Buffer)
	if !ok {
		return fmt.Errorf("MsgVersion.BtcDecode reader is not a " +
			"*bytes.Buffer")
	}

	err := readElements(buf, &msg.ValidatorVersion,
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
func (msg *MsgVersion) BtcEncode(w io.Writer, pver uint32) error {
	err := writeElements(w, msg.ValidatorVersion,
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
func (msg *MsgVersion) Command() string {
	return CmdVersion
}

// MaxPayloadLength returns the maximum length the payload can be for the
// receiver.  This is part of the Message interface implementation.
func (msg *MsgVersion) MaxPayloadLength(pver uint32) uint32 {
	// XXX: <= 106 different

	// Protocol version 4 bytes + timestamp 8 bytes + nonce 8 bytes
	return 20
}

// NewMsgVersion returns a new bitcoin version message that conforms to the
// Message interface using the passed parameters and defaults for the remaining
// fields.
func NewMsgVersion(nonce uint64) *MsgVersion {

	// Limit the timestamp to one second precision since the protocol
	// doesn't support better.
	return &MsgVersion{
		ValidatorVersion: int32(VALIDATOR_VERION),
		Timestamp:        time.Unix(time.Now().Unix(), 0),
		Nonce:            nonce,
	}

}
