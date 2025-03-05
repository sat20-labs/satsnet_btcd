package indexer

import (
	"fmt"
	"math"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

// memory util
func PrintMemUsage() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// For info on each, see: https://golang.org/pkg/runtime/#MemStats
	fmt.Printf("Alloc = %v MiB", bToMb(m.Alloc))
	fmt.Printf("\tTotalAlloc = %v MiB", bToMb(m.TotalAlloc))
	fmt.Printf("\tSys = %v MiB", bToMb(m.Sys))
	fmt.Printf("\tNumGC = %v\n", m.NumGC)
}

func GetSysMb() uint64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return bToMb(m.Sys)
}

func GetAlloc() uint64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return bToMb(m.Alloc)
}

func bToMb(b uint64) uint64 {
	return b / 1024 / 1024
}


func getPercentage(str string) (int, error) {
	// 只接受两位小数，或者100%
	str2 := strings.TrimSpace(str)

	var f float64
	var err error
	if strings.Contains(str2, "%") {
		str2 = strings.TrimRight(str2, "%") // 去掉百分号
		if strings.Contains(str2, ".") {
			parts := strings.Split(str2, ".")
			str3 := strings.Trim(parts[1], "0")
			if str3 != "" {
				return 0, fmt.Errorf("invalid format %s", str)
			}
		}
		f, err = strconv.ParseFloat(str2, 32)
	} else {
		regex := `^\d+(\.\d{0,2})?$`
		str2 = strings.TrimRight(str2, "0")
		var math bool
		math, err = regexp.MatchString(regex, str2)
		if err != nil || !math {
			return 0, fmt.Errorf("invalid format %s", str)
		}

		f, err = strconv.ParseFloat(str2, 32)
		f = f * 100
	}

	r := int(math.Round(f))
	if r > 100 {
		return 0, fmt.Errorf("invalid format %s", str)
	}

	return r, err
}
