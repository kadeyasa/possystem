package utils

import "strconv"

func UintToStr(n uint) string {
	return strconv.FormatUint(uint64(n), 10)
}
