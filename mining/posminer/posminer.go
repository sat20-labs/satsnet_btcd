// Copyright (c) 2014-2016 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package posminer

import (
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/sat20-labs/satoshinet/blockchain"
	"github.com/sat20-labs/satoshinet/btcutil"
	"github.com/sat20-labs/satoshinet/chaincfg"
	"github.com/sat20-labs/satoshinet/chaincfg/chainhash"
	"github.com/sat20-labs/satoshinet/mining"
	"github.com/sat20-labs/satoshinet/mining/posminer/utils"
	"github.com/sat20-labs/satoshinet/mining/posminer/validatechaindb"
	"github.com/sat20-labs/satoshinet/mining/posminer/validatormanager"
	"github.com/sat20-labs/satoshinet/wire"
)

const (
	// maxNonce is the maximum value a nonce can be in a block header.
	maxNonce = ^uint32(0) // 2^32 - 1

	// maxExtraNonce is the maximum value an extra nonce used in a coinbase
	// transaction can be.
	maxExtraNonce = ^uint64(0) // 2^64 - 1

	// hpsUpdateSecs is the number of seconds to wait in between each
	// update to the hashes per second monitor.
	hpsUpdateSecs = 10

	// hashUpdateSec is the number of seconds each worker waits in between
	// notifying the speed monitor with how many hashes have been completed
	// while they are actively searching for a solution.  This is done to
	// reduce the amount of syncs between the workers that must be done to
	// keep track of the hashes per second.
	hashUpdateSecs = 15

	// hashUpdateSec is the number of seconds each worker waits in between
	// notifying the speed monitor with how many hashes have been completed
	// while they are actively searching for a solution.  This is done to
	// reduce the amount of syncs between the workers that must be done to
	// keep track of the hashes per second.
	blockGenerateSecs = 60
)

var (
	// defaultNumWorkers is the default number of workers to use for mining
	// and is based on the number of processor cores.  This helps ensure the
	// system stays reasonably responsive under heavy load.
	//defaultNumWorkers = uint32(runtime.NumCPU())
	defaultNumWorkers = uint32(1) // only one worker for a node
)

// Config is a descriptor containing the cpu miner configuration.
type Config struct {
	// ChainParams identifies which chain parameters the cpu miner is
	// associated with.
	ChainParams *chaincfg.Params

	// Dial connects to the address on the named network. It cannot be nil.
	Dial func(net.Addr) (net.Conn, error)

	// Lookup returns the DNS host lookup function to use when
	// connecting to servers.
	Lookup func(string) ([]net.IP, error)

	// Btcd Dir
	BtcdDir string

	// BlockTemplateGenerator identifies the instance to use in order to
	// generate block templates that the miner will attempt to solve.
	BlockTemplateGenerator *mining.BlkTmplGenerator

	// MiningAddrs is the payment addresses to use for the generated blocks. 
	MiningAddr btcutil.Address
	MiningPubKey []byte

	TimerGenerate        bool 

	// ProcessBlock defines the function to call with any solved blocks.
	// It typically must run the provided block through the same set of
	// rules and handling as any other block coming from the network.
	ProcessBlock func(*btcutil.Block, blockchain.BehaviorFlags) (bool, error)

	// OnNewBlockMined is a notify message from posminer for notifying the satsnet
	// when a new block mined and record it in validatechain.
	OnNewBlockMined func(blockHash *chainhash.Hash, blockHeight int32)

	// ConnectedCount defines the function to use to obtain how many other
	// peers the server is connected to.  This is used by the automatic
	// persistent mining routine to determine whether or it should attempt
	// mining.  This is useful because there is no point in mining when not
	// connected to any peers since there would no be anyone to send any
	// found blocks to.
	ConnectedCount func() int32

	// IsCurrent defines the function to use to obtain whether or not the
	// block chain is current.  This is used by the automatic persistent
	// mining routine to determine whether or it should attempt mining.
	// This is useful because there is no point in mining if the chain is
	// not current since any solved blocks would be on a side chain and and
	// up orphaned anyways.
	IsCurrent func() bool

	// ValidatorId is the validator id for this node
	ValidatorId uint64
}

