package bootstrapnode

import (
	"encoding/hex"

	"github.com/sat20-labs/satsnet_btcd/wire"
	"github.com/sat20-labs/indexer/common"
	"github.com/sat20-labs/satsnet_btcd/indexer/share/indexer"
)

func IsBootStrapNode(_ uint64, pubKey []byte) bool {
	return hex.EncodeToString(pubKey) == common.BootstrapPubKey
}

// 包含bootstrap
func IsCoreNode(validatorId uint64, pubKey []byte) bool {
	if IsBootStrapNode(validatorId, pubKey) {
		return true
	}

	if hex.EncodeToString(pubKey) == common.CoreNodePubKey {
		return true
	}

	// 从索引器查询结果
	return indexer.ShareIndexer.IsCoreNode(hex.EncodeToString(pubKey))
}

func CheckValidatorID(validatorID uint64, pubKey []byte) bool {
	return IsCoreNode(validatorID, pubKey)
}

func HasCoreNodeEligibility(assets wire.TxAssets) bool {
	for _, asset := range assets {
		if asset.Name.String() == common.CORENODE_STAKING_ASSET_NAME {
			return asset.Amount >= common.CORENODE_STAKING_ASSET_AMOUNT
		}
	}
	return false
}
