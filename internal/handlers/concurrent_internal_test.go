package handlers

import (
	"fmt"
	"maps"
	"path/filepath"
	"sync"
	"testing"

	"github.com/dnitros/pironman/internal/config"
	"github.com/dnitros/pironman/internal/ipc"
	"github.com/dnitros/pironman/internal/rgb"
)

func TestRGBAndOLEDHandlersShareConfigMutexUnderConcurrentLoad(t *testing.T) {
	strip := &fakeStrip{}
	rgbStore, err := rgb.NewStore(strip, rgb.State{Enabled: false})
	if err != nil {
		t.Fatalf("rgb.NewStore: %v", err)
	}
	machine, _ := newTestMachine(t, false)

	cfg := config.Default()
	cfg.RGB.Enabled = false
	cfg.OLED.Enabled = false
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")

	if err := cfg.Save(cfgPath); err != nil {
		t.Fatalf("Save: %v", err)
	}

	var cfgMu sync.Mutex
	handlerMap := map[string]ipc.Handler{}
	maps.Copy(handlerMap, RGBHandlers(rgbStore, &cfg, cfgPath, &cfgMu))
	maps.Copy(handlerMap, OLEDHandlers(machine, &cfg, cfgPath, &cfgMu))

	path := startTestDaemon(t, handlerMap)

	const n = 50
	var wg sync.WaitGroup
	errs := make(chan error, 2*n)
	send := func(cmd string) {
		defer wg.Done()
		resp, err := ipc.Send(path, cmd, nil)
		if err != nil {
			errs <- fmt.Errorf("%s: %w", cmd, err)
			return
		}
		if !resp.OK {
			errs <- fmt.Errorf("%s: %s", cmd, resp.Error)
		}
	}
	for i := 0; i < n; i++ {
		wg.Add(2)
		go send("rgb.on")
		go send("oled.on")
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("concurrent send failed: %v", err)
	}

	saved, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if !saved.RGB.Enabled {
		t.Fatalf("expected the persisted config to have rgb.enabled=true after concurrent rgb.on/oled.on, got false — a lost update")
	}
	if !saved.OLED.Enabled {
		t.Fatalf("expected the persisted config to have oled.enabled=true after concurrent rgb.on/oled.on, got false — a lost update")
	}
}
