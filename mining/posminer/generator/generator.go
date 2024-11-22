package generator

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/sat20-labs/satsnet_btcd/btcec/ecdsa"
	"github.com/sat20-labs/satsnet_btcd/chaincfg/chainhash"
	"github.com/sat20-labs/satsnet_btcd/mining/posminer/validatorinfo"
)

type MinerInterface interface {
	// OnTimeGenerateBlock is invoke when time to generate block.
	OnTimeGenerateBlock() (*chainhash.Hash, error)
}

const (
	// If the generator id is NoGeneratorId, it means the generator is not saved in the peer
	NoGeneratorId = uint64(0xffffffffffffffff)
	MinerInterval = 40 * time.Second
)

type Generator struct {
	GeneratorId   uint64 // The validator id of generator
	Height        int32  // The block height for the generator
	Timestamp     int64  // The time of generator created
	Token         string // The token for the generator, it signed by generate
	Validatorinfo *validatorinfo.ValidatorInfo
	MinerTime     time.Time

	LocalMiner MinerInterface
}

const (
	HandOverTypeByEpochOrder = 0
	HandOverTypeByVote       = 1
)

type GeneratorHandOver struct {
	ValidatorId  uint64 // The current validator id (current generator, or voter)
	HandOverType int32  // HandOverType: 0: HandOver by current generator with Epoch member Order, 1: Vote by Epoch member
	Timestamp    int64  // The time of generator hand over
	Token        string // The token for generator handover, it sign by current generator (HandOver), if the type is vote, it is signed by voter
	GeneratorId  uint64 // The next validator id (next generator)
	Height       int32  // The next block height
}

func NewGenerator(validatorInfo *validatorinfo.ValidatorInfo, height int32, timestamp int64, token string) *Generator {
	if timestamp == 0 {
		timestamp = time.Now().Unix()
	}
	generator := &Generator{
		GeneratorId:   validatorInfo.ValidatorId,
		Height:        height,
		Timestamp:     timestamp,
		Token:         token,
		Validatorinfo: validatorInfo,
	}

	return generator
}

func (g *Generator) GetTokenData() []byte {

	// Token Data format: "satsnet:height:validatorid:timestamp"
	tokenData := fmt.Sprintf("satsnet:%d:%d:%d", g.Height, g.GeneratorId, g.Timestamp)
	tokenSource := sha256.Sum256([]byte(tokenData))
	return tokenSource[:]
}

func (g *Generator) SetToken(token string) {
	g.Token = token
}

func (g *Generator) VerifyToken(pubKey []byte) bool {
	signatureBytes, err := base64.StdEncoding.DecodeString(g.Token)
	if err != nil {
		log.Debugf("OnGeneratorUpdated: Invalid generator token, ignore it.")
		return false
	}

	tokenData := g.GetTokenData()

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
		log.Debugf("Signature is valid.")
		return true
	} else {
		log.Debugf("Signature is invalid.")
		return false
	}

}

func (g *Generator) GetNextGeneratorTokenData(handover *GeneratorHandOver) string {

	// Next Generator Token Data format: "satsnet:handovertype:validatorid:height:generatorid:timestamp"
	tokenData := fmt.Sprintf("satsnet:%d:%d:%d:%d:%d", handover.HandOverType, handover.ValidatorId, handover.Height, handover.GeneratorId, handover.Timestamp)
	tokenSource := sha256.Sum256([]byte(tokenData))
	return hex.EncodeToString(tokenSource[:])
}

func (g *Generator) SetHandOverTime(handOverTime time.Time) error {
	log.Debugf("[Generator]SetHandOverTime ...")
	now := time.Now()
	minerTime := handOverTime.Add(MinerInterval)
	if minerTime.After(now) == false {
		return fmt.Errorf("miner time is before now")
	}
	g.MinerTime = minerTime
	go g.minerHandler()
	return nil
}
func (g *Generator) ContinueNextSlot() error {
	log.Debugf("[Generator]ContinueNextSlot ...")

	now := time.Now()
	newMinerTime := g.MinerTime.Add(MinerInterval)
	if newMinerTime.After(now) == false {
		return fmt.Errorf("new miner time is before now")
	}
	g.MinerTime = newMinerTime
	go g.minerHandler()
	return nil
}

func (g *Generator) SetLocalMiner(localMiner MinerInterface) {
	g.LocalMiner = localMiner
}

func (g *Generator) MinerNewBlock() {
	log.Debugf("##################################################################")
	log.Debugf("[Generator]MinerNewBlock...")
	log.Debugf("[Generator]Miner time: %v", time.Now().Format("2006-01-02 15:04:05"))
	log.Debugf("[Generator]Miner height: %d", g.Height)
	if g.LocalMiner != nil {
		log.Debugf("[Generator]Call localMiner to generate a new block...")
		g.LocalMiner.OnTimeGenerateBlock()
	}
	log.Debugf("##################################################################")
}

func (g *Generator) minerHandler() {
	log.Debugf("[Generator]minerHandler start...")

	exitMinerHandler := make(chan struct{})
	log.Debugf("[Generator]Set Miner <height:%d> Timer at %s ", g.Height, g.MinerTime.Format("2006-01-02 15:04:05"))

	minerDuration := g.MinerTime.Sub(time.Now())
	time.AfterFunc(minerDuration, func() {
		g.MinerNewBlock()
		exitMinerHandler <- struct{}{}
	})

	// 这里阻塞主 goroutine 等待任务执行（可根据需要改为其他逻辑）
	select {
	case exitMinerHandler <- struct{}{}:
		log.Debugf("[Generator]minerHandler done .")
		return
	}
}