// POSMiner provides facilities for solving blocks (mining) using the POS in
// a concurrency-safe manner.  It consists of two main goroutines -- a speed
// monitor and a controller for worker goroutines which generate and solve
// blocks.  The number of goroutines can be set via the SetMaxGoRoutines
// function, but the default is based on the number of processor cores in the
// system which is typically sufficient.
type POSMiner struct {
	sync.Mutex
	g                 *mining.BlkTmplGenerator
	cfg               Config
	numWorkers        uint32
	started           bool
	discreteMining    bool
	submitBlockLock   sync.Mutex
	wg                sync.WaitGroup
	workerWg          sync.WaitGroup
	updateNumWorkers  chan struct{}
	queryHashesPerSec chan float64
	updateHashes      chan uint64
	speedMonitorQuit  chan struct{}
	quit              chan struct{}

	ValidatorMgr *validatormanager.ValidatorManager
}

// speedMonitor handles tracking the number of hashes per second the mining
// process is performing.  It must be run as a goroutine.
func (m *POSMiner) speedMonitor() {
	utils.Log.Tracef("POS miner speed monitor started")

	var hashesPerSec float64
	var totalHashes uint64
	ticker := time.NewTicker(time.Second * hpsUpdateSecs)
	defer ticker.Stop()

out:
	for {
		select {
		// Periodic updates from the workers with how many hashes they
		// have performed.
		case numHashes := <-m.updateHashes:
			totalHashes += numHashes

		// Time to update the hashes per second.
		case <-ticker.C:
			curHashesPerSec := float64(totalHashes) / hpsUpdateSecs
			if hashesPerSec == 0 {
				hashesPerSec = curHashesPerSec
			}
			hashesPerSec = (hashesPerSec + curHashesPerSec) / 2
			totalHashes = 0
			if hashesPerSec != 0 {
				utils.Log.Debugf("Hash speed: %6.0f kilohashes/s",
					hashesPerSec/1000)
			}

		// Request for the number of hashes per second.
		case m.queryHashesPerSec <- hashesPerSec:
			// Nothing to do.

		case <-m.speedMonitorQuit:
			break out
		}
	}

	m.wg.Done()
	utils.Log.Tracef("POS miner speed monitor done")
}

// submitBlock submits the passed block to network after ensuring it passes all
// of the consensus validation rules.
func (m *POSMiner) submitBlock(block *btcutil.Block) bool {
	m.submitBlockLock.Lock()
	defer m.submitBlockLock.Unlock()

	// Ensure the block is not stale since a new block could have shown up
	// while the solution was being found.  Typically that condition is
	// detected and all work on the stale block is halted to start work on
	// a new block, but the check only happens periodically, so it is
	// possible a block was found and submitted in between.
	msgBlock := block.MsgBlock()
	if !msgBlock.Header.PrevBlock.IsEqual(&m.g.BestSnapshot().Hash) {
		utils.Log.Debugf("Block submitted via POS miner with previous "+
			"block %s is stale", msgBlock.Header.PrevBlock)
		return false
	}

	// Process this block using the same rules as blocks coming from other
	// nodes.  This will in turn relay it to the network like normal.
	isOrphan, err := m.cfg.ProcessBlock(block, blockchain.BFNone)
	if err != nil {
		// Anything other than a rule violation is an unexpected error,
		// so log that error as an internal error.
		if _, ok := err.(blockchain.RuleError); !ok {
			utils.Log.Errorf("Unexpected error while processing "+
				"block submitted via POS miner: %v", err)
			return false
		}

		utils.Log.Debugf("Block submitted via POS miner rejected: %v", err)
		return false
	}
	if isOrphan {
		utils.Log.Debugf("Block submitted via POS miner is an orphan")
		return false
	}

	// The block was accepted.
	coinbaseTx := block.MsgBlock().Transactions[0].TxOut[0]
	utils.Log.Infof("Block submitted via POS miner accepted (hash %s, "+
		"amount %v)", block.Hash(), btcutil.Amount(coinbaseTx.Value))
	return true
}

