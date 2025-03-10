package bootstrapnode

import (
	"encoding/hex"

	"github.com/sat20-labs/satsnet_btcd/wire"
	"github.com/sat20-labs/indexer/common"
	"github.com/sat20-labs/satsnet_btcd/indexer/share/indexer"
)

func IsBootStrapNode(pubKey []byte) bool {
	return hex.EncodeToString(pubKey) == common.BootstrapPubKey
}

// 包含bootstrap
func IsCoreNode(pubKey []byte) bool {
	if IsBootStrapNode(pubKey) {
		return true
	}

	if hex.EncodeToString(pubKey) == common.CoreNodePubKey {
		return true
	}

	// 从索引器查询结果：该节点已经与引导节点建立了通道，并且将资产质押到通道中（通过HasCoreNodeEligibility判断）
	return indexer.ShareIndexer.IsCoreNode(hex.EncodeToString(pubKey))
}

func CheckValidatorID(pubKey []byte) bool {
	return IsCoreNode(pubKey)
}

func HasCoreNodeEligibility(assets wire.TxAssets) bool {
	for _, asset := range assets {
		if asset.Name.String() == common.CORENODE_STAKING_ASSET_NAME {
			return asset.Amount.Int64() >= common.CORENODE_STAKING_ASSET_AMOUNT
		}
	}
	return false
}
