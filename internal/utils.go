package internal

import "time"

var startTime = time.Now()

func getNow() time.Duration {
	return time.Since(startTime)
}

func Min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
