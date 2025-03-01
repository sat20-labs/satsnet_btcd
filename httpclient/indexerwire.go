package httpclient

import (
	"github.com/sat20-labs/satsnet_btcd/wire"
)

type BaseResp struct {
	Code int    `json:"code" example:"0"`
	Msg  string `json:"msg" example:"ok"`
}

type TxResp struct {
	BaseResp
	Data any `json:"data"`
}

type TxOut struct {
	Value    int64  `json:"Value"`
	PkScript []byte `json:"PkScript"`
}

type OffsetRange struct {
	Start int64
	End   int64 // 不包括End
}

type AssetOffsets []*OffsetRange

type UtxoAssetInfo struct {
	Asset   wire.AssetInfo     `json:"asset"`
	Offsets AssetOffsets `json:"offsets"`
}

type TxOutputInfo struct {
	UtxoId    uint64       `json:"utxoid"`
	OutPoint  string       `json:"outpoint"`
	OutValue  TxOut   `json:"outvalue"`
	AssetInfo []*UtxoAssetInfo `json:"assets"`
}

type TxOutputResp struct {
	BaseResp
	Data *TxOutputInfo `json:"data"`
}

type ListResp struct {
	Start int64  `json:"start" example:"0"`
	Total uint64 `json:"total" example:"9992"`
}

type AssetSummary struct {
	ListResp
	Data []*wire.AssetInfo `json:"data"`
}

type AssetSummaryResp struct {
	BaseResp
	Data *AssetSummary `json:"data"`
}
