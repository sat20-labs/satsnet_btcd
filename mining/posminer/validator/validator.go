package validator

import (
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/sat20-labs/satsnet_btcd/chaincfg"
	"github.com/sat20-labs/satsnet_btcd/mining/posminer/validatorpeer"
)

type ValidatorInfo struct {
	Addr            string
	ValidatorId     uint64
	PublicKey       []byte
	CreateTime      time.Time
	ActivitionCount int
	GeneratorCount  int
	DiscountCount   int
	FaultCount      int
	ValidatorScore  int
}

type ValidatorListener interface {
	// An new validator peer is connected
	OnNewValidatorPeerConnected(net.Addr, uint64)

	// An validator peer is disconnected
	OnValidatorPeerDisconnected(*Validator)

	// An validator peer is inactive
	OnValidatorPeerInactive(netAddr net.Addr)

	// Current validator list is updated
	OnValidatorListUpdated([]*ValidatorInfo)

	// Get current validator list in record this peer
	GetValidatorList() []*ValidatorInfo

	// Current Epoch is updated
	OnEpochUpdated([]*ValidatorInfo)

	// Get current epoch info in record this peer
	GetEpoch() []*ValidatorInfo

	// Current generator is updated
	OnGeneratorUpdated(*ValidatorInfo)

	// Get current generator info in record this peer
	GetGenerator() *ValidatorInfo
}

// Config is the struct to hold configuration options useful to Validator.
type Config struct {
	ValidatorId uint64
	// The listener for process message from/to this validator peer
	Listener ValidatorListener // ChainParams identifies which chain parameters the cpu miner is
	// associated with.
	ChainParams *chaincfg.Params
	// Dial connects to the address on the named network. It cannot be nil.
	Dial   func(net.Addr) (net.Conn, error)
	Lookup func(string) ([]net.IP, error)
}

type Generator struct {
	GeneratorTime time.Time
	Height        uint64
}

type Validator struct {
	PublicKey       []byte
	ValidatorId     uint64
	CreateTime      time.Time
	ActivitionCount int
	GeneratorCount  int
	DiscountCount   int
	FaultCount      int
	IsActivition    bool
	IsGenerator     bool
	GeneratorInfo   Generator
	ValidatorScore  int
	Cfg             *Config

	peer             *validatorpeer.RemotePeer
	isLocalValidator bool
}

func NewValidator(config *Config, addr net.Addr) (*Validator, error) {
	log.Debugf("NewValidator")
	validator := &Validator{
		ValidatorId: config.ValidatorId,
		Cfg:         config,
	}
	peer, err := validatorpeer.NewRemotePeer(validator.newPeerConfig(config), addr)
	if err != nil {
		log.Errorf("NewValidator failed: %v", err)
		return nil, err
	}
	validator.peer = peer
	log.Debugf("NewValidator success with peer: %v", peer.Addr())
	return validator, nil
}

// newPeerConfig returns the configuration for the given serverPeer.
func (v *Validator) newPeerConfig(config *Config) *validatorpeer.RemotePeerConfig {
	return &validatorpeer.RemotePeerConfig{
		RemoteValidator:  v,
		ChainParams:      config.ChainParams,
		Dial:             config.Dial,
		Lookup:           config.Lookup,
		LocalValidatorId: v.ValidatorId,
	}
}

// String returns the validator's info
// string.
//
// This function is safe for concurrent access.
func (p *Validator) String() string {
	if p.peer == nil {
		return "Remote Validator:not connected"
	}
	return fmt.Sprintf("Remote Validator : %s", p.peer.String())
}

// Addr returns the peer address.
//
// This function is safe for concurrent access.
func (v *Validator) GetValidatorAddr() net.Addr {
	// The address doesn't change after initialization, therefore it is not
	// protected by a mutex.
	if v.peer == nil {
		return nil
	}
	return v.peer.GetPeerAddr()
}

func (v *Validator) IsValidatorAddr(host net.IP) bool {
	// The address doesn't change after initialization, therefore it is not
	// protected by a mutex.
	if v.peer == nil {
		return false
	}

	addrPeer := v.peer.GetPeerAddr()
	if addrPeer == nil {
		return false
	}

	hostPeer := addrPeer.(*net.TCPAddr).IP

	if hostPeer.Equal(host) {
		return true
	}

	return false
}

func (v *Validator) RequestAllValidatorsInfo() error {
	if v.peer == nil {
		err := errors.New("invalid peer for the validator")
		log.Errorf("GetAllValidatorsInfo failed: %v", err)
		return err
	}

	//v.peer.RequestAllValidatorsInfo()

	// Generate RequestAllValidatorsInfo command to payload, and send it to peer

	return nil
}

func (v *Validator) Reconnected() {
	log.Debugf("Received a reconnected notify")
	// CHeck current peer is connected
	if v.peer == nil {
		// No any activie peer,cannot be reconnected
		return
	}

	if v.peer.Connected() == false {
		// current peer is not connected, will connect it
		log.Debugf("Will connect to the validator: %v", v.peer.Addr())
		v.peer.Connect()
	}
}

func (v *Validator) Start() error {
	if v.peer == nil {
		err := errors.New("invalid peer for the validator")
		log.Errorf("GetAllValidatorsInfo failed: %v", err)
		return err
	}

	log.Debugf("Will Connect to the validator: %v", v.peer.Addr())

	if v.peer.Connected() == false {
		return v.peer.Connect()
	}

	return nil
}

// Addr returns the peer address.
//
// This function is safe for concurrent access.
func (v *Validator) SetLocalValidator() {
	v.isLocalValidator = true
}

// OnPeerDisconnected is invoked when a remote peer connects to the local peer .
func (v *Validator) OnPeerDisconnected(addr net.Addr) {
	if v.Cfg == nil || v.Cfg.Listener == nil {
		return
	}

	v.Cfg.Listener.OnValidatorPeerDisconnected(v)
	return
}
