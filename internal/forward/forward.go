package forward

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"github.com/amarin/kforward/internal/discovery"
	"github.com/amarin/kforward/internal/state"
)

type Mapping struct {
	Local  int
	Remote int32
}

func ParseMapping(raw string) (Mapping, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Mapping{}, fmt.Errorf("port mapping must not be empty")
	}

	parts := strings.Split(raw, ":")
	if len(parts) != 2 {
		return Mapping{}, fmt.Errorf("port mapping must be local:remote, got %q", raw)
	}

	local, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil || local <= 0 || local > 65535 {
		return Mapping{}, fmt.Errorf("invalid local port in %q", raw)
	}

	remote, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil || remote <= 0 || remote > 65535 {
		return Mapping{}, fmt.Errorf("invalid remote port in %q", raw)
	}

	return Mapping{Local: local, Remote: int32(remote)}, nil
}

func Start(target discovery.Target, mapping Mapping) (*state.Forward, error) {
	args := []string{
		"port-forward",
		resourceRef(target),
		fmt.Sprintf("%d:%d", mapping.Local, mapping.Remote),
		"-n", target.Namespace,
	}

	cmd := exec.Command("kubectl", args...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start kubectl port-forward: %w", err)
	}

	path, err := state.WritePID(target.Namespace, target.Name, mapping.Local, int(mapping.Remote), cmd.Process.Pid)
	if err != nil {
		_ = cmd.Process.Kill()
		return nil, err
	}

	return &state.Forward{
		Namespace:  target.Namespace,
		Name:       target.Name,
		LocalPort:  mapping.Local,
		RemotePort: int(mapping.Remote),
		PID:        cmd.Process.Pid,
		FileName:   state.FileName(target.Namespace, target.Name, mapping.Local, int(mapping.Remote)),
		FilePath:   path,
	}, nil
}

func Stop(f state.Forward) error {
	process, err := os.FindProcess(f.PID)
	if err == nil {
		_ = process.Signal(syscall.SIGTERM)
	}

	if err := state.RemoveForward(f.FilePath); err != nil {
		return err
	}
	return nil
}

func resourceRef(target discovery.Target) string {
	switch target.Kind {
	case discovery.KindDeployment:
		return "deployment/" + target.Name
	default:
		return "svc/" + target.Name
	}
}
