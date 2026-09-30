package cli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dnitros/pironman/internal/ipc"
)

var errBoom = errors.New("boom")

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

func shortSocketDir(t *testing.T) string {
	t.Helper()

	dir, err := os.MkdirTemp("", "cli")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

func startTestDaemon(t *testing.T, handlers map[string]ipc.Handler) string {
	t.Helper()

	path := filepath.Join(shortSocketDir(t), "pironman.sock")
	srv := ipc.NewServer(handlers)
	if err := srv.Listen(path); err != nil {
		t.Fatalf("Listen: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		srv.Serve(ctx)
		close(done)
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})

	return path
}

func TestGatherDoctorReachableInstalledActive(t *testing.T) {
	path := startTestDaemon(t, map[string]ipc.Handler{
		"ping": func(args map[string]any) (any, error) {
			return map[string]string{"message": "pong"}, nil
		},
	})
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	mgr := &fakeServiceManager{installed: true, active: true}

	info, err := gatherDoctor(path, mgr, cfgPath, filepath.Join(t.TempDir(), "spidev0.0"), filepath.Join(t.TempDir(), "i2c-1"), filepath.Join(t.TempDir(), "input-devices"))
	if err != nil {
		t.Fatalf("gatherDoctor: %v", err)
	}
	if !info.Reachable {
		t.Fatalf("expected daemon to be reachable, got unreachable reason %q", info.UnreachableReason)
	}
	if !info.Installed || !info.Active {
		t.Fatalf("expected installed=true active=true, got %+v", info)
	}
}

func TestGatherDoctorUnreachableSocket(t *testing.T) {
	path := filepath.Join(t.TempDir(), "no-such-daemon.sock")
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	mgr := &fakeServiceManager{installed: false, active: false}

	info, err := gatherDoctor(path, mgr, cfgPath, filepath.Join(t.TempDir(), "spidev0.0"), filepath.Join(t.TempDir(), "i2c-1"), filepath.Join(t.TempDir(), "input-devices"))
	if err != nil {
		t.Fatalf("expected gatherDoctor to degrade gracefully, got error: %v", err)
	}
	if info.Reachable {
		t.Fatalf("expected daemon to be unreachable")
	}
	if info.UnreachableReason == "" {
		t.Fatalf("expected a clear unreachable reason")
	}
}

func TestGatherDoctorReportsUnsupportedPlatformWithoutHardErroring(t *testing.T) {
	path := filepath.Join(t.TempDir(), "no-such-daemon.sock")
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	mgr := &fakeServiceManager{unsupported: true, isInstalledErr: errBoom, isActiveErr: errBoom}

	info, err := gatherDoctor(path, mgr, cfgPath, filepath.Join(t.TempDir(), "spidev0.0"), filepath.Join(t.TempDir(), "i2c-1"), filepath.Join(t.TempDir(), "input-devices"))
	if err != nil {
		t.Fatalf("expected gatherDoctor to degrade gracefully on an unsupported platform, got: %v", err)
	}
	if info.PlatformSupported {
		t.Fatalf("expected PlatformSupported to be false")
	}
	if info.Installed || info.Active {
		t.Fatalf("expected installed/active to be skipped (not just false) when unsupported, got %+v", info)
	}
}

func TestGatherDoctorPropagatesIsInstalledError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "no-such-daemon.sock")
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	mgr := &fakeServiceManager{isInstalledErr: errBoom}

	if _, err := gatherDoctor(path, mgr, cfgPath, filepath.Join(t.TempDir(), "spidev0.0"), filepath.Join(t.TempDir(), "i2c-1"), filepath.Join(t.TempDir(), "input-devices")); err == nil {
		t.Fatalf("expected an error when IsInstalled fails")
	}
}

func TestGatherDoctorPropagatesIsActiveError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "no-such-daemon.sock")
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	mgr := &fakeServiceManager{installed: true, isActiveErr: errBoom}

	if _, err := gatherDoctor(path, mgr, cfgPath, filepath.Join(t.TempDir(), "spidev0.0"), filepath.Join(t.TempDir(), "i2c-1"), filepath.Join(t.TempDir(), "input-devices")); err == nil {
		t.Fatalf("expected an error when IsActive fails")
	}
}

