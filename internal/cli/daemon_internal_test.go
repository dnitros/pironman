package cli

import (
	"strings"
	"testing"
)

type fakeServiceManager struct {
	installed bool

	installErr, uninstallErr, startErr, stopErr, isInstalledErr error

	installCalled, uninstallCalled, startCalled, stopCalled bool
	installContent                                          string
}

func (f *fakeServiceManager) IsInstalled() (bool, error) {
	return f.installed, f.isInstalledErr
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

func TestDaemonInstallRequiresRoot(t *testing.T) {
	asNonRoot(t)
	mgr := &fakeServiceManager{}

	err := runDaemonInstall(mgr)
	if err == nil {
		t.Fatalf("expected an error when not root")
	}
	if mgr.installCalled {
		t.Fatalf("expected Install to not be called without root")
	}
}

func TestDaemonInstallCallsManagerWhenRoot(t *testing.T) {
	asRoot(t)
	mgr := &fakeServiceManager{}

	if err := runDaemonInstall(mgr); err != nil {
		t.Fatalf("runDaemonInstall: %v", err)
	}
	if !mgr.installCalled {
		t.Fatalf("expected Install to be called")
	}
	if !strings.Contains(mgr.installContent, "daemon run") {
		t.Fatalf("expected unit content to reference \"daemon run\", got %q", mgr.installContent)
	}
}

func TestDaemonUninstallRequiresRoot(t *testing.T) {
	asNonRoot(t)
	mgr := &fakeServiceManager{installed: true}

	err := runDaemonUninstall(mgr)
	if err == nil {
		t.Fatalf("expected an error when not root")
	}
	if mgr.uninstallCalled {
		t.Fatalf("expected Uninstall to not be called without root")
	}
}

func TestDaemonUninstallCallsManagerWhenRoot(t *testing.T) {
	asRoot(t)
	mgr := &fakeServiceManager{installed: true}

	if err := runDaemonUninstall(mgr); err != nil {
		t.Fatalf("runDaemonUninstall: %v", err)
	}
	if !mgr.uninstallCalled {
		t.Fatalf("expected Uninstall to be called")
	}
}

func TestDaemonStartFailsWhenNotInstalled(t *testing.T) {
	mgr := &fakeServiceManager{installed: false}

	err := runDaemonStart(mgr)
	if err == nil {
		t.Fatalf("expected an error when the service isn't installed")
	}
	if !strings.Contains(err.Error(), "daemon install") {
		t.Fatalf("expected error to point at `daemon install`, got: %v", err)
	}
	if mgr.startCalled {
		t.Fatalf("expected Start to not be called when not installed")
	}
}

func TestDaemonStartSucceedsWhenInstalled(t *testing.T) {
	mgr := &fakeServiceManager{installed: true}

	if err := runDaemonStart(mgr); err != nil {
		t.Fatalf("runDaemonStart: %v", err)
	}
	if !mgr.startCalled {
		t.Fatalf("expected Start to be called")
	}
}

func TestDaemonStopFailsWhenNotInstalled(t *testing.T) {
	mgr := &fakeServiceManager{installed: false}

	err := runDaemonStop(mgr)
	if err == nil {
		t.Fatalf("expected an error when the service isn't installed")
	}
	if !strings.Contains(err.Error(), "daemon install") {
		t.Fatalf("expected error to point at `daemon install`, got: %v", err)
	}
	if mgr.stopCalled {
		t.Fatalf("expected Stop to not be called when not installed")
	}
}

func TestDaemonStopSucceedsWhenInstalled(t *testing.T) {
	mgr := &fakeServiceManager{installed: true}

	if err := runDaemonStop(mgr); err != nil {
		t.Fatalf("runDaemonStop: %v", err)
	}
	if !mgr.stopCalled {
		t.Fatalf("expected Stop to be called")
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
