package cli

import (
	"context"
	"image"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dnitros/pironman/internal/fan"
	"github.com/dnitros/pironman/internal/groupaccess"
	"github.com/dnitros/pironman/internal/ipc"
	"github.com/dnitros/pironman/internal/oled"
	"github.com/dnitros/pironman/internal/rgb"
	"github.com/dnitros/pironman/internal/sysstats"
)

type fakeServiceManager struct {
	installed   bool
	active      bool
	unsupported bool

	installErr, uninstallErr, startErr, stopErr, enableErr, disableErr, isInstalledErr, isActiveErr error

	installCalled, uninstallCalled, startCalled, stopCalled, enableCalled, disableCalled bool
	installContent                                                                       string
}

func (f *fakeServiceManager) IsSupported() bool {
	return !f.unsupported
}

func (f *fakeServiceManager) IsInstalled() (bool, error) {
	return f.installed, f.isInstalledErr
}

func (f *fakeServiceManager) IsActive() (bool, error) {
	return f.active, f.isActiveErr
}

func (f *fakeServiceManager) Install(unitContent string) error {
	f.installCalled = true
	f.installContent = unitContent
	if f.installErr != nil {
		return f.installErr
	}
	f.installed = true
	return nil
}

func (f *fakeServiceManager) Uninstall() error {
	f.uninstallCalled = true
	if f.uninstallErr != nil {
		return f.uninstallErr
	}
	f.installed = false
	return nil
}

func (f *fakeServiceManager) Start() error {
	f.startCalled = true
	return f.startErr
}

func (f *fakeServiceManager) Stop() error {
	f.stopCalled = true
	return f.stopErr
}

func (f *fakeServiceManager) Enable() error {
	f.enableCalled = true
	return f.enableErr
}

func (f *fakeServiceManager) Disable() error {
	f.disableCalled = true
	return f.disableErr
}

func (f *fakeServiceManager) called(use string) bool {
	switch use {
	case "install":
		return f.installCalled
	case "uninstall":
		return f.uninstallCalled
	case "start":
		return f.startCalled
	case "stop":
		return f.stopCalled
	case "enable":
		return f.enableCalled
	case "disable":
		return f.disableCalled
	default:
		return false
	}
}

func asRoot(t *testing.T) {
	t.Helper()
	old := geteuid
	geteuid = func() int { return 0 }
	t.Cleanup(func() { geteuid = old })
}

func asNonRoot(t *testing.T) {
	t.Helper()
	old := geteuid
	geteuid = func() int { return 501 }
	t.Cleanup(func() { geteuid = old })
}

func daemonCommandByUse(t *testing.T, use string) daemonCommand {
	t.Helper()
	for _, c := range daemonCommands {
		if c.use == use {
			return c
		}
	}
	t.Fatalf("no daemon command named %q", use)
	return daemonCommand{}
}

func TestDaemonCommandsFailWhenUnsupported(t *testing.T) {
	for _, c := range daemonCommands {
		t.Run(c.use, func(t *testing.T) {
			asRoot(t)
			mgr := &fakeServiceManager{installed: true, unsupported: true}

			err := runDaemonCommand(mgr, c)
			if err == nil {
				t.Fatalf("expected an error on an unsupported platform")
			}
			if !strings.Contains(err.Error(), "systemd") {
				t.Fatalf("expected error to mention systemd, got: %v", err)
			}
			if mgr.called(c.use) {
				t.Fatalf("expected %s to not be called on an unsupported platform", c.use)
			}
		})
	}
}

func TestDaemonCommandsRequireRoot(t *testing.T) {
	for _, c := range daemonCommands {
		t.Run(c.use, func(t *testing.T) {
			asNonRoot(t)
			mgr := &fakeServiceManager{installed: true}

			err := runDaemonCommand(mgr, c)
			if err == nil {
				t.Fatalf("expected an error when not root")
			}
			if mgr.called(c.use) {
				t.Fatalf("expected %s to not be called without root", c.use)
			}
		})
	}
}

func TestDaemonCommandsRequiringInstallFailWhenNotInstalled(t *testing.T) {
	for _, c := range daemonCommands {
		if !c.requireInstalled {
			continue
		}
		t.Run(c.use, func(t *testing.T) {
			asRoot(t)
			mgr := &fakeServiceManager{installed: false}

			err := runDaemonCommand(mgr, c)
			if err == nil {
				t.Fatalf("expected an error when the service isn't installed")
			}
			if !strings.Contains(err.Error(), "daemon install") {
				t.Fatalf("expected error to point at `daemon install`, got: %v", err)
			}
			if mgr.called(c.use) {
				t.Fatalf("expected %s to not be called when not installed", c.use)
			}
		})
	}
}

