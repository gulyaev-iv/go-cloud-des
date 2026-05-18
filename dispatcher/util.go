package main

import "strings"

func safePathName(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, "\\", "_")
	value = strings.ReplaceAll(value, "/", "_")
	value = strings.ReplaceAll(value, ":", "_")
	if value == "" {
		return "experiment"
	}
	return value
}

func isHex(value string) bool {
	for _, ch := range value {
		if ch >= '0' && ch <= '9' {
			continue
		}
		if ch >= 'a' && ch <= 'f' {
			continue
		}
		if ch >= 'A' && ch <= 'F' {
			continue
		}
		return false
	}

	return true
}

func modelBinaryFileName(goos string) string {
	if goos == "windows" {
		return "model.exe"
	}

	return "model"
}
