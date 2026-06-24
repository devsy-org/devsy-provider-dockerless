package dockerless

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// ExecOptions bundles the parameters needed to execute a command inside a
// running container.
type ExecOptions struct {
	WorkspaceID string
	User        string
	Command     string
	Stdin       io.Reader
	Stdout      io.Writer
	Stderr      io.Writer
}

func (p *DockerlessProvider) ExecuteCommand(ctx context.Context, execOptions ExecOptions) error {
	containerDIR := filepath.Join(p.Config.TargetDir, "rootfs", execOptions.WorkspaceID)

	ppid, err := GetPid(execOptions.WorkspaceID)
	if err != nil {
		return fmt.Errorf("container %s is not running", execOptions.WorkspaceID)
	}

	// We want to enter the namespace of the PID1 inside the container.
	//nolint:gosec // ppid is an integer obtained from our own state dir
	pidBytes, err := exec.Command("pgrep", "-P", strconv.Itoa(ppid)).Output()
	if err != nil {
		return fmt.Errorf("container %s is not running", execOptions.WorkspaceID)
	}

	pid := string(bytes.TrimSpace(pidBytes))
	command := buildExecCommand(containerDIR, pid, execOptions.User, execOptions.Command)

	//nolint:gosec // args are built from constants and our own container state
	cmd := exec.Command("nsenter", command...)
	cmd.Stdin = execOptions.Stdin
	cmd.Stdout = execOptions.Stdout
	cmd.Stderr = execOptions.Stderr

	return cmd.Run()
}

// buildExecCommand assembles the nsenter arguments used to run command inside
// the namespaces of the container's PID1.
func buildExecCommand(containerDIR, pid, user, command string) []string {
	args := []string{
		"-m",
		"-u",
		"-i",
		"-p",
		"-r/proc/" + pid + "/root",
		"-w/proc/" + pid + "/root",
	}

	if _, err := os.Stat("/dev/net/tun"); err == nil {
		args = append(args, "-n")
	}

	// user namespace only if we're rootless
	if isRootless() {
		args = append(args, "-U", "--preserve-credentials")
	}

	args = append(args, "-t", pid, "sh", "-l", "-c")

	if user != "" && user != "0" && user != "root" {
		uid := findUserPasswd(containerDIR, user)
		command = "su -l " + uid + " -c " + command
	}

	return append(args, command)
}

func findUserPasswd(path, user string) string {
	//nolint:gosec // path is derived from provider config, not user input
	passwd, err := os.ReadFile(filepath.Join(path, "/etc/passwd"))
	if err != nil {
		return "root"
	}

	// find in /etc/passwd either ":uid:" or "username:"
	pattern := regexp.MustCompile(".*:" + user + ":.*")
	match := pattern.FindString(string(passwd))

	if len(match) == 0 {
		return user
	}

	return strings.Split(match, ":")[0]
}