// solveBlock attempts to find some combination of a nonce, extra nonce, and
// current timestamp which makes the passed block hash to a value less than the
// target difficulty.  The timestamp is updated periodically and the passed
// block is modified with all tweaks during this process.  This means that
// when the function returns true, the block is ready for submission.
//
// This function will return early with false when conditions that trigger a
// stale block such as a new block showing up or periodically when there are
// new transactions and enough time has elapsed without finding a solution.
func (m *POSMiner) solveBlock(msgBlock *wire.MsgBlock, blockHeight int32) bool {

	utils.Log.Debugf("solveBlock ...")
	// Choose a random extra nonce offset for this block template and
	// worker.
	enOffset, err := wire.RandomUint64()
	if err != nil {
		utils.Log.Errorf("Unexpected error while generating random "+
			"extra nonce offset: %v", err)
		enOffset = 0
	}

	// Create some convenience variables.
	header := &msgBlock.Header
	// targetDifficulty := blockchain.CompactToBig(header.Bits)

	// // Initial state.
	// lastGenerated := time.Now()
	// lastTxUpdate := m.g.TxSource().LastUpdated()
	//hashesCompleted := uint64(0)

	// Note that the entire extra nonce range is iterated and the offset is
	// added relying on the fact that overflow will wrap around 0 as
	// provided by the Go spec.
	//for extraNonce := uint64(0); extraNonce < maxExtraNonce; extraNonce++ {
	extraNonce, _ := wire.RandomUint64()
	// Update the extra nonce in the block template with the
	// new value by regenerating the coinbase script and
	// setting the merkle root to the new value.
	m.g.UpdateExtraNonce(msgBlock, blockHeight, extraNonce+enOffset)

	// Search through the entire nonce range for a solution while
	// periodically checking for early quit and stale block
	// conditions along with updates to the speed monitor.
	//for i := uint32(0); i <= maxNonce; i++ {
	i, _ := wire.RandomUint64()
	// select {
	// case <-quit:
	// 	return false

	// case <-ticker.C:
	// 	m.updateHashes <- hashesCompleted
	// 	hashesCompleted = 0

	// The current block is stale if the best block
	// has changed.
	best := m.g.BestSnapshot()
	if !header.PrevBlock.IsEqual(&best.Hash) {
		return false
	}

	// The current block is stale if the memory pool
	// has been updated since the block template was
	// generated and it has been at least one
	// minute.
	// if lastTxUpdate != m.g.TxSource().LastUpdated() &&
	// 	time.Now().After(lastGenerated.Add(time.Minute)) {

	// 	return false
	// }

	m.g.UpdateBlockTime(msgBlock)

	// default:
	// 	// Non-blocking select to fall through
	// }

	// Update the nonce and hash the block header.  Each
	// hash is actually a double sha256 (two hashes), so
	// increment the number of hashes completed for each
	// attempt accordingly.
	header.Nonce = uint32(i)
	// hash := header.BlockHash()
	// hashesCompleted += 2

	// The block is solved when the new block hash is less
	// than the target difficulty.  Yay!
	// if blockchain.HashToBig(&hash).Cmp(targetDifficulty) <= 0 {
	// 	m.updateHashes <- hashesCompleted
	// 	return true
	// }
	//}
	//}

	utils.Log.Debugf("solveBlock done.")
	return true
}

