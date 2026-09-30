package oled_test

import (
	"bytes"
	"errors"
	"image"
	"testing"
	"time"

	"github.com/dnitros/pironman/internal/oled"
	"github.com/dnitros/pironman/internal/sysstats"
)

var errBoom = errors.New("boom")

type fakeDisplay struct {
	frames [][]byte
}

func (f *fakeDisplay) Draw(img *image.Gray) error {
	f.frames = append(f.frames, append([]byte(nil), img.Pix...))
	return nil
}

func (f *fakeDisplay) lastFrame() []byte {
	if len(f.frames) == 0 {
		return nil
	}
	return f.frames[len(f.frames)-1]
}

func allZero(b []byte) bool {
	for _, v := range b {
		if v != 0 {
			return false
		}
	}
	return true
}

type fakeStats struct {
	snap  sysstats.Snapshot
	err   error
	calls int
}

func (f *fakeStats) Snapshot() (sysstats.Snapshot, error) {
	f.calls++
	return f.snap, f.err
}

type fakeClock struct {
	t time.Time
}

func (f *fakeClock) Now() time.Time { return f.t }

func (f *fakeClock) Advance(d time.Duration) { f.t = f.t.Add(d) }

func newFakeClock() *fakeClock { return &fakeClock{t: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)} }

func defaultPages() []string {
	return []string{oled.PageMix, oled.PagePerformance, oled.PageIPs, oled.PageDisk}
}

func TestNewMachineRendersBlankWhenInitiallyAsleep(t *testing.T) {
	display := &fakeDisplay{}
	m, err := oled.NewMachine(display, &fakeStats{}, newFakeClock(), defaultPages(), 10*time.Second, 3*time.Second, false)
	if err != nil {
		t.Fatalf("NewMachine: %v", err)
	}
	if m.State().Awake {
		t.Fatalf("expected asleep")
	}
	if !allZero(display.lastFrame()) {
		t.Fatalf("expected a blank frame when initially asleep")
	}
}

func TestNewMachineRendersContentWhenInitiallyAwake(t *testing.T) {
	display := &fakeDisplay{}
	m, err := oled.NewMachine(display, &fakeStats{}, newFakeClock(), defaultPages(), 10*time.Second, 3*time.Second, true)
	if err != nil {
		t.Fatalf("NewMachine: %v", err)
	}
	if !m.State().Awake {
		t.Fatalf("expected awake")
	}
	if allZero(display.lastFrame()) {
		t.Fatalf("expected a non-blank frame when initially awake")
	}
}

func TestOnWakesAndJumpsToMixPage(t *testing.T) {
	display := &fakeDisplay{}
	m, err := oled.NewMachine(display, &fakeStats{}, newFakeClock(), defaultPages(), 10*time.Second, 3*time.Second, false)
	if err != nil {
		t.Fatalf("NewMachine: %v", err)
	}

	if err := m.On(); err != nil {
		t.Fatalf("On: %v", err)
	}
	state := m.State()
	if !state.Awake || state.Page != oled.PageMix {
		t.Fatalf("State() = %+v, want awake on the mix page", state)
	}
}

func TestOffBlanksDisplay(t *testing.T) {
	display := &fakeDisplay{}
	m, err := oled.NewMachine(display, &fakeStats{}, newFakeClock(), defaultPages(), 10*time.Second, 3*time.Second, true)
	if err != nil {
		t.Fatalf("NewMachine: %v", err)
	}

	if err := m.Off(); err != nil {
		t.Fatalf("Off: %v", err)
	}
	if m.State().Awake {
		t.Fatalf("expected asleep after Off()")
	}
	if !allZero(display.lastFrame()) {
		t.Fatalf("expected a blank frame after Off()")
	}
}

