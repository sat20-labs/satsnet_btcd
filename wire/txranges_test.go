// Copyright (c) 2013-2016 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package wire

import (
	"testing"
)

// TestTxRangesAppend tests .
func TestTxRangesAppend(t *testing.T) {
	ranges1 := TxRanges{SatsRange{0, 10000}, SatsRange{1000000, 10000}}
	ranges2 := TxRanges{SatsRange{2000000, 10000}, SatsRange{3000000, 10000}}
	ranges3 := TxRanges{SatsRange{4000000, 10000}}
	ranges4 := TxRanges{SatsRange{5000000, 10000}, SatsRange{6000000, 10000}, SatsRange{7000000, 10000}}

	appendRanges := TxRanges{}
	appendRanges = TxRangesAppend(appendRanges, ranges1)
	size := appendRanges.GetSize()
	if size != 20000 {
		t.Errorf("Append: wrong size - got %d, want %d", size, 20000)
	}
	appendRanges = TxRangesAppend(appendRanges, ranges2)
	size = appendRanges.GetSize()
	if size != 40000 {
		t.Errorf("Append: wrong size - got %d, want %d", size, 40000)
	}
	appendRanges = TxRangesAppend(appendRanges, ranges3)
	size = appendRanges.GetSize()
	if size != 50000 {
		t.Errorf("Append: wrong size - got %d, want %d", size, 50000)
	}
	appendRanges = TxRangesAppend(appendRanges, ranges4)
	size = appendRanges.GetSize()
	if size != 80000 {
		t.Errorf("Append: wrong size - got %d, want %d", size, 80000)
	}

	for index, satsRange := range appendRanges {
		rangeStart := int64(index * 1000000)
		if rangeStart != satsRange.Start {
			t.Errorf("Index #%d Start got: %d want: %d", index, satsRange.Start,
				rangeStart)
			continue
		}
		if satsRange.Size != 10000 {
			t.Errorf("Index #%d Size got: %d want: 10000", index, satsRange.Size)
			continue
		}
	}

	t.Logf("Running TxRanges Append completed")
}

