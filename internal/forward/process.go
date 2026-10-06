package forward

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

func IsRunning(pid int) bool {
	if pid <= 0 {
		return false
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	return process.Signal(syscall.Signal(0)) == nil
}

type PortUser struct {
	PID     int
	Name    string // comm (basename)
	Args    string // full command line
	User    string
	Elapsed string
}

// FindPortUser returns info about whatever is listening on the given TCP port,
// or (nil, nil) if the port is free.
func FindPortUser(port int) (*PortUser, error) {
	out, err := exec.Command("lsof", "-ti", fmt.Sprintf("tcp:%d", port), "-sTCP:LISTEN").Output()
	if err != nil {
		return nil, nil
	}
	pidStr := strings.TrimSpace(strings.SplitN(string(out), "\n", 2)[0])
	if pidStr == "" {
		return nil, nil
	}
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return nil, fmt.Errorf("parse lsof output %q: %w", pidStr, err)
	}

	u := &PortUser{PID: pid}

	// user, elapsed, comm have no spaces — safe to split on whitespace
	if info, err := exec.Command("ps", "-p", pidStr, "-o", "user=,etime=,comm=").Output(); err == nil {
		fields := strings.Fields(strings.TrimSpace(string(info)))
		if len(fields) >= 1 {
			u.User = fields[0]
		}
		if len(fields) >= 2 {
			u.Elapsed = fields[1]
		}
		if len(fields) >= 3 {
			u.Name = fields[2]
		}
	}

	// args may contain spaces — read separately
	if args, err := exec.Command("ps", "-p", pidStr, "-o", "args=").Output(); err == nil {
		u.Args = strings.TrimSpace(string(args))
	}

	return u, nil
}

func KillProcess(pid int) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("find process %d: %w", pid, err)
	}
	if err := process.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("kill process %d: %w", pid, err)
	}
	return nil
}
