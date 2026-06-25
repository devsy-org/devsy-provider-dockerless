package dockerless

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/devsy-org/devsy-provider-dockerless/pkg/options"
	"github.com/devsy-org/devsy/pkg/devcontainer/config"
	"github.com/devsy-org/log"
)

type DockerlessProvider struct {
	Config *options.Options
	Log    log.Logger
}

func NewProvider(
	ctx context.Context,
	options *options.Options,
	logs log.Logger,
) (*DockerlessProvider, error) {
	// create provider
	provider := &DockerlessProvider{
		Config: options,
		Log:    logs,
	}

	return provider, nil
}

func (p *DockerlessProvider) Find(
	ctx context.Context,
	workspaceId string,
) (*config.ContainerDetails, error) {
	statusDIR := filepath.Join(p.Config.TargetDir, "status", workspaceId)
	detailsPath := filepath.Join(statusDIR, "containerDetails")

	// check if the rootfs exists
	if _, err := os.Stat(statusDIR); err != nil {
		return nil, fmt.Errorf("container %s does not exist", workspaceId)
	}

	// check if the containerDetails exists
	if _, err := os.Stat(detailsPath); err != nil {
		return nil, fmt.Errorf("container %s does not exist", workspaceId)
	}

	//nolint:gosec // path is derived from provider config, not user input
	containerDetailsBytes, err := os.ReadFile(detailsPath)
	if err != nil {
		return nil, err
	}

	containerDetails := config.ContainerDetails{}
	if err := json.Unmarshal(containerDetailsBytes, &containerDetails); err != nil {
		return nil, err
	}

	status := "stopped"
	if pid, err := GetPid(workspaceId); err == nil && pid > 1 {
		// file exists, pid is running
		status = "running"
	}

	containerDetails.State.Status = status

	return &containerDetails, nil
}

func (p *DockerlessProvider) Stop(ctx context.Context, workspaceId string) error {
	p.Log.Infof("stopping: %s", workspaceId)

	pid, err := GetPid(workspaceId)
	if err != nil {
		return err
	}

	p.Log.Debugf("found parent process: %d", pid)

	//nolint:gosec // pid is an integer obtained from our own state dir
	cmd := exec.Command("kill", "-9", strconv.Itoa(pid))
	return cmd.Run()
}

func (p *DockerlessProvider) Delete(ctx context.Context, workspaceId string) error {
	p.Log.Infof("deleting: %s", workspaceId)

	_ = p.Stop(ctx, workspaceId)

	containerDIR := filepath.Join(p.Config.TargetDir, "rootfs", workspaceId)
	statusDIR := filepath.Join(p.Config.TargetDir, "status", workspaceId)

	if err := os.RemoveAll(statusDIR); err != nil {
		return err
	}

	command, args := namespaceCommand(workspaceId)
	args = append(args, "rm", "-rf", containerDIR)

	//nolint:gosec // command/args are built from constants and provider config
	cmd := exec.Command(command, args...)
	return cmd.Run()
}
