package wire

import (
	"errors"
	"sort"
)

type SatsRange struct {
	Start int64
	Size  int64
}

// 返回 SatsRange 的结束值
func (s SatsRange) End() int64 {
	return s.Start + s.Size
}

type TxRanges []SatsRange

func TxRangesAppend(t TxRanges, ranges TxRanges) TxRanges {
	var r1, r2 SatsRange
	len1 := len(t)
	len2 := len(ranges)
	if len1 == 0 {
		// Directly append
		t = append(t, ranges...)
		return t
	}
	if len2 == 0 {
		return t
	}
	if len1 > 0 {
		r1 = t[len1-1]
		r2 = ranges[0]
		if r1.Start+r1.Size == r2.Start {
			// Two ranges is continues
			t[len1-1].Size += r2.Size
			t = append(t, ranges[1:]...)
		} else {
			t = append(t, ranges...)
		}
	}

	return t
}

// 合并两个 TxRanges，并按 Start 排序和合并连续的片段
func (d1 TxRanges) Merge(d2 TxRanges) TxRanges {
	// 将两个数据集合并
	merged := append(d1, d2...)

	// 对合并后的数据片段按 Start 排序
	sort.Slice(merged, func(i, j int) bool {
		return merged[i].Start < merged[j].Start
	})

	// 合并相邻且可合并的片段
	var result TxRanges
	for _, current := range merged {
		// 如果 result 为空或当前片段无法与上一个片段合并，直接添加到 result
		if len(result) == 0 || result[len(result)-1].End() < current.Start {
			result = append(result, current)
		} else {
			// 如果当前片段与上一个片段重叠或相连，合并它们
			last := &result[len(result)-1]
			last.Size = max(last.End(), current.End()) - last.Start
		}
	}

	return result
}

// 减去操作：原数据集减去指定数据集  r1 - r2
func (r1 TxRanges) Subtract(r2 TxRanges) (TxRanges, error) {
	var result TxRanges

	// 对r1按 Start 排序
	sort.Slice(r1, func(i, j int) bool {
		return r1[i].Start < r1[j].Start
	})

	// 对r2按 Start 排序
	sort.Slice(r2, func(i, j int) bool {
		return r2[i].Start < r2[j].Start
	})

	var err error
	result = r1
	// 遍历原数据集中的每个片段
	for _, origRange := range r2 {
		result, err = result.SubSatsRange(origRange)
		if err != nil {
			return nil, err
		}
	}

	return result, nil
}

// SubSatsRange 从数据集中减去一个单独的数据片段 r2
// 返回剩余的数据片段集
// 如果 r2 无法从数据集中减去（不相交），返回错误
// 数据集已经按 Start 排序
func (dataset TxRanges) SubSatsRange(r2 SatsRange) (TxRanges, error) {
	result := TxRanges{}

	// 遍历数据集中的每个片段
	for _, r1 := range dataset {
		if r2.Size > 0 {
			if r1.Start > r2.Start {
				// r2 Start 已经在 r1 的左侧, 无法删除 r2
				return nil, errors.New("range not in original TxRange")
			}
			// r2 已经还没有完全移除
			if r1.Start <= r2.Start && r1.End() > r2.Start {
				// 有相交， 且r2 在r1的右侧
				if r1.Start == r2.Start {
					// 起始相同
					if r1.Size == r2.Size {
						// r1 == r2
						r2.Size = 0 // r2 已经移除
						//r1 没有任何剩余， 直接跳过
						continue
					}
					if r1.Size > r2.Size {
						// r1 > r2
						r1.Start = r1.Start + r2.Size
						r1.Size -= r2.Size
						r2.Size = 0
						result = append(result, r1)
						continue
					}
					// r1 < r2
					r2.Start = r2.Start + r1.Size
					r2.Size -= r1.Size
					// r1 没有任何剩余， 直接跳过
					continue
				}

				// r2 在r1的右侧
				// 先将r1留下来的部分添加到result
				remRanges := SatsRange{Start: r1.Start, Size: r2.Start - r1.Start}
				result = append(result, remRanges)

				r1.Start = r2.Start
				r1.Size -= remRanges.Size

				if r1.Size > r2.Size {
					// r1 > r2
					r1.Start = r1.Start + r2.Size
					r1.Size -= r2.Size
					r2.Size = 0
					result = append(result, r1)
					continue
				}
				// r1 < r2
				r2.Start = r2.Start + r1.Size
				r2.Size -= r1.Size
				// r1 没有任何剩余， 直接跳过
				continue
			}
		}

		// r1 与 r2 无重叠, 将 r1 添加到 result
		result = append(result, r1)
	}
	if r2.Size > 0 {
		// 所有的片段都已经遍历，仍无法完全删除 r2
		return nil, errors.New("range not in original TxRange")
	}

	return result, nil
}

func isOverlapping(r1, r2 SatsRange) bool {
	// 判断条件是 r1 和 r2 的区间有重叠
	return r1.Start < r2.End() && r2.Start < r1.End()
}