// TestTxRangesPickup tests .
func TestTxRangesPickup(t *testing.T) {
	originalRanges := TxRanges{
		SatsRange{0, 10000},
		SatsRange{1000000, 10000},
		SatsRange{2000000, 10000},
		SatsRange{3000000, 10000},
		SatsRange{4000000, 10000},
		SatsRange{5000000, 10000},
		SatsRange{6000000, 10000},
		SatsRange{7000000, 10000}}

	// pickup a full range from head
	ranges1, err := originalRanges.Pickup(0, 10000)
	if err != nil {
		t.Errorf("Pickup1: unexpected error %v", err)
	}

	if len(ranges1) != 1 {
		t.Errorf("Pickup1: wrong length - got %d, want %d", len(ranges1), 1)
	}

	if ranges1[0].Start != 0 {
		t.Errorf("Pickup1: wrong start - got %d, want %d", ranges1[0].Start, 0)
	}

	if ranges1[0].Size != 10000 {
		t.Errorf("Pickup1: wrong size - got %d, want %d", ranges1[0].Size, 10000)
	}

	// pickup a full range from tail
	ranges2, err := originalRanges.Pickup(70000, 10000)
	if err != nil {
		t.Errorf("Pickup2: unexpected error %v", err)
	}

	if len(ranges2) != 1 {
		t.Errorf("Pickup2: wrong length - got %d, want %d", len(ranges2), 1)
	}

	if ranges2[0].Start != 7000000 {
		t.Errorf("Pickup2: wrong start - got %d, want %d", ranges2[0].Start, 7000000)
	}

	if ranges2[0].Size != 10000 {
		t.Errorf("Pickup2: wrong size - got %d, want %d", ranges2[0].Size, 10000)
	}

	// pickup a full range from medium
	ranges3, err := originalRanges.Pickup(40000, 10000)
	if err != nil {
		t.Errorf("Pickup3: unexpected error %v", err)
	}

	if len(ranges3) != 1 {
		t.Errorf("Pickup3: wrong length - got %d, want %d", len(ranges3), 1)
	}

	if ranges3[0].Start != 4000000 {
		t.Errorf("Pickup3: wrong start - got %d, want %d", ranges3[0].Start, 4000000)
	}

	if ranges3[0].Size != 10000 {
		t.Errorf("Pickup3: wrong size - got %d, want %d", ranges3[0].Size, 10000)
	}

	// pickup a half range from head
	ranges4, err := originalRanges.Pickup(0, 5000)
	if err != nil {
		t.Errorf("Pickup4: unexpected error %v", err)
	}

	if len(ranges4) != 1 {
		t.Errorf("Pickup4: wrong length - got %d, want %d", len(ranges4), 1)
	}

	if ranges4[0].Start != 0 {
		t.Errorf("Pickup4: wrong start - got %d, want %d", ranges4[0].Start, 0)
	}

	if ranges4[0].Size != 5000 {
		t.Errorf("Pickup4: wrong size - got %d, want %d", ranges4[0].Size, 5000)
	}

	// pickup a half range from first range tail
	ranges5, err := originalRanges.Pickup(5000, 5000)
	if err != nil {
		t.Errorf("Pickup5: unexpected error %v", err)
	}

	if len(ranges5) != 1 {
		t.Errorf("Pickup5: wrong length - got %d, want %d", len(ranges5), 1)
	}

	if ranges5[0].Start != 5000 {
		t.Errorf("Pickup5: wrong start - got %d, want %d", ranges5[0].Start, 0)
	}

	if ranges5[0].Size != 5000 {
		t.Errorf("Pickup5: wrong size - got %d, want %d", ranges5[0].Size, 5000)
	}

	// pickup a half range from tail
	ranges6, err := originalRanges.Pickup(75000, 5000)
	if err != nil {
		t.Errorf("Pickup6: unexpected error %v", err)
	}

	if len(ranges6) != 1 {
		t.Errorf("Pickup6: wrong length - got %d, want %d", len(ranges6), 1)
	}

	if ranges6[0].Start != 7005000 {
		t.Errorf("Pickup6: wrong start - got %d, want %d", ranges6[0].Start, 7005000)
	}

	if ranges6[0].Size != 5000 {
		t.Errorf("Pickup6: wrong size - got %d, want %d", ranges6[0].Size, 5000)
	}

	// pickup a half range from last range head
	ranges7, err := originalRanges.Pickup(70000, 5000)
	if err != nil {
		t.Errorf("Pickup7: unexpected error %v", err)
	}

	if len(ranges7) != 1 {
		t.Errorf("Pickup7: wrong length - got %d, want %d", len(ranges7), 1)
	}

	if ranges7[0].Start != 7000000 {
		t.Errorf("Pickup7: wrong start - got %d, want %d", ranges7[0].Start, 7000000)
	}

	if ranges7[0].Size != 5000 {
		t.Errorf("Pickup7: wrong size - got %d, want %d", ranges7[0].Size, 5000)
	}

	// pickup a half range from medium range head
	ranges8, err := originalRanges.Pickup(40000, 5000)
	if err != nil {
		t.Errorf("Pickup8: unexpected error %v", err)
	}

	if len(ranges8) != 1 {
		t.Errorf("Pickup8: wrong length - got %d, want %d", len(ranges8), 1)
	}

	if ranges8[0].Start != 4000000 {
		t.Errorf("Pickup8: wrong start - got %d, want %d", ranges8[0].Start, 4000000)
	}

	if ranges8[0].Size != 5000 {
		t.Errorf("Pickup8: wrong size - got %d, want %d", ranges8[0].Size, 5000)
	}

	// pickup a half range from medium range tail
	ranges9, err := originalRanges.Pickup(45000, 5000)
	if err != nil {
		t.Errorf("Pickup9: unexpected error %v", err)
	}

	if len(ranges9) != 1 {
		t.Errorf("Pickup9: wrong length - got %d, want %d", len(ranges9), 1)
	}

	if ranges9[0].Start != 4005000 {
		t.Errorf("Pickup9: wrong start - got %d, want %d", ranges9[0].Start, 4005000)
	}

	if ranges9[0].Size != 5000 {
		t.Errorf("Pickup9: wrong size - got %d, want %d", ranges9[0].Size, 5000)
	}

	// pickup 2 full range from head
	ranges10, err := originalRanges.Pickup(0, 20000)
	if err != nil {
		t.Errorf("Pickup10: unexpected error %v", err)
	}

	if len(ranges10) != 2 {
		t.Errorf("Pickup10: wrong length - got %d, want %d", len(ranges10), 2)
	}
	if len(ranges10) == 2 {
		if ranges10[0].Start != 0 {
			t.Errorf("Pickup10: wrong start - got %d, want %d", ranges10[0].Start, 0)
		}

		if ranges10[0].Size != 10000 {
			t.Errorf("Pickup10: wrong size - got %d, want %d", ranges10[0].Size, 10000)
		}

		if ranges10[1].Start != 1000000 {
			t.Errorf("Pickup10: wrong start - got %d, want %d", ranges10[1].Start, 1000000)
		}

		if ranges10[1].Size != 10000 {
			t.Errorf("Pickup10: wrong size - got %d, want %d", ranges10[1].Size, 10000)
		}
	}

	// pickup 2 full range from tail
	ranges11, err := originalRanges.Pickup(60000, 20000)
	if err != nil {
		t.Errorf("Pickup11: unexpected error %v", err)
	}

	if len(ranges11) != 2 {
		t.Errorf("Pickup11: wrong length - got %d, want %d", len(ranges11), 2)
	}
	if len(ranges11) == 2 {
		if ranges11[0].Start != 6000000 {
			t.Errorf("Pickup11: wrong start - got %d, want %d", ranges11[0].Start, 6000000)
		}

		if ranges11[0].Size != 10000 {
			t.Errorf("Pickup11: wrong size - got %d, want %d", ranges11[0].Size, 10000)
		}

		if ranges11[1].Start != 7000000 {
			t.Errorf("Pickup11: wrong start - got %d, want %d", ranges11[1].Start, 7000000)
		}

		if ranges11[1].Size != 10000 {
			t.Errorf("Pickup11: wrong size - got %d, want %d", ranges11[1].Size, 10000)
		}
	}

	// pickup 2 full range from medium
	ranges12, err := originalRanges.Pickup(40000, 20000)
	if err != nil {
		t.Errorf("Pickup12: unexpected error %v", err)
	}

	if len(ranges12) != 2 {
		t.Errorf("Pickup12: wrong length - got %d, want %d", len(ranges12), 2)
	}
	if len(ranges12) == 2 {
		if ranges12[0].Start != 4000000 {
			t.Errorf("Pickup12: wrong start - got %d, want %d", ranges12[0].Start, 4000000)
		}

		if ranges12[0].Size != 10000 {
			t.Errorf("Pickup12: wrong size - got %d, want %d", ranges12[0].Size, 10000)
		}

		if ranges12[1].Start != 5000000 {
			t.Errorf("Pickup12: wrong start - got %d, want %d", ranges12[1].Start, 5000000)
		}

		if ranges12[1].Size != 10000 {
			t.Errorf("Pickup12: wrong size - got %d, want %d", ranges12[1].Size, 10000)
		}
	}

	// pickup 2 full range from first range medium
	ranges13, err := originalRanges.Pickup(5000, 20000)
	if err != nil {
		t.Errorf("Pickup13: unexpected error %v", err)
	}

	if len(ranges13) != 3 {
		t.Errorf("Pickup13: wrong length - got %d, want %d", len(ranges13), 3)
	}
	if len(ranges13) == 3 {
		if ranges13[0].Start != 5000 {
			t.Errorf("Pickup13: wrong start - got %d, want %d", ranges13[0].Start, 5000)
		}

		if ranges13[0].Size != 5000 {
			t.Errorf("Pickup13: wrong size - got %d, want %d", ranges13[0].Size, 5000)
		}

		if ranges13[1].Start != 1000000 {
			t.Errorf("Pickup13: wrong start - got %d, want %d", ranges13[1].Start, 1000000)
		}

		if ranges13[1].Size != 10000 {
			t.Errorf("Pickup13: wrong size - got %d, want %d", ranges13[1].Size, 10000)
		}

		if ranges13[2].Start != 2000000 {
			t.Errorf("Pickup13: wrong start - got %d, want %d", ranges13[0].Start, 2000000)
		}

		if ranges13[2].Size != 5000 {
			t.Errorf("Pickup13: wrong size - got %d, want %d", ranges13[0].Size, 5000)
		}
	}

	// pickup 2 full range to last range medium
	ranges14, err := originalRanges.Pickup(55000, 20000)
	if err != nil {
		t.Errorf("Pickup14: unexpected error %v", err)
	}

	if len(ranges14) != 3 {
		t.Errorf("Pickup14: wrong length - got %d, want %d", len(ranges14), 3)
	}
	if len(ranges14) == 3 {
		if ranges14[0].Start != 5005000 {
			t.Errorf("Pickup14: wrong start - got %d, want %d", ranges14[0].Start, 5005000)
		}

		if ranges14[0].Size != 5000 {
			t.Errorf("Pickup14: wrong size - got %d, want %d", ranges14[0].Size, 5000)
		}

		if ranges14[1].Start != 6000000 {
			t.Errorf("Pickup14: wrong start - got %d, want %d", ranges14[1].Start, 6000000)
		}

		if ranges14[1].Size != 10000 {
			t.Errorf("Pickup14: wrong size - got %d, want %d", ranges14[1].Size, 10000)
		}

		if ranges14[2].Start != 7000000 {
			t.Errorf("Pickup14: wrong start - got %d, want %d", ranges14[0].Start, 7000000)
		}

		if ranges14[2].Size != 5000 {
			t.Errorf("Pickup14: wrong size - got %d, want %d", ranges14[0].Size, 5000)
		}
	}

	// pickup 2 full range to medium range medium
	ranges15, err := originalRanges.Pickup(35000, 20000)
	if err != nil {
		t.Errorf("Pickup15: unexpected error %v", err)
	}

	if len(ranges15) != 3 {
		t.Errorf("Pickup15: wrong length - got %d, want %d", len(ranges15), 3)
	}
	if len(ranges15) == 3 {
		if ranges15[0].Start != 3005000 {
			t.Errorf("Pickup15: wrong start - got %d, want %d", ranges15[0].Start, 3005000)
		}

		if ranges15[0].Size != 5000 {
			t.Errorf("Pickup15: wrong size - got %d, want %d", ranges15[0].Size, 5000)
		}

		if ranges15[1].Start != 4000000 {
			t.Errorf("Pickup15: wrong start - got %d, want %d", ranges15[1].Start, 4000000)
		}

		if ranges15[1].Size != 10000 {
			t.Errorf("Pickup15: wrong size - got %d, want %d", ranges15[1].Size, 10000)
		}

		if ranges15[2].Start != 5000000 {
			t.Errorf("Pickup15: wrong start - got %d, want %d", ranges15[0].Start, 5000000)
		}

		if ranges15[2].Size != 5000 {
			t.Errorf("Pickup15: wrong size - got %d, want %d", ranges15[0].Size, 5000)
		}
	}

	t.Logf("Running TxRanges pickup completed")

}

