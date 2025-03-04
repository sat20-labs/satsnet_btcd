package bootstrapnode

import (
	"encoding/hex"

	"github.com/sat20-labs/satsnet_btcd/stp"
)

const (
	// BootstrapPubKey is the public key of the bootstrap Certificate Issuer.
	BootstrapPubKey = "025fb789035bc2f0c74384503401222e53f72eefdebf0886517ff26ac7985f52ad" //
	BootStrapNodeId = 1

	CoreNodePubKey = "025fb789035bc2f0c74384503401222e53f72eefdebf0886517ff26ac7985f52ad" //
	CoreNodeId = 100

	// 0 invalid
	// 1-99 boostrap
	// 100-999 core
	// 1000-99999 normal miner
	MIN_BOOTSTRAP_NODEID = 1
	MAX_BOOTSTRAP_NODEID = 9
	MIN_CORE_NODEID      = 100
	MAX_CORE_NODEID      = 999
	MIN_NORMAL_NODEID    = 100000
)

func IsBootStrapNode(_ uint64, pubKey []byte) bool {
	return hex.EncodeToString(pubKey) == BootstrapPubKey
}

// 包含bootstrap
func IsCoreNode(validatorId uint64, pubKey []byte) bool {
	if IsBootStrapNode(validatorId, pubKey) {
		return true
	}

	if hex.EncodeToString(pubKey) == CoreNodePubKey {
		return true
	}

	// 其他根据规则生成的core node
	return stp.IsCoreNode(pubKey)
}

func CheckValidatorID(validatorID uint64, pubKey []byte) bool {
	return IsCoreNode(validatorID, pubKey)
}
