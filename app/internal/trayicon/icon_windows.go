//go:build windows

package trayicon

import "encoding/binary"

// Bytes returns a tiny original ICO keyboard glyph for the Windows tray.
func Bytes() []byte {
	const width, height = 32, 32
	const xorBytes = width * height * 4
	const maskBytes = 4 * height
	const dibBytes = 40 + xorBytes + maskBytes
	out := make([]byte, 22+dibBytes)
	binary.LittleEndian.PutUint16(out[2:4], 1) // ICO resource type.
	binary.LittleEndian.PutUint16(out[4:6], 1) // One image.
	out[6], out[7] = width, height
	binary.LittleEndian.PutUint16(out[10:12], 1)
	binary.LittleEndian.PutUint16(out[12:14], 32)
	binary.LittleEndian.PutUint32(out[14:18], dibBytes)
	binary.LittleEndian.PutUint32(out[18:22], 22)
	dib := out[22:]
	binary.LittleEndian.PutUint32(dib[0:4], 40)
	binary.LittleEndian.PutUint32(dib[4:8], width)
	binary.LittleEndian.PutUint32(dib[8:12], height*2) // XOR bitmap plus AND mask.
	binary.LittleEndian.PutUint16(dib[12:14], 1)
	binary.LittleEndian.PutUint16(dib[14:16], 32)
	binary.LittleEndian.PutUint32(dib[20:24], xorBytes+maskBytes)
	colors := [][3]byte{{134, 77, 220}, {22, 178, 224}, {45, 116, 245}, {28, 202, 128}}
	maskStart := 40 + xorBytes
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			inside := roundedSquare(x, y)
			color := [3]byte{18, 28, 48}
			if inside {
				for row := 0; row < 3; row++ {
					for col := 0; col < 4; col++ {
						left, top := 4+col*6, 5+row*7
						if x >= left && x < left+4 && y >= top && y < top+4 {
							color = colors[(row+col)%len(colors)]
						}
					}
				}
			}
			row := height - 1 - y
			offset := 40 + (row*width+x)*4
			dib[offset], dib[offset+1], dib[offset+2] = color[2], color[1], color[0]
			if inside {
				dib[offset+3] = 0xff
			} else {
				dib[maskStart+row*4+x/8] |= 1 << (7 - uint(x%8))
			}
		}
	}
	return out
}

func roundedSquare(x, y int) bool {
	if x >= 5 && x < 27 || y >= 5 && y < 27 {
		return true
	}
	for _, corner := range [][2]int{{5, 5}, {26, 5}, {5, 26}, {26, 26}} {
		dx, dy := x-corner[0], y-corner[1]
		if dx*dx+dy*dy <= 20 {
			return true
		}
	}
	return false
}
