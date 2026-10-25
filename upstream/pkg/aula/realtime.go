//go:build windows

package aula

import (
	"errors"
	"fmt"
	"syscall"
	"time"
)

// LEDFrame is one volatile full-color frame. Indices match the F108 Pro wired
// USB map. The real-time protocol deliberately has no flash-save operation.
type LEDFrame map[uint8][3]uint8

// PrepareRealtime completes the existing volatile handshake and firmware wait
// before an animation clock starts. It is idempotent for this open handle.
// The device still needs its full settling delay; no protocol delay is reduced.
func (d *Device) PrepareRealtime() error {
	if len(KeyNameToIndex) != len(realtimeLEDOrder) {
		return fmt.Errorf("F108 Pro key map has %d keys, real-time order has %d", len(KeyNameToIndex), len(realtimeLEDOrder))
	}
	if !d.realtimeInitialized {
		start := time.Now()
		d.realtimePreparation[0]++
		defer func() { d.realtimePreparation[1] += uint64(time.Since(start)) }()
		for i, packet := range buildRealtimeHandshakePackets() {
			if err := d.realtimeCommand(packet[:]); err != nil {
				return fmt.Errorf("enter volatile real-time mode step %d/4: %w", i+1, err)
			}
		}
		d.realtimePause(200 * time.Millisecond)
		d.realtimeInitialized = true
	}
	return nil
}

// SendRealtimeFrame retains lazy initialization for existing callers. Animation
// owners should explicitly PrepareRealtime before rendering their first frame.
func (d *Device) SendRealtimeFrame(frame LEDFrame) error {
	started := time.Now()
	preparationBefore := d.realtimePreparation[1]
	d.realtimeFrameTiming = RealtimeFrameTiming{}
	defer func() {
		d.realtimeProfiling = false
		d.realtimeFrameTiming.TotalNanos = uint64(time.Since(started))
		d.realtimeFrameTiming.InitializationNanos = d.realtimePreparation[1] - preparationBefore
	}()
	if err := d.PrepareRealtime(); err != nil {
		return err
	}
	d.realtimeProfiling = true
	packets := buildRealtimeFramePackets(frame)
	for i, packet := range packets {
		if i == 0 || i == len(packets)-1 {
			command := d.realtimeCommand
			if i == len(packets)-1 {
				// Leave time for the firmware to consume the data terminator.
				d.realtimePause(8 * time.Millisecond)
				command = d.applyRealtimeCommand
			}
			if err := command(packet[:]); err != nil {
				if i == 0 {
					return fmt.Errorf("enter RGB frame preview: %w", err)
				}
				return fmt.Errorf("apply RGB frame: %w", err)
			}
			continue
		}
		if err := d.realtimeSet(packet); err != nil {
			return fmt.Errorf("RGB frame packet %d/%d: %w", i+1, len(packets), err)
		}
		if i > 0 && i < len(packets)-2 {
			d.realtimePause(2 * time.Millisecond)
		}
	}
	return nil
}

type RealtimeStats struct {
	ApplyErrors, ApplyRetries, ApplyRecoveries  uint64
	InitializationAttempts, InitializationNanos uint64
	LastFrame                                   RealtimeFrameTiming
}

// Read by the same owner that sends frames, never by another USB worker.
func (d *Device) RealtimeStats() RealtimeStats {
	return RealtimeStats{ApplyErrors: d.realtimeApply[0], ApplyRetries: d.realtimeApply[1], ApplyRecoveries: d.realtimeApply[2],
		InitializationAttempts: d.realtimePreparation[0], InitializationNanos: d.realtimePreparation[1], LastFrame: d.realtimeFrameTiming}
}

func (d *Device) realtimeSet(report [reportSize]byte) error {
	if !d.realtimeProfiling {
		return d.setFeatureReport(report)
	}
	start := time.Now()
	err := d.setFeatureReport(report)
	d.realtimeFrameTiming.WriteNanos += uint64(time.Since(start))
	return err
}

func (d *Device) realtimeGet() ([reportSize]byte, error) {
	if !d.realtimeProfiling {
		return d.getFeatureReport()
	}
	start := time.Now()
	report, err := d.getFeatureReport()
	d.realtimeFrameTiming.ReadNanos += uint64(time.Since(start))
	return report, err
}

