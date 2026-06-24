package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/devsy-org/devsy-provider-dockerless/cmd"
)

// binDir is where the embedded rootless networking helpers are extracted at
// runtime so they can be found on the PATH.
const binDir = "/tmp/dockerless"

func main() {
	if err := extractBinaries(); err != nil {
		log.Fatal(err)
	}

	cmd.Execute()
}

// extractBinaries writes the embedded rootlesskit and slirp4netns helpers to
// binDir and prepends it to the PATH. On platforms where the helpers are not
// embedded (anything other than linux), this is a no-op.
func extractBinaries() error {
	binaries := map[string][]byte{
		"rootlesskit": rootlesskit,
		"slirp4netns": slirp4netns,
	}

	wrote := false
	for name, data := range binaries {
		if len(data) == 0 {
			continue
		}

		if err := writeBinary(name, data); err != nil {
			return err
		}
		wrote = true
	}

	if wrote {
		if err := os.Setenv("PATH", os.Getenv("PATH")+":"+binDir); err != nil {
			return err
		}
	}

	return nil
}

// writeBinary extracts a single embedded helper to binDir if it is not already
// present.
func writeBinary(name string, data []byte) error {
	path := filepath.Join(binDir, name)
	if _, err := os.Stat(path); err == nil {
		return nil
	}

	if err := os.MkdirAll(binDir, 0o750); err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o750) //nolint:gosec // helper must be executable
}
