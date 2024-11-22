package validatormanager

import (
	"errors"
	"fmt"
	"math/rand"
	"net"
	"slices"
	"sync"
	"time"

	"github.com/sat20-labs/satsnet_btcd/chaincfg"
	"github.com/sat20-labs/satsnet_btcd/chaincfg/chainhash"
	"github.com/sat20-labs/satsnet_btcd/mining/posminer/epoch"
	"github.com/sat20-labs/satsnet_btcd/mining/posminer/generator"
	"github.com/sat20-labs/satsnet_btcd/mining/posminer/localvalidator"
	"github.com/sat20-labs/satsnet_btcd/mining/posminer/validator"
	"github.com/sat20-labs/satsnet_btcd/mining/posminer/validatorcommand"
	"github.com/sat20-labs/satsnet_btcd/mining/posminer/validatorinfo"
	"github.com/sat20-labs/satsnet_btcd/wire"
)

const (
	MAX_CONNECTED        = 8
	BackupGeneratorCount = 2

	MinValidatorsCountEachEpoch = 2  // 每一个Epoch最少的验证者数量， 如果小于该数量， 则不会生成Epoch， 不会进行挖矿
	MaxValidatorsCountEachEpoch = 32 // 每一个Epoch最多有验证者数量， 如果大于该数量， 则会选择指定的数量生成Epoch
)

type PosMinerInterface interface {
	// OnTimeGenerateBlock is invoke when time to generate block.
	OnTimeGenerateBlock() (*chainhash.Hash, error)

	// GetBlockHeight invoke when get block height from pos miner.
	GetBlockHeight() int32
}

type Config struct {
	ChainParams *chaincfg.Params
	Dial        func(net.Addr) (net.Conn, error)
	Lookup      func(string) ([]net.IP, error)
	ValidatorId uint64
	BtcdDir     string

	PosMiner PosMinerInterface
}

type ValidatorManager struct {

	// ValidatorId uint64
	Cfg *Config

	myValidator *localvalidator.LocalValidator // 当前的验证者（本地验证者）
	//	PreValidatorList []*validator.Validator         // 预备验证者列表， 用于初始化的验证者列表， 主要有本地保存和种子Seed中获取的Validator地址生成
	ValidatorList []*validator.Validator // 所有的验证者列表
	CurrentEpoch  *epoch.Epoch           // 当前的Epoch
	NextEpoch     *epoch.Epoch           // 下一个正在排队的Epoch

	ConnectedList    []*validator.Validator // 所有已连接的验证者列表
	connectedListMtx sync.RWMutex

	newEpochMgr *NewEpochManager // Handle NewEpoch event

	quit chan struct{}
}

//var validatorMgr *ValidatorManager

func New(cfg *Config) *ValidatorManager {
	log.Debugf("New ValidatorManager")
	validatorMgr := &ValidatorManager{
		// ChainParams: cfg.ChainParams,
		// Dial:        cfg.Dial,
		// lookup:      cfg.Lookup,
		// ValidatorId: cfg.ValidatorId,
		Cfg: cfg,

		ValidatorList: make([]*validator.Validator, 0),
		ConnectedList: make([]*validator.Validator, 0),

		//CurrentEpoch: epoch.NewEpoch(),

		quit: make(chan struct{}),
	}

	localAddrs, _ := validatorMgr.getLocalAddr()
	//log.Debugf("Get local address: %s", localAddr.String())
	validatorCfg := validatorMgr.newValidatorConfig(validatorMgr.Cfg.ValidatorId, nil) // No remote validator

	var err error
	validatorMgr.myValidator, err = localvalidator.NewValidator(validatorCfg, localAddrs)
	if err != nil {
		log.Errorf("New LocalValidator failed: %v", err)
		return nil
	}
	validatorMgr.myValidator.Start()

	// err = validatorMgr.CurrentEpoch.AddValidatorToEpoch(&validatorMgr.myValidator.ValidatorInfo)
	// if err != nil {
	// 	log.Debugf("Add local validator to epoch failed: %v", err)
	// 	return nil
	// }

	log.Debugf("New ValidatorManager succeed")
	return validatorMgr
}

func (vm *ValidatorManager) newValidatorConfig(localValidatorID uint64, remoteValidatorInfo *validatorinfo.ValidatorInfo) *validator.Config {
	return &validator.Config{
		Listener:    vm,
		ChainParams: vm.Cfg.ChainParams,
		Dial:        vm.Cfg.Dial,
		Lookup:      vm.Cfg.Lookup,
		BtcdDir:     vm.Cfg.BtcdDir,

		LocalValidatorId:    localValidatorID,
		RemoteValidatorInfo: remoteValidatorInfo,
	}
}

