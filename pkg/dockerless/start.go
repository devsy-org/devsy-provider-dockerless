package dockerless

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/devsy-org/devsy/pkg/driver"
	"github.com/devsy-org/devsy/pkg/log"
)

func (p *DockerlessProvider) Start(ctx context.Context, workspaceId string) error {
	statusDIR := filepath.Join(p.Config.TargetDir, "status", workspaceId)

	// return early if the container is already running
	containerDetails, err := p.Find(ctx, workspaceId)
	if err == nil && containerDetails.State.Status == "running" {
		return nil
	}

	log.Debugf("container %s is not running, starting", workspaceId)

	log.Debugf("retrieving runOptions")

	//nolint:gosec // path is derived from provider config, not user input
	runOptionsBytes, err := os.ReadFile(filepath.Join(statusDIR, "runOptions"))
	if err != nil {
		return err
	}

	runOptions := driver.RunOptions{}

	err = json.Unmarshal(runOptionsBytes, &runOptions)
	if err != nil {
		return err
	}

	p.warnUnsupportedOptions(&runOptions)

	command, args := startNamespaceCommand(workspaceId)
	args = append(args,
		os.Args[0],
		"enter",
		base64.StdEncoding.EncodeToString([]byte(workspaceId)),
	)

	//nolint:gosec // command/args are built from constants and provider config
	cmd := exec.Command(command, args...)
	cmd.Env = os.Environ()

	log.Infof("starting the container")

	log.Debugf("executing helper command: %s %s", command, strings.Join(args, " "))

	err = cmd.Start()
	if err != nil {
		return err
	}

	return cmd.Process.Release()
}

// warnUnsupportedOptions logs warnings for run options the dockerless driver
// cannot honor.
func (p *DockerlessProvider) warnUnsupportedOptions(runOptions *driver.RunOptions) {
	if len(runOptions.SecurityOpt) > 0 {
		log.Warn("unsupported option by the dockerless driver: SecurityOpt")
	}

	if len(runOptions.CapAdd) > 0 {
		log.Warn("unsupported option by the dockerless driver: CapAdd")
	}
}

// startNamespaceCommand builds the namespace command used to start (enter) a
// container. When rootless, it enables slirp4netns networking if /dev/net/tun
// is available; otherwise it falls back to unshare.
func startNamespaceCommand(workspaceId string) (string, []string) {
	if !isRootless() {
		return commandUnshare, []string{
			"-m",
			"-p",
			"-u",
			"-f",
			flagMountProc,
		}
	}

	args := []string{
		flagPidns,
		flagCgroupns,
		flagUtsns,
		flagIpcns,
		flagStateDir,
		stateDir(workspaceId),
	}

	// Default to slirp4netns if we have /dev/net/tun access.
	if _, err := os.Stat("/dev/net/tun"); err == nil {
		args = append(args,
			flagNet,
			"slirp4netns",
			"--port-driver",
			"slirp4netns",
			"--disable-host-loopback",
			"--copy-up",
			"/etc",
		)
	}

	return commandRootlesskit, args
}
