package indexer

import (
	"github.com/sat20-labs/indexer/common"

	"github.com/sat20-labs/satsnet_btcd/wire"
	swire "github.com/sat20-labs/satsnet_btcd/wire"
	indexer "github.com/sat20-labs/indexer/common"
)

// return: utxoId->asset
func (b *IndexerMgr) GetAssetUTXOsInAddressWithTickV3(address string, ticker *swire.AssetName) (map[uint64]*common.AssetsInUtxo, error) {
	utxos, err := b.rpcService.GetUTXOs(address)
	if err != nil {
		return nil, err
	}

	result := make(map[uint64]*common.AssetsInUtxo)
	for utxoId := range utxos {
		utxo, err := b.rpcService.GetUtxoByID(utxoId)
		if err != nil {
			continue
		}
		info := b.GetTxOutputWithUtxoV3(utxo)
		if info == nil {
			continue
		}

		if ticker == nil {
			result[utxoId] = info
		} else if common.IsPlainAsset(ticker) {
			if len(info.Assets) == 0 {
				result[utxoId] = info
			}
		} else {
			for _, asset := range info.Assets {
				if asset.AssetName == *ticker {
					result[utxoId] = info
				}
			}
		}
	}

	return result, nil
}


func (b *IndexerMgr) GetTxOutputWithUtxoV3(utxo string) *common.AssetsInUtxo {
	output := b.GetTxOutputWithUtxo(utxo)
	if output == nil {
		return nil
	}

	var assetsInUtxo common.AssetsInUtxo
	assetsInUtxo.OutPoint = utxo
	assetsInUtxo.Value = output.OutValue.Value
	
	for _, asset := range output.OutValue.Assets {
		asset := common.DisplayAsset{
			AssetName:  asset.Name,
			Amount:     asset.Amount.String(),
			BindingSat: b.GetBindingSat(&asset.Name),
		}

		assetsInUtxo.Assets = append(assetsInUtxo.Assets, &asset)
	}

	return &assetsInUtxo
}


func (b *IndexerMgr) GetAssetSummaryInAddressV3(address string) map[common.TickerName]*common.Decimal {
	utxos, err := b.rpcService.GetUTXOs(address)
	if err != nil {
		return nil
	}

	value := int64(0)
	result := make(map[wire.AssetName]*common.Decimal)
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
				total, ok := result[asset.Name]
				if ok {
					total = total.Add(&asset.Amount)
				} else {
					total = &asset.Amount
				}
				result[asset.Name] = total
			}
			assetAmt = info.Assets.GetBindingSatAmout()
		}
		
		value += (v - assetAmt)
	}
	if value != 0 {
		result[common.ASSET_PLAIN_SAT] = indexer.NewDecimal(value, 0)
	}

	return result
}

// return: ticker -> asset info (inscriptinId -> asset ranges)
func (b *IndexerMgr) GetAssetsWithUtxoV3(utxo string) map[common.TickerName]*common.Decimal {

	info, err := b.rpcService.GetUtxoInfo(utxo)
	if err != nil {
		return nil
	}

	result := make(map[wire.AssetName]*common.Decimal)
	// 白聪资产去除绑定资产的聪
	assetAmt := int64(0)
	if len(info.Assets) != 0 {
		for _, asset := range info.Assets {
			total, ok := result[asset.Name]
			if ok {
				total = total.Add(&asset.Amount)
			} else {
				total = &asset.Amount
			}
			result[asset.Name] = total
		}
		assetAmt = info.Assets.GetBindingSatAmout()
	}
	value := (info.Value - assetAmt)
	if value != 0 {
		result[common.ASSET_PLAIN_SAT] = indexer.NewDecimal(value, 0)
	}
	return result
}
