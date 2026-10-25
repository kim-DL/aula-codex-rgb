//go:build windows

package aula

import (
	"fmt"
	"time"
)

// SetRealtimeClock uses the already exercised RGB host-control transaction,
// changing only its documented calendar fields. The single HID owner must
// serialize this complete transaction with RGB frames. No LCD pixel interface,
// profile finalize, unknown flag, or flash command is used.
func (d *Device) SetRealtimeClock(t time.Time) error {
	packets, err := realtimeClockPackets(t)
	if err != nil {
		return err
	}
	if !d.realtimeInitialized {
		return fmt.Errorf("clock requires prepared real-time owner")
	}
	for i, packet := range packets {
		if err := d.realtimeCommand(packet[:]); err != nil {
			return fmt.Errorf("clock transaction step %d: %w", i+1, err)
		}
	}
	return nil
}

func realtimeClockPackets(t time.Time) ([4][reportSize]byte, error) {
	if t.Year() < 2000 || t.Year() > 2099 {
		return [4][reportSize]byte{}, fmt.Errorf("clock year outside 2000..2099")
	}
	p := buildRealtimeHandshakePackets()
	p[2][3] = byte(t.Year() - 2000)
	p[2][4] = byte(t.Month())
	p[2][5] = byte(t.Day())
	p[2][6] = byte(t.Hour())
	p[2][7] = byte(t.Minute())
	p[2][8] = byte(t.Second())
	p[2][10] = byte(t.Weekday())
	return p, nil
}