// generateBlocks is a worker that is controlled by the miningWorkerController.
// It is self contained in that it creates block templates and attempts to solve
// them while detecting when it is performing stale work and reacting
// accordingly by generating a new block template.  When a block is solved, it
// is submitted.
//
// It must be run as a goroutine.
func (m *POSMiner) generateBlocks(quit chan struct{}) {
	utils.Log.Tracef("Starting generate blocks worker")

	// Start a ticker which is used to signal checks for stale work and
	// updates to the speed monitor.
	ticker := time.NewTicker(time.Second * blockGenerateSecs)
	defer ticker.Stop()
out:
	for {
		utils.Log.Debugf("generateBlocks ......")
		// Quit when the miner is stopped.
		select {
		case <-quit:
			break out
		case <-ticker.C:
			utils.Log.Debugf("Timeup for generate new Block ......")
			// Wait until there is a connection to at least one other peer
			// since there is no way to relay a found block or receive
			// transactions to work on when there are no connected peers.
			// if m.cfg.ConnectedCount() == 0 {
			// 	time.Sleep(time.Second)
			// 	continue
			// }

			// No point in searching for a solution before the chain is
			// synced.  Also, grab the same lock as used for block
			// submission, since the current block will be changing and
			// this would otherwise end up building a new block template on
			// a block that is in the process of becoming stale.
			m.submitBlockLock.Lock()
			utils.Log.Debugf("Lock block ...")
			curHeight := m.g.BestSnapshot().Height
			if curHeight != 0 && !m.cfg.IsCurrent() {
				m.submitBlockLock.Unlock()
				time.Sleep(time.Second)
				utils.Log.Debugf("curHeight = %d and not current.", curHeight)
				continue
			}

			// Choose a payment address at random.
			// rand.Seed(time.Now().UnixNano())
			// payToAddr := m.cfg.MiningAddrs[rand.Intn(len(m.cfg.MiningAddrs))]
			payToAddr := m.cfg.MiningAddr

			// Create a new block template using the available transactions
			// in the memory pool as a source of transactions to potentially
			// include in the block.
			utils.Log.Debugf("NewBlockTemplate...")
			template, err := m.g.NewBlockTemplate(payToAddr)
			m.submitBlockLock.Unlock()
			if err != nil {
				errStr := fmt.Sprintf("Failed to create new block "+
					"template: %v", err)
				utils.Log.Errorf(errStr)
				continue
			}

			utils.Log.Debugf("NewBlockTemplate done.")

			// Attempt to solve the block.  The function will exit early
			// with false when conditions that trigger a stale block, so
			// a new block template can be generated.  When the return is
			// true a solution was found, so submit the solved block.
			if m.solveBlock(template.Block, curHeight+1) {
				utils.Log.Debugf("solveBlock ...")
				utils.Log.Debugf("Block Header MerkleRoot is %s。", template.Block.Header.MerkleRoot.String())
				block := btcutil.NewBlock(template.Block)
				m.submitBlock(block)
			}
			//default:
			// Non-blocking select to fall through
		}
	}

	m.workerWg.Done()
	utils.Log.Tracef("Generate blocks worker done")
}

// miningWorkerController launches the worker goroutines that are used to
// generate block templates and solve them.  It also provides the ability to
// dynamically adjust the number of running worker goroutines.
//
// It must be run as a goroutine.
func (m *POSMiner) miningWorkerController() {
	// launchWorkers groups common code to launch a specified number of
	// workers for generating blocks.
	var runningWorkers []chan struct{}
	launchWorkers := func(numWorkers uint32) {
		for i := uint32(0); i < numWorkers; i++ {
			quit := make(chan struct{})
			runningWorkers = append(runningWorkers, quit)

			m.workerWg.Add(1)
			go m.generateBlocks(quit)
		}
	}

	// Launch the current number of workers by default.
	runningWorkers = make([]chan struct{}, 0, m.numWorkers)
	launchWorkers(m.numWorkers)

out:
	for {
		select {
		// Update the number of running workers.
		case <-m.updateNumWorkers:
			// No change.
			numRunning := uint32(len(runningWorkers))
			if m.numWorkers == numRunning {
				continue
			}

			// Add new workers.
			if m.numWorkers > numRunning {
				launchWorkers(m.numWorkers - numRunning)
				continue
			}

			// Signal the most recently created goroutines to exit.
			for i := numRunning - 1; i >= m.numWorkers; i-- {
				close(runningWorkers[i])
				runningWorkers[i] = nil
				runningWorkers = runningWorkers[:i]
			}

		case <-m.quit:
			utils.Log.Debugf("miningWorkerController quit: %d", len(runningWorkers))
			for index, quit := range runningWorkers {
				close(quit)
				utils.Log.Debugf("miningWorkerController quit: %d done", index)
			}
			break out
		}
	}

	// Wait until all workers shut down to stop the speed monitor since
	// they rely on being able to send updates to it.
	utils.Log.Debugf("Wait workerWg done...")

	m.workerWg.Wait()
	//close(m.speedMonitorQuit)
	m.wg.Done()

	utils.Log.Debugf("miningWorkerController done")
}

