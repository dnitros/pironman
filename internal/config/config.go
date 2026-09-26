package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"go.yaml.in/yaml/v3"
)

const DefaultPath = "/etc/pironman/config.yaml"

const PathEnvVar = "PIRONMAN_CONFIG_PATH"

func Path() string {
	if p := os.Getenv(PathEnvVar); p != "" {
		return p
	}
	return DefaultPath
}

type Config struct {
	RGB  RGB  `yaml:"rgb"`
	OLED OLED `yaml:"oled"`
	Fan  Fan  `yaml:"fan"`
}

type RGB struct {
	Enabled    bool   `yaml:"enabled"`
	Color      string `yaml:"color"`
	Brightness int    `yaml:"brightness"`
}

type OLED struct {
	Enabled               bool     `yaml:"enabled"`
	SleepTimeoutSeconds   int      `yaml:"sleep_timeout_seconds"`
	ScrollIntervalSeconds int      `yaml:"scroll_interval_seconds"`
	PageOrder             []string `yaml:"page_order"`
}

type Fan struct {
	CaseFanState string `yaml:"case_fan_state"`
}

func Default() Config {
	return Config{
		RGB: RGB{
			Enabled:    true,
			Color:      "#00ff00",
			Brightness: 80,
		},
		OLED: OLED{
			Enabled:               true,
			SleepTimeoutSeconds:   10,
			ScrollIntervalSeconds: 3,
			PageOrder:             []string{"mix", "performance", "ips", "disk"},
		},
		Fan: Fan{
			CaseFanState: "auto",
		},
	}
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Default(), nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("read config %s: %w", path, err)
	}

	cfg := Default()
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config %s: %w", path, err)
	}
	return cfg, nil
}

func (c Config) Save(path string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write config %s: %w", path, err)
	}
	return nil
}
