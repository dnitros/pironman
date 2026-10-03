package oled

import (
	"fmt"
	"image"
	"os"
	"slices"
	"sync"
	"time"

	"github.com/dnitros/pironman/internal/clock"
	"github.com/dnitros/pironman/internal/hardware"
	"github.com/dnitros/pironman/internal/pbm"
	"github.com/dnitros/pironman/internal/sysstats"
)

const (
	PageMix         = "mix"
	PagePerformance = "performance"
	PageIPs         = "ips"
	PageDisk        = "disk"
	PageImage       = "image"
)

var shutdownConfirmationLines = []string{"Hold to", "shut down"}
var poweringOffLines = []string{"Powering off..."}

type State struct {
	Awake bool
	Page  string
}

type terminalState int

const (
	terminalNone terminalState = iota
	terminalShutdownConfirmation
	terminalPoweringOff
)

type Machine struct {
	mu sync.Mutex

	display hardware.SSD1306Display
	stats   sysstats.Source
	clock   clock.Clock

	pages   []string
	pageIdx int

	sleepTimeout   time.Duration
	scrollInterval time.Duration

	awake        bool
	lastActivity time.Time
	lastScroll   time.Time
	scrollIdx    int

	imagePaths     []string
	imageIdx       int
	imageInterval  time.Duration
	imageChangedAt time.Time

	imageCachePath string
	imageCacheMod  time.Time
	imageCacheImg  *image.Gray

	terminal terminalState
}

func NewMachine(display hardware.SSD1306Display, stats sysstats.Source, clk clock.Clock, pages []string, sleepTimeout, scrollInterval time.Duration, initialAwake bool) (*Machine, error) {
	if len(pages) == 0 {
		return nil, fmt.Errorf("oled: page order must not be empty")
	}

	now := clk.Now()
	m := &Machine{
		display:        display,
		stats:          stats,
		clock:          clk,
		pages:          pages,
		sleepTimeout:   sleepTimeout,
		scrollInterval: scrollInterval,
		awake:          initialAwake,
		lastActivity:   now,
		lastScroll:     now,
	}
	if err := m.renderLocked(); err != nil {
		return nil, fmt.Errorf("apply initial OLED state: %w", err)
	}
	return m, nil
}

func NewConfiguredMachine(enabled bool, pageOrder []string, sleepTimeoutSeconds, scrollIntervalSeconds int, imagePaths []string, imageIntervalSeconds int, rotationDegrees int) (*Machine, error) {
	display, err := hardware.NewI2CSSD1306(hardware.I2CPort)
	if err != nil {
		return nil, fmt.Errorf("open SSD1306 display: %w", err)
	}
	if err := display.SetRotation(rotationDegrees); err != nil {
		return nil, fmt.Errorf("apply initial OLED rotation: %w", err)
	}
	stats := sysstats.NewProcSource(sysstats.DefaultStatPath, sysstats.DefaultThermalPath, sysstats.DefaultMemInfoPath, sysstats.DefaultMountsPath)
	m, err := NewMachine(display, stats, clock.RealClock{}, pageOrder,
		time.Duration(sleepTimeoutSeconds)*time.Second,
		time.Duration(scrollIntervalSeconds)*time.Second,
		enabled)
	if err != nil {
		return nil, err
	}
	if err := m.SetImages(imagePaths, time.Duration(imageIntervalSeconds)*time.Second); err != nil {
		return nil, fmt.Errorf("apply initial OLED images: %w", err)
	}
	return m, nil
}

func (m *Machine) State() State {
	m.mu.Lock()
	defer m.mu.Unlock()
	return State{Awake: m.awake, Page: m.pages[m.pageIdx]}
}

func (m *Machine) On() error {
	return m.SetPage(PageMix)
}

func (m *Machine) Off() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.terminal == terminalPoweringOff {
		return nil
	}

	m.terminal = terminalNone
	m.awake = false
	return m.renderLocked()
}

func (m *Machine) ShowShutdownConfirmation() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.terminal = terminalShutdownConfirmation
	return m.renderLocked()
}

func (m *Machine) ShowPoweringOff() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.terminal = terminalPoweringOff
	return m.renderLocked()
}

func (m *Machine) Advance() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.awake {
		m.awake = true
		m.resetScrollLocked()
		m.lastActivity = m.clock.Now()
		return m.renderLocked()
	}

	m.pageIdx = (m.pageIdx + 1) % len(m.pages)
	m.resetScrollLocked()
	m.lastActivity = m.clock.Now()
	return m.renderLocked()
}

