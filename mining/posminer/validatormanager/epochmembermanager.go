package validatormanager

import (
	"net"
	"sync"
	"time"

	"github.com/sat20-labs/satoshinet/btcec"
	"github.com/sat20-labs/satoshinet/mining/posminer/epoch"
	"github.com/sat20-labs/satoshinet/mining/posminer/utils"
	"github.com/sat20-labs/satoshinet/mining/posminer/validator"
	"github.com/sat20-labs/satoshinet/mining/posminer/validatorcommand"
	"github.com/sat20-labs/satoshinet/wire"
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
	PublicKey [btcec.PubKeyBytesLenCompressed]byte
	Result    uint32
	Token     string
}

type DelEpochMemberCollection struct {
	ResultList map[uint64]*DelEpochMemberResult
	StartTime  time.Time
}

type EpochMemberManager struct {
	ValidatorMgr     *ValidatorManager
	CurrentEpoch     *epoch.Epoch
	ConnectedList    map[uint64]*validator.Validator
	connectedListMtx sync.RWMutex

	DisconnectedList    map[uint64]*DisconnectEpochMember
	disconnectedListMtx sync.RWMutex

	receivedDelEpochMemberResult map[uint64]*DelEpochMemberCollection
}

func CreateEpochMemberManager(validatorMgr *ValidatorManager) *EpochMemberManager {
	return &EpochMemberManager{
		ValidatorMgr:                 validatorMgr,
		disconnectedListMtx:          sync.RWMutex{},
		connectedListMtx:             sync.RWMutex{},
		receivedDelEpochMemberResult: make(map[uint64]*DelEpochMemberCollection),
	}
}

func (em *EpochMemberManager) UpdateCurrentEpoch(currentEpoch *epoch.Epoch) {
	em.CurrentEpoch = currentEpoch

	em.updateValidatorsList()
}

func (em *EpochMemberManager) GetEpochMember(validatorID uint64) (*validator.Validator, bool) {

	utils.Log.Debugf("connectedListMtx4 Locked")
	em.connectedListMtx.RLock()
	defer func() {
		em.connectedListMtx.RUnlock()
		utils.Log.Debugf("connectedListMtx4 Unocked")
	}()

	if em.ConnectedList != nil {
		validatorItem, ok := em.ConnectedList[validatorID]
		if ok {
			return validatorItem, true
		}
	}

	utils.Log.Debugf("disconnectedListMtx1 Locked")
	em.disconnectedListMtx.RLock()
	defer func() {
		defer em.disconnectedListMtx.RUnlock()
		utils.Log.Debugf("disconnectedListMtx1 Unocked")
	}()

	if em.DisconnectedList != nil {
		disconnectItem, ok := em.DisconnectedList[validatorID]
		if ok {
			return disconnectItem.Validator, false
		}
	}

	return nil, false
}