// Start starts the validator manager. It loads saved validators peers and if the
// saved validators file not exists, it starts from dns seed. It connects to the
// validators and gets current all validators info.
func (vm *ValidatorManager) Start() {
	log.Debugf("StartValidatorManager")

	// Load saved validators peers

	// if the saved validators file not exists, start from dns seed
	addrs, _ := vm.getSeed(vm.Cfg.ChainParams)

	validatorCfg := vm.newValidatorConfig(vm.Cfg.ValidatorId, nil) // Start , remote validator info is nil (unkown validator)

	PreValidatorList := make([]*validator.Validator, 0)

	for _, addr := range addrs {
		log.Debugf("Try to connect validator: %s", addr.String())

		// New a validator with addr
		//addrsList := make([]net.Addr, 0, 1)
		//addrsList = append(addrsList, addr)

		validator, err := validator.NewValidator(validatorCfg, addr)
		if err != nil {
			log.Errorf("New Validator failed: %v", err)
			continue
		}
		isLocalValidator := vm.isLocalValidator(validator)
		if isLocalValidator {
			log.Debugf("Validator is local validator")
			//validator.SetLocalValidator()
			//vm.PreValidatorList = append(vm.PreValidatorList, validator)
		} else {
			log.Debugf("Validator is remote validator")
			PreValidatorList = append(PreValidatorList, validator)
		}

	}

	// Select Max validator to get current all validators info
	validatorCount := len(PreValidatorList)
	connectCount := validatorCount
	if connectCount > MAX_CONNECTED {
		connectCount = MAX_CONNECTED
	}

	for i := 0; i < connectCount; i++ {
		index := i
		if connectCount < validatorCount {
			index = rand.Intn(validatorCount)
		}
		validator := PreValidatorList[index]

		// Connect to the validator
		err := validator.Start()
		if err != nil {
			log.Errorf("Connect validator failed: %v", err)
			continue
		}

		// The validator is connected
		//vm.ConnectedList = append(vm.ConnectedList, validator)
		vm.AddActivieValidator(validator)
		//validator.RequestAllValidatorsInfo()
	}

	go vm.moniterHandler()
	go vm.syncValidatorsHandler()
	go vm.syncEpochHandler()

	//go vm.getGeneratorHandler()

	go vm.getCheckEpochHandler()
}

func (vm *ValidatorManager) Stop() {
	log.Debugf("ValidatorManager Stop")

	close(vm.quit)
}

func (vm *ValidatorManager) isLocalValidator(validator *validator.Validator) bool {
	addrs := vm.myValidator.GetValidatorAddrsList()
	if len(addrs) == 0 {
		return false
	}

	requestAddr := validator.GetValidatorAddr()
	if requestAddr == nil {
		return false
	}

	for _, addr := range addrs {
		if addr.String() == requestAddr.String() {
			return true
		}
	}
	return false
}

func (vm *ValidatorManager) isLocalValidatorById(validatorId uint64) bool {
	if vm.myValidator.ValidatorInfo.ValidatorId == validatorId {
		return true
	}

	return false
}

// Current validator list is updated
func (vm *ValidatorManager) OnValidatorListUpdated(validatorList []validatorinfo.ValidatorInfo, remoteAddr net.Addr) {
	log.Debugf("ValidatorList Update from [%s]", remoteAddr.String())
	log.Debugf("********************************* New Validator List ********************************")
	log.Debugf("Current Validator Count: %d", len(validatorList))
	for _, validatorInfo := range validatorList {
		log.Debugf("validator ID: %d", validatorInfo.ValidatorId)
		log.Debugf("validator Public: %x", validatorInfo.PublicKey[:])
		log.Debugf("validator Host: %s", validatorInfo.Host)
		log.Debugf("validator CreateTime: %s", validatorInfo.CreateTime.Format("2006-01-02 15:04:05"))
		log.Debugf("------------------------------------------------")
	}
	log.Debugf("*********************************        End        ********************************")

	for _, validatorInfo := range validatorList {
		// Find the validator
		validatorRemote := vm.FindRemoteValidator(validatorInfo.ValidatorId)
		if validatorRemote != nil {
			// Found
			// Update the validator
			validatorRemote.SyncVaildatorInfo(&validatorInfo)
		} else {
			// Not found
			// Is local validator
			if vm.isLocalValidatorById(validatorInfo.ValidatorId) {
				// Local validator
				continue
			}

			if validatorInfo.ValidatorId == 0 || validatorInfo.Host == "" {
				// Invalid validator
				continue
			}

			// Try to connect the validator
			// Add the validator
			validatorCfg := vm.newValidatorConfig(vm.Cfg.ValidatorId, &validatorInfo) // vm.ValidatorId is Local validator, validatorId is Remote validator when new validator connected

			addr, err := vm.getAddr(validatorInfo.Host)
			if err != nil {
				continue
			}
			validatorNew, err := validator.NewValidator(validatorCfg, addr)
			if err != nil {
				log.Errorf("New Validator failed: %v", err)
				continue
			}

			// Connect to the validator
			err = validatorNew.Start()
			if err != nil {
				log.Errorf("Connect validator failed: %v", err)
				continue
			}

			// The validator is connected
			//vm.ConnectedList = append(vm.ConnectedList, validator)
			vm.AddActivieValidator(validatorNew)
			//validator.RequestAllValidatorsInfo()

		}
	}
}

func (vm *ValidatorManager) OnValidatorInfoUpdated(validatorInfo *validatorinfo.ValidatorInfo, remoteAddr net.Addr) {
	// if vm.CurrentEpoch.IsExist(validatorInfo.ValidatorId) == false {
	// 	if vm.CurrentEpoch.IsValidEpochValidator(validatorInfo) == true {
	// 		vm.CurrentEpoch.AddValidatorToEpoch(validatorInfo)
	// 	}
	// }
}

