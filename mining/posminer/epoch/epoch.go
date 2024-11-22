package epoch

import (
	"fmt"
	"time"

	"github.com/sat20-labs/satsnet_btcd/btcec"
	"github.com/sat20-labs/satsnet_btcd/mining/posminer/generator"
	"github.com/sat20-labs/satsnet_btcd/mining/posminer/validatorinfo"
)

type EpochItem struct {
	ValidatorId uint64
	Host        string
	PublicKey   [btcec.PubKeyBytesLenCompressed]byte
	Index       uint32 // 第一次生成是确定的，后续不会改变， 当Epoch的Validators有改变（下线）时，保证EpochItem的Index不发生改变
}

type Epoch struct {
	EpochIndex      uint32               // Epoch Index, start from 0
	CreateHeight    int32                // 创建Epoch时当前Block的高度
	CreateTime      time.Time            // 当前Epoch的创建时间
	ItemList        []*EpochItem         // 当前Epoch包含的验证者列表，（已排序）， 在一个Epoch结束前不会改变
	Generator       *generator.Generator // 当前Generator
	CurGeneratorPos int32                // 当前Generator在ItemList中的位置
	//	CurrentHeight uint64               // 当前Block chain的高度
}

func NewEpoch() *Epoch {
	return &Epoch{
		ItemList:        make([]*EpochItem, 0),
		CreateTime:      time.Now(),
		CreateHeight:    0,
		Generator:       nil,
		CurGeneratorPos: -1, // -1 表示没有还没有Generator
	}
}

func (e *Epoch) AddValidatorToEpoch(validatorInfo *validatorinfo.ValidatorInfo) error {
	// Insert validatorInfo into e.ValidatorList
	if e.IsValidEpochValidator(validatorInfo) == false {
		err := fmt.Errorf("The validator is not valid epoch validator, validator: %v", validatorInfo)
		return err
	}

	for i := 0; i < len(e.ItemList); i++ {
		if e.ItemList[i].ValidatorId == validatorInfo.ValidatorId {
			// The validator is already in e.ValidatorList
			err := fmt.Errorf("The validator (%s:%d) has been existed", validatorInfo.Host, validatorInfo.ValidatorId)
			return err
		}
		if e.ItemList[i].ValidatorId > validatorInfo.ValidatorId {
			// Insert validatorInfo before e.ValidatorList[i]
			e.ItemList = append(e.ItemList, nil)
			copy(e.ItemList[i+1:], e.ItemList[i:])
			e.ItemList[i] = &EpochItem{
				ValidatorId: validatorInfo.ValidatorId,
				Host:        validatorInfo.Host,
				PublicKey:   validatorInfo.PublicKey,
			}
			return nil
		}
	}

	// Append to tail
	epochItem := &EpochItem{
		ValidatorId: validatorInfo.ValidatorId,
		Host:        validatorInfo.Host,
		PublicKey:   validatorInfo.PublicKey,
	}
	e.ItemList = append(e.ItemList, epochItem)

	log.Debugf("Add validator (%s:%d) to epoch", validatorInfo.Host, validatorInfo.ValidatorId)
	return nil
}
func (e *Epoch) IsExist(validatorId uint64) bool {
	for _, validatorInfo := range e.ItemList {
		if validatorInfo.ValidatorId == validatorId {
			// The validator is already in e.ValidatorList
			return true
		}
	}
	return false
}

func (e *Epoch) IsValidEpochValidator(validatorInfo *validatorinfo.ValidatorInfo) bool {
	if validatorInfo.ValidatorId == 0 {
		// Should be a valid validator id
		return false
	}

	if validatorInfo.CreateTime.IsZero() {
		// Should be a valid create time
		return false
	}

	if validatorInfo.PublicKey == [btcec.PubKeyBytesLenCompressed]byte{} {
		// Should be a valid public key
		return false
	}

	if validatorInfo.Host == "" {
		// Should be a valid host
		return false
	}

	return true
}

func (e *Epoch) RemoveValidatorFromEpoch(validatorId uint64) error {
	for i := 0; i < len(e.ItemList); i++ {
		if e.ItemList[i].ValidatorId == validatorId {
			// The validator is found, remove it
			e.ItemList = append(e.ItemList[:i], e.ItemList[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("The validator (%d) is not in epoch", validatorId)
}

func (e *Epoch) GetValidatorList() []*EpochItem {
	return e.ItemList
}

func (e *Epoch) GetGenerator() *generator.Generator {
	return e.Generator
}

func (e *Epoch) GetCurGeneratorPos() int32 {
	return e.CurGeneratorPos
}

func (e *Epoch) GetCreateTime() time.Time {
	return e.CreateTime
}

func (e *Epoch) UpdateGenerator(generator *generator.Generator) {
	e.Generator = generator
}

func (e *Epoch) VoteGenerator() *EpochItem {
	if len(e.ItemList) == 0 {
		return nil
	}

	// The Validator list is lined, Vote new generator as first in list
	return e.ItemList[0]
}

func (e *Epoch) GetNextValidatorsByEpochOrder(validatorId uint64, count int) []*EpochItem {
	orders := make([]*EpochItem, 0)
	if e.ItemList == nil {
		return nil
	}

	index := -1
	for i := 0; i < len(e.ItemList); i++ {
		if e.ItemList[i].ValidatorId == validatorId {
			// The validator is found, remove it
			index = i
			break
		}
	}

	if index == -1 {
		return nil
	}

	index++ // get the validator from next index
	for i := 0; i < count; i++ {
		if index >= len(e.ItemList) {
			index = 0
		}
		orders = append(orders, e.ItemList[index])
	}

	return orders
}

func (e *Epoch) GetNextValidator() *EpochItem {

	nextPos := e.CurGeneratorPos + 1
	if nextPos < 0 || nextPos >= int32(len(e.ItemList)) {
		return nil
	}
	return e.ItemList[nextPos]
}
