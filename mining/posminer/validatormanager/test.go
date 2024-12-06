package validatormanager

import (
	"encoding/hex"
	"time"

	"github.com/sat20-labs/satsnet_btcd/chaincfg/chainhash"
	"github.com/sat20-labs/satsnet_btcd/mining/posminer/epoch"
	"github.com/sat20-labs/satsnet_btcd/mining/posminer/validatechain"
)

func (vm *ValidatorManager) Testing() {
	vm.TestingVcState()
	vm.TestingVcBlocks()
}

func (vm *ValidatorManager) TestingVcState() {
	testingHeight := int64(3)
	testinghash, _ := chainhash.NewHashFromStr("5d2cffd29005647d364898a47ce26e2a1e6b7779d9e81b721ed078790f66d8f6")

	currentState := vm.validateChian.GetCurrentState()
	log.Debugf("Current State: %v", currentState)

	if currentState.LatestHeight == testingHeight && currentState.LatestHash.IsEqual(testinghash) {
		log.Debugf("Testing successed: %v", currentState.LatestHash)

		return
	}
	currentState.LatestHeight = testingHeight
	currentState.LatestHash = *testinghash
	vm.validateChian.UpdateCurrentState(currentState)

	log.Debugf("Testing updated.")
}

func (vm *ValidatorManager) TestingVcBlocks() {

	validatorId1 := uint64(10000020)
	publicKey1, _ := hex.DecodeString("0319f86fa35ef9bcfdca01de56cf65833a2af81c299a40dff88fe7874cb1423d02")

	validatorId2 := uint64(10000103)
	publicKey2, _ := hex.DecodeString("020b7a4bab178b0534386a3cc0439a9e94dfe80270e7c907adb337d3a12d2ff2ca")

	validatorId3 := uint64(10000104)
	publicKey3, _ := hex.DecodeString("02a4e1fd1819b5d6e1b85c0e8959e15f1d532c0f8a087203fdaae81f6191475b18")

	// New Epoch block
	newEpochBlock := &validatechain.VCBlock{}
	newEpochBlock.Header.Height = 0
	newEpochBlock.Header.Version = validatechain.Version_ValidateChain
	newEpochBlock.Header.DataType = validatechain.DataType_NewEpoch
	newEpochBlock.Header.CreateTime = time.Now().Unix()
	//newEpochBlock.Header.PrevHash = &chainhash.Hash{}

	newEpochData := &validatechain.DataNewEpoch{}
	newEpochData.CreatorId = validatorId1
	copy(newEpochData.PublicKey[:], publicKey1)
	newEpochData.EpochIndex = 0
	newEpochData.CreateTime = time.Now().Unix()
	newEpochData.Reason = validatechain.Reason_EpochCreate
	newEpochData.EpochItemList = make([]epoch.EpochItem, 0)

	item1 := epoch.EpochItem{ValidatorId: validatorId1}
	copy(item1.PublicKey[:], publicKey1)
	item2 := epoch.EpochItem{ValidatorId: validatorId2}
	copy(item2.PublicKey[:], publicKey2)
	item3 := epoch.EpochItem{ValidatorId: validatorId3}
	copy(item3.PublicKey[:], publicKey3)
	newEpochData.EpochItemList = append(newEpochData.EpochItemList, item1)
	newEpochData.EpochItemList = append(newEpochData.EpochItemList, item2)
	newEpochData.EpochItemList = append(newEpochData.EpochItemList, item3)

	voteItem1 := validatechain.EpochVoteItem{ValidatorId: validatorId1}
	hash1, _ := chainhash.NewHashFromStr("cc6d2efbec9792985894c83cb10b4315719cb9fba2dd0e04734e8aec7492bbdb")
	voteItem1.Hash = *hash1
	voteItem2 := validatechain.EpochVoteItem{ValidatorId: validatorId2}
	hash2, _ := chainhash.NewHashFromStr("1043f9c00d79df6de9fd68b0b22189dc2967024ec207e3e33648c2ef358f6e35")
	voteItem2.Hash = *hash2
	voteItem3 := validatechain.EpochVoteItem{ValidatorId: validatorId3}
	hash3, _ := chainhash.NewHashFromStr("dc2070edf24b29bfc307e1ca1a9357dd7304e0f7cdc654e2c54d28e456f69d1c")
	voteItem3.Hash = *hash3
	newEpochData.EpochVoteList = append(newEpochData.EpochVoteList, voteItem1)
	newEpochData.EpochVoteList = append(newEpochData.EpochVoteList, voteItem2)
	newEpochData.EpochVoteList = append(newEpochData.EpochVoteList, voteItem3)
	newEpochBlock.Data = newEpochData
	blockHash, err := newEpochBlock.GetHash()
	if err != nil {
		log.Errorf("Testing NewEpochBlock Hash failed: %v", err)
		return
	}

	log.Debugf("Testing NewEpochBlock Hash: %x", blockHash)

	err = vm.validateChian.SaveVCBlock(newEpochBlock)
	if err != nil {
		log.Errorf("Testing NewEpochBlock SaveBlock failed: %v", err)
		return
	}

	newBlock, err := vm.validateChian.GetVCBlock(blockHash)
	if err != nil {
		log.Errorf("Testing NewEpochBlock GetBlock failed: %v", err)
		return
	}

	logBlock(newBlock)

	log.Debugf("Testing TestingVcBlocks completed.")
}

func logBlock(block *validatechain.VCBlock) {
	log.Debugf("-------------------------Block header-------------------------")
	log.Debugf("Block Height: %d", block.Header.Height)
	log.Debugf("Block Hash: %s", block.Header.Hash.String())
	log.Debugf("Block Version: %d", block.Header.Version)
	log.Debugf("Block DataType: %d", block.Header.DataType)
	timeStr := time.Unix(block.Header.CreateTime, 0).Format("2006-01-02 15:04:05")
	log.Debugf("Block CreateTime: %s", timeStr)
	log.Debugf("Block PrevHash: %s", block.Header.PrevHash.String())

	switch vcd := block.Data.(type) { //nolint:gocritice := vcd.Data.(type)
	case *validatechain.DataNewEpoch:
		log.Debugf("-------------------------Block Data-------------------------")
		log.Debugf("CreatorId: %d", vcd.CreatorId)
		log.Debugf("PublicKey: %x", vcd.PublicKey[:]) //PublicKey
		log.Debugf("EpochIndex: %d", vcd.EpochIndex)
		timeStr = time.Unix(vcd.CreateTime, 0).Format("2006-01-02 15:04:05")
		log.Debugf("CreateTime: %s", timeStr)
		log.Debugf("Reason: %d", vcd.Reason)
		log.Debugf("EpochItemList Count: %d", len(vcd.EpochItemList))
		for _, item := range vcd.EpochItemList {
			log.Debugf("	validator ID: %d", item.ValidatorId)
			log.Debugf("	validator Public: %x", item.PublicKey[:])
			log.Debugf("------------------------------------------------")
		}
		log.Debugf("EpochVoteList Count: %d", len(vcd.EpochVoteList))
		for _, item := range vcd.EpochVoteList {
			log.Debugf("	validator ID: %d", item.ValidatorId)
			log.Debugf("	vote hash: %s", item.Hash.String())
			log.Debugf("------------------------------------------------")
		}
	case *validatechain.DataUpdateEpoch:
	case *validatechain.DataGeneratorHandOver:
	case *validatechain.DataMinerNewBlock:
	}

}