// Get current validator list in record this peer
func (vm *ValidatorManager) GetValidatorList(validatorID uint64) []*validatorinfo.ValidatorInfo {

	log.Debugf("GetValidatorList from validator [%d]", validatorID)

	validatorList := vm.getValidatorList()

	log.Debugf("********************************* Get Validator Summary From [%d] ********************************", validatorID)
	showValidatorList(validatorList)
	log.Debugf("*********************************        End        ********************************")

	return validatorList
}

func (vm *ValidatorManager) getValidatorList() []*validatorinfo.ValidatorInfo {
	validatorList := make([]*validatorinfo.ValidatorInfo, 0)

	// Add local validator
	localValidatorItem := validatorinfo.ValidatorInfo{
		ValidatorId:     vm.myValidator.ValidatorInfo.ValidatorId,
		Host:            vm.myValidator.ValidatorInfo.Host,
		PublicKey:       vm.myValidator.ValidatorInfo.PublicKey,
		CreateTime:      vm.myValidator.ValidatorInfo.CreateTime,
		ActivitionCount: vm.myValidator.ValidatorInfo.ActivitionCount,
		GeneratorCount:  vm.myValidator.ValidatorInfo.GeneratorCount,
		DiscountCount:   vm.myValidator.ValidatorInfo.DiscountCount,
		FaultCount:      vm.myValidator.ValidatorInfo.FaultCount,
		ValidatorScore:  vm.myValidator.ValidatorInfo.ValidatorScore,
	}
	validatorList = append(validatorList, &localValidatorItem)

	// Add all remote validators
	for _, validator := range vm.ConnectedList {
		validatorItem := validatorinfo.ValidatorInfo{
			ValidatorId:     validator.ValidatorInfo.ValidatorId,
			Host:            validator.ValidatorInfo.Host,
			PublicKey:       validator.ValidatorInfo.PublicKey,
			CreateTime:      validator.ValidatorInfo.CreateTime,
			ActivitionCount: validator.ValidatorInfo.ActivitionCount,
			GeneratorCount:  validator.ValidatorInfo.GeneratorCount,
			DiscountCount:   validator.ValidatorInfo.DiscountCount,
			FaultCount:      validator.ValidatorInfo.FaultCount,
			ValidatorScore:  validator.ValidatorInfo.ValidatorScore,
		}
		validatorList = append(validatorList, &validatorItem)
	}

	return validatorList
}

func (vm *ValidatorManager) FindRemoteValidator(validatorID uint64) *validator.Validator {
	// Find from all remote validators
	for _, validator := range vm.ConnectedList {
		if validator.ValidatorInfo.ValidatorId == validatorID {
			return validator
		}
	}
	return nil
}
func (vm *ValidatorManager) SyncValidators() {
	log.Debugf("[SyncValidators]Will sync validators...")
	for _, validator := range vm.ConnectedList {
		log.Debugf("[SyncValidators]Sync form %s...", validator.String())
		validator.SyncAllValidators()
	}
}

func (vm *ValidatorManager) SyncEpoch() {
	log.Debugf("[SyncEpoch]Will GetEpoch from connected validators...")
	for _, validator := range vm.ConnectedList {
		log.Debugf("[SyncEpoch]Get epoch form %s...", validator.String())
		validator.GetEpoch()
	}
}

func (vm *ValidatorManager) SyncGenerator() {
	log.Debugf("[SyncGenerator]Will GetGenerator from connected validators...")
	for _, validator := range vm.ConnectedList {
		log.Debugf("[SyncGenerator]GetGenerator form %s...", validator.String())
		validator.GetGenerator()
	}
}

// Current Epoch is updated
func (vm *ValidatorManager) OnEpochUpdated(currentEpoch *epoch.Epoch, nextEpoch *epoch.Epoch, remoteAddr net.Addr) {
	log.Debugf("OnEpochUpdated from validator [%s]", remoteAddr.String())
	showEpoch("OnEpochUpdated:Current Epoch", currentEpoch)
	showEpoch("OnEpochUpdated:Next Epoch", nextEpoch)
	// for _, validatorInfo := range epochList {
	// 	if vm.CurrentEpoch.IsExist(validatorInfo.ValidatorId) == false {
	// 		log.Debugf("Will add validator [%d] to epoch", validatorInfo.ValidatorId)
	// 		if vm.CurrentEpoch.IsValidEpochValidator(&validatorInfo) == true {
	// 			vm.CurrentEpoch.AddValidatorToEpoch(&validatorInfo)
	// 			log.Debugf("Validator [%d] has been added to epoch", validatorInfo.ValidatorId)
	// 		}
	// 	}
	// }
	isChanged := false
	if currentEpoch != nil {
		// Recevice valid epoch
		if vm.CurrentEpoch == nil {
			// Local is no valid epoch, update local epoch
			vm.CurrentEpoch = currentEpoch
			vm.NextEpoch = nextEpoch
			isChanged = true
		} else {
			if vm.CurrentEpoch.EpochIndex < currentEpoch.EpochIndex {
				// The renote epoch is updated to newest, update local epoch
				vm.CurrentEpoch = currentEpoch
				vm.NextEpoch = nextEpoch
				isChanged = true
			} else if vm.CurrentEpoch.EpochIndex == currentEpoch.EpochIndex {
				if nextEpoch != nil && vm.NextEpoch == nil {
					// The remote next epoch is updated, update local next epoch
					vm.NextEpoch = nextEpoch
					isChanged = true
				}
			}
		}
	}

	if isChanged {
		// Local epoch is updated, show new epoch
		log.Debug("The local epoch is updated")
		showEpoch("New current Epoch", vm.CurrentEpoch)
		showEpoch("New next Epoch", vm.NextEpoch)
	}

}

