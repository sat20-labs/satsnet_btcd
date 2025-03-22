package indexer

import (
	"github.com/sat20-labs/satoshinet/chaincfg"
	"github.com/sat20-labs/satoshinet/wire"

	indexer "github.com/sat20-labs/indexer/common"
	"github.com/sat20-labs/satoshinet/indexer/common"
)

// interface for RPC

func (b *IndexerMgr) IsMainnet() bool {
	return b.chaincfgParam.Name == "mainnet"
}

func (b *IndexerMgr) GetBaseDBVer() string {
	return b.compiling.GetBaseDBVer()
}

func (b *IndexerMgr) GetChainParam() *chaincfg.Params {
	return b.chaincfgParam
}

func (b *IndexerMgr) HasAssetInUtxo(utxo string) bool {
	info, err := b.rpcService.GetUtxoInfo(utxo)
	if err != nil {
		return false
	}
	return len(info.Assets) != 0
}

// return: utxoId->asset amount
func (b *IndexerMgr) GetAssetUTXOsInAddressWithTick(address string, ticker *wire.AssetName) (map[uint64]*common.TxOutput, error) {
	utxos, err := b.rpcService.GetUTXOs(address)
	if err != nil {
		return nil, err
	}

	result := make(map[uint64]*common.TxOutput)
	for utxoId := range utxos {
		utxo, err := b.rpcService.GetUtxoByID(utxoId)
		if err != nil {
			continue
		}
		info := b.GetTxOutputWithUtxo(utxo)
		if info == nil {
			continue
		}

		// 因为聪网的灵活性，只要一个utxo中的聪比绑定资产的聪更多，就可以认为该utxo含有白聪
		if ticker == nil {
			result[utxoId] = info
		} else if indexer.IsPlainAsset(ticker) {
			if info.HasPlainSat() {
				result[utxoId] = info
			}
		} else {
			amt := info.GetAsset(ticker)
			if amt.Sign() == 0 {
				continue
			}
			result[utxoId] = info
		}
	}

	return result, nil
}

// return: ticker -> amount
func (b *IndexerMgr) GetAssetSummaryInAddress(address string) map[wire.AssetName]*indexer.Decimal {
	utxos, err := b.rpcService.GetUTXOs(address)
	if err != nil {
		return nil
	}

	value := int64(0)
	result := make(map[wire.AssetName]*indexer.Decimal)
	for utxoId, v := range utxos {
		utxo, err := b.rpcService.GetUtxoByID(utxoId)
		if err != nil {
			continue
		}
		info, err := b.rpcService.GetUtxoInfo(utxo)
		if err != nil {
			continue
		}

		// 白聪资产去除绑定资产的聪
		assetAmt := int64(0)
		if len(info.Assets) != 0 {
			for _, asset := range info.Assets {
				amt, ok := result[asset.Name]
				if ok {
					amt = amt.Add(&asset.Amount)
				} else {
					amt = &asset.Amount
				}
				result[asset.Name] = amt
			}
			assetAmt = info.Assets.GetBindingSatAmout()
		}

		value += (v - assetAmt)
	}
	result[common.ASSET_PLAIN_SAT] = indexer.NewDecimal(value, 0)

	return result
}

// return: ticker -> []utxoId
func (b *IndexerMgr) GetAssetUTXOsInAddress(address string) map[wire.AssetName][]*common.TxOutput {
	utxos, err := b.rpcService.GetUTXOs(address)
	if err != nil {
		return nil
	}

	result := make(map[wire.AssetName][]*common.TxOutput)
	for utxoId := range utxos {
		utxo, err := b.rpcService.GetUtxoByID(utxoId)
		if err != nil {
			continue
		}
		info := b.GetTxOutputWithUtxo(utxo)
		if info == nil {
			continue
		}
		for _, asset := range info.OutValue.Assets {
			result[asset.Name] = append(result[asset.Name], info)
		}
		if len(info.OutValue.Assets) == 0 {
			result[common.ASSET_PLAIN_SAT] = append(result[common.ASSET_PLAIN_SAT], info)
		}
	}

	return result
}

func (b *IndexerMgr) GetTxOutputWithUtxo(utxo string) *common.TxOutput {

	info, err := b.rpcService.GetUtxoInfo(utxo)
	if err != nil {
		return nil
	}

	return &common.TxOutput{
		UtxoId:      info.UtxoId,
		OutPointStr: utxo,
		OutValue: wire.TxOut{
			Value:    info.Value,
			Assets:   info.Assets,
			PkScript: info.PkScript,
		},
	}
}

func (b *IndexerMgr) GetAscendData(fundingUtxo string) *common.AscendData {
	return b.rpcService.GetAscendData(fundingUtxo)
}

func (b *IndexerMgr) GetDescendData(nullDataUtxo string) *common.DescendData {
	return b.rpcService.GetDescendData(nullDataUtxo)
}

func (b *IndexerMgr) GetTickerInfo(ticker *wire.AssetName) *common.TickerInfo {
	return b.rpcService.GetTickerInfo(ticker)
}

func (b *IndexerMgr) GetBindingSat(ticker *wire.AssetName) int {
	info := b.rpcService.GetTickerInfo(ticker)
	if info == nil {
		return 0
	}
	return info.N
}

func (b *IndexerMgr) GetDecimalFromAmt(ticker *wire.AssetName, amt int64) *indexer.Decimal {
	tickerInfo := b.GetTickerInfo(ticker)
	maxSupply, err := indexer.NewDecimalFromString(tickerInfo.MaxSupply, tickerInfo.Precition)
	if err != nil {
		common.Log.Panic("")
	}
	return indexer.NewDecimalFromInt64WithMax(amt, maxSupply)
}

func (b *IndexerMgr) GetAllCoreNode() map[string]int {
	return b.rpcService.GetAllCoreNode()
}

func (b *IndexerMgr) IsCoreNode(pubkey string) bool {
	return b.rpcService.IsCoreNode(pubkey)
}
