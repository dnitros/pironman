package hardware

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const PWMFanCoolingStatePath = "/sys/class/thermal/cooling_device0/cur_state"

const PWMFanHwmonFanInputGlob = "/sys/class/hwmon/hwmon*/fan1_input"

type PWMFanState struct {
	Level    int
	SpeedRPM int
}

type PWMFanReader interface {
	Read() (PWMFanState, error)
}

type SysPWMFanReader struct {
	coolingStatePath string
	hwmonInputGlob   string
}

func NewSysPWMFanReader(coolingStatePath, hwmonInputGlob string) *SysPWMFanReader {
	return &SysPWMFanReader{coolingStatePath: coolingStatePath, hwmonInputGlob: hwmonInputGlob}
}

func (r *SysPWMFanReader) Read() (PWMFanState, error) {
	levelData, err := os.ReadFile(r.coolingStatePath)
	if err != nil {
		return PWMFanState{}, fmt.Errorf("read PWM fan cooling state %s: %w", r.coolingStatePath, err)
	}
	level, err := parsePWMFanInt(levelData)
	if err != nil {
		return PWMFanState{}, fmt.Errorf("parse PWM fan cooling state: %w", err)
	}

	inputPath, err := resolvePWMFanInputPath(r.hwmonInputGlob)
	if err != nil {
		return PWMFanState{}, fmt.Errorf("resolve PWM fan hwmon input: %w", err)
	}
	speedData, err := os.ReadFile(inputPath)
	if err != nil {
		return PWMFanState{}, fmt.Errorf("read PWM fan speed %s: %w", inputPath, err)
	}
	speed, err := parsePWMFanInt(speedData)
	if err != nil {
		return PWMFanState{}, fmt.Errorf("parse PWM fan speed: %w", err)
	}

	return PWMFanState{Level: level, SpeedRPM: speed}, nil
}

func parsePWMFanInt(data []byte) (int, error) {
	return strconv.Atoi(strings.TrimSpace(string(data)))
}

// The hwmon index isn't stable across kernel/OS images, so the input path is
// resolved by glob rather than a fixed hwmonN path.
func resolvePWMFanInputPath(glob string) (string, error) {
	matches, err := filepath.Glob(glob)
	if err != nil {
		return "", fmt.Errorf("glob %s: %w", glob, err)
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("no hwmon fan1_input matched %s", glob)
	}
	return matches[0], nil
}