// Get current epoch info in record this peer
func (vm *ValidatorManager) GetLocalEpoch(validatorID uint64) (*epoch.Epoch, *epoch.Epoch, error) {
	log.Debugf("GetLocalEpoch from validator [%d]", validatorID)
	// if vm.CurrentEpoch != nil {
	// 	return vm.CurrentEpoch.GetValidatorList()
	// }
	return vm.CurrentEpoch, vm.NextEpoch, nil
}

// Current generator is updated
func (vm *ValidatorManager) OnGeneratorUpdated(newGenerator *generator.Generator, validatorID uint64) {
	log.Debugf("OnGeneratorUpdated from validator [%d]", validatorID)

	if newGenerator == nil {
		log.Debugf("OnGeneratorUpdated: Invalid generator (nil generator)")
		return
	}

	if newGenerator.GeneratorId == generator.NoGeneratorId {
		log.Debugf("OnGeneratorUpdated: No generator in validator [%d]", validatorID)
		return
	}

	if newGenerator.GeneratorId == vm.Cfg.ValidatorId {
		log.Debugf("OnGeneratorUpdated: Current generator is local generator, ignore check")
		return
	}

	if vm.IsValidGenerator(newGenerator) == false {
		log.Debugf("OnGeneratorUpdated: Invalid generator in validator [%d]", validatorID)
		return
	}

	showGeneratorInfo(newGenerator)

	if vm.CurrentEpoch != nil {
		curGenerator := vm.CurrentEpoch.GetGenerator()
		needUpdate := false
		if curGenerator == nil {
			needUpdate = true
		} else if curGenerator.GeneratorId != newGenerator.GeneratorId {
			needUpdate = true
		}
		if needUpdate == true {
			log.Debugf("OnGeneratorUpdated: Will update current generator with received generator")
			vm.CurrentEpoch.UpdateGenerator(newGenerator)
		}
	}
}

// Current generator is updated
func (vm *ValidatorManager) OnGeneratorHandOver(handOverGenerator *generator.GeneratorHandOver, remoteAddr net.Addr) {
	log.Debugf("OnGeneratorHandOver from validator [%s]", remoteAddr.String())

	if handOverGenerator.HandOverType == generator.HandOverTypeByEpochOrder {
		// check the hand over generator is valid. it should be signed by current generator

	} else if handOverGenerator.HandOverType == generator.HandOverTypeByVote {
		// It should be check by the result of vote

	}

	// Record the hand over generator info, it should be math with generator notified by new generator
	if vm.isLocalValidatorById(handOverGenerator.GeneratorId) {
		// The next generator is local validator
		vm.SetLocalAsGenerator(handOverGenerator.Height, time.Unix(handOverGenerator.Timestamp, 0))
	}
}

func (vm *ValidatorManager) IsValidGenerator(generator *generator.Generator) bool {
	if generator == nil {
		return false
	}

	if generator.GeneratorId == 0 {
		return false
	}

	// Check the block height of current generator is next height
	nextHeight := vm.Cfg.PosMiner.GetBlockHeight() + 1
	if nextHeight != generator.Height {
		log.Debugf("OnGeneratorUpdated: Invalid generator height [%d], the next generator height is [%d]", generator.Height, nextHeight)
		//return false  // For test, not return false
	}

	validatorConnected := vm.FindRemoteValidator(generator.GeneratorId)
	if validatorConnected == nil {
		log.Debugf("OnGeneratorUpdated: Cannot find validator [%d] in connected validators, ignore it.", generator.GeneratorId)
		return false
	}

	if generator.Token == "" {
		log.Debugf("OnGeneratorUpdated: Invalid generator token, ignore it.")
		return false
	}

	// signatureBytes, err := base64.StdEncoding.DecodeString(generator.Token)
	// if err != nil {
	// 	log.Debugf("OnGeneratorUpdated: Invalid generator token, ignore it.")
	// 	return false
	// }

	// tokenData := generator.GetTokenData()

	// publicKey, err := secp256k1.ParsePubKey(validatorConnected.ValidatorInfo.PublicKey[:])

	// // 解析签名
	// // signature, err := btcec.ParseDERSignature(signatureBytes)
	// signature, err := ecdsa.ParseDERSignature(signatureBytes)
	// if err != nil {
	// 	log.Debugf("Failed to parse signature: %v", err)
	// 	return false
	// }

	// 使用公钥验证签名
	valid := generator.VerifyToken(validatorConnected.ValidatorInfo.PublicKey[:])
	if valid {
		log.Debugf("Signature is valid.")
		generator.Validatorinfo = &validatorConnected.ValidatorInfo
		return true
	} else {
		log.Debugf("Signature is invalid.")
		return false
	}

}

// Get current generator info in record this peer
func (vm *ValidatorManager) GetGenerator() *generator.Generator {
	if vm.CurrentEpoch == nil {
		return nil
	}
	curGenerator := vm.CurrentEpoch.GetGenerator()
	return curGenerator
}

