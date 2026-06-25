package dockerless

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/devsy-org/devsy/pkg/devcontainer/config"
	"github.com/devsy-org/devsy/pkg/driver"
	"github.com/devsy-org/devsy/pkg/log"
	"github.com/google/go-containerregistry/pkg/legacy"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
)

// CreateRootfs will generate a chrootable rootfs from input oci image reference, with input name and config.
// If input image is not found it will be automatically pulled.
// This function will read the oci-image manifest and properly unpack the layers in the right order to generate
// a valid rootfs.
// Untarring process will follow the keep-id option if specified in order to ensure no permission problems.
// Generated config will be saved inside the container's dir. This will NOT be an oci-compatible container config.
func (p *DockerlessProvider) Create(
	ctx context.Context,
	workspaceId string,
	runOptions *driver.RunOptions,
) error {
	image := imageName(runOptions.Image)

	imageDir := filepath.Join(p.Config.TargetDir, "images", image)
	containerDIR := filepath.Join(p.Config.TargetDir, "rootfs", workspaceId)
	statusDIR := filepath.Join(p.Config.TargetDir, "status", workspaceId)
	configPath := filepath.Join(statusDIR, "runOptions")

	// if the container already exists, exit
	if _, err := os.Stat(configPath); err == nil {
		return nil
	}

	if err := os.MkdirAll(containerDIR, 0o750); err != nil {
		return err
	}

	if err := os.MkdirAll(statusDIR, 0o750); err != nil {
		return err
	}

	layerConfig, err := p.unpackLayers(workspaceId, imageDir, containerDIR)
	if err != nil {
		return err
	}

	log.Debugf("preparing runoptions")
	mergeRunOptions(runOptions, layerConfig)

	if err := writeJSON(configPath, runOptions); err != nil {
		return err
	}

	containerDetails := initializeContainerDetails(workspaceId, runOptions)
	if err := writeJSON(
		filepath.Join(statusDIR, "containerDetails"),
		containerDetails,
	); err != nil {
		return err
	}

	log.Info("done")

	return nil
}

// unpackLayers reads the image manifest and config, unpacks every layer into
// the container rootfs and returns the parsed layer config.
func (p *DockerlessProvider) unpackLayers(
	workspaceId, imageDir, containerDIR string,
) (*legacy.LayerConfigFile, error) {
	manifest, err := readManifest(imageDir)
	if err != nil {
		return nil, err
	}

	layerConfig, err := readLayerConfig(imageDir)
	if err != nil {
		return nil, err
	}

	log.Info("preparing container rootfs")

	for index, layer := range manifest.Layers {
		layerDigest := strings.Split(layer.Digest.String(), ":")[1] + ".tar.gz"

		log.Debugf("unpacking layer %d of %d", index+1, len(manifest.Layers))

		if err := UntarFile(
			workspaceId,
			filepath.Join(imageDir, layerDigest),
			containerDIR,
		); err != nil {
			return nil, err
		}
	}

	log.Info("done")

	return layerConfig, nil
}

// imageName normalizes an image reference, falling back to the raw value when
// it cannot be parsed.
func imageName(image string) string {
	if ref, err := name.ParseReference(image); err == nil {
		return ref.Name()
	}

	return image
}

func readManifest(imageDir string) (*v1.Manifest, error) {
	path := filepath.Join(imageDir, "manifest.json")

	//nolint:gosec // path is derived from provider config, not user input
	manifestFile, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var manifest v1.Manifest
	if err := json.Unmarshal(manifestFile, &manifest); err != nil {
		return nil, err
	}

	return &manifest, nil
}

func readLayerConfig(imageDir string) (*legacy.LayerConfigFile, error) {
	path := filepath.Join(imageDir, "config.json")

	//nolint:gosec // path is derived from provider config, not user input
	configFile, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var layerConfig legacy.LayerConfigFile
	if err := json.Unmarshal(configFile, &layerConfig); err != nil {
		return nil, err
	}

	return &layerConfig, nil
}

// mergeRunOptions fills in the run options with defaults derived from the
// image's layer config (environment, entrypoint and command).
func mergeRunOptions(runOptions *driver.RunOptions, layerConfig *legacy.LayerConfigFile) {
	if runOptions.Env == nil {
		runOptions.Env = make(map[string]string)
	}

	// Merge container's default environment with the custom one.
	containerEnv := config.ListToObject(layerConfig.Config.Env)
	for k, v := range containerEnv {
		if runOptions.Env[k] == "" {
			runOptions.Env[k] = v
		}
	}

	// fallback TERM to xterm if not defined.
	if runOptions.Env["TERM"] == "" {
		runOptions.Env["TERM"] = "xterm"
	}

	// set entrypoint if empty.
	if runOptions.Entrypoint == "" {
		runOptions.Entrypoint = layerConfig.Config.Cmd[0]
		runOptions.Cmd = layerConfig.Config.Cmd[1:]
	}
}

// writeJSON marshals v as indented JSON and writes it to path.
func writeJSON(path string, v any) error {
	file, err := json.MarshalIndent(v, "", " ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, file, 0o600)
}

func initializeContainerDetails(
	workspaceId string,
	runOptions *driver.RunOptions,
) *config.ContainerDetails {
	return &config.ContainerDetails{
		ID:      workspaceId,
		Created: time.Now().String(),
		State: config.ContainerDetailsState{
			Status:    "exited",
			StartedAt: "",
		},
		Config: config.ContainerDetailsConfig{
			Labels: config.ListToObject(runOptions.Labels),
		},
	}
}
