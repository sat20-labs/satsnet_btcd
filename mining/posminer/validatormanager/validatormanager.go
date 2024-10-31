package validatormanager

import (
	"math/rand"
	"net"

	"github.com/sat20-labs/satsnet_btcd/chaincfg"
	"github.com/sat20-labs/satsnet_btcd/mining/posminer/epoch"
	"github.com/sat20-labs/satsnet_btcd/mining/posminer/localvalidator"
	"github.com/sat20-labs/satsnet_btcd/mining/posminer/validator"
	"github.com/sat20-labs/satsnet_btcd/wire"
)

const (
	MAX_CONNECTED = 8
)

type Config struct {
	ChainParams *chaincfg.Params
	Dial        func(net.Addr) (net.Conn, error)
	Lookup      func(string) ([]net.IP, error)
	ValidatorId uint64
}

type ValidatorManager struct {
	// ChainParams identifies which chain parameters the cpu miner is
	// associated with.
	ChainParams *chaincfg.Params
	// Dial connects to the address on the named network. It cannot be nil.
	Dial   func(net.Addr) (net.Conn, error)
	lookup func(string) ([]net.IP, error)

	ValidatorId uint64
	myValidator *localvalidator.LocalValidator // 当前的验证者
	//	PreValidatorList []*validator.Validator         // 预备验证者列表， 用于初始化的验证者列表， 主要有本地保存和种子Seed中获取的Validator地址生成
	ValidatorList []*validator.Validator // 所有的验证者列表
	ConnectedList []*validator.Validator // 所有已连接的验证者列表
	CurrentEpoch  *epoch.Epoch           // 当前的Epoch
}

//var validatorMgr *ValidatorManager

func New(cfg *Config) *ValidatorManager {
	log.Debugf("New ValidatorManager")
	validatorMgr := &ValidatorManager{
		ChainParams: cfg.ChainParams,
		Dial:        cfg.Dial,
		lookup:      cfg.Lookup,
		ValidatorId: cfg.ValidatorId,

		ValidatorList: make([]*validator.Validator, 0),
		ConnectedList: make([]*validator.Validator, 0),
	}
	localAddrs, _ := validatorMgr.getLocalAddr()
	//log.Debugf("Get local address: %s", localAddr.String())
	validatorCfg := validatorMgr.newValidatorConfig(validatorMgr.ValidatorId)

	var err error
	validatorMgr.myValidator, err = localvalidator.NewValidator(validatorCfg, localAddrs)
	if err != nil {
		log.Errorf("New LocalValidator failed: %v", err)
		return nil
	}
	validatorMgr.myValidator.Start()
	log.Debugf("New ValidatorManager succeed")
	return validatorMgr
}

func (vm *ValidatorManager) newValidatorConfig(validatorID uint64) *validator.Config {
	return &validator.Config{
		Listener:    vm,
		ChainParams: vm.ChainParams,
		Dial:        vm.Dial,
		Lookup:      vm.lookup,

		ValidatorId: validatorID,
	}
}

func (vm *ValidatorManager) Start() {
	log.Debugf("StartValidatorManager")

	// Load saved validators peers

	// if the saved validators file not exists, start from dns seed
	addrs, _ := vm.getSeed(vm.ChainParams)

	validatorCfg := vm.newValidatorConfig(0) // Start , use 0 (unkown validator id)

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
		vm.ConnectedList = append(vm.ConnectedList, validator)
		//validator.RequestAllValidatorsInfo()
	}

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

	//requestAddr := requestAddrs[0] // 请求验证者的地址, 仅检查第一个即可

	for _, addr := range addrs {
		if addr.String() == requestAddr.String() {
			return true
		}
	}
	return false
}

// Current validator list is updated
func (vm *ValidatorManager) OnValidatorListUpdated(valisatorList []*validator.ValidatorInfo) {

}

// Get current validator list in record this peer
func (vm *ValidatorManager) GetValidatorList() []*validator.ValidatorInfo {

	return nil
}

// Current Epoch is updated
func (vm *ValidatorManager) OnEpochUpdated([]*validator.ValidatorInfo) {

}

// Get current epoch info in record this peer
func (vm *ValidatorManager) GetEpoch() []*validator.ValidatorInfo {
	return nil
}

// Current generator is updated
func (vm *ValidatorManager) OnGeneratorUpdated(*validator.ValidatorInfo) {

}

// Get current generator info in record this peer
func (vm *ValidatorManager) GetGenerator() *validator.ValidatorInfo {

	return nil
}

func (vm *ValidatorManager) OnNewValidatorPeerConnected(netAddr net.Addr, validatorId uint64) {
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
	peerHost := netAddr.(*net.TCPAddr).IP
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

	validatorCfg := vm.newValidatorConfig(validatorId)
	validatorPeer, err := validator.NewValidator(validatorCfg, addrPeer)
	if err != nil {
		log.Errorf("New Validator failed: %v", err)
		return
	}
	// Connect to the validator
	err = validatorPeer.Start()
	if err != nil {
		log.Errorf("Connect validator failed: %v", err)
		return
	}

	vm.ConnectedList = append(vm.ConnectedList, validatorPeer)

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

	switch vm.ChainParams.Net {
	case wire.SatsNet:
		return 4829

	case wire.SatsTestNet:
		return 14829
	}

	return 4829
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
