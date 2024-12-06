package validatormanager

import (
	"net"
	"sync"
	"time"

	"github.com/sat20-labs/satsnet_btcd/mining/posminer/epoch"
	"github.com/sat20-labs/satsnet_btcd/mining/posminer/validator"
	"github.com/sat20-labs/satsnet_btcd/mining/posminer/validatorcommand"
	"github.com/sat20-labs/satsnet_btcd/wire"
)

const (
	EpochReconnectMaxTimes = 5
)

// EpochMemberManager是一个管理当前Epoch除了自己之外的成员的管理器
// 1. 监听成员是否离线
// 2. 离线成员在一定时间内尝试重新连接，多次重新连接失败后可以申请剔除
// 2. 管理是否需要剔除成员
// 3. 处理成员被剔除

type DisconnectEpochMember struct {
	Validator      *validator.Validator
	ReconnectTimes int
}

type DelEpochMemberResult struct {
	ResultList map[uint64]uint32
	StartTime  time.Time
}

type EpochMemberManager struct {
	ValidatorMgr     *ValidatorManager
	CurrentEpoch     *epoch.Epoch
	ConnectedList    map[uint64]*validator.Validator
	connectedListMtx sync.RWMutex

	DisconnectedList    map[uint64]*DisconnectEpochMember
	disconnectedListMtx sync.RWMutex

	receivedDelEpochMemberResult map[uint64]*DelEpochMemberResult
}

func CreateEpochMemberManager(validatorMgr *ValidatorManager) *EpochMemberManager {
	return &EpochMemberManager{
		ValidatorMgr:                 validatorMgr,
		disconnectedListMtx:          sync.RWMutex{},
		connectedListMtx:             sync.RWMutex{},
		receivedDelEpochMemberResult: make(map[uint64]*DelEpochMemberResult),
	}
}

func (em *EpochMemberManager) UpdateCurrentEpoch(currentEpoch *epoch.Epoch) {
	em.CurrentEpoch = currentEpoch

	em.updateValidatorsList()
}

func (em *EpochMemberManager) GetEpochMember(validatorID uint64) (*validator.Validator, bool) {

	em.connectedListMtx.RLock()
	defer em.connectedListMtx.RUnlock()

	if em.ConnectedList != nil {
		validatorItem, ok := em.ConnectedList[validatorID]
		if ok {
			return validatorItem, true
		}
	}

	em.disconnectedListMtx.RLock()
	defer em.disconnectedListMtx.RUnlock()

	if em.DisconnectedList != nil {
		disconnectItem, ok := em.DisconnectedList[validatorID]
		if ok {
			return disconnectItem.Validator, false
		}
	}

	return nil, false
}

func (em *EpochMemberManager) updateValidatorsList() {

	// 将原来的数据清空
	em.connectedListMtx.Lock()
	defer em.connectedListMtx.Unlock()

	em.disconnectedListMtx.Lock()
	defer em.disconnectedListMtx.Unlock()

	em.ConnectedList = make(map[uint64]*validator.Validator)
	em.DisconnectedList = make(map[uint64]*DisconnectEpochMember)

	if em.CurrentEpoch == nil {
		// 当前没有需要管理的epoch
		return
	}

	for _, epochItem := range em.CurrentEpoch.ItemList {
		if epochItem.ValidatorId == em.ValidatorMgr.Cfg.ValidatorId {
			// is local validator
			continue
		}
		validatorItem := em.ValidatorMgr.FindRemoteValidator(epochItem.ValidatorId)
		if validatorItem == nil {
			// 没有找到对应的validator, 需要将这个成员按照离线处理
			// Add the validator
			validatorCfg := em.ValidatorMgr.newValidatorConfig(em.ValidatorMgr.Cfg.ValidatorId, nil)

			addr, err := em.ValidatorMgr.getAddr(epochItem.Host)
			if err != nil {
				continue
			}
			validatorNew, err := validator.NewValidator(validatorCfg, addr)
			if err != nil {
				log.Errorf("New Validator failed: %v", err)
				continue
			}
			em.DisconnectedList[epochItem.ValidatorId] = &DisconnectEpochMember{
				Validator:      validatorNew,
				ReconnectTimes: 0,
			}
		} else {
			em.ConnectedList[epochItem.ValidatorId] = validatorItem
		}
	}

	if len(em.DisconnectedList) > 0 {
		// Disconnected list is not empty, try to reconnect
		go em.reconnectEpochHandler()
	}
}

