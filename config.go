package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	Token  string `json:"token"`
	ApiURL string `json:"apiUrl"`
}

func configPath() string {
	// ProgramData é acessível por todos os usuários incluindo SYSTEM
	programData := os.Getenv("ProgramData")
	if programData != "" {
		return filepath.Join(programData, "DeskPilot", "config.json")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".deskpilot", "config.json")
}

func LoadConfig() (*Config, error) {
	data, err := os.ReadFile(configPath())
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func SaveConfig(cfg *Config) error {
	path := configPath()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}
