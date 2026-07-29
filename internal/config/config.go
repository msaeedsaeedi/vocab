package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	AutoStart bool `json:"auto_start"`
}

func Default() *Config {
	return &Config{}
}

type Manager struct {
	dataDir string
	cfg     *Config
}

func NewManager(dataDir string) (*Manager, error) {
	cfg, err := Load(dataDir)
	if err != nil {
		return nil, err
	}
	return &Manager{dataDir: dataDir, cfg: cfg}, nil
}

func Load(dataDir string) (*Config, error) {
	p := filepath.Join(dataDir, "config.json")
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return Default(), nil
		}
		return nil, err
	}
	var c Config
	if json.Unmarshal(data, &c) != nil {
		return Default(), nil
	}
	return &c, nil
}

func (m *Manager) Get() *Config { return m.cfg }

func (m *Manager) Save() error {
	p := filepath.Join(m.dataDir, "config.json")
	data, err := json.MarshalIndent(m.cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0644)
}

func (m *Manager) SetAutoStart(v bool) {
	m.cfg.AutoStart = v
}