func (em *EpochMemberManager) updateValidatorsList() {

	utils.Log.Debugf("[EpochMemberManager]Update validators list...")
	// 将原来的数据清空
	utils.Log.Debugf("connectedListMtx5 Locked")
	em.connectedListMtx.Lock()
	defer func() {
		em.connectedListMtx.Unlock()
		utils.Log.Debugf("connectedListMtx5 Unocked")
	}()

	utils.Log.Debugf("disconnectedListMtx2 Locked")
	em.disconnectedListMtx.Lock()
	defer func() {
		defer em.disconnectedListMtx.Unlock()
		utils.Log.Debugf("disconnectedListMtx2 Unocked")
	}()

	em.ConnectedList = make(map[uint64]*validator.Validator)
	em.DisconnectedList = make(map[uint64]*DisconnectEpochMember)

	utils.Log.Debugf("[EpochMemberManager]Old validators list has cleared.")

	if em.CurrentEpoch == nil {
		// 当前没有需要管理的epoch
		utils.Log.Debugf("[EpochMemberManager]Empty current epoch, not to be managed.")
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
			validatorCfg := em.ValidatorMgr.newValidatorConfig(em.ValidatorMgr.Cfg.ValidatorId, em.ValidatorMgr.Cfg.ValidatorPubKey, nil)

			addr, err := em.ValidatorMgr.getAddr(epochItem.Host)
			if err != nil {
				continue
			}
			validatorNew, err := validator.NewValidator(validatorCfg, addr)
			if err != nil {
				utils.Log.Errorf("New Validator failed: %v", err)
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
	utils.Log.Debugf("[EpochMemberManager]Update validators list Done.")
}

func (em *EpochMemberManager) OnValidatorDisconnected(validatorID uint64) {

	utils.Log.Debugf("[EpochMemberManager]A epoch member is disconnected: %d", validatorID)

	utils.Log.Debugf("connectedListMtx6 Locked")
	em.connectedListMtx.Lock()
	defer func() {
		em.connectedListMtx.Unlock()
		utils.Log.Debugf("connectedListMtx6 Unocked")
	}()

	if em.ConnectedList == nil || em.DisconnectedList == nil {
		utils.Log.Errorf("[EpochMemberManager]Invalid epoch manager for em.ConnectedList = %v or em.DisconnectedList = %v", em.ConnectedList, em.DisconnectedList)
		return
	}

	validatorItem, ok := em.ConnectedList[validatorID]
	if !ok {
		// The validator is not in connected list
		utils.Log.Debugf("[EpochMemberManager]The disconnected epoch member isnot in connected list, nothing to do: %d", validatorID)
		return
	}

	utils.Log.Debugf("[EpochMemberManager]The %d will be removed from connected list, and added to disconnected list", validatorID)

	// remove it from connected list
	delete(em.ConnectedList, validatorID)

	utils.Log.Debugf("disconnectedListMtx3 Locked")
	em.disconnectedListMtx.Lock()
	defer func() {
		defer em.disconnectedListMtx.Unlock()
		utils.Log.Debugf("disconnectedListMtx3 Unocked")
	}()

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
	utils.Log.Debugf("[EpochMemberManager]reconnectEpochHandler ...")
	reconnectInterval := time.Second * 1
	reconnectTicker := time.NewTicker(reconnectInterval)
	defer reconnectTicker.Stop()

exit:
	for {
		utils.Log.Debugf("[EpochMemberManager]Waiting next timer for reconnect disconnected epoch member...")
		select {
		case <-reconnectTicker.C:
			isExit := em.reconnectEpochMember()
			if isExit {
				break exit
			}
		}
	}

	utils.Log.Debugf("[EpochMemberManager]reconnectEpochHandler done.")
}

func (em *EpochMemberManager) reconnectEpochMember() bool {

	utils.Log.Debugf("disconnectedListMtx4 Locked")
	em.disconnectedListMtx.Lock()
	disConnectedList := em.DisconnectedList
	em.disconnectedListMtx.Unlock()
	utils.Log.Debugf("disconnectedListMtx4 Unocked")

	if disConnectedList == nil {
		return true
	}

	for validatorID, validatorItem := range disConnectedList {
		utils.Log.Debugf("[EpochMemberManager]Try to reconnect validator: %d...", validatorID)

		if validatorItem.Validator.IsConnected() == false {
			err := validatorItem.Validator.Connect()
			if err != nil {
				validatorItem.ReconnectTimes++ // reconnect times + 1
				utils.Log.Errorf("[EpochMemberManager]Connect validator failed : %v [%d]", err, validatorItem.ReconnectTimes)

				if validatorItem.ReconnectTimes > EpochReconnectMaxTimes {
					// 已经确认离线，不再尝试重连
					delete(disConnectedList, validatorID)

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

					if reqPos < 0 {
						continue
					}

					if em.CurrentEpoch.ItemList[reqPos].ValidatorId == em.ValidatorMgr.Cfg.ValidatorId {
						// 需要发起剔除成员的请求的成员是自己， 则申请剔除离线成员
						utils.Log.Debugf("[EpochMemberManager]Request to delete validator from epoch list: %d...", validatorID)

						if em.ValidatorMgr.myValidator.IsBootStrapNode() {
							// 在多次尝试重连失败后，需要剔除成员
							em.ReqDelEpochMember(validatorID)
						}
						continue
					}
				}
				continue
			}
		}
		// The validator is connected, remove it from disconnected list
		utils.Log.Debugf("[EpochMemberManager]Reconnected to validator: %d...", validatorID)

		// remove it from connected list
		delete(disConnectedList, validatorID)

		utils.Log.Debugf("connectedListMtx8 Locked")
		em.connectedListMtx.Lock()
		// and add it to disconnected list
		em.ConnectedList[validatorID] = validatorItem.Validator
		em.connectedListMtx.Unlock()
		utils.Log.Debugf("connectedListMtx8 Unocked")

	}

	utils.Log.Debugf("disconnectedListMtx8 Locked")
	em.disconnectedListMtx.Lock()
	em.DisconnectedList = disConnectedList
	em.disconnectedListMtx.Unlock()
	utils.Log.Debugf("disconnectedListMtx8 Unocked")

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

	delMemberCollection := &DelEpochMemberCollection{
		ResultList: make(map[uint64]*DelEpochMemberResult),
		StartTime:  time.Now(),
	}

	em.receivedDelEpochMemberResult[delValidatorID] = delMemberCollection

	utils.Log.Debugf("connectedListMtx1 Locked")
	em.connectedListMtx.Lock()
	defer func() {
		em.connectedListMtx.Unlock()
		utils.Log.Debugf("connectedListMtx1 Unocked")
	}()

	utils.Log.Debugf("Will broadcast DelEpoch command from all connected validators...")
	for validatorId, validator := range em.ConnectedList {
		delMemberCollection.ResultList[validatorId] = &DelEpochMemberResult{PublicKey: validator.ValidatorInfo.PublicKey, Result: epoch.DelEpochMemberResult_NotConfirm, Token: ""}
		validator.SendCommand(CmdReqDelEpochMember)
	}

	// Add local validator confirm result
	delEpochMember := &epoch.DelEpochMember{
		ValidatorId:    em.ValidatorMgr.myValidator.ValidatorInfo.ValidatorId,
		DelValidatorId: delValidatorID,
		DelCode:        epoch.DelCode_Disconnect,
		EpochIndex:     em.CurrentEpoch.EpochIndex,
		Result:         epoch.DelEpochMemberResult_Agree,
	}

	tokenData := delEpochMember.GetDelEpochMemTokenData()
	// Sign the token by local validator private key
	token, err := em.ValidatorMgr.SignToken(tokenData)
	if err == nil {
		resultLocal := &DelEpochMemberResult{PublicKey: em.ValidatorMgr.myValidator.ValidatorInfo.PublicKey, Result: epoch.DelEpochMemberResult_Agree, Token: token}
		delMemberCollection.ResultList[em.ValidatorMgr.myValidator.ValidatorInfo.ValidatorId] = resultLocal
	}

	go em.delEpochMemberHandler(delValidatorID)
}

func (em *EpochMemberManager) OnConfirmedDelEpochMember(delEpochMember *epoch.DelEpochMember) {
	if delEpochMember == nil {
		return
	}

	confirmedValidatorId := delEpochMember.ValidatorId

	utils.Log.Debugf("connectedListMtx2 Locked")
	em.connectedListMtx.Lock()
	defer func() {
		em.connectedListMtx.Unlock()
		utils.Log.Debugf("connectedListMtx2 Unocked")
	}()

	confirmedValidator := em.ConnectedList[confirmedValidatorId]
	if confirmedValidator == nil {
		return
	}
	verified := delEpochMember.VerifyToken(confirmedValidator.ValidatorInfo.PublicKey[:])
	if !verified {
		// The confirm command isnot verified
		return
	}

	delValidatorID := delEpochMember.DelValidatorId

	delValidatorCollection, ok := em.receivedDelEpochMemberResult[delValidatorID]
	if !ok {
		return
	}

	// record validator confirm result
	result := &DelEpochMemberResult{PublicKey: confirmedValidator.ValidatorInfo.PublicKey, Result: delEpochMember.Result, Token: delEpochMember.Token}
	delValidatorCollection.ResultList[confirmedValidatorId] = result
}

func (em *EpochMemberManager) delEpochMemberHandler(delValidatorID uint64) {
	utils.Log.Debugf("[EpochMemberManager]delEpochMemberHandler ...")

	exitDelEpochHandler := make(chan struct{})
	duration := time.Second * 5
	time.AfterFunc(duration, func() {
		em.handleDelEpochMember(delValidatorID)
		exitDelEpochHandler <- struct{}{}
	})

	// 这里阻塞主 goroutine 等待任务执行（可根据需要改为其他逻辑）
	select {
	case exitDelEpochHandler <- struct{}{}:
		utils.Log.Debugf("[EpochMemberManager]delEpochMemberHandler done .")
		return
	}
}

func (em *EpochMemberManager) handleDelEpochMember(delValidatorID uint64) {

	delValidatorCollection, ok := em.receivedDelEpochMemberResult[delValidatorID]
	if !ok {
		return
	}

	totalCount := len(delValidatorCollection.ResultList)
	agreeCount := 0
	for _, result := range delValidatorCollection.ResultList {
		if result.Result == epoch.DelEpochMemberResult_Agree {
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
		em.ValidatorMgr.ConfirmDelEpoch(em.CurrentEpoch, delValidatorCollection)
		//
		updateEpochCmd := validatorcommand.NewMsgUpdateEpoch(em.CurrentEpoch)
		em.ValidatorMgr.BroadcastCommand(updateEpochCmd)

		// CurrentEpoch has been updated, check continue handover
		em.ValidatorMgr.CheckContinueHandOver()
	} else {
		// Disagree for del validatorID from epoch member
		utils.Log.Debugf("[EpochMemberManager]Disagree for del validatorID from epoch member: %d", delValidatorID)
	}
}

func (em *EpochMemberManager) NotifyEpochMemberDeleted(delValidatorID uint64) {
	CmdConfirmDelEpochMember := validatorcommand.NewMsgReqDelEpochMember(em.ValidatorMgr.Cfg.ValidatorId,
		validatorcommand.CmdDelEpochMemberTarget_Confirm,
		delValidatorID,
		epoch.DelCode_Disconnect,
		em.CurrentEpoch.EpochIndex)

	utils.Log.Debugf("Will broadcast ReqEpoch command from all connected validators...")

	utils.Log.Debugf("connectedListMtx3 Locked")
	em.connectedListMtx.Lock()
	defer func() {
		em.connectedListMtx.Unlock()
		utils.Log.Debugf("connectedListMtx3 Unocked")
	}()

	for _, validator := range em.ConnectedList {
		validator.SendCommand(CmdConfirmDelEpochMember)
	}
}

// Received a Del epoch member command
func (em *EpochMemberManager) ConfirmDelEpochMember(reqDelEpochMember *validatorcommand.MsgReqDelEpochMember, remoteAddr net.Addr) *epoch.DelEpochMember {
	utils.Log.Debugf("[ValidatorManager]ConfirmDelEpochMember received from validator [%s]...", remoteAddr.String())

	if reqDelEpochMember == nil {
		return nil
	}

	utils.Log.Debugf("[ValidatorManager]ConfirmDelEpochMember Will confirm the [%s] is or not connected?", reqDelEpochMember.DelValidatorId)

	switch reqDelEpochMember.Target {
	case validatorcommand.CmdDelEpochMemberTarget_Consult:
		// 需要Check指定要删除的validator是否已经离线， 如果确认离线， 则回复确认消息
		delValidator, isConnected := em.GetEpochMember(reqDelEpochMember.DelValidatorId)
		if !isConnected {
			// 回复同意删除消息
			utils.Log.Debugf("The validator [%d] is not connected", reqDelEpochMember.DelValidatorId)
			confirmDelMember := em.NewConfirmDelMember(reqDelEpochMember.DelValidatorId, reqDelEpochMember, epoch.DelEpochMemberResult_Agree)
			return confirmDelMember
		}

		// 发送一个ping消息给要删除的validator， 确认是否已经离线
		nonce, _ := wire.RandomUint64()
		delValidator.SendCommand(validatorcommand.NewMsgPing(nonce))

		em.checkMemberConnectedHandler(delValidator, reqDelEpochMember)
	case validatorcommand.CmdDelEpochMemberTarget_Confirm:
		// 已经确认删除，从本地的Epoch中删除validator
		em.CurrentEpoch.DelEpochMember(reqDelEpochMember.DelValidatorId)
		return nil
	}
	return nil
}

func (em *EpochMemberManager) checkMemberConnectedHandler(delValidator *validator.Validator, reqDelEpochMember *validatorcommand.MsgReqDelEpochMember) *epoch.DelEpochMember {
	utils.Log.Debugf("[EpochMemberManager]checkMemberConnectedHandler ...")

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
		utils.Log.Debugf("[NewEpochManager]newEpochHandler done .")
		return cfmDelEpochMember
	}
}

func (em *EpochMemberManager) handleCheckMemberConnected(delValidator *validator.Validator, reqDelEpochMember *validatorcommand.MsgReqDelEpochMember) *epoch.DelEpochMember {
	utils.Log.Debugf("[EpochMemberManager]handleCheckMemberConnected ...")

	lastReceived := delValidator.GetLastReceived()

	now := time.Now()
	// 计算时间间隔
	duration := now.Sub(lastReceived)

	// 判断是否小于1秒
	if duration < time.Second {
		utils.Log.Debugf("[EpochMemberManager]The member is response in 1 second, it's connected .")
		// 回复不同意删除消息
		confirmDelMember := em.NewConfirmDelMember(reqDelEpochMember.ValidatorId, reqDelEpochMember, epoch.DelEpochMemberResult_Reject)
		return confirmDelMember

	} else {
		utils.Log.Debugf("[EpochMemberManager]The member is not response pong in 1 second, it's disconnected .")
		// 回复同意删除消息
		confirmDelMember := em.NewConfirmDelMember(reqDelEpochMember.ValidatorId, reqDelEpochMember, epoch.DelEpochMemberResult_Agree)
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
	}

	tokenData := delEpochMember.GetDelEpochMemTokenData()
	// Sign the token by local validator private key
	token, err := em.ValidatorMgr.SignToken(tokenData)
	if err != nil {
		utils.Log.Errorf("Sign token failed: %v", err)
		return nil
	}

	delEpochMember.Token = token

	return delEpochMember
}
