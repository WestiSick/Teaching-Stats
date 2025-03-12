package utils

import (
	"fmt"
	"html/template"
	"math"
)

// FormatFileSize formats file size in bytes to a human-readable string (KB, MB, etc.)
func FormatFileSize(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%d B", size)
	}

	size_kb := float64(size) / 1024.0
	if size_kb < 1024.0 {
		return fmt.Sprintf("%.1f KB", size_kb)
	}

	size_mb := size_kb / 1024.0
	if size_mb < 1024.0 {
		return fmt.Sprintf("%.1f MB", size_mb)
	}

	size_gb := size_mb / 1024.0
	return fmt.Sprintf("%.1f GB", size_gb)
}

// GetTemplateFuncMap returns a map of custom template functions
func GetTemplateFuncMap() template.FuncMap {
	return template.FuncMap{
		"formatFileSize": FormatFileSize,
		"min": func(a, b int) int {
			if a < b {
				return a
			}
			return b
		},
		"max": func(a, b int) int {
			if a > b {
				return a
			}
			return b
		},
		"add": func(a, b int) int {
			return a + b
		},
		"sub": func(a, b int) int {
			return a - b
		},
		"mul": func(a, b int) int {
			return a * b
		},
		"div": func(a, b int) int {
			if b == 0 {
				return 0
			}
			return a / b
		},
		"round": func(val float64) int {
			return int(math.Round(val))
		},
	}
}