// Start begins the POS mining process as well as the speed monitor used to
// track hashing metrics.  Calling this function when the POS miner has
// already been started will have no effect.
//
// This function is safe for concurrent access.
func (m *POSMiner) Start() {
	m.Lock()
	defer m.Unlock()

	// Nothing to do if the miner is already running or if running in
	// discrete mode (using GenerateNBlocks).
	if m.started || m.discreteMining {
		return
	}

	m.quit = make(chan struct{})
	if m.cfg.TimerGenerate {
		utils.Log.Infof("POS miner started with timerGenerate")
		m.wg.Add(1)
		go m.miningWorkerController()
	}

	cfg := &validatormanager.Config{
		ChainParams: m.cfg.ChainParams,
		Dial:        m.cfg.Dial,
		Lookup:      m.cfg.Lookup,
		ValidatorId: m.cfg.ValidatorId,
		ValidatorPubKey: m.cfg.MiningPubKey,
		BtcdDir:     m.cfg.BtcdDir,
		PosMiner:    m,
	}
	// Start ValidatorManager
	m.ValidatorMgr = validatormanager.New(cfg)
	if m.ValidatorMgr != nil {
		m.ValidatorMgr.Start()
	}

	m.started = true
	utils.Log.Infof("POS miner started")
}

// Stop gracefully stops the mining process by signalling all workers, and the
// speed monitor to quit.  Calling this function when the POS miner has not
// already been started will have no effect.
//
// This function is safe for concurrent access.
func (m *POSMiner) Stop() {
	m.Lock()
	defer m.Unlock()

	// Nothing to do if the miner is not currently running or if running in
	// discrete mode (using GenerateNBlocks).
	if !m.started || m.discreteMining {
		return
	}

	if m.ValidatorMgr != nil {
		m.ValidatorMgr.Stop()
	}

	close(m.quit)
	utils.Log.Infof("Wait wg done")
	m.wg.Wait()
	m.started = false
	utils.Log.Infof("POS miner stopped")
}

// IsMining returns whether or not the POS miner has been started and is
// therefore currenting mining.
//
// This function is safe for concurrent access.
func (m *POSMiner) IsMining() bool {
	m.Lock()
	defer m.Unlock()

	return m.started
}

// HashesPerSecond returns the number of hashes per second the mining process
// is performing.  0 is returned if the miner is not currently running.
//
// This function is safe for concurrent access.
func (m *POSMiner) HashesPerSecond() float64 {
	m.Lock()
	defer m.Unlock()

	// Nothing to do if the miner is not currently running.
	if !m.started {
		return 0
	}

	return <-m.queryHashesPerSec
}

// SetNumWorkers sets the number of workers to create which solve blocks.  Any
// negative values will cause a default number of workers to be used which is
// based on the number of processor cores in the system.  A value of 0 will
// cause all POS mining to be stopped.
//
// This function is safe for concurrent access.
func (m *POSMiner) SetNumWorkers(numWorkers int32) {
	if numWorkers == 0 {
		m.Stop()
	}

	// Don't lock until after the first check since Stop does its own
	// locking.
	m.Lock()
	defer m.Unlock()

	// Use default if provided value is negative.
	if numWorkers < 0 {
		m.numWorkers = defaultNumWorkers
	} else {
		m.numWorkers = uint32(numWorkers)
	}

	// When the miner is already running, notify the controller about the
	// the change.
	if m.started {
		m.updateNumWorkers <- struct{}{}
	}
}

// NumWorkers returns the number of workers which are running to solve blocks.
//
// This function is safe for concurrent access.
func (m *POSMiner) NumWorkers() int32 {
	m.Lock()
	defer m.Unlock()

	return int32(m.numWorkers)
}

