package wire

import (
	"errors"
)

type SatsRange struct {
	Start int64
	Size  int64
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
			r1.Size += r2.Size
			t = append(t, ranges[1:]...)
		} else {
			t = append(t, ranges...)
		}
	}

	return t
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