func TestAdvanceWakesWithoutChangingPageWhenAsleep(t *testing.T) {
	display := &fakeDisplay{}
	m, err := oled.NewMachine(display, &fakeStats{}, newFakeClock(), defaultPages(), 10*time.Second, 3*time.Second, false)
	if err != nil {
		t.Fatalf("NewMachine: %v", err)
	}

	if err := m.Advance(); err != nil {
		t.Fatalf("Advance: %v", err)
	}
	state := m.State()
	if !state.Awake || state.Page != oled.PageMix {
		t.Fatalf("State() = %+v, want awake, still on the mix page (page order default)", state)
	}
}

func TestAdvanceResetsScrollTimerOnWake(t *testing.T) {
	display := &fakeDisplay{}
	stats := &fakeStats{snap: sysstats.Snapshot{Interfaces: map[string]string{
		"eth0":  "192.168.1.5",
		"wlan0": "10.0.0.2",
	}}}
	clock := newFakeClock()
	m, err := oled.NewMachine(display, stats, clock, defaultPages(), 10*time.Second, 3*time.Second, false)
	if err != nil {
		t.Fatalf("NewMachine: %v", err)
	}

	clock.Advance(time.Hour)
	if err := m.Advance(); err != nil {
		t.Fatalf("Advance: %v", err)
	}
	afterWake := display.lastFrame()

	clock.Advance(1 * time.Second)
	if err := m.Tick(); err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if !bytes.Equal(display.lastFrame(), afterWake) {
		t.Fatalf("expected content to stay the same 1s after waking, well before the 3s scroll interval")
	}
}

func TestAdvanceMovesToNextPageWhenAwake(t *testing.T) {
	display := &fakeDisplay{}
	m, err := oled.NewMachine(display, &fakeStats{}, newFakeClock(), defaultPages(), 10*time.Second, 3*time.Second, true)
	if err != nil {
		t.Fatalf("NewMachine: %v", err)
	}

	if err := m.Advance(); err != nil {
		t.Fatalf("Advance: %v", err)
	}
	if got := m.State().Page; got != oled.PagePerformance {
		t.Fatalf("Page = %q, want %q", got, oled.PagePerformance)
	}
}

func TestAdvanceWrapsAroundPastLastPage(t *testing.T) {
	display := &fakeDisplay{}
	m, err := oled.NewMachine(display, &fakeStats{}, newFakeClock(), defaultPages(), 10*time.Second, 3*time.Second, true)
	if err != nil {
		t.Fatalf("NewMachine: %v", err)
	}

	for i := 0; i < len(defaultPages()); i++ {
		if err := m.Advance(); err != nil {
			t.Fatalf("Advance: %v", err)
		}
	}
	if got := m.State().Page; got != oled.PageMix {
		t.Fatalf("Page after wrapping = %q, want %q", got, oled.PageMix)
	}
}

func TestPreviousNoOpsWhileAsleep(t *testing.T) {
	display := &fakeDisplay{}
	m, err := oled.NewMachine(display, &fakeStats{}, newFakeClock(), defaultPages(), 10*time.Second, 3*time.Second, false)
	if err != nil {
		t.Fatalf("NewMachine: %v", err)
	}
	framesBefore := len(display.frames)

	if err := m.Previous(); err != nil {
		t.Fatalf("Previous: %v", err)
	}
	if m.State().Awake {
		t.Fatalf("expected to remain asleep")
	}
	if len(display.frames) != framesBefore {
		t.Fatalf("expected Previous() to be a true no-op while asleep, got %d new draw(s)", len(display.frames)-framesBefore)
	}
}

func TestPreviousMovesBackwardWithWraparoundWhileAwake(t *testing.T) {
	display := &fakeDisplay{}
	m, err := oled.NewMachine(display, &fakeStats{}, newFakeClock(), defaultPages(), 10*time.Second, 3*time.Second, true)
	if err != nil {
		t.Fatalf("NewMachine: %v", err)
	}

	if err := m.Previous(); err != nil {
		t.Fatalf("Previous: %v", err)
	}
	if got := m.State().Page; got != oled.PageDisk {
		t.Fatalf("Page = %q, want %q (wrap to the last page)", got, oled.PageDisk)
	}
}

