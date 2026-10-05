package sysstats

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"
)

const (
	DefaultStatPath    = "/proc/stat"
	DefaultThermalPath = "/sys/class/thermal/thermal_zone0/temp"
	DefaultMemInfoPath = "/proc/meminfo"
	DefaultMountsPath  = "/proc/mounts"
)

type Disk struct {
	Type       string
	UsedBytes  uint64
	TotalBytes uint64
	Percent    float64
}

type Snapshot struct {
	CPUPercent    float64
	CPUTempC      float64
	MemUsedBytes  uint64
	MemTotalBytes uint64
	MemPercent    float64
	Interfaces    map[string]string
	Disks         []Disk
}

type Source interface {
	Snapshot() (Snapshot, error)
}

type ProcSource struct {
	statPath, thermalPath, meminfoPath, mountsPath string

	mu                  sync.Mutex
	prevIdle, prevTotal uint64
	havePrev            bool
}

func NewProcSource(statPath, thermalPath, meminfoPath, mountsPath string) *ProcSource {
	return &ProcSource{statPath: statPath, thermalPath: thermalPath, meminfoPath: meminfoPath, mountsPath: mountsPath}
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
		if ifc.Name == "docker0" || strings.HasPrefix(ifc.Name, "br-") {
			continue
		}
		interfaces[ifc.Name] = ifc.IP
	}

	var memPercent float64
	if totalBytes > 0 {
		memPercent = 100 * float64(usedBytes) / float64(totalBytes)
	}

	mountsData, err := os.ReadFile(s.mountsPath)
	if err != nil {
		return Snapshot{}, fmt.Errorf("read %s: %w", s.mountsPath, err)
	}
	disks := disksFromMounts(parseMounts(mountsData), diskUsage)

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
		Disks:         disks,
	}, nil
}

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
		idle = values[3]
		if len(values) > 4 {
			idle += values[4]
		}
		return idle, total, nil
	}
	return 0, 0, fmt.Errorf("no aggregate \"cpu \" line found")
}

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

func parseThermalTempC(data []byte) (float64, error) {
	milliC, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse thermal reading %q: %w", strings.TrimSpace(string(data)), err)
	}
	return float64(milliC) / 1000, nil
}

type mountEntry struct {
	device     string
	mountpoint string
}

// ponytail: doesn't decode the \NNN octal escapes the kernel writes for spaces/tabs/backslashes
// in device or mountpoint fields; decode them if a real mountpoint ever contains one.
func parseMounts(data []byte) []mountEntry {
	var entries []mountEntry
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		entries = append(entries, mountEntry{device: fields[0], mountpoint: fields[1]})
	}
	return entries
}

func classifyDiskType(device string) string {
	switch {
	case strings.HasPrefix(device, "/dev/nvme"):
		return "nvme"
	case strings.HasPrefix(device, "/dev/mmcblk"):
		return "sd"
	case strings.HasPrefix(device, "/dev/md"):
		return "raid"
	case strings.HasPrefix(device, "/dev/sd"):
		return "usb"
	default:
		return "hd"
	}
}

// ponytail: two whole-disk devices whose names differ only by a trailing
// digit (e.g. /dev/md0 vs /dev/md1, /dev/nvme0n1 vs /dev/nvme0n2) parent-name
// to the same string and would wrongly merge into one row; add a
// /sys/class/block/<name>/partition existence check if multi-array or
// multi-NVMe setups need distinguishing.
func parentDeviceName(device string) string {
	name := strings.TrimPrefix(device, "/dev/")
	if name == "" || !isDigit(name[len(name)-1]) {
		return name
	}
	if idx := strings.LastIndexByte(name, 'p'); idx > 0 && idx < len(name)-1 &&
		isDigit(name[idx-1]) && allDigits(name[idx+1:]) {
		return name[:idx]
	}
	i := len(name)
	for i > 0 && isDigit(name[i-1]) {
		i--
	}
	return name[:i]
}

func isDigit(b byte) bool { return b >= '0' && b <= '9' }

func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if !isDigit(s[i]) {
			return false
		}
	}
	return true
}

func percentOf(usedBytes, totalBytes uint64) float64 {
	if totalBytes == 0 {
		return 0
	}
	return 100 * float64(usedBytes) / float64(totalBytes)
}

func disksFromMounts(entries []mountEntry, usage func(mountpoint string) (usedBytes, totalBytes uint64, err error)) []Disk {
	var disks []Disk
	rowByParent := make(map[string]int)
	for _, e := range entries {
		if !strings.HasPrefix(e.device, "/dev/") {
			continue
		}
		usedBytes, totalBytes, err := usage(e.mountpoint)
		if err != nil {
			continue
		}
		parent := parentDeviceName(e.device)
		if i, ok := rowByParent[parent]; ok {
			disks[i].UsedBytes += usedBytes
			disks[i].TotalBytes += totalBytes
			disks[i].Percent = percentOf(disks[i].UsedBytes, disks[i].TotalBytes)
			continue
		}
		rowByParent[parent] = len(disks)
		disks = append(disks, Disk{
			Type:       classifyDiskType(e.device),
			UsedBytes:  usedBytes,
			TotalBytes: totalBytes,
			Percent:    percentOf(usedBytes, totalBytes),
		})
	}
	return disks
}

var diskUsage = defaultDiskUsage

func defaultDiskUsage(mountpoint string) (usedBytes, totalBytes uint64, err error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(mountpoint, &st); err != nil {
		return 0, 0, fmt.Errorf("statfs %s: %w", mountpoint, err)
	}
	totalBytes = uint64(st.Bsize) * st.Blocks
	freeBytes := uint64(st.Bsize) * st.Bfree
	return totalBytes - freeBytes, totalBytes, nil
}

type netIface struct {
	Name string
	IP   string
}

var listInterfaces = defaultListInterfaces

func defaultListInterfaces() ([]netIface, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	var result []netIface
	for _, ifc := range ifaces {
		if ifc.Flags&net.FlagRunning == 0 || ifc.Flags&net.FlagLoopback != 0 {
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