func (em *EpochMemberManager) OnValidatorDisconnected(validatorID uint64) {

	log.Debugf("[EpochMemberManager]A epoch member is disconnected: %d", validatorID)

	em.connectedListMtx.Lock()
	defer em.connectedListMtx.Unlock()

	if em.ConnectedList == nil || em.DisconnectedList == nil {
		log.Errorf("[EpochMemberManager]Invalid epoch manager for em.ConnectedList = %v or em.DisconnectedList = %v", em.ConnectedList, em.DisconnectedList)
		return
	}

	validatorItem, ok := em.ConnectedList[validatorID]
	if !ok {
		// The validator is not in connected list
		log.Debugf("[EpochMemberManager]The disconnected epoch member isnot in connected list, nothing to do: %d", validatorID)
		return
	}

	log.Debugf("[EpochMemberManager]The %d will be removed from connected list, and added to disconnected list", validatorID)

	// remove it from connected list
	delete(em.ConnectedList, validatorID)

	em.disconnectedListMtx.Lock()
	defer em.disconnectedListMtx.Unlock()

	// and add it to disconnected list
	em.DisconnectedList[validatorID] = &DisconnectEpochMember{
		Validator:      validatorItem,
		ReconnectTimes: 0,
	}

	if len(em.DisconnectedList) > 0 {
		// Disconnected list is not empty, try to reconnect
		go em.reconnectEpochHandler()
	}
}

// reconnectEpochHandler for reconnect epoch member when a epoch member is disconnected on a timer
func (em *EpochMemberManager) reconnectEpochHandler() {
	log.Debugf("[EpochMemberManager]reconnectEpochHandler ...")
	reconnectInterval := time.Second * 1
	reconnectTicker := time.NewTicker(reconnectInterval)
	defer reconnectTicker.Stop()

exit:
	for {
		log.Debugf("[EpochMemberManager]Waiting next timer for reconnect disconnected epoch member...")
		select {
		case <-reconnectTicker.C:
			isExit := em.reconnectEpochMember()
			if isExit {
				break exit
			}
		}
	}

	log.Debugf("[EpochMemberManager]reconnectEpochHandler done.")
}

func (em *EpochMemberManager) reconnectEpochMember() bool {

	em.disconnectedListMtx.Lock()
	defer em.disconnectedListMtx.Unlock()

	if em.DisconnectedList == nil {
		return true
	}

	for validatorID, validatorItem := range em.DisconnectedList {
		log.Debugf("[EpochMemberManager]Try to reconnect validator: %d...", validatorID)

		if validatorItem.Validator.IsConnected() == false {
			err := validatorItem.Validator.Connect()
			if err != nil {
				validatorItem.ReconnectTimes++ // reconnect times + 1
				log.Errorf("[EpochMemberManager]Connect validator failed : %v [%d]", err, validatorItem.ReconnectTimes)

				if validatorItem.ReconnectTimes > EpochReconnectMaxTimes {
					// 已经确认离线，不再尝试重连
					delete(em.DisconnectedList, validatorID)

					// 得到已经离线的成员的POS
					posGenerator := em.CurrentEpoch.GetCurGeneratorPos()
					posDisconnected := em.CurrentEpoch.GetValidatorPos(validatorID)
					if posDisconnected < posGenerator {
						// 离线成员已经出块，不需要从Epoch去剔除成员
						continue
					}

					// 有离线的下一个成员来负责发起剔除成员的请求， 如果离线的成员是最后一个成员，则有它的上一个成员来负责发起剔除成员的请求
					reqPos := posDisconnected + 1

					memCount := len(em.CurrentEpoch.ItemList)

					if posDisconnected == int32(memCount-1) {
						reqPos = posDisconnected - 1
					}

					if em.CurrentEpoch.ItemList[reqPos].ValidatorId == em.ValidatorMgr.Cfg.ValidatorId {
						// 需要发起剔除成员的请求的成员是自己， 则申请剔除离线成员
						log.Debugf("[EpochMemberManager]Request to delete validator from epoch list: %d...", validatorID)

						// 在多次尝试重连失败后，需要剔除成员
						em.ReqDelEpochMember(validatorID)
						continue
					}
				}
				continue
			}
		}
		// The validator is connected, remove it from disconnected list
		log.Debugf("[EpochMemberManager]Reconnected to validator: %d...", validatorID)

		// remove it from connected list
		delete(em.DisconnectedList, validatorID)

		// and add it to disconnected list
		em.ConnectedList[validatorID] = validatorItem.Validator
	}

	// Disconnected list is empty, exit
	if len(em.DisconnectedList) == 0 {
		return true
	}
	return false
}

