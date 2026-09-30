//go:build linux

package hardware

import (
	"encoding/binary"
	"fmt"
	"os"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

const (
	inputDevicesDir = "/dev/input"

	evKey    = 0x01
	keyPower = 116

	keyBitmapBytes = 96
)

const (
	iocRead  = 2
	iocWrite = 1

	evdevIOCType = 'E'

	eviocgbitEVKEYNr = 0x20 + evKey
	eviocgrabNr      = 0x90
)

func ioctlCode(dir, nr int, size uintptr) uint32 {
	return uint32(dir)<<30 | uint32(size)<<16 | uint32(evdevIOCType)<<8 | uint32(nr)
}

func ioctl(fd uintptr, req uint32, ptr unsafe.Pointer) error {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, uintptr(req), uintptr(ptr))
	if errno != 0 {
		return errno
	}
	return nil
}

func hasKeyPower(fd uintptr) (bool, error) {
	var bits [keyBitmapBytes]byte
	req := ioctlCode(iocRead, eviocgbitEVKEYNr, unsafe.Sizeof(bits))
	if err := ioctl(fd, req, unsafe.Pointer(&bits[0])); err != nil {
		return false, fmt.Errorf("EVIOCGBIT(EV_KEY): %w", err)
	}
	return bits[keyPower/8]&(1<<uint(keyPower%8)) != 0, nil
}

func grab(fd uintptr, on bool) error {
	var v int32
	if on {
		v = 1
	}
	req := ioctlCode(iocWrite, eviocgrabNr, unsafe.Sizeof(v))
	return ioctl(fd, req, unsafe.Pointer(&v))
}

type EvdevPowerButtonWatcher struct {
	file *os.File
	fd   uintptr

	closeOnce sync.Once
	closeErr  error
}

func NewEvdevPowerButtonWatcher() (*EvdevPowerButtonWatcher, error) {
	entries, err := os.ReadDir(inputDevicesDir)
	if err != nil {
		return nil, fmt.Errorf("list %s: %w", inputDevicesDir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		path := inputDevicesDir + "/" + entry.Name()
		fd, err := syscall.Open(path, syscall.O_RDWR, 0)
		if err != nil {
			continue
		}

		ok, err := hasKeyPower(uintptr(fd))
		if err != nil || !ok {
			_ = syscall.Close(fd)
			continue
		}

		if err := grab(uintptr(fd), true); err != nil {
			_ = syscall.Close(fd)
			return nil, fmt.Errorf("grab power-button device %s: %w", path, err)
		}
		if err := syscall.SetNonblock(fd, true); err != nil {
			_ = syscall.Close(fd)
			return nil, fmt.Errorf("set power-button device %s non-blocking: %w", path, err)
		}
		return &EvdevPowerButtonWatcher{file: os.NewFile(uintptr(fd), path), fd: uintptr(fd)}, nil
	}

	return nil, fmt.Errorf("power-button input device not found (no device advertises KEY_POWER)")
}

type rawInputEvent struct {
	Sec, Usec  int64
	Type, Code uint16
	Value      int32
}

func (w *EvdevPowerButtonWatcher) Next() (PowerButtonEvent, error) {
	for {
		var ev rawInputEvent
		if err := binary.Read(w.file, binary.LittleEndian, &ev); err != nil {
			return PowerButtonEvent{}, fmt.Errorf("read power-button event: %w", err)
		}
		if ev.Type != evKey || ev.Code != keyPower {
			continue
		}

		at := time.Unix(ev.Sec, ev.Usec*1000)
		switch ev.Value {
		case 1:
			return PowerButtonEvent{Pressed: true, At: at}, nil
		case 0:
			return PowerButtonEvent{Pressed: false, At: at}, nil
		default:
			continue
		}
	}
}

func (w *EvdevPowerButtonWatcher) Close() error {
	w.closeOnce.Do(func() {
		if err := grab(w.fd, false); err != nil {
			_ = w.file.Close()
			w.closeErr = fmt.Errorf("ungrab power-button device: %w", err)
			return
		}
		w.closeErr = w.file.Close()
	})
	return w.closeErr
}
