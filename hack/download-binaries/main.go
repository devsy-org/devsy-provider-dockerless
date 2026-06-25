// Command download-binaries fetches the rootless networking helpers
// (rootlesskit, slirp4netns) for every supported linux architecture and writes
// them into ./bin so they can be embedded into the provider binary by
// goreleaser. Each download is verified against a pinned SHA256 checksum.
//
// It is invoked from .goreleaser.yml's before hook and from the Taskfile.
// Existing binaries whose checksum already matches are left untouched, so the
// command is safe to run repeatedly.
package main

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const (
	rootlesskitVersion = "1.1.1"
	slirp4netnsVersion = "1.2.2"

	binDir      = "bin"
	httpTimeout = 5 * time.Minute
)

// helper describes a single binary to download for a single architecture.
type helper struct {
	// output is the file name written into binDir; it must match the path
	// referenced by the //go:embed directives.
	output string
	// url is the upstream release asset to download.
	url string
	// sha256 is the expected checksum of the final binary (after extraction
	// for tarballs).
	sha256 string
	// archiveMember, when non-empty, indicates url is a gzipped tarball and
	// names the entry to extract.
	archiveMember string
}

func helpers() []helper {
	rk := func(arch string) string {
		return fmt.Sprintf(
			"https://github.com/rootless-containers/rootlesskit/releases/download/v%s/rootlesskit-%s.tar.gz",
			rootlesskitVersion,
			arch,
		)
	}
	s4 := func(arch string) string {
		return fmt.Sprintf(
			"https://github.com/rootless-containers/slirp4netns/releases/download/v%s/slirp4netns-%s",
			slirp4netnsVersion,
			arch,
		)
	}

	return []helper{
		{
			output:        "rootlesskit-linux-amd64",
			url:           rk("x86_64"),
			sha256:        "c44953d51e15dce36d42ddc58ebfde085e2ebfb7f1c9096d2ac8bfefcdb0596e",
			archiveMember: "rootlesskit",
		},
		{
			output:        "rootlesskit-linux-arm64",
			url:           rk("aarch64"),
			sha256:        "225ddfdea7642e747cb1c8757dd9977bf2de418805636b41437e048ab789472d",
			archiveMember: "rootlesskit",
		},
		{
			output: "slirp4netns-linux-amd64",
			url:    s4("x86_64"),
			sha256: "2b59dd438ec1814dcd00c3106c0288ca174c3fe9a178f3400baa06818edaae8d",
		},
		{
			output: "slirp4netns-linux-arm64",
			url:    s4("aarch64"),
			sha256: "f7d4913ff27f017e22a5aa66a233f0d403549539a6fab594cbbad258f965af1a",
		},
	}
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	if err := os.MkdirAll(binDir, 0o750); err != nil {
		return fmt.Errorf("create %s: %w", binDir, err)
	}

	client := &http.Client{Timeout: httpTimeout}

	for _, h := range helpers() {
		dest := filepath.Join(binDir, h.output)

		if matchesChecksum(dest, h.sha256) {
			fmt.Fprintf(os.Stderr, "up to date: %s\n", h.output)
			continue
		}

		fmt.Fprintf(os.Stderr, "downloading: %s\n", h.output)
		if err := fetch(client, h, dest); err != nil {
			return fmt.Errorf("%s: %w", h.output, err)
		}
	}

	fmt.Fprintf(os.Stderr, "rootless helpers ready in %s\n", binDir)
	return nil
}

// fetch downloads a helper, extracts it if needed, verifies its checksum and
// atomically writes it to dest with executable permissions.
func fetch(client *http.Client, h helper, dest string) error {
	resp, err := client.Get(h.url)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %s for %s", resp.Status, h.url)
	}

	var src io.Reader = resp.Body
	if h.archiveMember != "" {
		member, cleanup, err := extractMember(resp.Body, h.archiveMember)
		if err != nil {
			return err
		}
		defer cleanup()
		src = member
	}

	return writeVerified(dest, src, h.sha256)
}

// extractMember reads a gzipped tarball and returns a reader positioned at the
// named member.
func extractMember(r io.Reader, name string) (io.Reader, func(), error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, nil, fmt.Errorf("gzip: %w", err)
	}

	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			_ = gz.Close()
			return nil, nil, fmt.Errorf("tar: %w", err)
		}

		if filepath.Base(header.Name) == name {
			return tr, func() { _ = gz.Close() }, nil
		}
	}

	_ = gz.Close()
	return nil, nil, fmt.Errorf("member %q not found in archive", name)
}

// writeVerified streams src to a temp file, checks its SHA256 against want and,
// on success, renames it into place with executable permissions.
func writeVerified(dest string, src io.Reader, want string) error {
	tmp, err := os.CreateTemp(filepath.Dir(dest), ".download-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
	}()

	hasher := sha256.New()
	if _, err := io.Copy(io.MultiWriter(tmp, hasher), src); err != nil {
		return err
	}

	if got := hex.EncodeToString(hasher.Sum(nil)); got != want {
		return fmt.Errorf("checksum mismatch: got %s, want %s", got, want)
	}

	if err := tmp.Close(); err != nil {
		return err
	}
	//nolint:gosec // the rootless helper must be executable
	if err := os.Chmod(tmpName, 0o750); err != nil {
		return err
	}

	return os.Rename(tmpName, dest)
}

// matchesChecksum reports whether the file at path already has the expected
// SHA256 checksum.
func matchesChecksum(path, want string) bool {
	file, err := os.Open(path) //nolint:gosec // path is a build-time constant under bin/
	if err != nil {
		return false
	}
	defer func() { _ = file.Close() }()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return false
	}

	return hex.EncodeToString(hasher.Sum(nil)) == want
}