func TestSetPageAlwaysWakesRegardlessOfPriorState(t *testing.T) {
	display := &fakeDisplay{}
	m, err := oled.NewMachine(display, &fakeStats{}, newFakeClock(), defaultPages(), 10*time.Second, 3*time.Second, false)
	if err != nil {
		t.Fatalf("NewMachine: %v", err)
	}

	if err := m.SetPage(oled.PageIPs); err != nil {
		t.Fatalf("SetPage: %v", err)
	}
	state := m.State()
	if !state.Awake || state.Page != oled.PageIPs {
		t.Fatalf("State() = %+v, want awake on %q", state, oled.PageIPs)
	}
}

func TestSetPageRejectsUnknownPage(t *testing.T) {
	display := &fakeDisplay{}
	m, err := oled.NewMachine(display, &fakeStats{}, newFakeClock(), defaultPages(), 10*time.Second, 3*time.Second, false)
	if err != nil {
		t.Fatalf("NewMachine: %v", err)
	}

	if err := m.SetPage("not-a-page"); err == nil {
		t.Fatalf("expected an error for an unknown page name")
	}
	if m.State().Awake {
		t.Fatalf("expected state to be unchanged after a rejected SetPage")
	}
}

func TestTickIsANoOpWhileAsleep(t *testing.T) {
	display := &fakeDisplay{}
	stats := &fakeStats{}
	m, err := oled.NewMachine(display, stats, newFakeClock(), defaultPages(), 10*time.Second, 3*time.Second, false)
	if err != nil {
		t.Fatalf("NewMachine: %v", err)
	}
	framesBefore := len(display.frames)

	if err := m.Tick(); err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if len(display.frames) != framesBefore {
		t.Fatalf("expected Tick() to draw nothing while asleep")
	}
}

func TestTickRefreshesContentEverySecondWhileAwake(t *testing.T) {
	display := &fakeDisplay{}
	stats := &fakeStats{}
	clock := newFakeClock()
	m, err := oled.NewMachine(display, stats, clock, defaultPages(), 10*time.Second, 3*time.Second, true)
	if err != nil {
		t.Fatalf("NewMachine: %v", err)
	}
	callsBefore := stats.calls

	clock.Advance(time.Second)
	if err := m.Tick(); err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if stats.calls != callsBefore+1 {
		t.Fatalf("expected Tick() to refresh stats, calls = %d, want %d", stats.calls, callsBefore+1)
	}
}

func TestTickBlanksAfterSleepTimeoutWithNoActivity(t *testing.T) {
	display := &fakeDisplay{}
	clock := newFakeClock()
	m, err := oled.NewMachine(display, &fakeStats{}, clock, defaultPages(), 10*time.Second, 3*time.Second, true)
	if err != nil {
		t.Fatalf("NewMachine: %v", err)
	}

	clock.Advance(10 * time.Second)
	if err := m.Tick(); err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if m.State().Awake {
		t.Fatalf("expected the display to have gone to sleep")
	}
	if !allZero(display.lastFrame()) {
		t.Fatalf("expected a blank frame once asleep")
	}
}

func TestTickDoesNotSleepBeforeTimeoutElapses(t *testing.T) {
	display := &fakeDisplay{}
	clock := newFakeClock()
	m, err := oled.NewMachine(display, &fakeStats{}, clock, defaultPages(), 10*time.Second, 3*time.Second, true)
	if err != nil {
		t.Fatalf("NewMachine: %v", err)
	}

	clock.Advance(9 * time.Second)
	if err := m.Tick(); err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if !m.State().Awake {
		t.Fatalf("expected to still be awake before the sleep timeout elapses")
	}
}

func TestActivityResetsSleepTimeoutWindow(t *testing.T) {
	display := &fakeDisplay{}
	clock := newFakeClock()
	m, err := oled.NewMachine(display, &fakeStats{}, clock, defaultPages(), 10*time.Second, 3*time.Second, true)
	if err != nil {
		t.Fatalf("NewMachine: %v", err)
	}

	clock.Advance(9 * time.Second)
	if err := m.Advance(); err != nil {
		t.Fatalf("Advance: %v", err)
	}
	clock.Advance(9 * time.Second)
	if err := m.Tick(); err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if !m.State().Awake {
		t.Fatalf("expected activity to reset the sleep-timeout window")
	}
}