func (vm *ValidatorManager) GetCurrentBlockHeight() int32 {
	return vm.Cfg.PosMiner.GetBlockHeight()
}

func (vm *ValidatorManager) GetCurrentEpoch() *epoch.Epoch {
	log.Debugf("GetCurrentEpoch:")

	return vm.CurrentEpoch
}

func (vm *ValidatorManager) GetNextEpoch() *epoch.Epoch {
	log.Debugf("GetCurrentEpoch:")

	return vm.NextEpoch
}

func (vm *ValidatorManager) GetLocalValidatorInfo() *validatorinfo.ValidatorInfo {

	return &vm.myValidator.ValidatorInfo
}

func (vm *ValidatorManager) OnConfirmEpoch(epoch *epoch.Epoch, remoteAddr net.Addr) {
	if remoteAddr != nil {
		log.Debugf("OnConfirmEpoch from validator [%s]", remoteAddr.String())
	} else {
		log.Debugf("OnConfirmEpoch from local validator")
	}

	// The epoch will be saved in next epoch, And wait current epoch to completed, and continue miner with next epoch
	vm.NextEpoch = epoch
}

func (vm *ValidatorManager) OnNewValidatorPeerConnected(netAddr net.Addr, validatorInfo *validatorinfo.ValidatorInfo) {
	// New validator peer is connected
	log.Debugf("[ValidatorManager]New validator peer connected: %s", netAddr.String())

	// get validator in validator list

	// Get host from netAddr
	// addrs, err := net.LookupHost(netAddr.String())
	// if err != nil {
	// 	log.Errorf("LookupHost failed: %v", err)
	// 	return
	// }
	// if len(addrs) == 0 {
	// 	log.Errorf("Get host from netAddr failed: %s", netAddr.String())
	// 	return
	// }

	peerHost := validatorinfo.GetAddrHost(netAddr)
	validatorPeer := vm.LookupValidator(peerHost)
	if validatorPeer != nil {
		// The validator is already connected, will try to check connection again
		log.Debugf("[ValidatorManager]New validator has added in connectedlist: %s", netAddr.String())
		validatorPeer.Reconnected()
		return
	}

	// Will new an validator
	port := vm.GetValidatorPort()
	addrPeer := &net.TCPAddr{
		IP:   peerHost,
		Port: port,
	}

	validatorCfg := vm.newValidatorConfig(vm.Cfg.ValidatorId, validatorInfo) // vm.ValidatorId is Local validator, validatorId is Remote validator when new validator connected
	peerValidator, err := validator.NewValidator(validatorCfg, addrPeer)
	if err != nil {
		log.Errorf("New Validator failed: %v", err)
		return
	}
	// Connect to the validator
	err = peerValidator.Start()
	if err != nil {
		log.Errorf("Connect validator failed: %v", err)
		return
	}

	//vm.ConnectedList = append(vm.ConnectedList, peerValidator)
	vm.AddActivieValidator(peerValidator)

	log.Debugf("[ValidatorManager]New validator added to connectedlist: %s", netAddr.String())
}

func (vm *ValidatorManager) OnValidatorPeerDisconnected(validator *validator.Validator) {
	// Remote validator peer disconnected, it will be notify by remote validator when it cannot connect or sent any command
	log.Debugf("[ValidatorManager]validator peer is disconnected: %s", validator.String())
	vm.removeValidator(validator)
}

func (vm *ValidatorManager) OnValidatorPeerInactive(netAddr net.Addr) {
	// Remote validator peer is inactive, it will be notify by local validator when it is long time to not received any command
	log.Debugf("[ValidatorManager]validator peer in inactive: %s", netAddr.String())
}

// Get current generator info in record this peer
func (vm *ValidatorManager) GetValidatorPort() int {

	switch vm.Cfg.ChainParams.Net {
	case wire.SatsNet:
		return 4829

	case wire.SatsTestNet:
		return 14829
	}

	return 4829
}

func (vm *ValidatorManager) AddActivieValidator(validator *validator.Validator) error {
	if validator.IsConnected() == false {
		return fmt.Errorf("validator is not connected")
	}

	vm.connectedListMtx.Lock()
	defer vm.connectedListMtx.Unlock()

	addr := validator.GetValidatorAddr()

	peerHost := validatorinfo.GetAddrHost(addr)
	validatorPeer := vm.LookupValidator(peerHost)
	if validatorPeer != nil {
		// The validator is already in connectedlist
		validator.Stop()
		return fmt.Errorf("validator alreadly in connected list")
	}
	vm.ConnectedList = append(vm.ConnectedList, validator)

	//sortsValidatorList(vm.ConnectedList)

	// err := vm.CurrentEpoch.AddValidatorToEpoch(&validator.ValidatorInfo)
	// if err != nil {
	// 	log.Debugf("Add validator to epoch failed: %v", err)
	// }

	return nil
}

func (vm *ValidatorManager) LookupValidator(host net.IP) *validator.Validator {
	for _, validator := range vm.ConnectedList {
		if validator.IsValidatorAddr(host) {
			return validator
		}
	}
	return nil
}

