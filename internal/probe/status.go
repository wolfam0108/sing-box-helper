package probe

import (
	"bufio"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// Status is a snapshot of the runtime state we care about.
type Status struct {
	SingBoxRunning bool   `json:"sing_box_running"`
	SingBoxPID     int    `json:"sing_box_pid,omitempty"`
	SingBoxVersion string `json:"sing_box_version,omitempty"`
	TunUp          bool   `json:"tun_up"`
	TunName        string `json:"tun_name"`
}

// CollectStatus assembles a Status by reading /proc and running sing-box
// version. On non-Linux dev machines /proc isn't there and the process
// check returns false — that's the desired fallback.
func CollectStatus(tunName string) Status {
	s := Status{TunName: tunName}

	if pid, ok := findProcess("sing-box"); ok {
		s.SingBoxRunning = true
		s.SingBoxPID = pid
	}
	if v, err := singBoxVersion(); err == nil {
		s.SingBoxVersion = v
	}

	// net.InterfaceByName works on both Linux and Windows; on Windows the
	// caller will obviously not have a "singtun" interface — UP=false.
	if iface, err := net.InterfaceByName(tunName); err == nil {
		s.TunUp = iface.Flags&net.FlagUp != 0
	}

	return s
}

// findProcess returns the PID of the first process whose /proc/PID/comm
// matches name, or (0, false). Returns false on non-Linux platforms where
// /proc doesn't exist.
func findProcess(name string) (int, bool) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return 0, false
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		commPath := "/proc/" + e.Name() + "/comm"
		f, err := os.Open(commPath)
		if err != nil {
			continue
		}
		sc := bufio.NewScanner(f)
		sc.Scan()
		comm := strings.TrimSpace(sc.Text())
		_ = f.Close()
		if comm == name {
			return pid, true
		}
	}
	return 0, false
}

// singBoxVersion shells out to `sing-box version` and returns the first
// line of stdout (the "sing-box version X.Y.Z" line). Empty string + error
// if sing-box isn't in PATH.
func singBoxVersion() (string, error) {
	out, err := exec.Command("sing-box", "version").Output()
	if err != nil {
		return "", err
	}
	sc := bufio.NewScanner(strings.NewReader(string(out)))
	sc.Scan()
	return strings.TrimSpace(sc.Text()), nil
}
