package state

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/amarin/kforward/internal/config"
)

type Forward struct {
	Namespace  string
	Name       string
	LocalPort  int
	RemotePort int
	PID        int
	FileName   string
	FilePath   string
}

func (f Forward) Label() string {
	return fmt.Sprintf("%s/%s  localhost:%d -> :%d  (pid %d)", f.Namespace, f.Name, f.LocalPort, f.RemotePort, f.PID)
}

func FileName(namespace, name string, localPort, remotePort int) string {
	return fmt.Sprintf("%s__%s__%d__%d", namespace, name, localPort, remotePort)
}

func ParseFileName(fileName string) (namespace, name string, localPort, remotePort int, err error) {
	parts := strings.Split(fileName, "__")
	if len(parts) != 4 {
		return "", "", 0, 0, fmt.Errorf("invalid port-forward file name %q", fileName)
	}

	localPort, err = strconv.Atoi(parts[2])
	if err != nil {
		return "", "", 0, 0, fmt.Errorf("parse local port in %q: %w", fileName, err)
	}

	remotePort, err = strconv.Atoi(parts[3])
	if err != nil {
		return "", "", 0, 0, fmt.Errorf("parse remote port in %q: %w", fileName, err)
	}

	return parts[0], parts[1], localPort, remotePort, nil
}

func WritePID(namespace, name string, localPort, remotePort, pid int) (string, error) {
	dir, err := config.EnsurePortForwardsDir()
	if err != nil {
		return "", err
	}

	fileName := FileName(namespace, name, localPort, remotePort)
	path := filepath.Join(dir, fileName)

	if err := os.WriteFile(path, []byte(strconv.Itoa(pid)), 0o644); err != nil {
		return "", fmt.Errorf("write pid file: %w", err)
	}
	return path, nil
}

func ListForwards() ([]Forward, error) {
	dir, err := config.EnsurePortForwardsDir()
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read port-forwards directory: %w", err)
	}

	var forwards []Forward
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		namespace, name, localPort, remotePort, err := ParseFileName(entry.Name())
		if err != nil {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
		if err != nil {
			continue
		}

		forwards = append(forwards, Forward{
			Namespace:  namespace,
			Name:       name,
			LocalPort:  localPort,
			RemotePort: remotePort,
			PID:        pid,
			FileName:   entry.Name(),
			FilePath:   path,
		})
	}

	return forwards, nil
}

func RemoveForward(filePath string) error {
	if err := os.Remove(filePath); err != nil {
		return fmt.Errorf("remove pid file: %w", err)
	}
	return nil
}