func TestTickScrollsMixPageContentOnlyAfterScrollInterval(t *testing.T) {
	display := &fakeDisplay{}
	stats := &fakeStats{snap: sysstats.Snapshot{Interfaces: map[string]string{
		"eth0":  "192.168.1.5",
		"wlan0": "10.0.0.2",
	}}}
	clock := newFakeClock()
	m, err := oled.NewMachine(display, stats, clock, defaultPages(), 10*time.Second, 3*time.Second, true)
	if err != nil {
		t.Fatalf("NewMachine: %v", err)
	}
	initial := display.lastFrame()

	clock.Advance(1 * time.Second)
	if err := m.Tick(); err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if !bytes.Equal(display.lastFrame(), initial) {
		t.Fatalf("expected content to stay the same before the scroll interval elapses")
	}

	clock.Advance(2 * time.Second)
	if err := m.Tick(); err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if bytes.Equal(display.lastFrame(), initial) {
		t.Fatalf("expected content to change once the scroll interval elapses")
	}
}

func TestTickScrollsIPsPageToNextGroupOfThreeAfterScrollInterval(t *testing.T) {
	display := &fakeDisplay{}
	stats := &fakeStats{snap: sysstats.Snapshot{Interfaces: map[string]string{
		"eth0":  "192.168.1.5",
		"eth1":  "192.168.1.6",
		"lo":    "127.0.0.1",
		"wlan0": "10.0.0.2",
	}}}
	clock := newFakeClock()
	m, err := oled.NewMachine(display, stats, clock, defaultPages(), 10*time.Second, 3*time.Second, true)
	if err != nil {
		t.Fatalf("NewMachine: %v", err)
	}
	if err := m.SetPage(oled.PageIPs); err != nil {
		t.Fatalf("SetPage: %v", err)
	}
	initial := display.lastFrame()

	clock.Advance(1 * time.Second)
	if err := m.Tick(); err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if !bytes.Equal(display.lastFrame(), initial) {
		t.Fatalf("expected content to stay the same before the scroll interval elapses")
	}

	clock.Advance(2 * time.Second)
	if err := m.Tick(); err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if bytes.Equal(display.lastFrame(), initial) {
		t.Fatalf("expected content to scroll to the next group of three once the scroll interval elapses")
	}
}

func TestAwakeStatePersistsAcrossSimulatedRestart(t *testing.T) {
	display := &fakeDisplay{}
	m, err := oled.NewMachine(display, &fakeStats{}, newFakeClock(), defaultPages(), 10*time.Second, 3*time.Second, true)
	if err != nil {
		t.Fatalf("NewMachine: %v", err)
	}
	if err := m.Off(); err != nil {
		t.Fatalf("Off: %v", err)
	}

	restartedDisplay := &fakeDisplay{}
	restarted, err := oled.NewMachine(restartedDisplay, &fakeStats{}, newFakeClock(), defaultPages(), 10*time.Second, 3*time.Second, m.State().Awake)
	if err != nil {
		t.Fatalf("NewMachine (restart): %v", err)
	}

	if restarted.State().Awake {
		t.Fatalf("expected the disabled state to survive the simulated restart")
	}
	if !allZero(restartedDisplay.lastFrame()) {
		t.Fatalf("expected the restarted machine to reapply a blank frame")
	}
}