func (em *EpochMemberManager) ReqDelEpochMember(delValidatorID uint64) {
	CmdReqDelEpochMember := validatorcommand.NewMsgReqDelEpochMember(em.ValidatorMgr.Cfg.ValidatorId,
		validatorcommand.CmdDelEpochMemberTarget_Consult,
		delValidatorID,
		epoch.DelCode_Disconnect,
		em.CurrentEpoch.EpochIndex)
	// vm.newEpochMgr = CreateNewEpochManager(vm)
	//em.ValidatorMgr.BroadcastCommand(CmdReqDelEpochMember)

	delValidatorResult := &DelEpochMemberResult{
		ResultList: make(map[uint64]uint32),
		StartTime:  time.Now(),
	}

	em.receivedDelEpochMemberResult[delValidatorID] = delValidatorResult

	em.connectedListMtx.Lock()
	defer em.connectedListMtx.Unlock()

	log.Debugf("Will broadcast ReqEpoch command from all connected validators...")
	for validatorId, validator := range em.ConnectedList {
		delValidatorResult.ResultList[validatorId] = epoch.DelEpochMemberResult_NotConfirm
		validator.SendCommand(CmdReqDelEpochMember)
	}

	// Add local validator confirm result
	delValidatorResult.ResultList[em.ValidatorMgr.Cfg.ValidatorId] = epoch.DelEpochMemberResult_Agree

	go em.delEpochMemberHandler(delValidatorID)
}

func (em *EpochMemberManager) OnConfirmedDelEpochMember(delEpochMember *epoch.DelEpochMember) {
	if delEpochMember == nil {
		return
	}

	confirmedValidatorId := delEpochMember.ValidatorId

	em.connectedListMtx.Lock()
	defer em.connectedListMtx.Unlock()

	confirmedValidator := em.ConnectedList[confirmedValidatorId]
	if confirmedValidator == nil {
		return
	}
	verified := delEpochMember.VerifyToken(confirmedValidator.ValidatorInfo.PublicKey[:])
	if verified == false {
		// The confirm command isnot verified
		return
	}

	delValidatorID := delEpochMember.DelValidatorId

	delValidatorResult, ok := em.receivedDelEpochMemberResult[delValidatorID]
	if !ok {
		return
	}

	// record validator confirm result
	delValidatorResult.ResultList[confirmedValidatorId] = delEpochMember.Result
}

func (em *EpochMemberManager) delEpochMemberHandler(delValidatorID uint64) {
	log.Debugf("[NewEpochManager]newEpochHandler ...")

	exitNewEpochHandler := make(chan struct{})
	duration := time.Second * 5
	time.AfterFunc(duration, func() {
		em.handleDelEpochMember(delValidatorID)
		exitNewEpochHandler <- struct{}{}
	})

	// 这里阻塞主 goroutine 等待任务执行（可根据需要改为其他逻辑）
	select {
	case exitNewEpochHandler <- struct{}{}:
		log.Debugf("[NewEpochManager]newEpochHandler done .")
		return
	}
}

func (em *EpochMemberManager) handleDelEpochMember(delValidatorID uint64) {

	delValidatorResult, ok := em.receivedDelEpochMemberResult[delValidatorID]
	if !ok {
		return
	}

	totalCount := len(delValidatorResult.ResultList)
	agreeCount := 0
	for _, result := range delValidatorResult.ResultList {
		if result == epoch.DelEpochMemberResult_Agree {
			agreeCount++
		}
	}

	minAgreeCount := (totalCount * 2) / 3

	if agreeCount >= minAgreeCount {
		// Agree for del validatorID from epoch member

		// remove req result
		delete(em.receivedDelEpochMemberResult, delValidatorID)

		// Del the member and broadcast for Update epoch
		//em.NotifyEpochMemberDeleted(delValidatorID)
		em.CurrentEpoch.DelEpochMember(delValidatorID)
		updateEpochCmd := validatorcommand.NewMsgUpdateEpoch(em.CurrentEpoch)
		em.ValidatorMgr.BroadcastCommand(updateEpochCmd)

		// Call local validator for handle this action
		em.ValidatorMgr.OnUpdateEpoch(em.CurrentEpoch)
	}
}

func (em *EpochMemberManager) NotifyEpochMemberDeleted(delValidatorID uint64) {
	CmdConfirmDelEpochMember := validatorcommand.NewMsgReqDelEpochMember(em.ValidatorMgr.Cfg.ValidatorId,
		validatorcommand.CmdDelEpochMemberTarget_Confirm,
		delValidatorID,
		epoch.DelCode_Disconnect,
		em.CurrentEpoch.EpochIndex)

	log.Debugf("Will broadcast ReqEpoch command from all connected validators...")

	em.connectedListMtx.Lock()
	defer em.connectedListMtx.Unlock()

	for _, validator := range em.ConnectedList {
		validator.SendCommand(CmdConfirmDelEpochMember)
	}
}

