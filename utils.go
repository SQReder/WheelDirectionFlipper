package main

import (
	"strings"
)

func contains(array []string, value string) bool {
	for i := 0; i < len(array); i++ {
		if array[i] == value {
			return true
		}
	}

	return false
}

func joinPath(segments ...string) string {
	return strings.Join(segments, `\`)
}