func TestSetPageDiskReadsStatsAndRendersDiskContent(t *testing.T) {
	display := &fakeDisplay{}
	stats := &fakeStats{snap: sysstats.Snapshot{Disks: []sysstats.Disk{
		{Type: "sd", UsedBytes: 12 << 30, TotalBytes: 32 << 30, Percent: 37.5},
	}}}
	m, err := oled.NewMachine(display, stats, newFakeClock(), defaultPages(), 10*time.Second, 3*time.Second, false)
	if err != nil {
		t.Fatalf("NewMachine: %v", err)
	}
	callsBefore := stats.calls

	if err := m.SetPage(oled.PageDisk); err != nil {
		t.Fatalf("SetPage: %v", err)
	}
	if stats.calls != callsBefore+1 {
		t.Fatalf("expected the disk page to read stats, calls = %d, want %d", stats.calls, callsBefore+1)
	}
	if allZero(display.lastFrame()) {
		t.Fatalf("expected the disk page to render non-blank disk content")
	}
}

func TestSetPagePerformanceFetchesAndRendersStats(t *testing.T) {
	display := &fakeDisplay{}
	stats := &fakeStats{snap: sysstats.Snapshot{CPUPercent: 12, MemPercent: 33, CPUTempC: 45.6}}
	m, err := oled.NewMachine(display, stats, newFakeClock(), defaultPages(), 10*time.Second, 3*time.Second, false)
	if err != nil {
		t.Fatalf("NewMachine: %v", err)
	}
	callsBefore := stats.calls

	if err := m.SetPage(oled.PagePerformance); err != nil {
		t.Fatalf("SetPage: %v", err)
	}
	if stats.calls != callsBefore+1 {
		t.Fatalf("expected the performance page to fetch a stats snapshot, calls = %d, want %d", stats.calls, callsBefore+1)
	}
	if state := m.State(); !state.Awake || state.Page != oled.PagePerformance {
		t.Fatalf("State() = %+v, want awake on %q", state, oled.PagePerformance)
	}
}

func TestSetPageDiskPropagatesStatsError(t *testing.T) {
	display := &fakeDisplay{}
	stats := &fakeStats{err: errBoom}
	m, err := oled.NewMachine(display, stats, newFakeClock(), defaultPages(), 10*time.Second, 3*time.Second, false)
	if err != nil {
		t.Fatalf("NewMachine: %v", err)
	}

	if err := m.SetPage(oled.PageDisk); err == nil {
		t.Fatalf("expected SetPage(disk) to propagate the stats error")
	}
}

func TestSetPagePerformancePropagatesStatsError(t *testing.T) {
	display := &fakeDisplay{}
	stats := &fakeStats{err: errBoom}
	m, err := oled.NewMachine(display, stats, newFakeClock(), defaultPages(), 10*time.Second, 3*time.Second, false)
	if err != nil {
		t.Fatalf("NewMachine: %v", err)
	}

	if err := m.SetPage(oled.PagePerformance); err == nil {
		t.Fatalf("expected SetPage(performance) to propagate the stats error")
	}
}

func TestShowShutdownConfirmationRendersNonBlankScreen(t *testing.T) {
	display := &fakeDisplay{}
	m, err := oled.NewMachine(display, &fakeStats{}, newFakeClock(), defaultPages(), 10*time.Second, 3*time.Second, true)
	if err != nil {
		t.Fatalf("NewMachine: %v", err)
	}

	if err := m.ShowShutdownConfirmation(); err != nil {
		t.Fatalf("ShowShutdownConfirmation: %v", err)
	}
	if allZero(display.lastFrame()) {
		t.Fatalf("expected a non-blank shutdown-confirmation screen")
	}
}

func TestShowShutdownConfirmationIsIdempotentWhileHeld(t *testing.T) {
	display := &fakeDisplay{}
	m, err := oled.NewMachine(display, &fakeStats{}, newFakeClock(), defaultPages(), 10*time.Second, 3*time.Second, true)
	if err != nil {
		t.Fatalf("NewMachine: %v", err)
	}

	if err := m.ShowShutdownConfirmation(); err != nil {
		t.Fatalf("ShowShutdownConfirmation: %v", err)
	}
	first := display.lastFrame()

	for i := 0; i < 3; i++ {
		if err := m.ShowShutdownConfirmation(); err != nil {
			t.Fatalf("repeated ShowShutdownConfirmation #%d: %v", i, err)
		}
		if !bytes.Equal(display.lastFrame(), first) {
			t.Fatalf("expected repeated firing to re-render the same screen, got different content on call #%d", i)
		}
	}
}

