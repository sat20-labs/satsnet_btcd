package wire

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
)

// //////////////////////////////////////////////////////////////
// 定义在聪网中
type AssetName struct {
	Protocol string // 必填，比如ordx, ordinals, brc20，runes，eth，等等
	Type     string // 可选，默认是ft，参考indexer的定义
	Ticker   string // 如果Type是nft类型，ticker是合集名称#铭文序号（或者聪序号）
}

func NewAssetNameFromString(name string) *AssetName {
	parts := strings.Split(name, ":")
	if len(parts) != 3 {
		return nil
	}
	return &AssetName{
		Protocol: parts[0],
		Type: parts[1],
		Ticker: parts[2],
	}
}

func (p *AssetName) String() string {
	return p.Protocol + ":" + p.Type + ":" + p.Ticker
}

type AssetInfo struct {
	Name       AssetName
	Amount     int64  // 资产数量
	BindingSat uint16 // 非0 -> 每一聪绑定的资产的数量, 0 -> 不绑定聪
}

func (p *AssetInfo) Add(another *AssetInfo) error {
	if p.Name == another.Name {
		if p.Amount+another.Amount < 0 {
			return fmt.Errorf("out of bound")
		}
		p.Amount += another.Amount
	} else {
		return fmt.Errorf("not the same asset")
	}
	return nil
}

func (p *AssetInfo) Subtract(another *AssetInfo) error {
	if p.Name == another.Name {
		if p.Amount < another.Amount {
			return fmt.Errorf("not enough asset to subtract")
		}
		p.Amount -= another.Amount
	} else {
		return fmt.Errorf("not the same asset")
	}
	return nil
}

func (p *AssetInfo) Clone() *AssetInfo {
	if p == nil {
		return nil
	}
	return &AssetInfo{
		Name:       p.Name,
		Amount:     p.Amount,
		BindingSat: p.BindingSat,
	}
}

func (p *AssetInfo) Equal(another *AssetInfo) bool {
	if another == nil {
		return false
	}
	return p.Name == another.Name && p.Amount == another.Amount &&
		p.BindingSat == another.BindingSat
}

// 有序数组，根据名字排序
type TxAssets []AssetInfo

// TxAssetsAppend 合并两个资产列表，返回新的列表
func TxAssetsAppend(a, b *TxAssets) TxAssets {
	if a == nil {
		if b == nil {
			return nil
		}
		return b.Clone()
	}
	result := a.Clone()
	err := result.Merge(b)
	if err != nil {
		return nil
	}
	return result
}

func (p *TxAssets) Clone() TxAssets {
	if p == nil {
		return nil
	}

	newAssets := make(TxAssets, len(*p))
	for i, asset := range *p {
		newAssets[i] = *asset.Clone()
	}

	return newAssets
}

func (p *TxAssets) Sort() {
	sort.Slice(*p, func(i, j int) bool {
		if (*p)[i].Name.Protocol != (*p)[j].Name.Protocol {
			return (*p)[i].Name.Protocol < (*p)[j].Name.Protocol
		}
		if (*p)[i].Name.Type != (*p)[j].Name.Type {
			return (*p)[i].Name.Type < (*p)[j].Name.Type
		}
		return (*p)[i].Name.Ticker < (*p)[j].Name.Ticker
	})
}

// Binary search to find the index of an AssetName
func (p *TxAssets) findIndex(name *AssetName) (int, bool) {
	index := sort.Search(len(*p), func(i int) bool {
		if (*p)[i].Name.Protocol != name.Protocol {
			return (*p)[i].Name.Protocol >= name.Protocol
		}
		if (*p)[i].Name.Type != name.Type {
			return (*p)[i].Name.Type >= name.Type
		}
		return (*p)[i].Name.Ticker >= name.Ticker
	})
	if index < len(*p) && (*p)[index].Name == *name {
		return index, true
	}
	return index, false
}

func (p *TxAssets) Equal(another *TxAssets) bool {
	if p == nil && another == nil{
		return true
	}
	if len(*p) != len(*another) {
		return false
	}
	
	for i, asset := range *p {
		if asset != (*another)[i] {
			return false
		}
	}
	return true
}

// 将另一个资产列表合并到当前列表中
func (p *TxAssets) Merge(another *TxAssets) error {
	if another == nil {
		return nil
	}
	cp := p.Clone()
	for _, asset := range *another {
		if err := cp.Add(&asset); err != nil {
			return err
		}
	}
	*p = cp
	return nil
}

// Subtract 从当前列表中减去另一个资产列表
func (p *TxAssets) Split(another *TxAssets) error {
	if another == nil {
		return nil
	}
	cp := p.Clone()
	for _, asset := range *another {
		if err := cp.Subtract(&asset); err != nil {
			return err
		}
	}
	*p = cp
	return nil
}

// Align 数组中聪绑定的资产的数量不超过value
func (p *TxAssets) Align(value int64) TxAssets {
	var result TxAssets
	for _, asset := range *p {
		if asset.BindingSat > 0 && asset.Amount > value * int64(asset.BindingSat) {
			sub := AssetInfo{
				Name: asset.Name,
				Amount: asset.Amount - value * int64(asset.BindingSat),
				BindingSat: asset.BindingSat,
			}
			result = append(result, sub)
			asset.Amount = value * int64(asset.BindingSat)
		}
	}
	return result
}

