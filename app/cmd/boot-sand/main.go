//go:build windows

// One-shot development tool, never linked into the resident app.
// Only changes built-in main backlight; does not upload firmware or remap keys.
package main

import (
	"aula-codex-rgb-lab/internal/instance"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/parsiya/f108-pro/pkg/aula"
)

func main() {
	apply := flag.Bool("apply-boot-sand", false, "explicitly save dim static sand to keyboard once")
	output := flag.String("output", "boot-sand-result.json", "result evidence file")
	flag.Parse()
	cfg := aula.LightingConfig{Mode: aula.ModeStatic, R: 26, G: 16, B: 7, Brightness: 5, Speed: 0}
	result := map[string]any{"started": time.Now().UTC(), "rgb": [3]uint8{26, 16, 7}, "mode": "static", "brightness_register": 5, "usb_write_completed": false, "power_cycle_verified": false, "requested_persistent_transactions": 0}
	if !*apply {
		result["plan_only"] = true
		b, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(b))
		return
	}
	err := func() error {
		release, acquired, err := instance.Acquire()
		if err != nil {
			return err
		}
		if !acquired {
			return fmt.Errorf("resident RGB app is still running; no device opened")
		}
		defer release()
		d, err := aula.Open()
		if err != nil {
			return err
		}
		defer d.Close()
		// Exactly one transaction. Do not retry an ambiguous persistent write.
		result["requested_persistent_transactions"] = 1
		if err := d.SetLighting(cfg); err != nil {
			return err
		}
		result["usb_write_completed"] = true
		// Give the board time to finish its ordinary settings finalize before closing.
		time.Sleep(time.Second)
		return nil
	}()
	if err != nil {
		result["error"] = err.Error()
	}
	result["finished"] = time.Now().UTC()
	b, _ := json.MarshalIndent(result, "", "  ")
	if saveErr := os.WriteFile(*output, b, 0600); saveErr != nil {
		fmt.Fprintln(os.Stderr, saveErr)
		os.Exit(1)
	}
	fmt.Println(string(b))
	if err != nil {
		os.Exit(1)
	}
}
