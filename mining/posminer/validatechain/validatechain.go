package validatechain

import (
	"github.com/sat20-labs/satoshinet/chaincfg/chainhash"
	"github.com/sat20-labs/satoshinet/mining/posminer/validatechaindb"
)

const (
	Version_ValidateChain = 1
)

type ValidateChain struct {
	vcStore      *validatechaindb.ValidateChainStore
	currentState ValidateChainState
}

func NewValidateChain(vcStore *validatechaindb.ValidateChainStore) *ValidateChain {
	if vcStore == nil {
		return nil
	}

	chain := &ValidateChain{
		vcStore: vcStore,
	}

	stateData, err := vcStore.GetState()
	if err == nil {
		chain.currentState.Decode(stateData)
	}

	return chain
}
func (vc *ValidateChain) Start() error {
	err := vc.SyncValidateChain()
	if err != nil {
		return err
	}

	return nil
}

func (vc *ValidateChain) SyncValidateChain() error {
	return nil
}

// 得到本地当前的最新状态
func (vc *ValidateChain) GetCurrentState() *ValidateChainState {
	return &vc.currentState
}

// 更新当前链的最新状态
func (vc *ValidateChain) UpdateCurrentState(state *ValidateChainState) error {
	vc.currentState.LatestHeight = state.LatestHeight
	vc.currentState.LatestHash = state.LatestHash
	vc.currentState.LatestEpochIndex = state.LatestEpochIndex

	stateData, err := vc.currentState.Encode()
	if err != nil {
		return err
	}
	err = vc.vcStore.SetState(stateData)
	if err != nil {
		return err
	}

	return nil
}

// 根据Hash获取块数据
func (vc *ValidateChain) GetVCBlock(hash *chainhash.Hash) (*VCBlock, error) {
	dataBlock, err := vc.vcStore.GetBlockData(hash[:])
	if err != nil {
		return nil, err
	}

	vcBlock := &VCBlock{}
	err = vcBlock.Decode(dataBlock)
	if err != nil {
		return nil, err
	}

	return vcBlock, nil
}

// 写入一个块数据
func (vc *ValidateChain) SaveVCBlock(vcBlock *VCBlock) error {
	hash, err := vcBlock.GetHash()
	if err != nil {
		return err
	}

	blockData, err := vcBlock.Encode()
	if err != nil {
		return err
	}
	err = vc.vcStore.SaveBlock(hash[:], blockData)
	if err != nil {
		return err
	}

	return nil
}

// 根据高度获取BlockHash
func (vc *ValidateChain) GetVCBlockHash(height int64) (*chainhash.Hash, error) {
	hash, err := vc.vcStore.GetVCBlockHash(height)
	if err != nil {
		return nil, err
	}

	return hash, nil
}

// 更新一个高度与blockhash
func (vc *ValidateChain) SaveVCBlockHash(height int64, hash *chainhash.Hash) error {
	err := vc.vcStore.SaveVCBlockHash(height, hash)

	return err
}

// 写入投票的数据
func (vc *ValidateChain) SaveEPBlock(epBlock *EPBlock) error {
	hash, err := epBlock.GetHash()
	if err != nil {
		return err
	}

	epblockData, err := epBlock.Encode()
	if err != nil {
		return err
	}
	err = vc.vcStore.SaveEPBlock(hash[:], epblockData)
	if err != nil {
		return err
	}

	return nil
}

// 根据hash获取EpochVoteBlock
func (vc *ValidateChain) GetEPBlock(hash *chainhash.Hash) (*EPBlock, error) {
	dataBlock, err := vc.vcStore.GetEPBlockData(hash[:])
	if err != nil {
		return nil, err
	}

	epBlock := &EPBlock{}
	err = epBlock.Decode(dataBlock)
	if err != nil {
		return nil, err
	}

	return epBlock, nil
}
