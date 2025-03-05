package indexer

import (
	indexer "github.com/sat20-labs/indexer/common"
	"github.com/sat20-labs/satsnet_btcd/chaincfg"
	"github.com/sat20-labs/satsnet_btcd/indexer/common"
)

type Indexer interface {

	Start() error
	Stop()

	IsMainnet() bool
	GetChainParam() *chaincfg.Params
	GetBaseDBVer() string
	GetChainTip() int
	GetSyncHeight() int
	GetBlockInfo(int) (*common.BlockInfo, error)

	// base indexer
	GetAddressById(addressId uint64) string
	GetAddressId(address string) uint64
	GetUtxoById(utxoId uint64) string
	GetUtxoId(utxo string) uint64
	// return: utxoId->value
	GetUTXOsWithAddress(address string) (map[uint64]int64, error)
	// return: utxo, sat ranges

	GetBindingSat(ticker *common.TickerName) int
	// Asset
	// return: tick->amount
	GetAssetSummaryInAddress(address string) map[common.TickerName]int64
	GetAssetSummaryInAddressV3(address string) map[common.TickerName]*indexer.Decimal
	// return: tick->UTXOs
	GetAssetUTXOsInAddress(address string) map[common.TickerName][]*common.TxOutput
	// return: utxo->asset amount
	GetAssetUTXOsInAddressWithTick(address string, tickerName *common.TickerName) (map[uint64]*common.TxOutput, error)
	GetAssetUTXOsInAddressWithTickV3(address string, ticker *common.TickerName) (map[uint64]*indexer.AssetsInUtxo, error)
	HasAssetInUtxo(utxo string) bool
	GetTxOutputWithUtxo(utxo string) *common.TxOutput
	GetTxOutputWithUtxoV3(utxo string) *indexer.AssetsInUtxo
	GetAscendData(fundingUtxo string) *common.AscendData
	GetDescendData(nullDataUtxo string) *common.DescendData
	GetAllCoreNode() map[string]int
	IsCoreNode(pubkey string) bool
}