func TestDaemonCommandsCallTheRightManagerMethod(t *testing.T) {
	for _, c := range daemonCommands {
		t.Run(c.use, func(t *testing.T) {
			asRoot(t)
			mgr := &fakeServiceManager{installed: true}

			if err := runDaemonCommand(mgr, c); err != nil {
				t.Fatalf("runDaemonCommand: %v", err)
			}
			if !mgr.called(c.use) {
				t.Fatalf("expected %s to be called", c.use)
			}
		})
	}
}

func TestDaemonInstallCallsManagerWithUnitContent(t *testing.T) {
	asRoot(t)
	mgr := &fakeServiceManager{}
	install := daemonCommandByUse(t, "install")

	if err := runDaemonCommand(mgr, install); err != nil {
		t.Fatalf("runDaemonCommand: %v", err)
	}
	if !strings.Contains(mgr.installContent, "daemon run") {
		t.Fatalf("expected unit content to reference \"daemon run\", got %q", mgr.installContent)
	}
}

func stubAddUserToGroup(t *testing.T, err error) *string {
	t.Helper()
	var got string
	old := groupaccess.AddMember
	groupaccess.AddMember = func(user string) error {
		got = user
		return err
	}
	t.Cleanup(func() { groupaccess.AddMember = old })
	return &got
}

func stubIsUserInGroup(t *testing.T, member bool, err error) {
	t.Helper()
	old := groupaccess.IsMember
	groupaccess.IsMember = func(string) (bool, error) { return member, err }
	t.Cleanup(func() { groupaccess.IsMember = old })
}

func TestDaemonInstallAddsSudoUserToGroup(t *testing.T) {
	asRoot(t)
	t.Setenv("SUDO_USER", "pi")
	stubIsUserInGroup(t, false, nil)
	calledWith := stubAddUserToGroup(t, nil)
	mgr := &fakeServiceManager{}
	install := daemonCommandByUse(t, "install")

	if err := runDaemonCommand(mgr, install); err != nil {
		t.Fatalf("runDaemonCommand: %v", err)
	}
	if *calledWith != "pi" {
		t.Fatalf("expected addUserToGroup to be called with %q, got %q", "pi", *calledWith)
	}
}

func TestDaemonInstallSkipsGroupAddWithoutSudoUser(t *testing.T) {
	asRoot(t)
	t.Setenv("SUDO_USER", "")
	calledWith := stubAddUserToGroup(t, nil)
	mgr := &fakeServiceManager{}
	install := daemonCommandByUse(t, "install")

	if err := runDaemonCommand(mgr, install); err != nil {
		t.Fatalf("runDaemonCommand: %v", err)
	}
	if *calledWith != "" {
		t.Fatalf("expected addUserToGroup to not be called, got user %q", *calledWith)
	}
}

func TestDaemonInstallSucceedsWhenGroupAddFails(t *testing.T) {
	asRoot(t)
	t.Setenv("SUDO_USER", "pi")
	stubIsUserInGroup(t, false, nil)
	stubAddUserToGroup(t, errBoom)
	mgr := &fakeServiceManager{}
	install := daemonCommandByUse(t, "install")

	if err := runDaemonCommand(mgr, install); err != nil {
		t.Fatalf("expected runDaemonCommand to succeed even when the group-add fails, got: %v", err)
	}
}

func TestDaemonInstallSkipsAddWhenAlreadyMember(t *testing.T) {
	asRoot(t)
	t.Setenv("SUDO_USER", "pi")
	stubIsUserInGroup(t, true, nil)
	calledWith := stubAddUserToGroup(t, nil)
	mgr := &fakeServiceManager{}
	install := daemonCommandByUse(t, "install")

	if err := runDaemonCommand(mgr, install); err != nil {
		t.Fatalf("runDaemonCommand: %v", err)
	}
	if *calledWith != "" {
		t.Fatalf("expected addUserToGroup to not be called when already a member, got user %q", *calledWith)
	}
}

func TestDaemonInstallAttemptsAddWhenMembershipCheckFails(t *testing.T) {
	asRoot(t)
	t.Setenv("SUDO_USER", "pi")
	stubIsUserInGroup(t, false, errBoom)
	calledWith := stubAddUserToGroup(t, nil)
	mgr := &fakeServiceManager{}
	install := daemonCommandByUse(t, "install")

	if err := runDaemonCommand(mgr, install); err != nil {
		t.Fatalf("runDaemonCommand: %v", err)
	}
	if *calledWith != "pi" {
		t.Fatalf("expected addUserToGroup to still be attempted when the membership check errors, got %q", *calledWith)
	}
}

func TestGroupJoinHintUsesSudoUser(t *testing.T) {
	t.Setenv("SUDO_USER", "pi")

	got := groupJoinHint()
	if !strings.Contains(got, "usermod -aG pironman pi") {
		t.Fatalf("expected hint to reference SUDO_USER, got %q", got)
	}
}