// Add 将另一个资产列表合并到当前列表中
func (p *TxAssets) Add(asset *AssetInfo) error {
	if asset == nil {
		return nil
	}
	index, found := p.findIndex(&asset.Name)
	if found {
		if (*p)[index].Amount+asset.Amount < 0 {
			return fmt.Errorf("out of bounds")
		}
		(*p)[index].Amount += asset.Amount
	} else {
		*p = append(*p, AssetInfo{}) // Extend slice
		copy((*p)[index+1:], (*p)[index:])
		(*p)[index] = *asset
	}
	return nil
}

// Subtract 从当前列表中减去另一个资产列表
func (p *TxAssets) Subtract(asset *AssetInfo) error {
	if asset == nil {
		return nil
	}
	if asset.Amount == 0 {
		return nil
	}

	index, found := p.findIndex(&asset.Name)
	if !found {
		return errors.New("asset not found")
	}
	if (*p)[index].Amount < asset.Amount {
		return errors.New("insufficient asset amount")
	}
	(*p)[index].Amount -= asset.Amount
	if (*p)[index].Amount == 0 {
		*p = append((*p)[:index], (*p)[index+1:]...)
	}
	return nil
}

// PickUp 从资产列表中提取指定名称和数量的资产，原资产不改变
func (p *TxAssets) PickUp(asset *AssetName, amt int64) (*AssetInfo, error) {
	if asset == nil {
		return nil, fmt.Errorf("need a specific asset")
	}
	index, found := p.findIndex(asset)
	if !found {
		return nil, errors.New("asset not found")
	}
	if (*p)[index].Amount < amt {
		return nil, errors.New("insufficient asset amount")
	}
	
	picked := AssetInfo{Name: *asset, Amount: amt, BindingSat: (*p)[index].BindingSat}
	return &picked, nil
}

func (p *TxAssets) Find(asset *AssetName) (*AssetInfo, error) {
	index, found := p.findIndex(asset)
	if !found {
		return nil, errors.New("asset not found")
	}
	return &(*p)[index], nil
}

func (p *TxAssets) GetBindingSatAmout() int64 {
	amount := int64(0)
	for _, asset := range *p {
		if asset.BindingSat != 0 {
			if amount < asset.Amount / int64(asset.BindingSat) {
				amount = asset.Amount / int64(asset.BindingSat)
			}
		}
	}
	return amount
}

func (p *TxAssets) Serialize() ([]byte, error) {
	var w bytes.Buffer

	buf := binarySerializer.Borrow()
	defer binarySerializer.Return(buf)

	err := AssetsWriteToBuf(&w, 0, *p, buf)
	if err != nil {
		return nil, err
	}

	return w.Bytes(), nil
}

func (p *TxAssets) Deserialize(r []byte) (error) {

	buf := binarySerializer.Borrow()
	defer binarySerializer.Return(buf)

	sbuf := scriptPool.Borrow()
	defer scriptPool.Return(sbuf)

	assets, err := AssetsReadFromBuf(bytes.NewReader(r), 0, buf, sbuf[:])
	if err != nil {
		return err
	}

	*p = assets
	return nil
}


func AssetsWriteToBuf(w io.Writer, pver uint32, assets TxAssets, buf []byte) error {
	// get count for sats range, and write to w
	assetsCount := uint64(len(assets))
	err := WriteVarIntBuf(w, pver, assetsCount, buf)
	if err != nil {
		return err
	}

	for _, asset := range assets {
		// Write asset, Name（Protocol，Type，Ticker）, Amount, BindingSat
		err = WriteVarBytesBuf(w, pver, []byte(asset.Name.Protocol), buf)
		if err != nil {
			return err
		}
		err = WriteVarBytesBuf(w, pver, []byte(asset.Name.Type), buf)
		if err != nil {
			return err
		}
		err = WriteVarBytesBuf(w, pver, []byte(asset.Name.Ticker), buf)
		if err != nil {
			return err
		}
		err = WriteVarIntBuf(w, pver, uint64(asset.Amount), buf)
		if err != nil {
			return err
		}
		err = WriteVarIntBuf(w, pver, uint64(asset.BindingSat), buf)
		if err != nil {
			return err
		}
	}
	return nil
}

func AssetsReadFromBuf(r io.Reader, pver uint32, buf, s []byte) (TxAssets, error) {
	// count fr sats range
	count, err := ReadVarIntBuf(r, pver, buf)
	if err != nil {
		return nil, err
	}

	assets := make(TxAssets, 0)
	for i := uint64(0); i < count; i++ {
		newAsset := AssetInfo{}
		// Get sats start and size
		newAsset.Name.Protocol, err = readString(r, pver, buf, s, "asset protocol")
		if err != nil {
			return nil, err
		}
		newAsset.Name.Type, err = readString(r, pver, buf, s, "asset type")
		if err != nil {
			return nil, err
		}
		newAsset.Name.Ticker, err = readString(r, pver, buf, s, "asset ticker")
		if err != nil {
			return nil, err
		}
		amount, err := ReadVarIntBuf(r, pver, buf)
		if err != nil {
			return nil, err
		}
		newAsset.Amount = int64(amount)
		bindingSat, err := ReadVarIntBuf(r, pver, buf)
		if err != nil {
			return nil, err
		}
		newAsset.BindingSat = uint16(bindingSat)
		assets = append(assets, newAsset)
	}
	return assets, nil
}
