package main

import "fmt"

func formatBytes(bytes int64) string {
	const (
		kib = 1024
		mib = kib * 1024
		gib = mib * 1024
	)

	switch {
	case bytes >= gib:
		return fmt.Sprintf("%.1f GB", float64(bytes)/gib)
	case bytes >= mib:
		return fmt.Sprintf("%.1f MB", float64(bytes)/mib)
	case bytes >= kib:
		return fmt.Sprintf("%.1f KB", float64(bytes)/kib)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
