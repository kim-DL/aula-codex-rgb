//go:build windows

package aula

import (
	"errors"
	"reflect"
	"testing"
)

type bootSandRecordingTransport struct {
	sets   [][reportSize]byte
	reads  int
	failAt int
}

func (t *bootSandRecordingTransport) SetFeatureReport(p [reportSize]byte) error {
	t.sets = append(t.sets, p)
	if t.failAt == len(t.sets) {
		return errors.New("injected failure")
	}
	return nil
}
func (t *bootSandRecordingTransport) GetFeatureReport() ([reportSize]byte, error) {
	t.reads++
	return [reportSize]byte{}, nil
}
func (t *bootSandRecordingTransport) InitLCD() error { return errors.New("LCD access forbidden") }
func (t *bootSandRecordingTransport) WriteLCDPage([]byte) error {
	return errors.New("LCD access forbidden")
}
func (t *bootSandRecordingTransport) ReadLCDAck() ([]byte, error) {
	return nil, errors.New("LCD access forbidden")
}
func (t *bootSandRecordingTransport) Close() {}

func bootSandExpectedReports() [][reportSize]byte {
	out := make([][reportSize]byte, 5)
	out[0][0], out[0][1] = 4, 0x18
	out[1][0], out[1][1], out[1][8] = 4, 0x13, 1
	out[2][0], out[2][1], out[2][2], out[2][3] = 1, 26, 16, 7
	out[2][9], out[2][14], out[2][15] = 5, 0x55, 0xaa
	out[3][0], out[3][1] = 4, 2
	out[4][0], out[4][1] = 4, 0xf0
	return out
}
func TestBootSandLightingWritesOnlyTheFiveApprovedReports(t *testing.T) {
	tx := &bootSandRecordingTransport{}
	d := Device{transport: tx}
	if err := d.SetLighting(LightingConfig{Mode: ModeStatic, R: 26, G: 16, B: 7, Brightness: 5}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(tx.sets, bootSandExpectedReports()) {
		t.Fatalf("unexpected lighting reports: %x", tx.sets)
	}
	if tx.reads != 3 {
		t.Fatalf("unexpected readbacks: %d", tx.reads)
	}
}
func TestBootSandLightingDoesNotFinalizeAfterFailedEarlierWrite(t *testing.T) {
	for _, failAt := range []int{1, 2, 3, 4} {
		tx := &bootSandRecordingTransport{failAt: failAt}
		d := Device{transport: tx}
		if err := d.SetLighting(LightingConfig{Mode: ModeStatic, R: 26, G: 16, B: 7, Brightness: 5}); err == nil {
			t.Fatal("failure ignored")
		}
		if len(tx.sets) != failAt {
			t.Fatal("continued after failure")
		}
		for _, p := range tx.sets {
			if p[0] == 4 && p[1] == 0xf0 {
				t.Fatal("finalize sent after failure")
			}
		}
	}
}