func (d *Device) realtimePause(duration time.Duration) {
	if !d.realtimeProfiling {
		time.Sleep(duration)
		return
	}
	start := time.Now()
	time.Sleep(duration)
	d.realtimeFrameTiming.RequestedSleepNanos += uint64(duration)
	d.realtimeFrameTiming.ActualSleepNanos += uint64(time.Since(start))
}

// Only failed I/O at the volatile frame-apply stage is retried. A failed data
// packet, disconnected device, access error or exhausted burst still escapes
// to the existing close/reconnect path. No initialization/profile/flash packet
// is introduced by recovery.
func (d *Device) applyRealtimeCommand(payload []byte) error {
	var report [reportSize]byte
	copy(report[:], payload)
	retries := 0
	retry := func(err error) bool {
		d.realtimeApply[0]++
		if !errors.Is(err, syscall.Errno(31)) || retries >= 2 {
			return false
		}
		d.realtimeApply[1]++
		retries++
		d.realtimePause(time.Duration(retries) * 20 * time.Millisecond)
		return true
	}
	for {
		err := d.realtimeSet(report)
		if err == nil {
			break
		}
		if !retry(err) {
			return err
		}
	}
	// A failed readback does not justify repeating a successful apply write.
	d.realtimePause(8 * time.Millisecond)
	for {
		_, err := d.realtimeGet()
		if err == nil {
			break
		}
		if !retry(err) {
			return err
		}
	}
	d.realtimePause(2 * time.Millisecond)
	if retries > 0 {
		d.realtimeApply[2]++
	}
	return nil
}

func (d *Device) realtimeCommand(payload []byte) error {
	var report [reportSize]byte
	copy(report[:], payload)
	if err := d.realtimeSet(report); err != nil {
		return err
	}
	d.realtimePause(2 * time.Millisecond)
	if _, err := d.realtimeGet(); err != nil {
		return err
	}
	d.realtimePause(2 * time.Millisecond)
	return nil
}

// buildRealtimeHandshakePackets is the host-control handshake documented by
// the AULA F108 Driver reference. It never selects a profile or writes flash.
func buildRealtimeHandshakePackets() [4][reportSize]byte {
	var packets [4][reportSize]byte
	packets[0][0], packets[0][1] = 0x04, 0x18
	packets[1][0], packets[1][1], packets[1][8] = 0x04, 0x28, 0x01
	copy(packets[2][:], []byte{0x00, 0x01, 0x5a, 0x1a, 0x03, 0x09, 0x00, 0x01, 0x02, 0x00, 0x01})
	packets[2][62], packets[2][63] = 0xaa, 0x55
	packets[3][0], packets[3][1] = 0x04, 0x02
	return packets
}

func buildRealtimeFramePackets(frame LEDFrame) [][reportSize]byte {
	packets := make([][reportSize]byte, 0, 10)
	var preview [reportSize]byte
	preview[0], preview[1], preview[8] = 0x04, 0x20, 0x08
	packets = append(packets, preview)
	for start := 0; start < len(realtimeLEDOrder); start += 16 {
		var report [reportSize]byte
		end := start + 16
		if end > len(realtimeLEDOrder) {
			end = len(realtimeLEDOrder)
		}
		offset := 0
		for _, idx := range realtimeLEDOrder[start:end] {
			color := frame[idx]
			report[offset], report[offset+1], report[offset+2], report[offset+3] = idx, color[0], color[1], color[2]
			offset += 4
		}
		packets = append(packets, report)
	}
	packets = append(packets, [reportSize]byte{}) // Frame-data terminator.
	var apply [reportSize]byte
	apply[0], apply[1] = 0x04, 0x02
	packets = append(packets, apply)
	return packets
}

// F108 Pro's 104 wired LED addresses in the stable protocol order published
// by the AULA-F108-Driver project. The address travels with each RGB triplet.
var realtimeLEDOrder = []uint8{
	1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 112, 113, 115,
	19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 103, 116, 117,
	118, 119, 120, 121, 32, 33, 34, 122, 37, 38, 39, 40, 41, 42, 43, 44,
	45, 46, 47, 48, 49, 67, 50, 51, 52, 123, 55, 56, 57, 58, 59, 60,
	61, 62, 63, 64, 65, 66, 85, 68, 69, 70, 73, 74, 75, 76, 77, 78,
	79, 80, 81, 82, 83, 84, 101, 86, 87, 88, 106, 91, 92, 93, 94, 95,
	96, 97, 98, 99, 100, 102, 104, 105,
}