func TestGatherDoctorReportsMissingConfigAsDefaulted(t *testing.T) {
	path := startTestDaemon(t, map[string]ipc.Handler{
		"ping": func(args map[string]any) (any, error) { return nil, nil },
	})
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	mgr := &fakeServiceManager{installed: true, active: true}

	info, err := gatherDoctor(path, mgr, cfgPath, filepath.Join(t.TempDir(), "spidev0.0"), filepath.Join(t.TempDir(), "i2c-1"), filepath.Join(t.TempDir(), "input-devices"))
	if err != nil {
		t.Fatalf("gatherDoctor: %v", err)
	}
	if !strings.Contains(info.ConfigStatus, "missing") {
		t.Fatalf("expected config status to note the file is missing, got %q", info.ConfigStatus)
	}
}

func TestGatherDoctorReportsReadableConfig(t *testing.T) {
	path := startTestDaemon(t, map[string]ipc.Handler{
		"ping": func(args map[string]any) (any, error) { return nil, nil },
	})
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	writeFile(t, cfgPath, "rgb:\n  enabled: true\n")
	mgr := &fakeServiceManager{installed: true, active: true}

	info, err := gatherDoctor(path, mgr, cfgPath, filepath.Join(t.TempDir(), "spidev0.0"), filepath.Join(t.TempDir(), "i2c-1"), filepath.Join(t.TempDir(), "input-devices"))
	if err != nil {
		t.Fatalf("gatherDoctor: %v", err)
	}
	if info.ConfigStatus != "readable" {
		t.Fatalf("expected config status \"readable\", got %q", info.ConfigStatus)
	}
}

func TestGatherDoctorReportsMalformedConfig(t *testing.T) {
	path := startTestDaemon(t, map[string]ipc.Handler{
		"ping": func(args map[string]any) (any, error) { return nil, nil },
	})
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	writeFile(t, cfgPath, "not: [valid: yaml")
	mgr := &fakeServiceManager{installed: true, active: true}

	info, err := gatherDoctor(path, mgr, cfgPath, filepath.Join(t.TempDir(), "spidev0.0"), filepath.Join(t.TempDir(), "i2c-1"), filepath.Join(t.TempDir(), "input-devices"))
	if err != nil {
		t.Fatalf("gatherDoctor: %v", err)
	}
	if !strings.Contains(info.ConfigStatus, "not readable") {
		t.Fatalf("expected config status to flag the parse error, got %q", info.ConfigStatus)
	}
}

func TestGatherDoctorReportsSocketPermissions(t *testing.T) {
	path := startTestDaemon(t, map[string]ipc.Handler{
		"ping": func(args map[string]any) (any, error) { return nil, nil },
	})
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	mgr := &fakeServiceManager{installed: true, active: true}

	info, err := gatherDoctor(path, mgr, cfgPath, filepath.Join(t.TempDir(), "spidev0.0"), filepath.Join(t.TempDir(), "i2c-1"), filepath.Join(t.TempDir(), "input-devices"))
	if err != nil {
		t.Fatalf("gatherDoctor: %v", err)
	}
	if !strings.Contains(info.SocketPermissions, "660") {
		t.Fatalf("expected socket permissions to report mode 0660, got %q", info.SocketPermissions)
	}
}

func TestGatherDoctorReportsMissingSocketGracefully(t *testing.T) {
	path := filepath.Join(t.TempDir(), "no-such-daemon.sock")
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	mgr := &fakeServiceManager{}

	info, err := gatherDoctor(path, mgr, cfgPath, filepath.Join(t.TempDir(), "spidev0.0"), filepath.Join(t.TempDir(), "i2c-1"), filepath.Join(t.TempDir(), "input-devices"))
	if err != nil {
		t.Fatalf("expected gatherDoctor to degrade gracefully, got error: %v", err)
	}
	if info.SocketPermissions == "" {
		t.Fatalf("expected a non-empty (error-describing) socket permissions string")
	}
}