// subtractRange 将 r2 从 r1 中减去，返回剩余的片段
// 如果 r2 完全包含在 r1 内部且不与边界重叠，则返回两个片段
func subtractRange(r1, r2 SatsRange) (TxRanges, error) {
	// 如果 r2 完全包含 r1，则返回一个空的 SatsRange（被完全移除了）
	if r2.Start <= r1.Start && r2.End() >= r1.End() {
		return nil, nil
	}

	// 如果 r2 部分重叠 r1 的左边，截断 r1 的左边部分
	if r2.Start <= r1.Start && r2.End() < r1.End() {
		r1.Start = r2.End()
		r1.Size = r1.End() - r1.Start
		return []SatsRange{r1}, nil
	}

	// 如果 r2 部分重叠 r1 的右边，截断 r1 的右边部分
	if r2.Start > r1.Start && r2.End() >= r1.End() {
		r1.Size = r2.Start - r1.Start
		return []SatsRange{r1}, nil
	}

	// 如果 r2 完全包含在 r1 内部且不与边界重叠
	if r2.Start > r1.Start && r2.End() < r1.End() {
		// 分裂成两个片段
		leftPart := SatsRange{
			Start: r1.Start,
			Size:  r2.Start - r1.Start,
		}
		rightPart := SatsRange{
			Start: r2.End(),
			Size:  r1.End() - r2.End(),
		}
		return []SatsRange{leftPart, rightPart}, nil
	}

	return nil, errors.New("无法处理此种情况")
}

// Cut an TxRanges with the given count from head of the current TxRanges, current TxRanges will be cutted
func TxRangesCut(t TxRanges, count int64) (TxRanges, TxRanges, error) {
	size := t.GetSize()
	if count > size {
		err := errors.New("cut count is too big")
		return t, nil, err
	}

	if count == 0 {
		// Nothing to cut
		return t, TxRanges{}, nil
	}

	if count == size {
		//All ranges are cutted
		cuttedRanges := t
		t = TxRanges{}
		return t, cuttedRanges, nil
	}

	cuttedRanges := make(TxRanges, 0)
	remainingValue := count
	//remaining := ordinals
	//transferred := make([]*Range, 0)
	for remainingValue > 0 {
		currentRange := t[0]
		//start := currentRange.Start
		//transferSize := currentRange.Size
		if currentRange.Size > remainingValue {
			// Enough to cut the current range
			newRange := SatsRange{Start: currentRange.Start, Size: remainingValue}
			cuttedRanges = append(cuttedRanges, newRange)
			t[0].Start += remainingValue
			t[0].Size -= remainingValue

			return t, cuttedRanges, nil
		}

		// Not enough to cut the current range
		cuttedRanges = append(cuttedRanges, currentRange)
		t = t[1:]
		remainingValue = remainingValue - currentRange.Size
	}

	return t, cuttedRanges, nil
}

// Pickup an TxRanges with the given offset and count from the current TxRanges, current TxRanges is not changed
func (t TxRanges) Pickup(offset, count int64) (TxRanges, error) {
	size := t.GetSize()
	if offset+count > size {
		err := errors.New("pickup count is too big")
		return nil, err
	}

	if count == 0 {
		// Nothing to pickup
		return TxRanges{}, nil
	}

	if offset == 0 && count == size {
		//All ranges are pickup
		return t, nil
	}

	pickupRanges := make(TxRanges, 0)
	remainingValue := count
	pos := int64(0)
	//remaining := ordinals
	//transferred := make([]*Range, 0)
	//for remainingValue > 0 {
	for _, currentRange := range t {
		if pos < offset {
			if pos+currentRange.Size <= offset {
				pos = pos + currentRange.Size
				continue
			}
			// Will pickup from current range

			start := currentRange.Start + (offset - pos)
			rangeSize := currentRange.Size - (offset - pos)
			if rangeSize > remainingValue {
				rangeSize = remainingValue
			}
			newRange := SatsRange{Start: start, Size: rangeSize}
			pickupRanges = append(pickupRanges, newRange)
			remainingValue = remainingValue - rangeSize
		} else {
			// Will pickup from current range
			start := currentRange.Start
			rangeSize := currentRange.Size
			if rangeSize > remainingValue {
				rangeSize = remainingValue
			}
			newRange := SatsRange{Start: start, Size: rangeSize}
			pickupRanges = append(pickupRanges, newRange)
			remainingValue = remainingValue - rangeSize
		}

		pos = pos + currentRange.Size

		if remainingValue <= 0 {
			break
		}
	}

	// check valid
	pickupSize := pickupRanges.GetSize()
	if count != pickupSize {
		err := errors.New("pickup count is wrong")
		return nil, err
	}

	return pickupRanges, nil
}

// Calculate the tx range total size
func (t TxRanges) GetSize() int64 {
	size := int64(0)
	for _, rng := range t {
		size += (rng.Size)
	}
	return size
}

func (t TxRanges) EqualSatRanges(ranges TxRanges) bool {
	if t.GetSize() != ranges.GetSize() {
		return false
	}

	indexT := 0
	indexR := 0

	startT := t[indexT].Start
	sizeT := t[indexT].Size
	startR := ranges[indexR].Start
	sizeR := ranges[indexR].Size

	for {
		if startT != startR {
			return false
		}

		if sizeT == sizeR {
			indexT++
			indexR++

			if indexT >= len(t) {
				return true
			}
			startT = t[indexT].Start
			sizeT = t[indexT].Size
			startR = ranges[indexR].Start
			sizeR = ranges[indexR].Size

			continue
		}

		if sizeT > sizeR {
			startT += sizeR
			sizeT -= sizeR

			indexR++

			if indexR >= len(ranges) {
				// ranges has to end, t is not
				return false
			}
			startR = ranges[indexR].Start
			sizeR = ranges[indexR].Size
			continue
		}

		// sizeT < sizeR
		startR += sizeT
		sizeR -= sizeT

		indexT++

		if indexT >= len(t) {
			// t has to end, ranges is not
			return false
		}
		startT = t[indexT].Start
		sizeT = t[indexT].Size
		continue
	}
}
