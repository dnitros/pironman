package handlers

import (
	"image"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/dnitros/pironman/internal/config"
	"github.com/dnitros/pironman/internal/ipc"
	"github.com/dnitros/pironman/internal/oled"
	"github.com/dnitros/pironman/internal/sysstats"
)

type fakeDisplay struct {
	drawCalls int
}

func (f *fakeDisplay) Draw(img *image.Gray) error {
	f.drawCalls++
	return nil
}

type fakeStatsSource struct{}

func (fakeStatsSource) Snapshot() (sysstats.Snapshot, error) { return sysstats.Snapshot{}, nil }

type fixedClock struct{ t time.Time }

func (f fixedClock) Now() time.Time { return f.t }

func newTestMachine(t *testing.T, initialAwake bool) (*oled.Machine, *fakeDisplay) {
	t.Helper()
	display := &fakeDisplay{}
	m, err := oled.NewMachine(display, fakeStatsSource{}, fixedClock{t: time.Now()},
		[]string{oled.PageMix, oled.PagePerformance, oled.PageIPs, oled.PageDisk},
		10*time.Second, 3*time.Second, initialAwake)
	if err != nil {
		t.Fatalf("oled.NewMachine: %v", err)
	}
	return m, display
}

func TestOLEDHandlersOnPersistsStateAndWakesMachine(t *testing.T) {
	machine, display := newTestMachine(t, false)
	cfg := config.Default()
	cfg.OLED.Enabled = false
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")

	path := startTestDaemon(t, OLEDHandlers(machine, &cfg, cfgPath, &sync.Mutex{}))

	drawsBefore := display.drawCalls
	resp, err := ipc.Send(path, "oled.on", nil)
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if !resp.OK {
		t.Fatalf("expected ok=true, got error %q", resp.Error)
	}
	if display.drawCalls <= drawsBefore {
		t.Fatalf("expected oled.on to draw to the display")
	}

	saved, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if !saved.OLED.Enabled {
		t.Fatalf("expected the saved config to have oled.enabled=true")
	}
	if !machine.State().Awake {
		t.Fatalf("expected the machine to report awake")
	}
}

func TestOLEDHandlersOffPersistsStateAndBlanksMachine(t *testing.T) {
	machine, _ := newTestMachine(t, true)
	cfg := config.Default()
	cfg.OLED.Enabled = true
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")

	path := startTestDaemon(t, OLEDHandlers(machine, &cfg, cfgPath, &sync.Mutex{}))

	resp, err := ipc.Send(path, "oled.off", nil)
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if !resp.OK {
		t.Fatalf("expected ok=true, got error %q", resp.Error)
	}

	saved, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if saved.OLED.Enabled {
		t.Fatalf("expected the saved config to have oled.enabled=false")
	}
	if machine.State().Awake {
		t.Fatalf("expected the machine to report asleep")
	}
}

func TestOLEDHandlersPageNextWakesWhenAsleepAndPersists(t *testing.T) {
	machine, _ := newTestMachine(t, false)
	cfg := config.Default()
	cfg.OLED.Enabled = false
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")

	path := startTestDaemon(t, OLEDHandlers(machine, &cfg, cfgPath, &sync.Mutex{}))

	resp, err := ipc.Send(path, "oled.page", map[string]any{"page": "next"})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if !resp.OK {
		t.Fatalf("expected ok=true, got error %q", resp.Error)
	}
	if !machine.State().Awake {
		t.Fatalf("expected \"next\" to wake the machine from asleep, not advance the page")
	}

	saved, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if !saved.OLED.Enabled {
		t.Fatalf("expected the saved config to have oled.enabled=true")
	}
}

func TestOLEDHandlersPageNextAdvancesWhenAlreadyAwake(t *testing.T) {
	machine, _ := newTestMachine(t, true)
	cfg := config.Default()
	cfg.OLED.Enabled = true
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")

	path := startTestDaemon(t, OLEDHandlers(machine, &cfg, cfgPath, &sync.Mutex{}))

	resp, err := ipc.Send(path, "oled.page", map[string]any{"page": "next"})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if !resp.OK {
		t.Fatalf("expected ok=true, got error %q", resp.Error)
	}
	if got := machine.State().Page; got != oled.PagePerformance {
		t.Fatalf("Page = %q, want %q", got, oled.PagePerformance)
	}
}

func TestOLEDHandlersPagePrevNoOpsWhileAsleepWithoutPersisting(t *testing.T) {
	machine, _ := newTestMachine(t, false)
	cfg := config.Default()
	cfg.OLED.Enabled = false
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")

	path := startTestDaemon(t, OLEDHandlers(machine, &cfg, cfgPath, &sync.Mutex{}))

	resp, err := ipc.Send(path, "oled.page", map[string]any{"page": "prev"})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if !resp.OK {
		t.Fatalf("expected ok=true (a no-op still succeeds), got error %q", resp.Error)
	}
	if machine.State().Awake {
		t.Fatalf("expected the machine to remain asleep")
	}
	if _, err := os.Stat(cfgPath); !os.IsNotExist(err) {
		t.Fatalf("expected no config file to be written for a no-op \"prev\", stat err = %v", err)
	}
}

func TestOLEDHandlersPageNameJumpsDirectlyAndWakes(t *testing.T) {
	machine, _ := newTestMachine(t, false)
	cfg := config.Default()
	cfg.OLED.Enabled = false
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")

	path := startTestDaemon(t, OLEDHandlers(machine, &cfg, cfgPath, &sync.Mutex{}))

	resp, err := ipc.Send(path, "oled.page", map[string]any{"page": "ips"})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if !resp.OK {
		t.Fatalf("expected ok=true, got error %q", resp.Error)
	}
	state := machine.State()
	if !state.Awake || state.Page != oled.PageIPs {
		t.Fatalf("State() = %+v, want awake on %q", state, oled.PageIPs)
	}

	saved, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if !saved.OLED.Enabled {
		t.Fatalf("expected the saved config to have oled.enabled=true")
	}
}

func TestOLEDHandlersPageRejectsUnknownPageWithoutPersisting(t *testing.T) {
	machine, _ := newTestMachine(t, false)
	cfg := config.Default()
	cfg.OLED.Enabled = false
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")

	path := startTestDaemon(t, OLEDHandlers(machine, &cfg, cfgPath, &sync.Mutex{}))

	resp, err := ipc.Send(path, "oled.page", map[string]any{"page": "not-a-page"})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if resp.OK {
		t.Fatalf("expected ok=false for an unknown page")
	}
	if machine.State().Awake {
		t.Fatalf("expected the machine to remain untouched for an unknown page")
	}
	if _, err := os.Stat(cfgPath); !os.IsNotExist(err) {
		t.Fatalf("expected no config file to be written for a rejected page name, stat err = %v", err)
	}
}

func TestOLEDHandlersPageRejectsMissingArgument(t *testing.T) {
	machine, _ := newTestMachine(t, false)
	cfg := config.Default()
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")

	path := startTestDaemon(t, OLEDHandlers(machine, &cfg, cfgPath, &sync.Mutex{}))

	resp, err := ipc.Send(path, "oled.page", nil)
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if resp.OK {
		t.Fatalf("expected ok=false when \"page\" is missing")
	}
}
