package main

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml"
)

const (
	providerName = "dockerless"
	githubOwner  = "devsy-org"
	githubRepo   = "devsy-provider-dockerless"
)

type Provider struct {
	Name        string            `yaml:"name"`
	Version     string            `yaml:"version"`
	Icon        string            `yaml:"icon"`
	Home        string            `yaml:"home"`
	Description string            `yaml:"description"`
	Options     Options           `yaml:"options"`
	Agent       Agent             `yaml:"agent"`
	Exec        map[string]string `yaml:"exec"`
}

type Options map[string]Option

type Option struct {
	Description string `yaml:"description,omitempty"`
	Default     string `yaml:"default,omitempty"`
	Required    bool   `yaml:"required,omitempty"`
	Global      bool   `yaml:"global,omitempty"`
}

type Agent struct {
	ContainerInactivityTimeout string         `yaml:"containerInactivityTimeout"`
	Local                      bool           `yaml:"local"`
	Docker                     Docker         `yaml:"docker"`
	Binaries                   map[string]any `yaml:"binaries"`
	Driver                     string         `yaml:"driver"`
	Custom                     CustomDriver   `yaml:"custom"`
}

type Docker struct {
	Install bool `yaml:"install"`
}

type CustomDriver struct {
	FindDevContainer    string `yaml:"findDevContainer"`
	CommandDevContainer string `yaml:"commandDevContainer"`
	StartDevContainer   string `yaml:"startDevContainer"`
	StopDevContainer    string `yaml:"stopDevContainer"`
	RunDevContainer     string `yaml:"runDevContainer"`
	DeleteDevContainer  string `yaml:"deleteDevContainer"`
	TargetArchitecture  string `yaml:"targetArchitecture"`
}

type Binary struct {
	OS       string `yaml:"os"`
	Arch     string `yaml:"arch"`
	Path     string `yaml:"path"`
	Checksum string `yaml:"checksum"`
}

type buildConfig struct {
	version     string
	projectRoot string
	isRelease   bool
	checksums   map[string]string
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) != 2 {
		return fmt.Errorf("expected version as argument")
	}

	cfg, err := newBuildConfig(os.Args[1])
	if err != nil {
		return err
	}

	output, err := yaml.Marshal(buildProvider(cfg))
	if err != nil {
		return fmt.Errorf("marshal yaml: %w", err)
	}

	_, err = os.Stdout.Write(output)
	return err
}

func newBuildConfig(version string) (*buildConfig, error) {
	checksums, err := parseChecksums("./dist/checksums.txt")
	if err != nil {
		return nil, fmt.Errorf("parse checksums: %w", err)
	}

	projectRoot := os.Getenv("PROJECT_ROOT")
	if projectRoot == "" {
		owner := getEnvOrDefault("GITHUB_OWNER", githubOwner)
		projectRoot = fmt.Sprintf(
			"https://github.com/%s/%s/releases/download/%s",
			owner,
			githubRepo,
			version,
		)
	}

	isRelease := strings.Contains(projectRoot, "github.com") &&
		strings.Contains(projectRoot, "/releases/")

	return &buildConfig{
		version:     version,
		projectRoot: projectRoot,
		isRelease:   isRelease,
		checksums:   checksums,
	}, nil
}

func buildProvider(cfg *buildConfig) Provider {
	return Provider{
		Name:        providerName,
		Version:     cfg.version,
		Icon:        "https://raw.githubusercontent.com/devsy-org/devsy/main/desktop/src/renderer/public/icons/providers/docker.svg",
		Home:        "https://github.com/devsy-org/devsy",
		Description: "Devsy without Docker",
		Options:     buildOptions(),
		Agent:       buildAgent(cfg),
		Exec: map[string]string{
			"command": "\"${DEVSY}\" helper sh -c \"${COMMAND}\"",
		},
	}
}

func buildOptions() Options {
	return Options{
		"TARGET_DIR": {
			Description: "Root directory for the container and images.",
			Required:    true,
		},
	}
}

func buildAgent(cfg *buildConfig) Agent {
	return Agent{
		ContainerInactivityTimeout: "${INACTIVITY_TIMEOUT}",
		Local:                      true,
		Docker: Docker{
			Install: false,
		},
		Binaries: map[string]any{
			"DOCKERLESS_PROVIDER": buildBinaryList(cfg, allPlatforms()),
		},
		Driver: "custom",
		Custom: CustomDriver{
			FindDevContainer:    "\"${DOCKERLESS_PROVIDER}\" find",
			CommandDevContainer: "\"${DOCKERLESS_PROVIDER}\" command",
			StartDevContainer:   "\"${DOCKERLESS_PROVIDER}\" start",
			StopDevContainer:    "\"${DOCKERLESS_PROVIDER}\" stop",
			RunDevContainer:     "\"${DOCKERLESS_PROVIDER}\" run",
			DeleteDevContainer:  "\"${DOCKERLESS_PROVIDER}\" delete",
			TargetArchitecture:  "\"${DOCKERLESS_PROVIDER}\" target-architecture",
		},
	}
}

func buildBinaryList(cfg *buildConfig, platforms []string) []Binary {
	result := make([]Binary, 0, len(platforms))
	for _, platform := range platforms {
		result = append(result, buildBinary(cfg, platform))
	}
	return result
}

func buildBinary(cfg *buildConfig, platform string) Binary {
	os, arch, _ := strings.Cut(platform, "/")

	path := cfg.projectRoot
	if !cfg.isRelease {
		if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
			base, _ := url.Parse(path)
			joined, _ := url.JoinPath(base.String(), buildDir(platform))
			path = joined
		} else {
			absPath, _ := filepath.Abs(path)
			path = filepath.Join(absPath, buildDir(platform))
		}
	}

	filename := fmt.Sprintf("devsy-provider-%s-%s-%s", providerName, os, arch)

	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		path, _ = url.JoinPath(path, filename)
	} else {
		path = filepath.Join(path, filename)
	}

	return Binary{
		OS:       os,
		Arch:     arch,
		Path:     path,
		Checksum: cfg.checksums[filename],
	}
}

func buildDir(platform string) string {
	dirs := map[string]string{
		"linux/amd64": "build_linux_amd64_v1",
		"linux/arm64": "build_linux_arm64_v8.0",
	}
	return dirs[platform]
}

func allPlatforms() []string {
	return []string{"linux/amd64", "linux/arm64"}
}

func parseChecksums(path string) (map[string]string, error) {
	file, err := os.Open(path) //nolint:gosec // path is a build-time constant, not user input
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	checksums := make(map[string]string)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if checksum, filename, ok := strings.Cut(scanner.Text(), "  "); ok {
			checksums[strings.TrimSpace(filename)] = checksum
		}
	}

	return checksums, scanner.Err()
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