func TestGatherDoctorReportsSPIEnabled(t *testing.T) {
	path := filepath.Join(t.TempDir(), "no-such-daemon.sock")
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	mgr := &fakeServiceManager{}
	spiPath := filepath.Join(t.TempDir(), "spidev0.0")
	writeFile(t, spiPath, "")

	info, err := gatherDoctor(path, mgr, cfgPath, spiPath, filepath.Join(t.TempDir(), "i2c-1"), filepath.Join(t.TempDir(), "input-devices"))
	if err != nil {
		t.Fatalf("gatherDoctor: %v", err)
	}
	if !info.SPIEnabled {
		t.Fatalf("expected SPIEnabled to be true when %q exists", spiPath)
	}
}

func TestGatherDoctorReportsSPIDisabled(t *testing.T) {
	path := filepath.Join(t.TempDir(), "no-such-daemon.sock")
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	mgr := &fakeServiceManager{}
	spiPath := filepath.Join(t.TempDir(), "spidev0.0")

	info, err := gatherDoctor(path, mgr, cfgPath, spiPath, filepath.Join(t.TempDir(), "i2c-1"), filepath.Join(t.TempDir(), "input-devices"))
	if err != nil {
		t.Fatalf("gatherDoctor: %v", err)
	}
	if info.SPIEnabled {
		t.Fatalf("expected SPIEnabled to be false when %q is absent", spiPath)
	}
}

func TestGatherDoctorReportsI2CEnabled(t *testing.T) {
	path := filepath.Join(t.TempDir(), "no-such-daemon.sock")
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	mgr := &fakeServiceManager{}
	i2cPath := filepath.Join(t.TempDir(), "i2c-1")
	writeFile(t, i2cPath, "")

	info, err := gatherDoctor(path, mgr, cfgPath, filepath.Join(t.TempDir(), "spidev0.0"), i2cPath, filepath.Join(t.TempDir(), "input-devices"))
	if err != nil {
		t.Fatalf("gatherDoctor: %v", err)
	}
	if !info.I2CEnabled {
		t.Fatalf("expected I2CEnabled to be true when %q exists", i2cPath)
	}
}

func TestGatherDoctorReportsI2CDisabled(t *testing.T) {
	path := filepath.Join(t.TempDir(), "no-such-daemon.sock")
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	mgr := &fakeServiceManager{}
	i2cPath := filepath.Join(t.TempDir(), "i2c-1")

	info, err := gatherDoctor(path, mgr, cfgPath, filepath.Join(t.TempDir(), "spidev0.0"), i2cPath, filepath.Join(t.TempDir(), "input-devices"))
	if err != nil {
		t.Fatalf("gatherDoctor: %v", err)
	}
	if info.I2CEnabled {
		t.Fatalf("expected I2CEnabled to be false when %q is absent", i2cPath)
	}
}

func TestGatherDoctorReportsPowerButtonFound(t *testing.T) {
	path := filepath.Join(t.TempDir(), "no-such-daemon.sock")
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	mgr := &fakeServiceManager{}
	inputDevicesPath := filepath.Join(t.TempDir(), "devices")
	writeFile(t, inputDevicesPath, "I: Bus=0019 Vendor=0000 Product=0001 Version=0000\n"+
		"N: Name=\"Power Button\"\n"+
		"H: Handlers=kbd event0\n"+
		"B: EV=3\n"+
		"B: KEY=10000000000000 0\n")

	info, err := gatherDoctor(path, mgr, cfgPath, filepath.Join(t.TempDir(), "spidev0.0"), filepath.Join(t.TempDir(), "i2c-1"), inputDevicesPath)
	if err != nil {
		t.Fatalf("gatherDoctor: %v", err)
	}
	if !info.PowerButtonFound {
		t.Fatalf("expected PowerButtonFound to be true when a KEY_POWER-capable device is listed")
	}
}