func (vm *ValidatorManager) removeValidator(validator *validator.Validator) {
	for index, validatorItem := range vm.ConnectedList {
		if validatorItem == validator {

			// if vm.CurrentEpoch.IsExist(validatorItem.ValidatorInfo.ValidatorId) == true {
			// 	// If the validator is in epoch, remove it
			// 	err := vm.CurrentEpoch.RemoveValidatorFromEpoch(validatorItem.ValidatorInfo.ValidatorId)
			// 	if err != nil {
			// 		log.Debugf("Remove validator from epoch failed: %v", err)
			// 	}
			// }

			if index == len(vm.ConnectedList)-1 {
				vm.ConnectedList = vm.ConnectedList[:index]
				return
			} else if index == 0 {
				vm.ConnectedList = vm.ConnectedList[1:]
				return
			}
			vm.ConnectedList = append(vm.ConnectedList[:index], vm.ConnectedList[index+1:]...)
			return
		}

	}
}

func sortsValidatorList(validatorList []*validatorinfo.ValidatorInfo) {

	slices.SortFunc(validatorList, func(a, b *validatorinfo.ValidatorInfo) int {
		if a.ValidatorId > b.ValidatorId {
			return 1
		} else {
			return -1
		}
	})
}

// moniterHandler for show current validator list in local on a timer
func (vm *ValidatorManager) moniterHandler() {
	monitorInterval := time.Second * 25
	monitorTicker := time.NewTicker(monitorInterval)
	defer monitorTicker.Stop()

exit:
	for {
		select {
		case <-monitorTicker.C:
			vm.showCurrentValidatorManager()
		case <-vm.quit:
			break exit
		}
	}

	log.Debugf("[ValidatorManager]moniterHandler done.")
}

func (vm *ValidatorManager) showCurrentValidatorManager() {
	validatorList := vm.getValidatorList()

	log.Debugf("********************************* Validator Summary ********************************")
	showValidatorList(validatorList)
	log.Debugf("*********************************        End        ********************************")

	showEpoch("Current Epoch", vm.CurrentEpoch)
	showEpoch("Next Epoch", vm.NextEpoch)
}

func showValidatorList(validatorList []*validatorinfo.ValidatorInfo) {
	log.Debugf("Current Validator Count: %d", len(validatorList))
	for _, validatorInfo := range validatorList {
		log.Debugf("validator ID: %d", validatorInfo.ValidatorId)
		log.Debugf("validator Public: %x", validatorInfo.PublicKey[:])
		log.Debugf("validator Host: %s", validatorInfo.Host)
		log.Debugf("validator CreateTime: %s", validatorInfo.CreateTime.Format("2006-01-02 15:04:05"))
		log.Debugf("------------------------------------------------")
	}
}

func showEpoch(title string, epoch *epoch.Epoch) {
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

		showGeneratorInfo(epoch.GetGenerator())
		//Generator     *generator.Generator // 当前Generator
		log.Debugf("CurGeneratorPos: %d", epoch.CurGeneratorPos)

	}
	log.Debugf("*********************************        End        ********************************")
}
func showGeneratorInfo(generator *generator.Generator) {
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
}

// syncValidatorsHandler for sync validator list from remote peer on a timer
func (vm *ValidatorManager) syncValidatorsHandler() {

	// First sync validator list , and then sync epoch in 60s
	vm.SyncValidators()

	syncInterval := time.Second * 60
	syncTicker := time.NewTicker(syncInterval)
	defer syncTicker.Stop()

exit:
	for {
		log.Debugf("[ValidatorManager]Waiting next timer for syncing validator list...")
		select {
		case <-syncTicker.C:
			vm.SyncValidators()
		case <-vm.quit:
			break exit
		}
	}

	log.Debugf("[ValidatorManager]syncValidatorsHandler done.")
}

// syncEpochHandler for sync validator list from remote peer on a timer
func (vm *ValidatorManager) syncEpochHandler() {

	// Sync Epoch first
	vm.SyncEpoch()

	syncInterval := time.Second * 30
	syncTicker := time.NewTicker(syncInterval)
	defer syncTicker.Stop()

exit:
	for {
		log.Debugf("[ValidatorManager]Waiting next timer for syncing epoch list...")
		select {
		case <-syncTicker.C:
			vm.SyncEpoch()
		case <-vm.quit:
			break exit
		}
	}

	log.Debugf("[ValidatorManager]syncEpochHandler done.")
}

func (vm *ValidatorManager) getGeneratorHandler() {
	log.Debugf("[ValidatorManager]getGeneratorHandler ...")

	exitGeneraterHandler := make(chan struct{})
	// Sync current generator from all connected validators
	vm.SyncGenerator()
	// 计算从现在到目标时间的间隔
	duration := time.Second * 10
	time.AfterFunc(duration, func() {
		vm.CheckGenerator()
		exitGeneraterHandler <- struct{}{}
	})

	// 这里阻塞主 goroutine 等待任务执行（可根据需要改为其他逻辑）
	select {
	case exitGeneraterHandler <- struct{}{}:
		log.Debugf("[ValidatorManager]getGeneratorHandler done .")
		return
	case <-vm.quit:
		return
	}
}

