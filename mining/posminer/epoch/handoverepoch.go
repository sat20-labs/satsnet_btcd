package epoch

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/sat20-labs/satsnet_btcd/btcec/ecdsa"
)

type HandOverEpoch struct {
	// Request validator id
	ValidatorId    uint64
	Timestamp      int64  // The time of generator hand over, it should be time of completed miner last block
	Token          string // The token for epoch handover, it sign by current generator (HandOver), if the type is New Epoch, it is signed by Epoch Requester (Epoch member)
	NextEpochIndex uint32 // Next epoch index
	NextHeight     int32  // The next block height
}

func (he *HandOverEpoch) GetNextEpochTokenData() []byte {

	// Next epoch Token Data format: "satsnet:nextepoch:validatorid:nextepochIndex:nextheight:timestamp"
	tokenData := fmt.Sprintf("satsnet:nextepoch:%d:%d:%d:%d", he.ValidatorId, he.NextEpochIndex, he.NextHeight, he.Timestamp)
	tokenSource := sha256.Sum256([]byte(tokenData))
	//return hex.EncodeToString(tokenSource[:])

	return tokenSource[:]
}

func (he *HandOverEpoch) VerifyToken(pubKey []byte) bool {
	signatureBytes, err := base64.StdEncoding.DecodeString(he.Token)
	if err != nil {
		log.Debugf("[HandOverEpoch]VerifyToken: Invalid generator token, ignore it.")
		return false
	}

	tokenData := he.GetNextEpochTokenData()

	publicKey, err := secp256k1.ParsePubKey(pubKey[:])

	// 解析签名
	// signature, err := btcec.ParseDERSignature(signatureBytes)
	signature, err := ecdsa.ParseDERSignature(signatureBytes)
	if err != nil {
		log.Debugf("Failed to parse signature: %v", err)
		return false
	}

	// 使用公钥验证签名
	valid := signature.Verify(tokenData, publicKey)
	if valid {
		log.Debugf("[HandOverEpoch]VerifyToken:Signature is valid.")
		return true
	} else {
		log.Debugf("[HandOverEpoch]VerifyToken:Signature is invalid.")
		return false
	}

}
