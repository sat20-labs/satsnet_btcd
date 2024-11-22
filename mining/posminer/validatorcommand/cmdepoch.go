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
	"github.com/sat20-labs/satsnet_btcd/mining/posminer/generator"
)

const (
	MaxMsgEpochLength = (20 + 256*60) * 2
)

// MsgEpoch implements the Message interface try to get all validators
// message.  It is used for a peer to get all validators by sorts from the another peer.
// The remote peer must then respond validators message.
type MsgEpoch struct {
	// Current validators count and validators for this epoch
	//ValidatorCount int32
	//Validators     []epoch.EpochItem
	CurrentEpoch *epoch.Epoch
	NextEpoch    *epoch.Epoch
}

// BtcDecode decodes r using the bitcoin protocol encoding into the receiver.
// The version message is special in that the protocol version hasn't been
// negotiated yet.  As a result, the pver field is ignored and any fields which
// are added in new versions are optional.  This also mean that r must be a
// *bytes.Buffer so the number of remaining bytes can be ascertained.
//
// This is part of the Message interface implementation.
func (msg *MsgEpoch) BtcDecode(r io.Reader, pver uint32) error {
	buf, ok := r.(*bytes.Buffer)
	if !ok {
		return fmt.Errorf("MsgEpoch.BtcDecode reader is not a " +
			"*bytes.Buffer")
	}

	// Read CurrentEpoch
	epoch, err := msg.ReadEpoch(buf)
	if err != nil {
		return err
	}

	msg.CurrentEpoch = epoch

	// Read NextEpoch
	epoch, err = msg.ReadEpoch(buf)
	if err != nil {
		return err
	}

	msg.NextEpoch = epoch

	return nil
}

func (msg *MsgEpoch) ReadEpoch(buf *bytes.Buffer) (*epoch.Epoch, error) {
	var exist uint32
	err := readElements(buf, &exist)
	if err != nil {
		return nil, err
	}

	if exist == 0 {
		return nil, nil
	}

	receivedEpoch := &epoch.Epoch{
		ItemList: make([]*epoch.EpochItem, 0),
	}
	// EpochIndex      uint32               // Epoch Index, start from 0
	// CreateHeight    int32                // 创建Epoch时当前Block的高度
	// CreateTime      time.Time            // 当前Epoch的创建时间
	// ItemList        []*EpochItem         // 当前Epoch包含的验证者列表，（已排序）， 在一个Epoch结束前不会改变
	// Generator       *generator.Generator // 当前Generator
	// CurGeneratorPos int32                // 当前Generator在ItemList中的位置

	// Read EpochIndex, CreateHeight, CreateTime
	err = readElements(buf, &receivedEpoch.EpochIndex, &receivedEpoch.CreateHeight, (*int64Time)(&receivedEpoch.CreateTime))
	if err != nil {
		return nil, err
	}

	// Read ItemList
	itemCount := uint32(0)
	err = readElements(buf, &itemCount)
	if err != nil {
		return nil, err
	}

	for i := 0; i < int(itemCount); i++ {
		var epochItem epoch.EpochItem
		err = readElements(buf, &epochItem.ValidatorId)
		if err != nil {
			return nil, err
		}
		err = readElements(buf, &epochItem.Host)
		if err != nil {
			return nil, err
		}
		err = readElements(buf, &epochItem.PublicKey)
		if err != nil {
			return nil, err
		}
		err = readElements(buf, &epochItem.Index)
		if err != nil {
			return nil, err
		}
		receivedEpoch.ItemList = append(receivedEpoch.ItemList, &epochItem)
	}

	// read Generator
	err = readElements(buf, &exist)
	if err != nil {
		return nil, err
	}
	if exist != 0 {
		// Generator exist, read Generator
		var generator generator.Generator
		err := readElements(buf,
			&generator.GeneratorId,
			&generator.Timestamp,
			&generator.Height,
			&generator.Token)
		if err != nil {
			return nil, err
		}
		receivedEpoch.Generator = &generator
	}

	// read CurGeneratorPos
	err = readElements(buf, &receivedEpoch.CurGeneratorPos)
	if err != nil {
		return nil, err
	}

	return receivedEpoch, nil

}

