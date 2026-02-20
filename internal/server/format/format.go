package format

import "fmt"

func Bytes(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%d B", size)
	}

	units := []string{"KiB", "MiB", "GiB", "TiB", "PiB"}
	value := float64(size)
	unitIdx := -1
	for value >= 1024 && unitIdx < len(units)-1 {
		value /= 1024
		unitIdx++
	}

	return fmt.Sprintf("%.2f %s", value, units[unitIdx])
}

func BytesPerSecond(bytesPerSecond float64) string {
	if bytesPerSecond < 1024 {
		return fmt.Sprintf("%.2f B/s", bytesPerSecond)
	}

	units := []string{"KiB/s", "MiB/s", "GiB/s", "TiB/s", "PiB/s"}
	value := bytesPerSecond
	unitIdx := -1
	for value >= 1024 && unitIdx < len(units)-1 {
		value /= 1024
		unitIdx++
	}

	return fmt.Sprintf("%.2f %s", value, units[unitIdx])
}