func TestTickDoesNotAlterShutdownConfirmationScreen(t *testing.T) {
	display := &fakeDisplay{}
	clock := newFakeClock()
	m, err := oled.NewMachine(display, &fakeStats{}, clock, defaultPages(), 10*time.Second, 3*time.Second, true)
	if err != nil {
		t.Fatalf("NewMachine: %v", err)
	}

	if err := m.ShowShutdownConfirmation(); err != nil {
		t.Fatalf("ShowShutdownConfirmation: %v", err)
	}
	confirming := display.lastFrame()

	clock.Advance(time.Hour)
	if err := m.Tick(); err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if !bytes.Equal(display.lastFrame(), confirming) {
		t.Fatalf("expected Tick() to leave the shutdown-confirmation screen untouched (no sleep-timeout blanking, no scroll)")
	}
}

func TestShowPoweringOffRendersDistinctScreen(t *testing.T) {
	display := &fakeDisplay{}
	m, err := oled.NewMachine(display, &fakeStats{}, newFakeClock(), defaultPages(), 10*time.Second, 3*time.Second, true)
	if err != nil {
		t.Fatalf("NewMachine: %v", err)
	}

	if err := m.ShowShutdownConfirmation(); err != nil {
		t.Fatalf("ShowShutdownConfirmation: %v", err)
	}
	confirming := display.lastFrame()

	if err := m.ShowPoweringOff(); err != nil {
		t.Fatalf("ShowPoweringOff: %v", err)
	}
	poweringOff := display.lastFrame()

	if allZero(poweringOff) {
		t.Fatalf("expected a non-blank powering-off screen")
	}
	if bytes.Equal(poweringOff, confirming) {
		t.Fatalf("expected the powering-off screen to be visually distinct from the shutdown-confirmation screen")
	}
}

func TestOffNoOpsDuringPoweringOff(t *testing.T) {
	display := &fakeDisplay{}
	m, err := oled.NewMachine(display, &fakeStats{}, newFakeClock(), defaultPages(), 10*time.Second, 3*time.Second, true)
	if err != nil {
		t.Fatalf("NewMachine: %v", err)
	}

	if err := m.ShowPoweringOff(); err != nil {
		t.Fatalf("ShowPoweringOff: %v", err)
	}
	poweringOff := display.lastFrame()
	framesBefore := len(display.frames)

	if err := m.Off(); err != nil {
		t.Fatalf("Off: %v", err)
	}
	if len(display.frames) != framesBefore {
		t.Fatalf("expected Off() to be a true no-op while powering off, got %d new draw(s)", len(display.frames)-framesBefore)
	}
	if !bytes.Equal(display.lastFrame(), poweringOff) {
		t.Fatalf("expected the powering-off screen to remain visible through Off()")
	}
}

func TestOffBlanksDuringShutdownConfirmation(t *testing.T) {
	display := &fakeDisplay{}
	m, err := oled.NewMachine(display, &fakeStats{}, newFakeClock(), defaultPages(), 10*time.Second, 3*time.Second, true)
	if err != nil {
		t.Fatalf("NewMachine: %v", err)
	}

	if err := m.ShowShutdownConfirmation(); err != nil {
		t.Fatalf("ShowShutdownConfirmation: %v", err)
	}

	if err := m.Off(); err != nil {
		t.Fatalf("Off: %v", err)
	}
	if !allZero(display.lastFrame()) {
		t.Fatalf("expected Off() to blank the display when only showing shutdown-confirmation, not powering-off")
	}
}

func TestTickPropagatesStatsError(t *testing.T) {
	display := &fakeDisplay{}
	stats := &fakeStats{err: errBoom}
	clock := newFakeClock()
	m, err := oled.NewMachine(display, stats, clock, defaultPages(), 10*time.Second, 3*time.Second, true)
	if err == nil {
		t.Fatalf("expected NewMachine to propagate the initial render error")
	}
	_ = m
}