// BtcEncode encodes the receiver to w using the bitcoin protocol encoding.
// This is part of the Message interface implementation.
func (msg *MsgEpoch) BtcEncode(w io.Writer, pver uint32) error {
	// err := writeElements(w, msg.ValidatorCount)
	// if err != nil {
	// 	return err
	// }

	// for _, validator := range msg.Validators {
	// 	err = writeElements(w, validator.ValidatorId)
	// 	if err != nil {
	// 		return err
	// 	}
	// 	err = writeElements(w, validator.Host)
	// 	if err != nil {
	// 		return err
	// 	}

	// 	err = writeElements(w, validator.PublicKey)
	// 	if err != nil {
	// 		return err
	// 	}
	// 	err = writeElements(w, validator.Index)
	// 	if err != nil {
	// 		return err
	// 	}

	// }
	err := msg.WriteEpoch(w, msg.CurrentEpoch)
	if err != nil {
		return err
	}

	err = msg.WriteEpoch(w, msg.NextEpoch)
	if err != nil {
		return err
	}
	return nil
}

func (msg *MsgEpoch) WriteEpoch(w io.Writer, epoch *epoch.Epoch) error {

	var exist uint32
	if epoch == nil {
		exist = 0
	} else {
		exist = 1
	}
	err := writeElements(w, exist)
	if err != nil {
		return err
	}

	if exist == 0 {
		// Epoch not exist
		return nil
	}

	// EpochIndex      uint32               // Epoch Index, start from 0
	// CreateHeight    int32                // 创建Epoch时当前Block的高度
	// CreateTime      time.Time            // 当前Epoch的创建时间
	// ItemList        []*EpochItem         // 当前Epoch包含的验证者列表，（已排序）， 在一个Epoch结束前不会改变
	// Generator       *generator.Generator // 当前Generator
	// CurGeneratorPos int32                // 当前Generator在ItemList中的位置

	// Write EpochIndex, CreateHeight, CreateTime
	err = writeElements(w, epoch.EpochIndex, epoch.CreateHeight, epoch.CreateTime.Unix())
	if err != nil {
		return err
	}

	// Write ItemList
	itemCount := uint32(len(epoch.ItemList))
	err = writeElements(w, &itemCount)
	if err != nil {
		return err
	}

	for i := 0; i < int(itemCount); i++ {
		epochItem := epoch.ItemList[i]
		err = writeElements(w, epochItem.ValidatorId)
		if err != nil {
			return err
		}
		err = writeElements(w, epochItem.Host)
		if err != nil {
			return err
		}
		err = writeElements(w, epochItem.PublicKey)
		if err != nil {
			return err
		}
		err = writeElements(w, epochItem.Index)
		if err != nil {
			return err
		}
	}

	// write Generator
	if epoch.Generator == nil {
		exist = 0
	} else {
		exist = 1
	}
	err = writeElements(w, &exist)
	if err != nil {
		return err
	}
	if exist != 0 {
		// Generator exist, write Generator
		err := writeElements(w,
			epoch.Generator.GeneratorId,
			epoch.Generator.Timestamp,
			epoch.Generator.Height,
			epoch.Generator.Token)
		if err != nil {
			return err
		}
	}

	// write CurGeneratorPos
	err = writeElements(w, epoch.CurGeneratorPos)
	if err != nil {
		return err
	}

	return nil

}

// Command returns the protocol command string for the message.  This is part
// of the Message interface implementation.
func (msg *MsgEpoch) Command() string {
	return CmdEpoch
}

// MaxPayloadLength returns the maximum length the payload can be for the
// receiver.  This is part of the Message interface implementation.
func (msg *MsgEpoch) MaxPayloadLength(pver uint32) uint32 {
	//  MaxMsgEpochLength bytes
	//return MaxMsgEpochLength
	return 12 // For test
}