func (vm *ValidatorManager) CheckGenerator() {
	log.Debugf("[ValidatorManager]CheckGenerator...")
	curGenerator := vm.GetGenerator()
	if curGenerator != nil {
		log.Debugf("[ValidatorManager]Has a generator, Nothing to do...")
		showGeneratorInfo(curGenerator)
		return
	} else {
		log.Debugf("Generator is nil, Will generate a new generator")
		currentEpoch := vm.GetCurrentEpoch()
		if currentEpoch == nil || len(currentEpoch.ItemList) == 0 {
			// No any validator in epoch, do nothing
			log.Debugf("[ValidatorManager]No any validator in epoch, Nothing to do...")
			return
		}

		epochList := currentEpoch.ItemList

		testBecomeGenerate := false
		if vm.Cfg.ValidatorId == 10000020 {
			testBecomeGenerate = true
		}

		if testBecomeGenerate == true || (len(epochList) == 1 && epochList[0].ValidatorId == vm.Cfg.ValidatorId) {
			// Only local validator in epoch, become local validator a generator
			log.Debugf("[ValidatorManager]Only local validator in epoch, become local validator a generator...")
			height := vm.Cfg.PosMiner.GetBlockHeight()
			vm.SetLocalAsGenerator(height, time.Now())
			return
		}

		log.Debugf("[ValidatorManager]Will Vote a new generator...")
		// Vote a new generator
		votedValidator := vm.CurrentEpoch.VoteGenerator()
		if votedValidator == nil {
			// It should be empty in epoch list
			return
		}

		// Broadcast Vote result to epoch list
	}
}

func (vm *ValidatorManager) SetLocalAsGenerator(height int32, handoverTime time.Time) {
	vm.myValidator.BecomeGenerator(height, handoverTime)
	myGenerator := vm.myValidator.GetMyGenerator()

	showGeneratorInfo(myGenerator)

	//vm.CurrentEpoch.UpdateGenerator(myGenerator)
	// broadcast Generator
	// Will Send New Generator to all Connected Validators
	cmdGenerator := validatorcommand.NewMsgGenerator(myGenerator)
	vm.BroadcastCommand(cmdGenerator)
}

func (vm *ValidatorManager) OnTimeGenerateBlock() (*chainhash.Hash, error) {
	log.Debugf("[ValidatorManager]OnTimeGenerateBlock...")

	myGenerator := vm.myValidator.GetMyGenerator()
	if myGenerator == nil {
		err := errors.New("Invalid local generator")
		return nil, err
	}
	heightGenerator := myGenerator.Height

	// Notify validator manager to generate new block
	hash, err := vm.Cfg.PosMiner.OnTimeGenerateBlock()
	if err != nil {
		log.Debugf("[LocalValidator]OnTimeGenerateBlock failed: %v", err)
		// Generate block failed, it should be no tx to be mined, wait for next time
		if err.Error() == "no any new tx in mempool" || err.Error() == "no any new tx need to be mining" {
			// No any tx to be mined, continue next slot
			vm.myValidator.ContinueNextSlot()
			return nil, err
		}

		// It should some error in the peer, hand over to next validator to miner, the height is not changed
	} else {
		log.Debugf("[LocalValidator]OnTimeGenerateBlock succeed, Hash: %s", hash.String())
		// New block generated, should be hand over to next validator with next height
		heightGenerator = heightGenerator + 1
	}

	if vm.CurrentEpoch == nil {
		return nil, errors.New("current epoch is nil")
	}
	count := 1 + BackupGeneratorCount
	nextGenerators := vm.CurrentEpoch.GetNextValidatorsByEpochOrder(myGenerator.GeneratorId, count)
	if nextGenerators == nil || len(nextGenerators) == 0 || vm.isLocalValidatorById(nextGenerators[0].ValidatorId) {
		log.Debugf("[LocalValidator]Next generator is local validator, conitnue miner by local validator")
		// No any generator or next generator is local validator, continue miner by local validator
		vm.myValidator.ContinueNextSlot()
		return hash, nil
	}
	// Will handover to next validator, clear my generator
	vm.myValidator.ClearMyGenerator()

	handOver := generator.GeneratorHandOver{
		ValidatorId:  myGenerator.GeneratorId,
		HandOverType: generator.HandOverTypeByEpochOrder,
		Height:       heightGenerator,
		Timestamp:    myGenerator.MinerTime.Unix(), // Last miner time
		GeneratorId:  nextGenerators[0].ValidatorId,
	}

	log.Debugf("[ValidatorManager]HandOver: %+v", handOver)
	// Will Send HandOver to all Connected Validators
	cmdHandOver := validatorcommand.NewMsgHandOver(&handOver)
	vm.BroadcastCommand(cmdHandOver)
	return hash, err
}

func (vm *ValidatorManager) BroadcastCommand(command validatorcommand.Message) {
	log.Debugf("[ValidatorManager]Will broadcast command from all connected validators...")
	for _, validator := range vm.ConnectedList {
		log.Debugf("[ValidatorManager]Send command to %s...", validator.String())
		validator.SendCommand(command)
	}
}