func TestGatherDoctorReportsPowerButtonNotFound(t *testing.T) {
	path := filepath.Join(t.TempDir(), "no-such-daemon.sock")
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	mgr := &fakeServiceManager{}
	inputDevicesPath := filepath.Join(t.TempDir(), "devices")
	writeFile(t, inputDevicesPath, "I: Bus=0003 Vendor=046d Product=c52b Version=0111\n"+
		"N: Name=\"Mouse\"\n"+
		"H: Handlers=mouse0 event1\n"+
		"B: EV=17\n"+
		"B: KEY=70000 0 0 0\n")

	info, err := gatherDoctor(path, mgr, cfgPath, filepath.Join(t.TempDir(), "spidev0.0"), filepath.Join(t.TempDir(), "i2c-1"), inputDevicesPath)
	if err != nil {
		t.Fatalf("gatherDoctor: %v", err)
	}
	if info.PowerButtonFound {
		t.Fatalf("expected PowerButtonFound to be false when no listed device advertises KEY_POWER")
	}
}

func TestGatherDoctorReportsPowerButtonMissingFileGracefully(t *testing.T) {
	path := filepath.Join(t.TempDir(), "no-such-daemon.sock")
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	mgr := &fakeServiceManager{}

	info, err := gatherDoctor(path, mgr, cfgPath, filepath.Join(t.TempDir(), "spidev0.0"), filepath.Join(t.TempDir(), "i2c-1"), filepath.Join(t.TempDir(), "no-such-devices-file"))
	if err != nil {
		t.Fatalf("expected gatherDoctor to degrade gracefully when %s is absent, got error: %v", inputDevicesPath, err)
	}
	if info.PowerButtonFound {
		t.Fatalf("expected PowerButtonFound to be false when the input-devices file is absent")
	}
}

func TestGatherDoctorReportsI2CToolsInstalled(t *testing.T) {
	path := filepath.Join(t.TempDir(), "no-such-daemon.sock")
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	mgr := &fakeServiceManager{}
	binDir := t.TempDir()
	writeFile(t, filepath.Join(binDir, "i2cdetect"), "#!/bin/sh\n")
	if err := os.Chmod(filepath.Join(binDir, "i2cdetect"), 0o755); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	t.Setenv("PATH", binDir)

	info, err := gatherDoctor(path, mgr, cfgPath, filepath.Join(t.TempDir(), "spidev0.0"), filepath.Join(t.TempDir(), "i2c-1"), filepath.Join(t.TempDir(), "input-devices"))
	if err != nil {
		t.Fatalf("gatherDoctor: %v", err)
	}
	if !info.I2CToolsInstalled {
		t.Fatalf("expected I2CToolsInstalled to be true when i2cdetect is on PATH")
	}
}

func TestHasKeyPowerCapabilityDetectsPowerButtonBit(t *testing.T) {
	if !hasKeyPowerCapability("10000000000000 0") {
		t.Fatalf("expected KEY_POWER bit to be detected")
	}
}

func TestHasKeyPowerCapabilityIgnoresUnrelatedBits(t *testing.T) {
	if hasKeyPowerCapability("3") {
		t.Fatalf("expected a single low word with no KEY_POWER bit to report false")
	}
}

func TestHasKeyPowerCapabilityHandlesMalformedHex(t *testing.T) {
	if hasKeyPowerCapability("zz zz") {
		t.Fatalf("expected malformed hex to report false, not error out")
	}
}

func TestHasKeyPowerCapabilityHandlesEmptyValue(t *testing.T) {
	if hasKeyPowerCapability("") {
		t.Fatalf("expected an empty value to report false")
	}
}

func TestGatherDoctorReportsI2CToolsNotInstalled(t *testing.T) {
	path := filepath.Join(t.TempDir(), "no-such-daemon.sock")
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	mgr := &fakeServiceManager{}
	t.Setenv("PATH", t.TempDir())

	info, err := gatherDoctor(path, mgr, cfgPath, filepath.Join(t.TempDir(), "spidev0.0"), filepath.Join(t.TempDir(), "i2c-1"), filepath.Join(t.TempDir(), "input-devices"))
	if err != nil {
		t.Fatalf("gatherDoctor: %v", err)
	}
	if info.I2CToolsInstalled {
		t.Fatalf("expected I2CToolsInstalled to be false when i2cdetect is not on PATH")
	}
}
