package validatormanager

import (
	"errors"
	"fmt"
	"time"

	"github.com/sat20-labs/satsnet_btcd/mining/posminer/epoch"
	"github.com/sat20-labs/satsnet_btcd/mining/posminer/validatorcommand"
)

// NewEpochManager是一个处理NewEpoch事件的管理器
// 1. 发送ReqNewEpoch事件时创建NewEpochManager
// 2. 记录所有发送的Validator的ID， 用于处理NewEpoch时的依据
// 3. 接收到NewEpoch事件时交给NewEpochManager进行处理

type NewEpochManager struct {
	ValidatorMgr  *ValidatorManager
	started       bool
	receivedEpoch map[uint64]*epoch.Epoch
}

func CreateNewEpochManager(validatorMgr *ValidatorManager) *NewEpochManager {
	return &NewEpochManager{
		ValidatorMgr:  validatorMgr,
		receivedEpoch: make(map[uint64]*epoch.Epoch),
		started:       false,
	}
}

func (nem *NewEpochManager) Start() {
	// All commands are sent, will due to result in 5 seconds
	go nem.newEpochHandler()
	nem.started = true
}

func (nem *NewEpochManager) NewReqEpoch(validatorId uint64) {
	if nem.started == true {
		err := errors.New("NewEpochManager is started, cannot add validatorId as invited validator")
		log.Errorf("NewReqEpoch failed: %v", err)
		return
	}
	nem.receivedEpoch[validatorId] = &epoch.Epoch{}
}

func (nem *NewEpochManager) AddReceivedEpoch(validatorId uint64, epoch *epoch.Epoch) error {
	if _, ok := nem.receivedEpoch[validatorId]; !ok {
		err := errors.New("Isnot invited validatorId for received epoch")
		log.Errorf("AddReceivedEpoch failed: %v", err)
		return err

	}
	// Update the epoch of the validator
	nem.receivedEpoch[validatorId] = epoch
	return nil
}

// newEpochHandler starts a goroutine that waits for a specified duration
// and then triggers the handleNewEpoch function. It utilizes a channel to
// signal the completion of the task. The function blocks the main goroutine
// until the task is finished or the channel receives a signal, with an
// interval of 5 seconds before executing the handleNewEpoch.
func (nem *NewEpochManager) newEpochHandler() {
	log.Debugf("[NewEpochManager]newEpochHandler ...")

	exitNewEpochHandler := make(chan struct{})
	duration := time.Second * 5
	time.AfterFunc(duration, func() {
		nem.handleNewEpoch()
		exitNewEpochHandler <- struct{}{}
	})

	// 这里阻塞主 goroutine 等待任务执行（可根据需要改为其他逻辑）
	select {
	case exitNewEpochHandler <- struct{}{}:
		log.Debugf("[NewEpochManager]newEpochHandler done .")
		return
	}
}

type ValidEpochItem struct {
	EpochCount int
	epoch      *epoch.Epoch
}

type NewEpochResult struct {
	invalidEpoch []*epoch.Epoch
	validEpoch   []*ValidEpochItem
}