// GenerateNBlocks generates the requested number of blocks. It is self
// contained in that it creates block templates and attempts to solve them while
// detecting when it is performing stale work and reacting accordingly by
// generating a new block template.  When a block is solved, it is submitted.
// The function returns a list of the hashes of generated blocks.
func (m *POSMiner) GenerateNBlocks(n uint32) ([]*chainhash.Hash, error) {
	m.Lock()

	// Respond with an error if server is already mining.
	if m.started || m.discreteMining {
		m.Unlock()
		return nil, errors.New("Server is already POS mining. Please call " +
			"`setgenerate 0` before calling discrete `generate` commands.")
	}

	m.started = true
	m.discreteMining = true

	//m.speedMonitorQuit = make(chan struct{})
	//m.wg.Add(1)
	//go m.speedMonitor()

	m.Unlock()

	utils.Log.Tracef("Generating %d blocks", n)

	i := uint32(0)
	blockHashes := make([]*chainhash.Hash, n)

	// Start a ticker which is used to signal checks for stale work and
	// updates to the speed monitor.
	ticker := time.NewTicker(time.Second * hashUpdateSecs)
	defer ticker.Stop()

	for {
		// Read updateNumWorkers in case someone tries a `setgenerate` while
		// we're generating. We can ignore it as the `generate` RPC call only
		// uses 1 worker.
		select {
		case <-m.updateNumWorkers:
		default:
		}

		// Grab the lock used for block submission, since the current block will
		// be changing and this would otherwise end up building a new block
		// template on a block that is in the process of becoming stale.
		m.submitBlockLock.Lock()
		curHeight := m.g.BestSnapshot().Height

		// Choose a payment address at random.
		// rand.Seed(time.Now().UnixNano())
		// payToAddr := m.cfg.MiningAddrs[rand.Intn(len(m.cfg.MiningAddrs))]
		payToAddr := m.cfg.MiningAddr

		// Create a new block template using the available transactions
		// in the memory pool as a source of transactions to potentially
		// include in the block.
		template, err := m.g.NewBlockTemplate(payToAddr)
		m.submitBlockLock.Unlock()
		if err != nil {
			errStr := fmt.Sprintf("Failed to create new block "+
				"template: %v", err)
			utils.Log.Errorf(errStr)
			continue
		}

		// Attempt to solve the block.  The function will exit early
		// with false when conditions that trigger a stale block, so
		// a new block template can be generated.  When the return is
		// true a solution was found, so submit the solved block.
		if m.solveBlock(template.Block, curHeight+1) {
			block := btcutil.NewBlock(template.Block)
			m.submitBlock(block)
			blockHashes[i] = block.Hash()
			i++
			if i == n {
				utils.Log.Tracef("Generated %d blocks", i)
				m.Lock()
				//close(m.speedMonitorQuit)
				//m.wg.Wait()
				m.started = false
				m.discreteMining = false
				m.Unlock()
				return blockHashes, nil
			}
		}
	}
}

var (
	lastValidBlockHash chainhash.Hash
)

// New returns a new instance of a POS miner for the provided configuration.
// Use Start to begin the mining process.  See the documentation for POSMiner
// type for more details.
func New(cfg *Config) *POSMiner {
	lastValidBlockHash = cfg.BlockTemplateGenerator.BestSnapshot().Hash // For test
	return &POSMiner{
		g:                 cfg.BlockTemplateGenerator,
		cfg:               *cfg,
		numWorkers:        defaultNumWorkers,
		updateNumWorkers:  make(chan struct{}),
		queryHashesPerSec: make(chan float64),
		updateHashes:      make(chan uint64),
	}
}

// OnTimeGenerateBlock is invoke when time to generate block.
func (m *POSMiner) OnTimeGenerateBlock() (*chainhash.Hash, int32, error) {
	utils.Log.Debugf("Timeup for OnTimeGenerateBlock ......")

	//return m.GenerateNewTestBlock()

	return m.GenerateNewBlock()
}

// OnTimeGenerateBlock is invoke when time to generate block.
func (m *POSMiner) OnNewBlockMined(blockHash *chainhash.Hash, blockHeight int32) {
	utils.Log.Debugf("[POSMiner]OnNewBlockMined ......")

	m.cfg.OnNewBlockMined(blockHash, blockHeight)

	//	lastValidBlockHash = m.cfg.BlockTemplateGenerator.BestSnapshot().Hash // For test
	lastValidBlockHash = *blockHash // For test
}

func (m *POSMiner) GenerateNewTestBlock() (*chainhash.Hash, int32, error) {
	utils.Log.Debugf("GenerateNewTestBlock ......")

	// return nil, errors.New("Test generate failed.")
	curHeight := m.g.BestSnapshot().Height
	if curHeight != 0 && !m.cfg.IsCurrent() {
		time.Sleep(time.Second)
		utils.Log.Debugf("curHeight = %d and not current %d.", curHeight, m.cfg.IsCurrent())
		err := fmt.Errorf("the blockchain is not best chain")
		return nil, 0, err
	}

	currentBlockHash := m.g.BestSnapshot().Hash // Foe test
	if lastValidBlockHash.IsEqual(&currentBlockHash) {
		// No new tx is mind
		err := fmt.Errorf("no any new tx in mempool")
		return nil, 0, err
	}

	lastValidBlockHash = currentBlockHash
	return &currentBlockHash, curHeight, nil

	// blockHash := chainhash.DoubleHashRaw(func(w io.Writer) error {
	// 	buf := make([]byte, 128)
	// 	for i := 0; i < 128; i++ {
	// 		data := rand.Int31()
	// 		buf[i] = byte(data)
	// 	}
	// 	if _, err := w.Write(buf[:]); err != nil {
	// 		return err
	// 	}

	// 	return nil
	// })

	// return &blockHash, curHeight + 1, nil
}