func TestGroupJoinHintFallsBackWithoutSudoUser(t *testing.T) {
	t.Setenv("SUDO_USER", "")

	got := groupJoinHint()
	if !strings.Contains(got, "usermod -aG pironman <your-username>") {
		t.Fatalf("expected hint to use a placeholder username, got %q", got)
	}
}

type fakeStrip struct {
	onCalls, offCalls int
}

func (f *fakeStrip) On() error             { f.onCalls++; return nil }
func (f *fakeStrip) Off() error            { f.offCalls++; return nil }
func (f *fakeStrip) SetColor(r, g, b byte) {}

func newTestIPCServer(t *testing.T) *ipc.Server {
	t.Helper()
	sockDir, err := os.MkdirTemp("", "pironman-test")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(sockDir) })

	srv := ipc.NewServer(nil)
	if err := srv.Listen(filepath.Join(sockDir, "s.sock")); err != nil {
		t.Fatalf("srv.Listen: %v", err)
	}
	return srv
}

func canceledContext() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

func TestServeDaemonRunsShutdownHookAfterServe(t *testing.T) {
	strip := &fakeStrip{}
	rgbStore, err := rgb.NewStore(strip, rgb.State{Enabled: true})
	if err != nil {
		t.Fatalf("rgb.NewStore: %v", err)
	}
	strip.onCalls, strip.offCalls = 0, 0

	srv := newTestIPCServer(t)
	hook := func() error {
		_, err := rgbStore.Off()
		return err
	}

	if err := serveDaemon(canceledContext(), srv, hook); err != nil {
		t.Fatalf("serveDaemon: %v", err)
	}
	if strip.offCalls != 1 {
		t.Fatalf("expected Serve to be followed by exactly one Off() call, got %d", strip.offCalls)
	}
	if rgbStore.Enabled() {
		t.Fatalf("expected RGB store to report disabled after shutdown")
	}
}

func TestServeDaemonRunsRemainingHooksWhenOneFails(t *testing.T) {
	srv := newTestIPCServer(t)
	var secondRan bool
	failing := func() error { return errBoom }
	second := func() error { secondRan = true; return nil }

	if err := serveDaemon(canceledContext(), srv, failing, second); err != nil {
		t.Fatalf("serveDaemon: %v", err)
	}
	if !secondRan {
		t.Fatalf("expected the second shutdown hook to run even though the first failed")
	}
}

type fakeOLEDDisplay struct{}

func (fakeOLEDDisplay) Draw(img *image.Gray) error { return nil }

type fakeOLEDStatsSource struct{}

func (fakeOLEDStatsSource) Snapshot() (sysstats.Snapshot, error) { return sysstats.Snapshot{}, nil }

type fixedOLEDClock struct{ t time.Time }

func (f fixedOLEDClock) Now() time.Time { return f.t }

func TestShutdownDoesNotDeadlockWhenServeReturnsWithCtxStillLive(t *testing.T) {
	srv := newTestIPCServer(t)

	machine, err := oled.NewMachine(fakeOLEDDisplay{}, fakeOLEDStatsSource{}, fixedOLEDClock{t: time.Now()},
		[]string{oled.PageMix}, 10*time.Second, 3*time.Second, true)
	if err != nil {
		t.Fatalf("oled.NewMachine: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stopTicker := startOLEDTickLoop(ctx, machine)

	go func() {
		time.Sleep(20 * time.Millisecond)
		srv.Close()
	}()

	done := make(chan error, 1)
	go func() {
		done <- serveDaemon(ctx, srv, stopTicker)
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatalf("expected serveDaemon to propagate the Accept error triggered by closing the listener")
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("serveDaemon deadlocked: the shutdown hook never returned")
	}
}

type fakeFanRelay struct{}

func (fakeFanRelay) Set(on bool) error { return nil }

type fakeFanStatsSource struct{}

func (fakeFanStatsSource) Snapshot() (sysstats.Snapshot, error) { return sysstats.Snapshot{}, nil }

func TestFanShutdownDoesNotDeadlockWhenServeReturnsWithCtxStillLive(t *testing.T) {
	srv := newTestIPCServer(t)

	machine, err := fan.NewMachine(fakeFanRelay{}, fakeFanStatsSource{}, fan.ModeAuto)
	if err != nil {
		t.Fatalf("fan.NewMachine: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stopTicker := startFanTickLoop(ctx, machine)

	go func() {
		time.Sleep(20 * time.Millisecond)
		srv.Close()
	}()

	done := make(chan error, 1)
	go func() {
		done <- serveDaemon(ctx, srv, stopTicker)
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatalf("expected serveDaemon to propagate the Accept error triggered by closing the listener")
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("serveDaemon deadlocked: the shutdown hook never returned")
	}
}
