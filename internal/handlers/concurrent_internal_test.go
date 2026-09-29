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

// TestRGBAndOLEDHandlersShareConfigMutexUnderConcurrentLoad guards against
// RGBHandlers and OLEDHandlers each locking a private mutex around the same
// *config.Config: without one shared lock, concurrent rgb.* and oled.*
// requests race on *cfg (a real data race, since Config embeds a slice) and
// can lose one handler's persisted write to the other's.
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
	// Pre-write the false starting state: config.Load falls back to
	// config.Default() (both fields true) for a missing file, which would
	// make the assertions below pass even if every send silently failed.
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