// Received a Del epoch member command
func (em *EpochMemberManager) ConfirmDelEpochMember(reqDelEpochMember *validatorcommand.MsgReqDelEpochMember, remoteAddr net.Addr) *epoch.DelEpochMember {
	log.Debugf("[ValidatorManager]ConfirmDelEpochMember received from validator [%s]...", remoteAddr.String())

	if reqDelEpochMember == nil {
		return nil
	}

	if reqDelEpochMember.Target == validatorcommand.CmdDelEpochMemberTarget_Consult {
		// 需要Check指定的validator是否已经离线， 如果确认离线， 则回复确认消息
		validator, isConnected := em.GetEpochMember(reqDelEpochMember.ValidatorId)
		if !isConnected {
			// 回复同意删除消息
			log.Debugf("The validator [%d] is not connected", reqDelEpochMember.ValidatorId)
			confirmDelMember := em.NewConfirmDelMember(reqDelEpochMember.ValidatorId, reqDelEpochMember, epoch.DelEpochMemberResult_Agree)
			return confirmDelMember
		}
		nonce, _ := wire.RandomUint64()
		validator.SendCommand(validatorcommand.NewMsgPing(nonce))

		em.checkMemberConnectedHandler(validator, reqDelEpochMember)
	} else if reqDelEpochMember.Target == validatorcommand.CmdDelEpochMemberTarget_Confirm {
		// 已经确认删除，从本地的Epoch中删除validator
		em.CurrentEpoch.DelEpochMember(reqDelEpochMember.ValidatorId)
		return nil
	}
	return nil
}

func (em *EpochMemberManager) checkMemberConnectedHandler(delValidator *validator.Validator, reqDelEpochMember *validatorcommand.MsgReqDelEpochMember) *epoch.DelEpochMember {
	log.Debugf("[EpochMemberManager]checkMemberConnectedHandler ...")

	exitCheckHandler := make(chan struct{})
	var cfmDelEpochMember *epoch.DelEpochMember
	duration := time.Second * 1
	time.AfterFunc(duration, func() {
		cfmDelEpochMember = em.handleCheckMemberConnected(delValidator, reqDelEpochMember)
		exitCheckHandler <- struct{}{}
	})

	// 这里阻塞主 goroutine 等待任务执行（可根据需要改为其他逻辑）
	select {
	case exitCheckHandler <- struct{}{}:
		log.Debugf("[NewEpochManager]newEpochHandler done .")
		return cfmDelEpochMember
	}
}

func (em *EpochMemberManager) handleCheckMemberConnected(delValidator *validator.Validator, reqDelEpochMember *validatorcommand.MsgReqDelEpochMember) *epoch.DelEpochMember {
	log.Debugf("[EpochMemberManager]handleCheckMemberConnected ...")

	lastReceived := delValidator.GetLastReceived()

	now := time.Now()
	// 计算时间间隔
	duration := now.Sub(lastReceived)

	// 判断是否小于1秒
	if duration < time.Second {
		log.Debugf("[EpochMemberManager]The member is response in 1 second, it's connected .")
		// 回复同意删除消息
		confirmDelMember := em.NewConfirmDelMember(reqDelEpochMember.ValidatorId, reqDelEpochMember, epoch.DelEpochMemberResult_Agree)
		return confirmDelMember

	} else {
		log.Debugf("[EpochMemberManager]The member is not response pong in 1 second, it's disconnected .")
		// 回复不同意删除消息
		confirmDelMember := em.NewConfirmDelMember(reqDelEpochMember.ValidatorId, reqDelEpochMember, epoch.DelEpochMemberResult_Reject)
		return confirmDelMember
	}
}

func (em *EpochMemberManager) NewConfirmDelMember(delValidatorId uint64, reqDelEpochMember *validatorcommand.MsgReqDelEpochMember, result uint32) *epoch.DelEpochMember {
	delEpochMember := &epoch.DelEpochMember{
		ValidatorId:    em.ValidatorMgr.Cfg.ValidatorId,
		DelValidatorId: delValidatorId,
		DelCode:        reqDelEpochMember.DelCode,
		EpochIndex:     reqDelEpochMember.EpochIndex,
		Result:         result,
		Timestamp:      time.Now().Unix(),
	}

	tokenData := delEpochMember.GetDelEpochMemTokenData()
	// Sign the token by local validator private key
	token, err := em.ValidatorMgr.SignToken(tokenData)
	if err != nil {
		log.Errorf("Sign token failed: %v", err)
		return nil
	}

	delEpochMember.Token = token

	return delEpochMember
}
