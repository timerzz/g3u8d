package utils

import (
	"fmt"
	"time"
)

func SizePercentFormat(lastTime time.Time, size int64) string {
	s := time.Now().Sub(lastTime).Seconds()
	showSize := float64(size) / s
	units := []string{"B", "KB", "MB", "GB", "TB", "PB"}
	idx := 0

	for showSize >= 1024 {
		showSize = showSize / 1024
		idx++
	}
	return fmt.Sprintf("%.2f %s / s", showSize, units[idx])
}