func (m *POSMiner) GenerateNewBlock() (*chainhash.Hash, int32, error) {
	// Wait until there is a connection to at least one other peer
	// since there is no way to relay a found block or receive
	// transactions to work on when there are no connected peers.
	// if m.cfg.ConnectedCount() == 0 {
	// 	time.Sleep(time.Second)
	// 	continue
	// }

	// No point in searching for a solution before the chain is
	// synced.  Also, grab the same lock as used for block
	// submission, since the current block will be changing and
	// this would otherwise end up building a new block template on
	// a block that is in the process of becoming stale.
	utils.Log.Debugf("GenerateNewBlock by VC ...")
	m.submitBlockLock.Lock()
	utils.Log.Debugf("Lock block ...")
	curHeight := m.g.BestSnapshot().Height
	if curHeight != 0 && !m.cfg.IsCurrent() {
		m.submitBlockLock.Unlock()
		time.Sleep(time.Second)
		utils.Log.Debugf("curHeight = %d and not current.", curHeight)
		err := fmt.Errorf("The blockchain is not best chain.")
		return nil, 0, err
	}

	// Choose a payment address at random.
	// rand.Seed(time.Now().UnixNano())
	// payToAddr := m.cfg.MiningAddrs[rand.Intn(len(m.cfg.MiningAddrs))]
	payToAddr := m.cfg.MiningAddr

	// Create a new block template using the available transactions
	// in the memory pool as a source of transactions to potentially
	// include in the block.
	utils.Log.Debugf("NewBlockTemplate...")
	template, err := m.g.NewBlockTemplate(payToAddr)
	m.submitBlockLock.Unlock()
	if err != nil {
		errStr := fmt.Sprintf("Failed to create new block "+
			"template: %v", err)
		utils.Log.Errorf(errStr)
		return nil, 0, err
	}

	utils.Log.Debugf("NewBlockTemplate done.")

	// Attempt to solve the block.  The function will exit early
	// with false when conditions that trigger a stale block, so
	// a new block template can be generated.  When the return is
	// true a solution was found, so submit the solved block.
	if m.solveBlock(template.Block, curHeight+1) {
		utils.Log.Debugf("solveBlock ...")
		block := btcutil.NewBlock(template.Block)
		m.submitBlock(block)
		blockHash := block.Hash()
		return blockHash, curHeight + 1, nil
	}

	err = fmt.Errorf("Failed to solve block")
	return nil, 0, err
}

// GetBlockHeight invoke when get block height from pos miner.
func (m *POSMiner) GetBlockHeight() int32 {
	return m.g.BestSnapshot().Height
}

func (m *POSMiner) GetVCStore() *validatechaindb.ValidateChainStore {
	if m.ValidatorMgr == nil {
		return nil
	}
	return m.ValidatorMgr.GetVCStore()
}

func (m *POSMiner) GetCurrentEpochMember(includeSelf bool) ([]string, error) {
	if m.ValidatorMgr == nil {
		return nil, errors.New("Validator Manager is nil")
	}
	return m.ValidatorMgr.GetCurrentEpochMember(includeSelf)
}

func (m *POSMiner) GetMempoolTxSize() int32 {

	if m.g == nil {
		utils.Log.Debugf("[PosMiner] Invalid mempool generator.")
		return 0
	}
	txSource := m.g.TxSource()
	if txSource == nil {
		utils.Log.Debugf("[PosMiner] Invalid mempool tx source.")
		return 0
	}
	sourceTxns := txSource.MiningDescs()

	txSize := len(sourceTxns)

	utils.Log.Debugf("[PosMiner] Current mempool tx size = %d", txSize)

	return int32(txSize)
}
