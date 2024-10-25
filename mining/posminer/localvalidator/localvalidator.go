package localvalidator

import (
	"net"
	"time"

	"github.com/sat20-labs/satsnet_btcd/mining/posminer/validator"
	"github.com/sat20-labs/satsnet_btcd/mining/posminer/validatorpeer"
)

type Generator struct {
	GeneratorTime time.Time
	Height        uint64
}

type LocalValidator struct {
	validator.Validator
	localPeer *validatorpeer.LocalPeer
}

func NewValidator(config *validator.Config, addrs []net.Addr) (*LocalValidator, error) {
	log.Debugf("NewValidator")
	validator := &LocalValidator{}
	validator.Cfg = config
	peer, err := validatorpeer.NewLocalPeer(validator.newPeerConfig(config), addrs)
	if err != nil {
		log.Errorf("NewValidator failed: %v", err)
		return nil, err
	}
	validator.localPeer = peer
	log.Debugf("NewValidator success with peer: %v", peer.Addr())
	return validator, nil
}

// newPeerConfig returns the configuration for the given serverPeer.
func (v *LocalValidator) newPeerConfig(config *validator.Config) *validatorpeer.LocalPeerConfig {
	return &validatorpeer.LocalPeerConfig{
		LocalValidator: v,
		ChainParams:    config.ChainParams,
		Dial:           config.Dial,
		Lookup:         config.Lookup,
	}
}

// Addr returns the peer address.
//
// This function is safe for concurrent access.
func (v *LocalValidator) Start() {
	// Start the peer listener for local peer
	v.localPeer.Start()
}

// OnPeerConnected is invoked when a remote peer connects to the local peer .
func (v *LocalValidator) OnPeerConnected(addr net.Addr) {
	// It will nitify the validator manager, an new validator peer is connected
	log.Debugf("[LocalValidator]Receive a new validator peer connected[%s]", addr.String())
	if v.Cfg == nil || v.Cfg.Listener == nil {
		return
	}
	v.Cfg.Listener.OnNewValidatorPeerConnected(addr)
	return
}

// GetAllValidators invoke when get all validators.
func (v *LocalValidator) GetAllValidators() []byte {
	return nil
}

func (v *LocalValidator) GetValidatorAddrsList() []net.Addr {
	// The address doesn't change after initialization, therefore it is not
	// protected by a mutex.
	if v.localPeer == nil {
		return nil
	}
	return v.localPeer.GetPeerAddrsList()
}
