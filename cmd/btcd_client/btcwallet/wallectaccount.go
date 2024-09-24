package btcwallet

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type WalletAccount struct {
	WalletName string `json:"WalletName"`
	Mnemonic   string `json:"Mnemonic"`
	PublicKey  []byte `json:"PublicKey"`
	PrivateKey []byte `json:"PrivateKey"`
	WalletInfo *BTCWalletInfo
}
type WalletAccountManager struct {
	DefaultWallet     string
	WalletCount       int              `json:"WalletCount"`
	WalletAccountList []*WalletAccount `json:"WalletAccountList"`
}

const (
	FILENAME_LOCAL_WALLET = "localwallet.json"
)

var (
	accountManager = &WalletAccountManager{}
	homeDir        = ""
)

func LoadLocalWallet(workDir string) error {
	homeDir = workDir

	walletAccountFilename := filepath.Join(homeDir, FILENAME_LOCAL_WALLET)

	if PathExists(walletAccountFilename) == false {
		// The file isnot exists, the collection Data is empty
		accountManager.WalletCount = 0
		accountManager.WalletAccountList = make([]*WalletAccount, 0)
		return nil
	}
	data, err := os.ReadFile(walletAccountFilename)
	if err != nil {
		fmt.Printf("err = %v\n", err)
		return err
	}
	//accountManager := &NFTCollectionData{}

	accountManager.WalletCount = 0
	accountManager.WalletAccountList = make([]*WalletAccount, 0)

	err = json.Unmarshal(data, accountManager)
	if err != nil {
		fmt.Printf("err = %v\n", err)
		return err
	}
	if accountManager.DefaultWallet == "" {
		accountManager.DefaultWallet = accountManager.WalletAccountList[0].WalletName
		saveLocalWallet()
	}
	return nil
}

func saveLocalWallet() error {
	walletAccountFilename := filepath.Join(homeDir, FILENAME_LOCAL_WALLET)

	// if PathExists(walletAccountFilename) == false {
	// 	// The file isnot exists, the collection Data is empty
	// 	return nil
	// }
	data, err := json.Marshal(accountManager)
	if err != nil {
		fmt.Printf("err = %v\n", err)
		return err
	}

	err = os.WriteFile(walletAccountFilename, data, 0644)
	if err != nil {
		fmt.Printf("err = %v\n", err)
		return err
	}

	return nil
}

// PathExists 文件是否存在
func PathExists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	return false
}

func AddChildAccount(walletname string, mnemonic string, publicKey []byte, privateKey []byte) error {
	accountManager.WalletAccountList = append(accountManager.WalletAccountList, &WalletAccount{
		WalletName: walletname,
		Mnemonic:   mnemonic,
		PublicKey:  publicKey,
		PrivateKey: privateKey,
	})

	if accountManager.DefaultWallet == "" {
		// No default wallet, set current wallet is default
		accountManager.DefaultWallet = walletname
	}

	saveLocalWallet()
	return nil
}

func GetUserAccountID(walletName string) ([]byte, error) {
	if accountManager == nil {
		err := fmt.Errorf("accountManager is nil")
		return nil, err
	}

	for _, v := range accountManager.WalletAccountList {
		if v.WalletName == walletName {
			return v.PublicKey, nil
		}
	}

	err := fmt.Errorf("cannot found wallet: %s", walletName)
	return nil, err
}

func GetPrikey(walletName string) ([]byte, error) {
	if accountManager == nil {
		err := fmt.Errorf("accountManager is nil")
		return nil, err
	}

	for _, v := range accountManager.WalletAccountList {
		if v.WalletName == walletName {
			return v.PrivateKey, nil
		}
	}

	err := fmt.Errorf("cannot found wallet: %s", walletName)
	return nil, err
}

func GetAllWalletAccounts() ([]string, error) {
	if accountManager == nil {
		err := fmt.Errorf("accountManager is nil")
		return nil, err
	}
	accounts := make([]string, 0)
	for _, v := range accountManager.WalletAccountList {
		accounts = append(accounts, v.WalletName)
	}
	return accounts, nil
}

func DelChildAccount(walletname string) error {
	return nil
}

func GetDefaultWalletName() string {
	return accountManager.DefaultWallet
}

func SetWalletInfo(walletName string, WalletInfo *BTCWalletInfo) error {
	if accountManager == nil {
		err := fmt.Errorf("accountManager is nil")
		return err
	}

	for _, wallet := range accountManager.WalletAccountList {
		if wallet.WalletName == walletName {
			wallet.WalletInfo = WalletInfo
			saveLocalWallet()
			return nil
		}
	}

	err := fmt.Errorf("cannot found wallet: %s", walletName)
	return err
}

func GetWalletInfo(walletName string) (*BTCWalletInfo, error) {
	if accountManager == nil {
		err := fmt.Errorf("accountManager is nil")
		return nil, err
	}

	for _, wallet := range accountManager.WalletAccountList {
		if wallet.WalletName == walletName {
			return wallet.WalletInfo, nil
		}
	}

	err := fmt.Errorf("cannot found wallet: %s", walletName)
	return nil, err
}
