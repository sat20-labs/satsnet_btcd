package localvalidator

import (
	"errors"


	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/sat20-labs/satsnet_btcd/chaincfg"
	"github.com/sat20-labs/satsnet_btcd/stp"
)

type ValidatorKey struct {
	publicKey  *secp256k1.PublicKey
}

func InitValidatorKey(path string, netParams *chaincfg.Params) (*ValidatorKey, error) {

	publicKey, err := stp.GetPubKey()
	if err != nil {
		return nil, err
	}

	pubKey, err := secp256k1.ParsePubKey(publicKey)
	if err != nil {
		return nil, err
	}

	vlildatorKey := &ValidatorKey{
		publicKey:  pubKey,
	}

	return vlildatorKey, nil
}

func (v *ValidatorKey) GetPublicKey() ([]byte, error) {

	if v.publicKey == nil {
		return nil, errors.New("invalid public key")
	}

	return v.publicKey.SerializeCompressed(), nil
}

func (v *ValidatorKey) SignData(data []byte) ([]byte, error) {
	return stp.SignMsg(data)
}