func TestTxRangesCut(t *testing.T) {
	originalRanges := TxRanges{
		SatsRange{0, 10000},
		SatsRange{1000000, 10000},
		SatsRange{2000000, 10000},
		SatsRange{3000000, 10000},
		SatsRange{4000000, 10000},
		SatsRange{5000000, 10000},
		SatsRange{6000000, 10000},
		SatsRange{7000000, 10000}}

	// Cut a full range
	newRanges, cuttedRanges, err := TxRangesCut(originalRanges, 10000)
	if err != nil {
		t.Errorf("Cut1: unexpected error %v", err)
	}
	if len(newRanges) != 7 {
		t.Errorf("Cut1: wrong length - got %d, want %d", len(newRanges), 7)
	}
	if len(cuttedRanges) != 1 {
		t.Errorf("Cut1: wrong length - got %d, want %d", len(cuttedRanges), 1)
	}

	sizeNewRange := newRanges.GetSize()
	if sizeNewRange != 70000 {
		t.Errorf("Cut1: wrong size - got %d, want %d", sizeNewRange, 70000)
	}

	sizeCuttedRange := cuttedRanges.GetSize()
	if sizeCuttedRange != 10000 {
		t.Errorf("Cut1: wrong size - got %d, want %d", sizeNewRange, 10000)
	}

	if len(newRanges) == 7 {
		if newRanges[0].Start != 1000000 {
			t.Errorf("NewRanges1: wrong start - got %d, want %d", newRanges[0].Start, 1000000)
		}
		if newRanges[0].Size != 10000 {
			t.Errorf("NewRanges1: wrong start - got %d, want %d", newRanges[0].Size, 10000)
		}
	}
	if len(cuttedRanges) == 1 {
		if cuttedRanges[0].Start != 0 {
			t.Errorf("cuttedRanges1: wrong start - got %d, want %d", cuttedRanges[0].Start, 0)
		}
		if cuttedRanges[0].Size != 10000 {
			t.Errorf("cuttedRanges1: wrong start - got %d, want %d", cuttedRanges[0].Size, 10000)
		}
	}

	// Cut 2 full range
	newRanges, cuttedRanges, err = TxRangesCut(originalRanges, 20000)
	if err != nil {
		t.Errorf("Cut2: unexpected error %v", err)
	}
	if len(newRanges) != 6 {
		t.Errorf("Cut2: wrong length - got %d, want %d", len(newRanges), 6)
	}
	if len(cuttedRanges) != 2 {
		t.Errorf("Cut2: wrong length - got %d, want %d", len(cuttedRanges), 2)
	}

	sizeNewRange = newRanges.GetSize()
	if sizeNewRange != 60000 {
		t.Errorf("Cut2: wrong size - got %d, want %d", sizeNewRange, 60000)
	}

	sizeCuttedRange = cuttedRanges.GetSize()
	if sizeCuttedRange != 20000 {
		t.Errorf("Cut2: wrong size - got %d, want %d", sizeNewRange, 20000)
	}

	if len(newRanges) == 6 {
		if newRanges[0].Start != 2000000 {
			t.Errorf("NewRanges2: wrong start - got %d, want %d", newRanges[0].Start, 2000000)
		}
		if newRanges[0].Size != 10000 {
			t.Errorf("NewRanges2: wrong start - got %d, want %d", newRanges[0].Size, 10000)
		}
	}
	if len(cuttedRanges) == 2 {
		if cuttedRanges[0].Start != 0 {
			t.Errorf("cuttedRanges2: wrong start - got %d, want %d", cuttedRanges[0].Start, 0)
		}
		if cuttedRanges[0].Size != 10000 {
			t.Errorf("cuttedRanges2: wrong start - got %d, want %d", cuttedRanges[0].Size, 10000)
		}
		if cuttedRanges[1].Start != 1000000 {
			t.Errorf("cuttedRanges2: wrong start - got %d, want %d", cuttedRanges[1].Start, 1000000)
		}
		if cuttedRanges[1].Size != 10000 {
			t.Errorf("cuttedRanges2: wrong start - got %d, want %d", cuttedRanges[1].Size, 10000)
		}
	}

	// Cut part range
	newRanges, cuttedRanges, err = TxRangesCut(originalRanges, 4000)
	if err != nil {
		t.Errorf("Cut3: unexpected error %v", err)
	}
	if len(newRanges) != 8 {
		t.Errorf("Cut3: wrong length - got %d, want %d", len(newRanges), 8)
	}
	if len(cuttedRanges) != 1 {
		t.Errorf("Cut3: wrong length - got %d, want %d", len(cuttedRanges), 1)
	}

	sizeNewRange = newRanges.GetSize()
	if sizeNewRange != 76000 {
		t.Errorf("Cut3: wrong size - got %d, want %d", sizeNewRange, 76000)
	}

	sizeCuttedRange = cuttedRanges.GetSize()
	if sizeCuttedRange != 4000 {
		t.Errorf("Cut3: wrong size - got %d, want %d", sizeNewRange, 4000)
	}

	if len(newRanges) == 8 {
		if newRanges[0].Start != 4000 {
			t.Errorf("NewRanges3: wrong start - got %d, want %d", newRanges[0].Start, 4000)
		}
		if newRanges[0].Size != 6000 {
			t.Errorf("NewRanges3: wrong start - got %d, want %d", newRanges[0].Size, 6000)
		}
	}
	if len(cuttedRanges) == 1 {
		if cuttedRanges[0].Start != 0 {
			t.Errorf("cuttedRanges3: wrong start - got %d, want %d", cuttedRanges[0].Start, 0)
		}
		if cuttedRanges[0].Size != 4000 {
			t.Errorf("cuttedRanges3: wrong start - got %d, want %d", cuttedRanges[0].Size, 4000)
		}
	}

	// The original ranges is changed, restore it
	originalRanges = TxRanges{
		SatsRange{0, 10000},
		SatsRange{1000000, 10000},
		SatsRange{2000000, 10000},
		SatsRange{3000000, 10000},
		SatsRange{4000000, 10000},
		SatsRange{5000000, 10000},
		SatsRange{6000000, 10000},
		SatsRange{7000000, 10000}}

	// Cut one and part range
	newRanges, cuttedRanges, err = TxRangesCut(originalRanges, 14000)
	if err != nil {
		t.Errorf("Cut4: unexpected error %v", err)
	}
	if len(newRanges) != 7 {
		t.Errorf("Cut4: wrong length - got %d, want %d", len(newRanges), 7)
	}
	if len(cuttedRanges) != 2 {
		t.Errorf("Cut4: wrong length - got %d, want %d", len(cuttedRanges), 2)
	}

	sizeNewRange = newRanges.GetSize()
	if sizeNewRange != 66000 {
		t.Errorf("Cut4: wrong size - got %d, want %d", sizeNewRange, 66000)
	}

	sizeCuttedRange = cuttedRanges.GetSize()
	if sizeCuttedRange != 14000 {
		t.Errorf("Cut4: wrong size - got %d, want %d", sizeNewRange, 14000)
	}

	if len(newRanges) == 7 {
		if newRanges[0].Start != 1004000 {
			t.Errorf("NewRanges4: wrong start - got %d, want %d", newRanges[0].Start, 1004000)
		}
		if newRanges[0].Size != 6000 {
			t.Errorf("NewRanges4: wrong start - got %d, want %d", newRanges[0].Size, 6000)
		}
	}
	if len(cuttedRanges) == 2 {
		if cuttedRanges[0].Start != 0 {
			t.Errorf("cuttedRanges4: wrong start - got %d, want %d", cuttedRanges[0].Start, 0)
		}
		if cuttedRanges[0].Size != 10000 {
			t.Errorf("cuttedRanges4: wrong start - got %d, want %d", cuttedRanges[0].Size, 10000)
		}

		if cuttedRanges[1].Start != 1000000 {
			t.Errorf("cuttedRanges4: wrong start - got %d, want %d", cuttedRanges[1].Start, 1000000)
		}
		if cuttedRanges[1].Size != 4000 {
			t.Errorf("cuttedRanges4: wrong start - got %d, want %d", cuttedRanges[1].Size, 4000)
		}
	}

	// Cut two and part range
	// The original ranges is changed, restore it
	originalRanges = TxRanges{
		SatsRange{0, 10000},
		SatsRange{1000000, 10000},
		SatsRange{2000000, 10000},
		SatsRange{3000000, 10000},
		SatsRange{4000000, 10000},
		SatsRange{5000000, 10000},
		SatsRange{6000000, 10000},
		SatsRange{7000000, 10000}}

	newRanges, cuttedRanges, err = TxRangesCut(originalRanges, 24000)
	if err != nil {
		t.Errorf("Cut5: unexpected error %v", err)
	}
	if len(newRanges) != 6 {
		t.Errorf("Cut5: wrong length - got %d, want %d", len(newRanges), 6)
	}
	if len(cuttedRanges) != 3 {
		t.Errorf("Cut5: wrong length - got %d, want %d", len(cuttedRanges), 3)
	}

	sizeNewRange = newRanges.GetSize()
	if sizeNewRange != 56000 {
		t.Errorf("Cut5: wrong size - got %d, want %d", sizeNewRange, 56000)
	}

	sizeCuttedRange = cuttedRanges.GetSize()
	if sizeCuttedRange != 24000 {
		t.Errorf("Cut5: wrong size - got %d, want %d", sizeNewRange, 24000)
	}

	if len(newRanges) == 6 {
		if newRanges[0].Start != 2004000 {
			t.Errorf("NewRanges5: wrong start - got %d, want %d", newRanges[0].Start, 2004000)
		}
		if newRanges[0].Size != 6000 {
			t.Errorf("NewRanges5: wrong start - got %d, want %d", newRanges[0].Size, 6000)
		}
	}
	if len(cuttedRanges) == 3 {
		if cuttedRanges[0].Start != 0 {
			t.Errorf("cuttedRanges5: wrong start - got %d, want %d", cuttedRanges[0].Start, 0)
		}
		if cuttedRanges[0].Size != 10000 {
			t.Errorf("cuttedRanges5: wrong start - got %d, want %d", cuttedRanges[0].Size, 10000)
		}

		if cuttedRanges[1].Start != 1000000 {
			t.Errorf("cuttedRanges5: wrong start - got %d, want %d", cuttedRanges[1].Start, 1000000)
		}
		if cuttedRanges[1].Size != 10000 {
			t.Errorf("cuttedRanges5: wrong start - got %d, want %d", cuttedRanges[1].Size, 10000)
		}

		if cuttedRanges[2].Start != 2000000 {
			t.Errorf("cuttedRanges5: wrong start - got %d, want %d", cuttedRanges[1].Start, 2000000)
		}
		if cuttedRanges[2].Size != 4000 {
			t.Errorf("cuttedRanges5: wrong start - got %d, want %d", cuttedRanges[1].Size, 4000)
		}
	}

	// Cut all range
	// The original ranges is changed, restore it
	originalRanges = TxRanges{
		SatsRange{0, 10000},
		SatsRange{1000000, 10000},
		SatsRange{2000000, 10000},
		SatsRange{3000000, 10000},
		SatsRange{4000000, 10000},
		SatsRange{5000000, 10000},
		SatsRange{6000000, 10000},
		SatsRange{7000000, 10000}}

	newRanges, cuttedRanges, err = TxRangesCut(originalRanges, 80000)
	if err != nil {
		t.Errorf("Cut6: unexpected error %v", err)
	}
	if len(newRanges) != 0 {
		t.Errorf("Cut6: wrong length - got %d, want %d", len(newRanges), 0)
	}
	if len(cuttedRanges) != 8 {
		t.Errorf("Cut6: wrong length - got %d, want %d", len(cuttedRanges), 8)
	}

	sizeNewRange = newRanges.GetSize()
	if sizeNewRange != 0 {
		t.Errorf("Cut6: wrong size - got %d, want %d", sizeNewRange, 0)
	}

	sizeCuttedRange = cuttedRanges.GetSize()
	if sizeCuttedRange != 80000 {
		t.Errorf("Cut6: wrong size - got %d, want %d", sizeNewRange, 80000)
	}

	t.Logf("Running TxRanges cut completed")
}
