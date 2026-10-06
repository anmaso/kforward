package config

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	AppDirName         = "kforward"
	PortForwardsDirName = "port-forwards"
)

func HomeDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return home, nil
}

func ConfigDir() (string, error) {
	home, err := HomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", AppDirName), nil
}

func PortForwardsDir() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, PortForwardsDirName), nil
}

func EnsurePortForwardsDir() (string, error) {
	dir, err := PortForwardsDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create port-forwards directory: %w", err)
	}
	return dir, nil
}