func (nem *NewEpochManager) handleNewEpoch() {
	log.Debugf("[NewEpochManager]handleNewEpoch ...")

	log.Debugf("****************************************************************************************")
	log.Debugf("Received Epochs: summary:")

	invitedCount := len(nem.receivedEpoch)

	for validatorId, epoch := range nem.receivedEpoch {
		title := fmt.Sprintf("Received Epoch from %d", validatorId)
		showEpoch(title, epoch)
	}

	// view the req epoch result
	result := NewEpochResult{
		invalidEpoch: make([]*epoch.Epoch, 0),
		validEpoch:   make([]*ValidEpochItem, 0),
	}

	for _, epoch := range nem.receivedEpoch {
		if isValidEpoch(epoch) == false {
			result.invalidEpoch = append(result.invalidEpoch, epoch)
			continue
		}

		matched := false
		// Find the epoch match with valid epoch
		for _, validItem := range result.validEpoch {
			isSame := isSameEpoch(epoch, validItem.epoch)
			if isSame == true {
				matched = true
				validItem.EpochCount++
				break
			}
		}
		if matched == false {
			result.validEpoch = append(result.validEpoch, &ValidEpochItem{
				EpochCount: 1,
				epoch:      epoch,
			})
		}

	}

	log.Debugf("Received Epochs: invaited count:%d", invitedCount)

	// check the valid epoch result
	minValidCount := (invitedCount * 2) / 3

	log.Debugf("Received Epochs: min valid count:%d", minValidCount)
	localValidatorId := nem.ValidatorMgr.GetMyValidatorId()

	for _, validItem := range result.validEpoch {
		if validItem.EpochCount >= minValidCount {
			log.Debugf("valid epoch:")
			log.Debugf("epoch count: %d", validItem.EpochCount)
			showEpoch("valid epoch:", validItem.epoch)

			// The new epoch is confirmed, the result will be sent to all validators
			epochConfirmed := &epoch.Epoch{
				EpochIndex:      validItem.epoch.EpochIndex,
				CreateTime:      time.Now(), // epoch confirmed time
				CurGeneratorPos: epoch.Pos_Epoch_NotStarted,
				ItemList:        validItem.epoch.ItemList,
			}

			// Notify local peer for the confirmed epoch
			nem.ValidatorMgr.OnConfirmEpoch(epochConfirmed, nil)

			confirmedEpochCmg := validatorcommand.NewMsgConfirmEpoch(localValidatorId, epochConfirmed)
			nem.ValidatorMgr.BroadcastCommand(confirmedEpochCmg)

			// Check current epoch is valid
			if nem.ValidatorMgr.GetCurrentEpoch() == nil {
				// Current epoch is not valid, will req new epoch to miner new block
				nextEpoch := nem.ValidatorMgr.GetNextEpoch()
				if nextEpoch != nil {
					nextBlockHight := nem.ValidatorMgr.GetCurrentBlockHeight() + 1
					timeStamp := time.Now().Unix()
					handoverEpoch := &epoch.HandOverEpoch{
						ValidatorId:    localValidatorId,
						Timestamp:      timeStamp,
						NextEpochIndex: nextEpoch.EpochIndex,
						NextHeight:     nextBlockHight,
					}
					tokenData := handoverEpoch.GetNextEpochTokenData()
					token, err := nem.ValidatorMgr.myValidator.CreateToken(tokenData)
					if err != nil {
						log.Errorf("Create token failed: %v", err)
						return
					}
					handoverEpoch.Token = token
					nextEpochCmd := validatorcommand.NewMsgNextEpoch(handoverEpoch)
					nem.ValidatorMgr.BroadcastCommand(nextEpochCmd)

					// Notify local peer for changing to next epoch
					nem.ValidatorMgr.OnNextEpoch(handoverEpoch)
				}

			}

			// valid epoch only one
			break

		} else {
			log.Debugf("invalid epoch:")
			log.Debugf("epoch count: %d", validItem.EpochCount)
			showEpoch("invalid epoch:", validItem.epoch)
		}
	}

	log.Debugf("Received Epochs: summary end.")
	log.Debugf("****************************************************************************************")
}

func isValidEpoch(epoch *epoch.Epoch) bool {
	if epoch == nil {
		return false
	}

	if len(epoch.ItemList) == 0 {
		return false
	}

	return true
}

func isSameEpoch(epoch1 *epoch.Epoch, epoch2 *epoch.Epoch) bool {
	if epoch1 == nil || epoch2 == nil {
		return false
	}

	if epoch1.EpochIndex != epoch2.EpochIndex {
		return false
	}

	if len(epoch1.ItemList) != len(epoch2.ItemList) {
		return false
	}

	for i := 0; i < len(epoch1.ItemList); i++ {
		if epoch1.ItemList[i].ValidatorId != epoch2.ItemList[i].ValidatorId {
			return false
		}
	}

	return true
}
