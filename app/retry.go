package main

import "time"

// Retry bursts are bounded; a disconnected keyboard never permanently stops
// the hardware worker. No pending frames are retained during the cooldown.
func reconnectDelay(failures int) time.Duration {
	if failures >= 10 {
		return 30 * time.Second
	}
	return 3 * time.Second
}

func scheduleGap(previous, current time.Time) bool {
	return !previous.IsZero() && current.UnixNano()-previous.UnixNano() > int64(5*time.Second)
}
