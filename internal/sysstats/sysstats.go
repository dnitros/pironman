// Package sysstats reads CPU, memory, and network stats for the OLED mix page.
package sysstats

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
)

// Default paths for a Raspberry Pi's real /proc and /sys.
const (
	DefaultStatPath    = "/proc/stat"
	DefaultThermalPath = "/sys/class/thermal/thermal_zone0/temp"
	DefaultMemInfoPath = "/proc/meminfo"
)

type Snapshot struct {
	CPUPercent    float64
	CPUTempC      float64
	MemUsedBytes  uint64
	MemTotalBytes uint64
	MemPercent    float64
	Interfaces    map[string]string // interface name -> IPv4 address
}

type Source interface {
	Snapshot() (Snapshot, error)
}

// ProcSource reads stats from /proc, /sys, and Go's net package.
type ProcSource struct {
	statPath, thermalPath, meminfoPath string

	mu                  sync.Mutex
	prevIdle, prevTotal uint64
	havePrev            bool
}

func NewProcSource(statPath, thermalPath, meminfoPath string) *ProcSource {
	return &ProcSource{statPath: statPath, thermalPath: thermalPath, meminfoPath: meminfoPath}
}

func (s *ProcSource) Snapshot() (Snapshot, error) {
	statData, err := os.ReadFile(s.statPath)
	if err != nil {
		return Snapshot{}, fmt.Errorf("read %s: %w", s.statPath, err)
	}
	idle, total, err := parseCPUStat(statData)
	if err != nil {
		return Snapshot{}, fmt.Errorf("parse %s: %w", s.statPath, err)
	}

	thermalData, err := os.ReadFile(s.thermalPath)
	if err != nil {
		return Snapshot{}, fmt.Errorf("read %s: %w", s.thermalPath, err)
	}
	tempC, err := parseThermalTempC(thermalData)
	if err != nil {
		return Snapshot{}, fmt.Errorf("parse %s: %w", s.thermalPath, err)
	}

	meminfoData, err := os.ReadFile(s.meminfoPath)
	if err != nil {
		return Snapshot{}, fmt.Errorf("read %s: %w", s.meminfoPath, err)
	}
	usedBytes, totalBytes, err := parseMemInfo(meminfoData)
	if err != nil {
		return Snapshot{}, fmt.Errorf("parse %s: %w", s.meminfoPath, err)
	}

	ifaces, err := listInterfaces()
	if err != nil {
		return Snapshot{}, fmt.Errorf("list network interfaces: %w", err)
	}
	interfaces := make(map[string]string, len(ifaces))
	for _, ifc := range ifaces {
		interfaces[ifc.Name] = ifc.IP
	}

	var memPercent float64
	if totalBytes > 0 {
		memPercent = 100 * float64(usedBytes) / float64(totalBytes)
	}

	s.mu.Lock()
	var cpuPercent float64
	if s.havePrev {
		deltaTotal := total - s.prevTotal
		deltaIdle := idle - s.prevIdle
		if deltaTotal > 0 {
			cpuPercent = 100 * float64(deltaTotal-deltaIdle) / float64(deltaTotal)
		}
	}
	s.prevIdle, s.prevTotal, s.havePrev = idle, total, true
	s.mu.Unlock()

	return Snapshot{
		CPUPercent:    cpuPercent,
		CPUTempC:      tempC,
		MemUsedBytes:  usedBytes,
		MemTotalBytes: totalBytes,
		MemPercent:    memPercent,
		Interfaces:    interfaces,
	}, nil
}

// parseCPUStat parses /proc/stat's aggregate "cpu " line into an idle and a
// total jiffy count. idle folds in iowait, matching top/htop's convention.
func parseCPUStat(data []byte) (idle, total uint64, err error) {
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 5 || fields[0] != "cpu" {
			continue
		}
		values := make([]uint64, 0, len(fields)-1)
		for _, f := range fields[1:] {
			v, err := strconv.ParseUint(f, 10, 64)
			if err != nil {
				return 0, 0, fmt.Errorf("parse cpu field %q: %w", f, err)
			}
			values = append(values, v)
			total += v
		}
		idle = values[3] // idle
		if len(values) > 4 {
			idle += values[4] // iowait
		}
		return idle, total, nil
	}
	return 0, 0, fmt.Errorf("no aggregate \"cpu \" line found")
}

// parseMemInfo parses /proc/meminfo's MemTotal/MemAvailable lines (kB) into byte counts.
func parseMemInfo(data []byte) (usedBytes, totalBytes uint64, err error) {
	var total, available uint64
	var haveTotal, haveAvailable bool

	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		switch fields[0] {
		case "MemTotal:":
			v, err := strconv.ParseUint(fields[1], 10, 64)
			if err != nil {
				return 0, 0, fmt.Errorf("parse MemTotal %q: %w", fields[1], err)
			}
			total, haveTotal = v, true
		case "MemAvailable:":
			v, err := strconv.ParseUint(fields[1], 10, 64)
			if err != nil {
				return 0, 0, fmt.Errorf("parse MemAvailable %q: %w", fields[1], err)
			}
			available, haveAvailable = v, true
		}
	}
	if !haveTotal || !haveAvailable {
		return 0, 0, fmt.Errorf("missing MemTotal or MemAvailable")
	}

	const kB = 1024
	totalBytes = total * kB
	usedBytes = (total - available) * kB
	return usedBytes, totalBytes, nil
}

// parseThermalTempC parses a /sys/class/thermal/*/temp file (millidegrees Celsius).
func parseThermalTempC(data []byte) (float64, error) {
	milliC, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse thermal reading %q: %w", strings.TrimSpace(string(data)), err)
	}
	return float64(milliC) / 1000, nil
}

type netIface struct {
	Name string
	IP   string
}

// listInterfaces is a package-level var so tests can stub it out.
var listInterfaces = defaultListInterfaces

func defaultListInterfaces() ([]netIface, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	var result []netIface
	for _, ifc := range ifaces {
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := ifc.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			ipNet, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			if v4 := ipNet.IP.To4(); v4 != nil {
				result = append(result, netIface{Name: ifc.Name, IP: v4.String()})
				break
			}
		}
	}
	return result, nil
}