func (msg *MsgEpoch) LogCommandInfo(log btclog.Logger) {
	log.Debugf("Command MsgEpoch:")
	// log.Debugf("Validator Count: %d", msg.ValidatorCount)
	// for index, validator := range msg.Validators {
	// 	log.Debugf("——————————————————————————————————")
	// 	log.Debugf("No: %d", index)
	// 	log.Debugf("Validator Id: %d", validator.ValidatorId)
	// 	log.Debugf("Validator Host: %s", validator.Host)
	// 	log.Debugf("Validator PublicKey: %x", validator.PublicKey)
	// 	log.Debugf("Validator Index: %d", validator.Index)
	// 	log.Debugf("")
	// }
	showEpoch(log, "MsgEpoch: CurrentEpoch", msg.CurrentEpoch)
	showEpoch(log, "MsgEpoch: NexEpoch", msg.NextEpoch)
	log.Debugf("——————————————————————————————————")
}

// NewMsgEpoch returns a new bitcoin version message that conforms to the
// Message interface using the passed parameters and defaults for the remaining
// fields.
func NewMsgEpoch(currentEpoch, nextEpoch *epoch.Epoch) *MsgEpoch {

	// Limit the timestamp to one second precision since the protocol
	// doesn't support better.
	validatorsMsg := &MsgEpoch{
		CurrentEpoch: currentEpoch,
		NextEpoch:    nextEpoch,
	}
	// validatorsMsg.ValidatorCount = int32(len(validatorList))
	// validatorsMsg.Validators = make([]epoch.EpochItem, 0)

	// for _, validator := range validatorList {
	// 	validatorItem := epoch.EpochItem{
	// 		ValidatorId: validator.ValidatorId,
	// 		Host:        validator.Host,
	// 		PublicKey:   validator.PublicKey,
	// 		Index:       validator.Index,
	// 	}
	// 	validatorsMsg.Validators = append(validatorsMsg.Validators, validatorItem)
	// }

	return validatorsMsg

}

func showEpoch(log btclog.Logger, title string, epoch *epoch.Epoch) {
	log.Debugf("********************************* %s Summary ********************************", title)
	if epoch == nil {
		log.Debugf("Invalid epoch")
	} else {
		log.Debugf("EpochIndex: %d", epoch.EpochIndex)
		log.Debugf("CreateHeight: %d", epoch.CreateHeight)
		log.Debugf("CreateTime: %s", epoch.CreateTime.Format("2006-01-02 15:04:05"))
		log.Debugf("EpochIndex: %d", epoch.EpochIndex)
		log.Debugf("Validator Count in Epoch: %d", len(epoch.ItemList))
		for _, epochItem := range epoch.ItemList {
			log.Debugf("validator ID: %d", epochItem.ValidatorId)
			log.Debugf("validator Public: %x", epochItem.PublicKey[:])
			log.Debugf("validator Host: %s", epochItem.Host)
			log.Debugf("validator Index: %d", epochItem.Index)
			log.Debugf("------------------------------------------------")
		}

		log.Debugf("Epoch generator: ")

		generator := epoch.Generator
		if generator == nil {
			log.Debugf("	No generator")
		} else {
			log.Debugf("	Generator ID: %d", generator.GeneratorId)
			if generator.Validatorinfo != nil {
				log.Debugf("	Generator Public: %x", generator.Validatorinfo.PublicKey[:])
				log.Debugf("	Generator Host: %s", generator.Validatorinfo.Host)
				log.Debugf("	Generator ConnectTime: %s", generator.Validatorinfo.CreateTime.Format("2006-01-02 15:04:05"))
			} else {
				log.Debugf("	Invalid validator info for Generator.")
			}
			log.Debugf("	Generator TimeStamp: %s", time.Unix(generator.Timestamp, 0).Format("2006-01-02 15:04:05"))
			log.Debugf("	Generator Token: %s", generator.Token)
			log.Debugf("	Generator Block Height: %d", generator.Height)
		}

		//Generator     *generator.Generator // 当前Generator
		log.Debugf("CurGeneratorPos: %d", epoch.CurGeneratorPos)

	}
	log.Debugf("*********************************        End        ********************************")
}