func (m *Machine) Previous() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.awake {
		return nil
	}

	m.pageIdx = (m.pageIdx - 1 + len(m.pages)) % len(m.pages)
	m.resetScrollLocked()
	m.lastActivity = m.clock.Now()
	return m.renderLocked()
}

func (m *Machine) SetRotation(degrees int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.display.SetRotation(degrees); err != nil {
		return err
	}
	return m.renderLocked()
}

func (m *Machine) SetImages(paths []string, interval time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.imagePaths = paths
	m.imageInterval = interval
	m.imageIdx = 0
	m.imageChangedAt = m.clock.Now()
	m.imageCachePath = ""
	m.imageCacheImg = nil

	if m.pages[m.pageIdx] != PageImage {
		return nil
	}
	return m.renderLocked()
}

func (m *Machine) SetPage(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	idx := slices.Index(m.pages, name)
	if idx == -1 {
		return fmt.Errorf("oled: unknown page %q", name)
	}

	m.pageIdx = idx
	m.awake = true
	m.resetScrollLocked()
	m.lastActivity = m.clock.Now()
	return m.renderLocked()
}

func (m *Machine) Tick() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.terminal != terminalNone {
		return nil
	}

	if !m.awake {
		return nil
	}

	now := m.clock.Now()
	if now.Sub(m.lastActivity) >= m.sleepTimeout {
		m.awake = false
		return m.renderLocked()
	}

	if now.Sub(m.lastScroll) >= m.scrollInterval {
		m.scrollIdx++
		m.lastScroll = now
	}

	if m.pages[m.pageIdx] == PageImage && len(m.imagePaths) > 1 && now.Sub(m.imageChangedAt) >= m.imageInterval {
		m.imageIdx = (m.imageIdx + 1) % len(m.imagePaths)
		m.imageChangedAt = now
	}

	return m.renderLocked()
}

func (m *Machine) resetScrollLocked() {
	m.scrollIdx = 0
	m.lastScroll = m.clock.Now()
	m.imageIdx = 0
	m.imageChangedAt = m.clock.Now()
}

func (m *Machine) renderLocked() error {
	switch m.terminal {
	case terminalShutdownConfirmation:
		return m.display.Draw(renderLines(shutdownConfirmationLines))
	case terminalPoweringOff:
		return m.display.Draw(renderLines(poweringOffLines))
	}

	if !m.awake {
		return m.display.Draw(image.NewGray(image.Rect(0, 0, hardware.SSD1306Width, hardware.SSD1306Height)))
	}

	if m.pages[m.pageIdx] == PageImage {
		return m.display.Draw(m.currentImageLocked())
	}

	lines, err := m.pageLinesLocked()
	if err != nil {
		return err
	}
	return m.display.Draw(renderLines(lines))
}

func (m *Machine) currentImageLocked() *image.Gray {
	if len(m.imagePaths) == 0 {
		return renderLines([]string{"no image", "configured"})
	}

	path := m.imagePaths[m.imageIdx%len(m.imagePaths)]
	img, err := m.loadImageLocked(path)
	if err != nil {
		return renderLines([]string{"image error", err.Error()})
	}
	return img
}

func (m *Machine) loadImageLocked(path string) (*image.Gray, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if path == m.imageCachePath && info.ModTime().Equal(m.imageCacheMod) {
		return m.imageCacheImg, nil
	}

	img, err := pbm.DecodeFile(path)
	if err != nil {
		return nil, err
	}
	b := img.Bounds()
	if b.Dx() != hardware.SSD1306Width || b.Dy() != hardware.SSD1306Height {
		return nil, fmt.Errorf("%s is %dx%d, must be %dx%d", path, b.Dx(), b.Dy(), hardware.SSD1306Width, hardware.SSD1306Height)
	}

	m.imageCachePath = path
	m.imageCacheMod = info.ModTime()
	m.imageCacheImg = img
	return img, nil
}

func (m *Machine) pageLinesLocked() ([]string, error) {
	page := m.pages[m.pageIdx]

	snap, err := m.stats.Snapshot()
	if err != nil {
		return nil, fmt.Errorf("read stats: %w", err)
	}

	switch page {
	case PageMix:
		return mixLines(snap, m.scrollIdx), nil
	case PagePerformance:
		return performanceLines(snap), nil
	case PageIPs:
		return ipsLines(snap, m.scrollIdx), nil
	case PageDisk:
		return diskLines(snap, m.scrollIdx), nil
	default:
		return nil, fmt.Errorf("oled: unknown page %q", page)
	}
}
