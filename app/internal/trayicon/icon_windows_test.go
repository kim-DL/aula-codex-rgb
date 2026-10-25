//go:build windows

package trayicon

import (
	"encoding/binary"
	"testing"
)

func TestGeneratedIconHasValidSingleImageContainer(t *testing.T) {
	icon := Bytes()
	if len(icon) < 22 || binary.LittleEndian.Uint16(icon[2:4]) != 1 || binary.LittleEndian.Uint16(icon[4:6]) != 1 {
		t.Fatal("invalid ICO header")
	}
	if icon[6] != 32 || icon[7] != 32 || int(binary.LittleEndian.Uint32(icon[14:18])) != len(icon)-22 {
		t.Fatal("invalid ICO image dimensions or size")
	}
}
