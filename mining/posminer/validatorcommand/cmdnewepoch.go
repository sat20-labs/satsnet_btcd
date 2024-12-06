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
	"github.com/sat20-labs/satsnet_btcd/mining/posminer/epoch"
	"github.com/sat20-labs/satsnet_btcd/mining/posminer/utils"
)

const (
	MaxMsgNewEpochLength = 256 * 60 // 69 len of validatorinfo.ValidatorInfo to be sent
)

// MsgNewEpoch implements the Message interface try to get all validators
// message.  It is used for a peer to get all validators by sorts from the another peer.
// The remote peer must then respond validators message.
type MsgNewEpoch struct {
	// ValidatorId of the new epoch supported
	ValidatorId uint64 // Validator Id

	// epoch info for the new epoch
	EpochIndex   uint32            // Epoch Index, start from 0
	CreateHeight int32             // 创建Epoch时当前Block的高度
	CreateTime   time.Time         // 当前Epoch的创建时间
	ItemList     []epoch.EpochItem // 当前Epoch包含的验证者列表，（已排序）， 在一个Epoch结束前不会改变
}

// BtcDecode decodes r using the bitcoin protocol encoding into the receiver.
// The version message is special in that the protocol version hasn't been
// negotiated yet.  As a result, the pver field is ignored and any fields which
// are added in new versions are optional.  This also mean that r must be a
// *bytes.Buffer so the number of remaining bytes can be ascertained.
//
// This is part of the Message interface implementation.
func (msg *MsgNewEpoch) BtcDecode(r io.Reader, pver uint32) error {
	buf, ok := r.(*bytes.Buffer)
	if !ok {
		return fmt.Errorf("MsgNewEpoch.BtcDecode reader is not a " +
			"*bytes.Buffer")
	}
	err := utils.ReadElements(buf, &msg.ValidatorId)
	if err != nil {
		return err
	}

	err = utils.ReadElements(buf, &msg.EpochIndex, &msg.CreateHeight, (*utils.Int64Time)(&msg.CreateTime))
	if err != nil {
		return err
	}
	validatorCount := uint32(0)
	err = utils.ReadElements(buf, &validatorCount)
	if err != nil {
		return err
	}
	for i := 0; i < int(validatorCount); i++ {
		epochItem := epoch.EpochItem{}
		err = utils.ReadElements(buf, &epochItem.ValidatorId)
		if err != nil {
			return err
		}
		err = utils.ReadElements(buf, &epochItem.Host)
		if err != nil {
			return err
		}
		err = utils.ReadElements(buf, &epochItem.PublicKey)
		if err != nil {
			return err
		}
		err = utils.ReadElements(buf, &epochItem.Index)
		if err != nil {
			return err
		}
		msg.ItemList = append(msg.ItemList, epochItem)
	}

	return nil
}

// BtcEncode encodes the receiver to w using the bitcoin protocol encoding.
// This is part of the Message interface implementation.
func (msg *MsgNewEpoch) BtcEncode(w io.Writer, pver uint32) error {
	err := utils.WriteElements(w, msg.ValidatorId)
	if err != nil {
		return err
	}
	err = utils.WriteElements(w, msg.EpochIndex, msg.CreateHeight, msg.CreateTime.Unix())
	if err != nil {
		return err
	}

	validatorCount := uint32(len(msg.ItemList))
	err = utils.WriteElements(w, validatorCount)
	if err != nil {
		return err
	}

	for _, validator := range msg.ItemList {
		err = utils.WriteElements(w, validator.ValidatorId)
		if err != nil {
			return err
		}
		err = utils.WriteElements(w, validator.Host)
		if err != nil {
			return err
		}

		err = utils.WriteElements(w, validator.PublicKey)
		if err != nil {
			return err
		}
		err = utils.WriteElements(w, validator.Index)
		if err != nil {
			return err
		}

	}

	return nil
}

// Command returns the protocol command string for the message.  This is part
// of the Message interface implementation.
func (msg *MsgNewEpoch) Command() string {
	return CmdNewEpoch
}

// MaxPayloadLength returns the maximum length the payload can be for the
// receiver.  This is part of the Message interface implementation.
func (msg *MsgNewEpoch) MaxPayloadLength(pver uint32) uint32 {
	//  ValidatorId 8 bytes + EpochIndex 4 bytes + CreateHeight 4 bytes + CreateTime 8 bytes + ValidatorCount 4 bytes
	//  Max validator count is 256 (Max Epoch Size) , EpochItem is 60 bytes
	return 28 + 256*60 // 69 len of validatorinfo.ValidatorInfo to be sent

}

func (msg *MsgNewEpoch) LogCommandInfo(log btclog.Logger) {
	log.Debugf("Command MsgNewEpoch:")
	log.Debugf("Validator Id: %d", msg.ValidatorId)

	log.Debugf("Epoch Index: %d", msg.EpochIndex)
	log.Debugf("Create Height: %d", msg.CreateHeight)
	log.Debugf("Create Time: %s", msg.CreateTime.Format(time.DateTime))

	for index, validator := range msg.ItemList {
		log.Debugf("——————————————————————————————————")
		log.Debugf("No: %d", index)
		log.Debugf("Validator Id: %d", validator.ValidatorId)
		log.Debugf("Validator Host: %s", validator.Host)
		log.Debugf("Validator PublicKey: %x", validator.PublicKey)
		log.Debugf("Validator Index: %d", validator.Index)
		log.Debugf("")
	}
	log.Debugf("——————————————————————————————————")
}

// NewMsgNewEpoch returns a new bitcoin version message that conforms to the
// Message interface using the passed parameters and defaults for the remaining
// fields.
func NewMsgNewEpoch(validatorId uint64, newEpoch *epoch.Epoch) *MsgNewEpoch {

	// Limit the timestamp to one second precision since the protocol
	// doesn't support better.
	newEpochMsg := &MsgNewEpoch{
		ValidatorId:  validatorId,
		EpochIndex:   newEpoch.EpochIndex,
		CreateHeight: newEpoch.CreateHeight,
		CreateTime:   newEpoch.CreateTime,
		ItemList:     make([]epoch.EpochItem, 0),
	}

	for _, epochItem := range newEpoch.ItemList {
		validatorItem := epoch.EpochItem{
			ValidatorId: epochItem.ValidatorId,
			Host:        epochItem.Host,
			PublicKey:   epochItem.PublicKey,
			Index:       epochItem.Index,
		}
		newEpochMsg.ItemList = append(newEpochMsg.ItemList, validatorItem)
	}

	return newEpochMsg

}