// Req new epoch from remote peer
func (vm *ValidatorManager) ReqNewEpoch(validatorID uint64, epochIndex uint32) (*epoch.Epoch, error) {

	currentValidatorCount := len(vm.ConnectedList) + 1
	if currentValidatorCount < MinValidatorsCountEachEpoch {
		return nil, errors.New("no enough validators to new epoch")
	}

	//nextEpochIndex := vm.getCurrentEpochIndex() + 1
	currentBlockHeight := vm.Cfg.PosMiner.GetBlockHeight()
	newEpoch := &epoch.Epoch{
		EpochIndex:      epochIndex,
		CreateHeight:    currentBlockHeight,
		CreateTime:      time.Now(),
		ItemList:        make([]*epoch.EpochItem, 0),
		CurGeneratorPos: -1,
		Generator:       nil, // will be set by local validator
	}

	validators := vm.getValidatorList()
	sortsValidatorList(validators)
	// Fill item to EpochItem list
	for i := 0; i < len(validators); i++ {
		validator := validators[i]

		item := &epoch.EpochItem{
			ValidatorId: validator.ValidatorId,
			Host:        validator.Host,
			PublicKey:   validator.PublicKey,
			Index:       uint32(i)}

		newEpoch.ItemList = append(newEpoch.ItemList, item)
	}

	showEpoch("New Epoch", newEpoch)

	return newEpoch, nil
}

// OnNextEpoch from remote peer
func (vm *ValidatorManager) OnNextEpoch(handoverEpoch *epoch.HandOverEpoch) {
	log.Debugf("[ValidatorManager]OnNextEpoch ...")

	// Change the next epoch to current epoch, and clear next epoch
	vm.CurrentEpoch = vm.NextEpoch
	vm.NextEpoch = nil

	if vm.CurrentEpoch != nil {
		nextEpochValidator := vm.CurrentEpoch.GetNextValidator()
		if nextEpochValidator.ValidatorId == vm.GetMyValidatorId() {
			// The first validator of new current epoch is local validator
			vm.SetLocalAsGenerator(handoverEpoch.NextHeight, time.Unix(handoverEpoch.Timestamp, 0))
		}
	}
}

func (vm *ValidatorManager) getCurrentEpochIndex() uint32 {
	if vm.CurrentEpoch == nil {
		return 0
	}
	return vm.CurrentEpoch.EpochIndex
}

func (vm *ValidatorManager) getCheckEpochHandler() {
	log.Debugf("[ValidatorManager]getCheckEpochHandler ...")

	exitGeneraterHandler := make(chan struct{})
	// Sync current epoch from all connected validators
	// 计算从现在到目标时间的间隔, 启动15秒后检查一次
	duration := time.Second * 15
	time.AfterFunc(duration, func() {
		vm.CheckEpoch()
		exitGeneraterHandler <- struct{}{}
	})

	// 这里阻塞主 goroutine 等待任务执行（可根据需要改为其他逻辑）
	select {
	case exitGeneraterHandler <- struct{}{}:
		log.Debugf("[ValidatorManager]getCheckEpochHandler done .")
		return
	case <-vm.quit:
		return
	}

}

// 启动15秒后检查一次， 如果当前没有有效的epoch， 并且所有连接的validator超过最小validator数量， 就申请生成新的epoch
func (vm *ValidatorManager) CheckEpoch() {
	log.Debugf("[ValidatorManager]CheckEpoch ...")
	if vm.CurrentEpoch != nil {
		log.Debugf("[ValidatorManager]Has a epoch, Nothing to do...")
		showEpoch("Current Epoch", vm.CurrentEpoch)
		return
	}

	// 没有有效的Epoch， 就申请生成新的epoch
	AcitvityValidatorCount := len(vm.ConnectedList) + 1
	if AcitvityValidatorCount >= MinValidatorsCountEachEpoch {
		log.Debugf("Not valid epoch to miner, will req new epoch to miner new block")
		nextEpochIndex := vm.getCurrentEpochIndex() + 1
		// Will Send CmdReqEpoch to all Connected Validators
		CmdReqEpoch := validatorcommand.NewMsgReqEpoch(vm.Cfg.ValidatorId, nextEpochIndex)
		vm.newEpochMgr = CreateNewEpochManager(vm)
		//vm.BroadcastCommand(CmdReqEpoch)
		log.Debugf("Will broadcast ReqEpoch command from all connected validators...")
		for _, validator := range vm.ConnectedList {
			validator.SendCommand(CmdReqEpoch)
			vm.newEpochMgr.NewReqEpoch(validator.ValidatorInfo.ValidatorId)
		}

		// Add local validator to invited list
		vm.newEpochMgr.NewReqEpoch(vm.Cfg.ValidatorId)
		vm.newEpochMgr.Start()

		// Add local new epoch result to newepochmanager
		epoch, _ := vm.ReqNewEpoch(vm.Cfg.ValidatorId, nextEpochIndex)
		vm.newEpochMgr.AddReceivedEpoch(vm.Cfg.ValidatorId, epoch)
	} else {
		log.Debugf("Not enough validators, cannot new epoch to miner new block")
	}
}

func (vm *ValidatorManager) OnNewEpoch(validatorId uint64, epoch *epoch.Epoch) {
	log.Debugf("[ValidatorManager]OnNewEpoch received from validator [%d]...", validatorId)
	title := fmt.Sprintf("Received [%d] New Epoch", validatorId)
	showEpoch(title, epoch)

	// Todo: due to received epoch, for new next epoch
	if vm.newEpochMgr != nil {
		vm.newEpochMgr.AddReceivedEpoch(validatorId, epoch)
	}
}

func (vm *ValidatorManager) GetMyValidatorId() uint64 {
	return vm.Cfg.ValidatorId
}
