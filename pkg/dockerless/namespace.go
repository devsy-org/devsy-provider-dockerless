package dockerless

import (
	"os"
	"path/filepath"
)

// Commands used to enter new namespaces depending on whether we run rootless.
const (
	commandRootlesskit = "rootlesskit"
	commandUnshare     = "unshare"
)

// rootlesskit flags shared across the rootless namespace commands.
const (
	flagPidns     = "--pidns"
	flagCgroupns  = "--cgroupns"
	flagUtsns     = "--utsns"
	flagIpcns     = "--ipcns"
	flagNet       = "--net"
	flagStateDir  = "--state-dir"
	flagMountProc = "--mount-proc"
)

// stateDir returns the rootlesskit state directory for a given workspace.
func stateDir(workspaceId string) string {
	return filepath.Join("/tmp", "dockerless", workspaceId)
}

// isRootless reports whether the current process runs as a non-root user and
// therefore needs rootlesskit to create namespaces.
func isRootless() bool {
	return os.Getuid() > 0
}

// namespaceCommand returns the command and base arguments used to enter the
// container namespaces. When running rootless it uses rootlesskit with host
// networking; otherwise it falls back to unshare. Additional arguments
// (e.g. the command to run) should be appended by the caller.
func namespaceCommand(workspaceId string) (string, []string) {
	if isRootless() {
		return commandRootlesskit, []string{
			flagPidns,
			flagCgroupns,
			flagUtsns,
			flagIpcns,
			flagNet,
			"host",
			flagStateDir,
			stateDir(workspaceId),
		}
	}

	return commandUnshare, []string{
		"-m",
		"-p",
		"-u",
		"-f",
		flagMountProc,
	}
}
