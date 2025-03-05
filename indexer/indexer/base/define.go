package base

const SyncStatsKey = "syncStats"
const BaseDBVerKey = "dbver"

type SyncStats struct {
	ChainTip       int    `json:"chainTip"`
	SyncHeight     int    `json:"syncHeight"`
	SyncBlockHash  string `json:"syncBlockHash"`
	ReorgsDetected []int  `json:"reorgsDetected"`
	AllUtxoCount   uint64
	AddressCount   uint64
	UtxoCount      uint64
	TotalSats      int64
	AscendCount    int
	DescendCount   int
}

type IrregularSubsidy struct {
	TotalLeakSats  int64
	SatsLeakBlocks map[int]int64
}

func (p *SyncStats) Clone () *SyncStats {
	c := &SyncStats{
		ChainTip: p.ChainTip,
		SyncHeight: p.SyncHeight,
		SyncBlockHash: p.SyncBlockHash,
		AllUtxoCount: p.AllUtxoCount,
		AddressCount: p.AddressCount,
		UtxoCount: p.UtxoCount,
		TotalSats: p.TotalSats,
	}
	c.ReorgsDetected = make([]int, len(p.ReorgsDetected))
	copy(c.ReorgsDetected, p.ReorgsDetected)
	return c
}
