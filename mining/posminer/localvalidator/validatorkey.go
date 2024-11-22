package localvalidator

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/sat20-labs/satsnet_btcd/btcec/ecdsa"
	"github.com/sat20-labs/satsnet_btcd/btcutil/hdkeychain"
	"github.com/sat20-labs/satsnet_btcd/chaincfg"
	"github.com/tyler-smith/go-bip39"
)

type ValidatorKey struct {
	masterKey  *hdkeychain.ExtendedKey
	privateKey *secp256k1.PrivateKey
	publicKey  *secp256k1.PublicKey
}

const (
	keyName = "validatorKey.dat"
)

func InitValidatorKey(path string, netParams *chaincfg.Params) (*ValidatorKey, error) {

	keyPath := filepath.Join(path, keyName)

	maskerKey, err := loadValidatorKey(keyPath, netParams)
	if err != nil {
		// Local key failed, will create a new key
		maskerKey, err = createValidatorKey(keyPath, netParams)
		if err != nil {
			// Create validator key failed
			err := fmt.Errorf("Create validator key failed: %s", err.Error())
			return nil, err
		}
	}

	privateKey, err := maskerKey.ECPrivKey()
	if err != nil {
		err := errors.New("invalid private key")
		fmt.Println(err.Error())
		return nil, err
	}

	publicKey := privateKey.PubKey()

	vlildatorKey := &ValidatorKey{
		masterKey:  maskerKey,
		privateKey: privateKey,
		publicKey:  publicKey,
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

	if v.privateKey == nil {
		return nil, errors.New("invalid private key")
	}

	signature := ecdsa.Sign(v.privateKey, data)

	return signature.Serialize(), nil
}

func createValidatorKey(path string, netParams *chaincfg.Params) (*hdkeychain.ExtendedKey, error) {

	entropy, err := bip39.NewEntropy(128)
	if err != nil {
		fmt.Println(err.Error())
		return nil, err
	}

	masterKey, err := getValodatorKey(entropy, netParams)
	if err != nil {
		fmt.Println(err.Error())
		return nil, err
	}
	err = saveLocalEntory(path, entropy)
	if err != nil {
		fmt.Println(err.Error())
		return nil, err
	}
	return masterKey, nil
}

func getValodatorKey(entropy []byte, netParams *chaincfg.Params) (*hdkeychain.ExtendedKey, error) {

	mnemonic, err := bip39.NewMnemonic(entropy)
	if err != nil {
		fmt.Println(err.Error())
		return nil, err
	}

	fmt.Printf("mnemonic: %s\n", mnemonic)

	seed := bip39.NewSeed(mnemonic, "") //这里可以选择传入指定密码或者空字符串，不同密码生成的助记词不同

	validatorKey, err := hdkeychain.NewMaster(seed, netParams)
	if err != nil {
		// wallet account is not created
		fmt.Println(err.Error())
		return nil, err
	}

	return validatorKey, nil
}

func loadValidatorKey(path string, netParams *chaincfg.Params) (*hdkeychain.ExtendedKey, error) {

	entropy, err := loadLocalEntory(path)
	if err != nil {
		fmt.Println(err.Error())
		return nil, err
	}
	return getValodatorKey(entropy, netParams)
}

func loadLocalEntory(path string) ([]byte, error) {
	entory, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return entory, nil
}

func saveLocalEntory(path string, entory []byte) error {
	err := os.WriteFile(path, entory, 0600)
	if err != nil {
		return err
	}
	return nil
}
