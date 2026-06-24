package dockerless

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/devsy-org/devsy/pkg/devcontainer/config"
	"github.com/devsy-org/devsy/pkg/driver"
)

// mountTypeBind is the only mount type supported by the dockerless driver.
const mountTypeBind = "bind"

func (p *DockerlessProvider) Enter(ctx context.Context, workspaceId string) error {
	containerDIR := filepath.Join(p.Config.TargetDir, "rootfs", workspaceId)
	statusDIR := filepath.Join(p.Config.TargetDir, "status", workspaceId)

	//nolint:gosec // path is derived from provider config, not user input
	runOptionsBytes, err := os.ReadFile(filepath.Join(statusDIR, "runOptions"))
	if err != nil {
		return err
	}

	runOptions := driver.RunOptions{}
	if err := json.Unmarshal(runOptionsBytes, &runOptions); err != nil {
		return err
	}

	if err := prepareMounts(containerDIR); err != nil {
		return err
	}

	mounts, err := collectMounts(&runOptions)
	if err != nil {
		return err
	}

	if err := performMounts(mounts, containerDIR); err != nil {
		return err
	}

	if err := syscall.Chdir(containerDIR); err != nil {
		return err
	}

	// then we set up the hostname.
	if err := syscall.Sethostname([]byte(workspaceId)); err != nil {
		return fmt.Errorf("error setting hostname for namespace: %w", err)
	}

	//nolint:gosec // entrypoint/cmd come from the container image config
	cmd := exec.Command(runOptions.Entrypoint, runOptions.Cmd...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Chroot: containerDIR,
	}
	cmd.Env = config.ObjectToList(runOptions.Env)

	return cmd.Run()
}

// collectMounts assembles the list of mounts to apply inside the container:
// the default resolv.conf/hosts binds, the workspace mount and any extra mounts.
func collectMounts(runOptions *driver.RunOptions) ([]*config.Mount, error) {
	mounts := []*config.Mount{
		{
			Source: "/etc/resolv.conf",
			Target: "/etc/resolv.conf",
			Type:   mountTypeBind,
		},
		{
			Source: "/etc/hosts",
			Target: "/etc/hosts",
			Type:   mountTypeBind,
		},
	}

	if mount := runOptions.WorkspaceMount; mount != nil {
		if mount.Target == "" {
			return nil, fmt.Errorf("workspace mount target is empty")
		}
		mounts = append(mounts, mount)
	}

	return append(mounts, runOptions.Mounts...), nil
}

func prepareMounts(rootfs string) error {
	if err := MountBind("/proc", filepath.Join(rootfs, "/proc")); err != nil {
		return err
	}

	if err := MountTmpfs(filepath.Join(rootfs, "/tmp")); err != nil {
		return err
	}

	if err := MountBind("/dev", filepath.Join(rootfs, "/dev")); err != nil {
		return err
	}

	if err := MountShm(filepath.Join(rootfs, "/dev/shm")); err != nil {
		return err
	}

	if err := MountDevPts(filepath.Join(rootfs, "/dev/pts")); err != nil {
		return err
	}

	return MountBind(filepath.Join(rootfs, "dev/pts/ptmx"), filepath.Join(rootfs, "dev/ptmx"))
}

func performMounts(mounts []*config.Mount, rootfs string) error {
	for _, mount := range mounts {
		if mount.Type != mountTypeBind {
			return fmt.Errorf(
				"unsupported mount type '%s' in mount '%s'",
				mount.Type,
				mount.String(),
			)
		}

		if err := performBindMount(mount, rootfs); err != nil {
			return err
		}
	}

	return nil
}

// performBindMount prepares the target path and bind-mounts a single mount into
// the container rootfs.
func performBindMount(mount *config.Mount, rootfs string) error {
	info, err := os.Stat(mount.Source)
	if err != nil {
		return err
	}

	target := filepath.Join(rootfs, mount.Target)
	if info.IsDir() {
		_ = os.MkdirAll(target, 0o750)
	} else {
		//nolint:gosec // target is derived from provider config, not user input
		file, _ := os.Create(target)
		defer func() { _ = file.Close() }()
	}

	return MountBind(mount.Source, target)
}
