package handlers

import (
	"image"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/dnitros/pironman/internal/config"
	"github.com/dnitros/pironman/internal/hardware"
	"github.com/dnitros/pironman/internal/ipc"
	"github.com/dnitros/pironman/internal/oled"
	"github.com/dnitros/pironman/internal/pbm"
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

func newTestMachineWithImagePage(t *testing.T, initialAwake bool) (*oled.Machine, *fakeDisplay) {
	t.Helper()
	display := &fakeDisplay{}
	m, err := oled.NewMachine(display, fakeStatsSource{}, fixedClock{t: time.Now()},
		[]string{oled.PageMix, oled.PagePerformance, oled.PageIPs, oled.PageDisk, oled.PageImage},
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

func writeTestSourcePBM(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "src.pbm")
	img := image.NewGray(image.Rect(0, 0, hardware.SSD1306Width, hardware.SSD1306Height))
	if err := pbm.EncodeFile(path, img); err != nil {
		t.Fatalf("pbm.EncodeFile: %v", err)
	}
	return path
}

func TestOLEDHandlersImageRejectsWhenPageOrderMissingImage(t *testing.T) {
	machine, _ := newTestMachine(t, false)
	cfg := config.Default()
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")

	path := startTestDaemon(t, OLEDHandlers(machine, &cfg, cfgPath, &sync.Mutex{}))

	resp, err := ipc.Send(path, "oled.image", map[string]any{"paths": []any{writeTestSourcePBM(t)}})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if resp.OK {
		t.Fatalf("expected ok=false when %q is not in oled.page_order", oled.PageImage)
	}
	if _, err := os.Stat(cfgPath); !os.IsNotExist(err) {
		t.Fatalf("expected no config file to be written when the page_order check fails, stat err = %v", err)
	}
}

func TestOLEDHandlersImageRejectsMissingPaths(t *testing.T) {
	machine, _ := newTestMachineWithImagePage(t, false)
	cfg := config.Default()
	cfg.OLED.PageOrder = append(cfg.OLED.PageOrder, oled.PageImage)
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")

	path := startTestDaemon(t, OLEDHandlers(machine, &cfg, cfgPath, &sync.Mutex{}))

	resp, err := ipc.Send(path, "oled.image", nil)
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if resp.OK {
		t.Fatalf("expected ok=false when \"paths\" is missing")
	}
}

func TestOLEDHandlersImagePersistsConvertsAndSwitchesPage(t *testing.T) {
	machine, display := newTestMachineWithImagePage(t, false)
	cfg := config.Default()
	cfg.OLED.PageOrder = append(cfg.OLED.PageOrder, oled.PageImage)
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")

	path := startTestDaemon(t, OLEDHandlers(machine, &cfg, cfgPath, &sync.Mutex{}))

	drawsBefore := display.drawCalls
	resp, err := ipc.Send(path, "oled.image", map[string]any{"paths": []any{writeTestSourcePBM(t)}})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if !resp.OK {
		t.Fatalf("expected ok=true, got error %q", resp.Error)
	}
	if display.drawCalls <= drawsBefore {
		t.Fatalf("expected oled.image to draw to the display")
	}

	state := machine.State()
	if !state.Awake || state.Page != oled.PageImage {
		t.Fatalf("State() = %+v, want awake on %q", state, oled.PageImage)
	}

	saved, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if !saved.OLED.Enabled {
		t.Fatalf("expected the saved config to have oled.enabled=true")
	}
	if len(saved.OLED.ImagePaths) != 1 {
		t.Fatalf("saved.OLED.ImagePaths = %v, want exactly one persisted path", saved.OLED.ImagePaths)
	}
	imagesDir := filepath.Join(filepath.Dir(cfgPath), "images")
	if filepath.Dir(saved.OLED.ImagePaths[0]) != imagesDir {
		t.Fatalf("persisted path %q, want it under %q", saved.OLED.ImagePaths[0], imagesDir)
	}
	if saved.OLED.ImageIntervalSeconds != config.Default().OLED.ImageIntervalSeconds {
		t.Fatalf("ImageIntervalSeconds = %d, want the unmodified default", saved.OLED.ImageIntervalSeconds)
	}
}

func TestOLEDHandlersImageAppliesIntervalOverride(t *testing.T) {
	machine, _ := newTestMachineWithImagePage(t, false)
	cfg := config.Default()
	cfg.OLED.PageOrder = append(cfg.OLED.PageOrder, oled.PageImage)
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")

	path := startTestDaemon(t, OLEDHandlers(machine, &cfg, cfgPath, &sync.Mutex{}))

	resp, err := ipc.Send(path, "oled.image", map[string]any{
		"paths":    []any{writeTestSourcePBM(t)},
		"interval": float64(20),
	})
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
	if saved.OLED.ImageIntervalSeconds != 20 {
		t.Fatalf("ImageIntervalSeconds = %d, want 20", saved.OLED.ImageIntervalSeconds)
	}
}

func TestOLEDHandlersImageInvertFlipsPersistedResult(t *testing.T) {
	machine, _ := newTestMachineWithImagePage(t, false)
	cfg := config.Default()
	cfg.OLED.PageOrder = append(cfg.OLED.PageOrder, oled.PageImage)
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")

	path := startTestDaemon(t, OLEDHandlers(machine, &cfg, cfgPath, &sync.Mutex{}))

	// writeTestSourcePBM produces an all-unlit (Y=0) source image.
	resp, err := ipc.Send(path, "oled.image", map[string]any{
		"paths":  []any{writeTestSourcePBM(t)},
		"invert": true,
	})
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
	img, err := pbm.DecodeFile(saved.OLED.ImagePaths[0])
	if err != nil {
		t.Fatalf("pbm.DecodeFile: %v", err)
	}
	for _, v := range img.Pix {
		if v == 0 {
			t.Fatalf("expected every pixel to be lit after inverting an all-unlit source, found an unlit pixel")
		}
	}
}

func TestOLEDHandlersImagePropagatesConversionErrorWithoutPersisting(t *testing.T) {
	machine, _ := newTestMachineWithImagePage(t, false)
	cfg := config.Default()
	cfg.OLED.PageOrder = append(cfg.OLED.PageOrder, oled.PageImage)
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")

	path := startTestDaemon(t, OLEDHandlers(machine, &cfg, cfgPath, &sync.Mutex{}))

	resp, err := ipc.Send(path, "oled.image", map[string]any{
		"paths": []any{filepath.Join(t.TempDir(), "missing.png")},
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if resp.OK {
		t.Fatalf("expected ok=false for an unreadable source path")
	}
	if _, err := os.Stat(cfgPath); !os.IsNotExist(err) {
		t.Fatalf("expected no config file to be written when conversion fails, stat err = %v", err)
	}
	if machine.State().Page == oled.PageImage {
		t.Fatalf("expected the machine to remain untouched when conversion fails")
	}
}

func TestOLEDHandlersImageReplacingPathsRemovesOrphanedFiles(t *testing.T) {
	machine, _ := newTestMachineWithImagePage(t, false)
	cfg := config.Default()
	cfg.OLED.PageOrder = append(cfg.OLED.PageOrder, oled.PageImage)
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")

	path := startTestDaemon(t, OLEDHandlers(machine, &cfg, cfgPath, &sync.Mutex{}))

	// Persisted filenames are positional (image-0, image-1, ...), so an
	// orphan can only appear when a later call configures fewer images than
	// a previous one left behind.
	resp, err := ipc.Send(path, "oled.image", map[string]any{"paths": []any{writeTestSourcePBM(t), writeTestSourcePBM(t)}})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if !resp.OK {
		t.Fatalf("expected ok=true, got error %q", resp.Error)
	}
	if len(cfg.OLED.ImagePaths) != 2 {
		t.Fatalf("ImagePaths = %v, want 2 persisted paths", cfg.OLED.ImagePaths)
	}
	secondPersisted := cfg.OLED.ImagePaths[1]
	if _, err := os.Stat(secondPersisted); err != nil {
		t.Fatalf("expected the second persisted image to exist: %v", err)
	}

	resp, err = ipc.Send(path, "oled.image", map[string]any{"paths": []any{writeTestSourcePBM(t)}})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if !resp.OK {
		t.Fatalf("expected ok=true, got error %q", resp.Error)
	}

	if _, err := os.Stat(secondPersisted); !os.IsNotExist(err) {
		t.Fatalf("expected the orphaned second image to be removed once a later call configures fewer paths, stat err = %v", err)
	}
}

func TestOLEDHandlersImagePersistsDistinctFilesForSameBasenameSources(t *testing.T) {
	machine, _ := newTestMachineWithImagePage(t, false)
	cfg := config.Default()
	cfg.OLED.PageOrder = append(cfg.OLED.PageOrder, oled.PageImage)
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")

	path := startTestDaemon(t, OLEDHandlers(machine, &cfg, cfgPath, &sync.Mutex{}))

	dirA, dirB := t.TempDir(), t.TempDir()
	srcA := filepath.Join(dirA, "img.pbm")
	srcB := filepath.Join(dirB, "img.pbm")
	blank := image.NewGray(image.Rect(0, 0, hardware.SSD1306Width, hardware.SSD1306Height))
	if err := pbm.EncodeFile(srcA, blank); err != nil {
		t.Fatalf("EncodeFile(A): %v", err)
	}
	if err := pbm.EncodeFile(srcB, blank); err != nil {
		t.Fatalf("EncodeFile(B): %v", err)
	}

	resp, err := ipc.Send(path, "oled.image", map[string]any{"paths": []any{srcA, srcB}})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if !resp.OK {
		t.Fatalf("expected ok=true, got error %q", resp.Error)
	}
	if len(cfg.OLED.ImagePaths) != 2 || cfg.OLED.ImagePaths[0] == cfg.OLED.ImagePaths[1] {
		t.Fatalf("expected two distinct persisted paths for same-basename sources, got %v", cfg.OLED.ImagePaths)
	}
	for _, p := range cfg.OLED.ImagePaths {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("expected %q to exist: %v", p, err)
		}
	}
}
