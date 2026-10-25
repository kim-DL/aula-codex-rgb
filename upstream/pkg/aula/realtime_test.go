//go:build windows

package aula

import "testing"

func TestRealtimeOrderCoversPhysicalKeyMapExactly(t *testing.T) {
	if len(realtimeLEDOrder) != 104 || len(KeyNameToIndex) != 104 {
		t.Fatalf("unexpected map size: order=%d map=%d", len(realtimeLEDOrder), len(KeyNameToIndex))
	}
	seen := make(map[uint8]bool, len(realtimeLEDOrder))
	for _, idx := range realtimeLEDOrder {
		if seen[idx] {
			t.Fatalf("duplicate LED address %d", idx)
		}
		seen[idx] = true
	}
	for name, idx := range KeyNameToIndex {
		if !seen[idx] {
			t.Errorf("mapped key %s at LED %d is absent from frame order", name, idx)
		}
	}
}

func TestRealtimeFramePacketsContainF1ColorAndNoFlashCommand(t *testing.T) {
	frame := LEDFrame{}
	for _, idx := range realtimeLEDOrder {
		frame[idx] = [3]uint8{0, 36, 12}
	}
	frame[KeyF1] = [3]uint8{0, 42, 45}
	packets := buildRealtimeFramePackets(frame)
	if len(packets) != 10 {
		t.Fatalf("frame packet count=%d, want 10", len(packets))
	}
	if packets[0][0] != 0x04 || packets[0][1] != 0x20 || packets[0][8] != 0x08 {
		t.Fatalf("unexpected preview command: %x", packets[0][:9])
	}
	foundF1 := false
	for _, packet := range packets {
		for i := 0; i < len(packet); i += 4 {
			if packet[i] == KeyF1 && packet[i+1] == 0 && packet[i+2] == 42 && packet[i+3] == 45 {
				foundF1 = true
			}
		}
		if packet[0] == 0x04 && packet[1] == 0xf0 {
			t.Fatal("frame contains a flash-save command")
		}
	}
	if !foundF1 {
		t.Fatal("F1 cyan color is absent from the frame payload")
	}
	if last := packets[len(packets)-1]; last[0] != 0x04 || last[1] != 0x02 {
		t.Fatalf("unexpected apply command: %x", last[:2])
	}
}

func TestRealtimeHandshakeIsVolatileAndFixed(t *testing.T) {
	packets := buildRealtimeHandshakePackets()
	want := [][2]byte{{0x04, 0x18}, {0x04, 0x28}, {0x00, 0x01}, {0x04, 0x02}}
	for i, pair := range want {
		if packets[i][0] != pair[0] || packets[i][1] != pair[1] {
			t.Fatalf("handshake step %d=%x %x", i+1, packets[i][0], packets[i][1])
		}
	}
	if packets[1][8] != 0x01 || packets[2][62] != 0xaa || packets[2][63] != 0x55 {
		t.Fatal("real-time handshake signature mismatch")
	}
	for _, packet := range packets {
		if packet[0] == 0x04 && packet[1] == 0xf0 {
			t.Fatal("handshake contains a flash-save command")
		}
	}
}
