package dockerless

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/devsy-org/devsy/pkg/driver"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
)

// Pull will pull a given image and save it to ImageDir.
// This function uses github.com/google/go-containerregistry/pkg/crane to pull
// the image's manifest, and performs the downloading of each layer separately.
// Each layer is deduplicated between images in order to save space, using hardlinks.
func (p *DockerlessProvider) Pull(ctx context.Context, runOptions *driver.RunOptions) error {
	// First we try to get the fully qualified uri of the image
	// eg alpine:latest -> index.docker.io/library/alpine:latest
	image := runOptions.Image
	ref, err := name.ParseReference(image)
	if err == nil {
		image = ref.Name()
	}

	targetDIR := filepath.Join(p.Config.TargetDir, "images", ref.Name())

	p.Log.Infof("downloading %s", ref.Name())
	// if we already downloaded the image, exit
	if _, err := os.Stat(filepath.Join(targetDIR, "manifest.json")); err == nil {
		p.Log.Infof("image %s already found", ref.Name())

		return nil
	}

	p.Log.Debugf("getting info about %s", ref.Name())
	// Pull will just get us the v1.Image struct, from
	// which we get all the information we need
	imageManifest, err := crane.Pull(image)
	if err != nil {
		return err
	}

	if err := p.downloadLayers(targetDIR, imageManifest); err != nil {
		return err
	}

	return p.saveImageMetadata(targetDIR, image, imageManifest)
}

// downloadLayers downloads every layer of the image into targetDIR and removes
// any stale files left over from previous downloads.
func (p *DockerlessProvider) downloadLayers(targetDIR string, imageManifest v1.Image) error {
	p.Log.Debugf("preparing to get layers")
	layers, err := imageManifest.Layers()
	if err != nil {
		return err
	}

	// Prepare the image path.
	if !Exist(targetDIR) {
		if err := os.MkdirAll(targetDIR, 0o750); err != nil {
			return err
		}
	}

	p.Log.Infof("downloading layers")
	keepFiles := []string{}
	for index, layer := range layers {
		p.Log.Infof("downloading layer %d of %d", index+1, len(layers))

		fileName, err := downloadLayer(targetDIR, layer)
		if err != nil {
			return err
		}

		keepFiles = append(keepFiles, fileName)
	}

	return cleanupImageDir(targetDIR, keepFiles)
}

// cleanupImageDir removes every file in targetDIR that is not part of keepFiles.
func cleanupImageDir(targetDIR string, keepFiles []string) error {
	fileList, err := os.ReadDir(targetDIR)
	if err != nil {
		return err
	}

	keep := strings.Join(keepFiles, ":")
	for _, file := range fileList {
		if !strings.Contains(keep, filepath.Base(file.Name())) {
			if err := os.Remove(filepath.Join(targetDIR, file.Name())); err != nil {
				return err
			}
		}
	}

	return nil
}

// saveImageMetadata persists the manifest, config and fully qualified image
// name for later use.
func (p *DockerlessProvider) saveImageMetadata(
	targetDIR, image string,
	imageManifest v1.Image,
) error {
	p.Log.Debugf("saving manifest.json")
	// we save the manifest.json for later use. This contains
	// the information on how the layers are ordered and
	// how to unpack them
	rawManifest, err := imageManifest.RawManifest()
	if err != nil {
		return err
	}

	if err := os.WriteFile(
		filepath.Join(targetDIR, "manifest.json"),
		rawManifest,
		0o600,
	); err != nil {
		return err
	}

	p.Log.Debugf("saving config.json")
	// The config.json file is also saved, indicating lots of information
	// about the image, like default env, entrypoint and so on
	rawConfig, err := imageManifest.RawConfigFile()
	if err != nil {
		return err
	}

	if err := os.WriteFile(filepath.Join(targetDIR, "config.json"), rawConfig, 0o600); err != nil {
		return err
	}

	p.Log.Debugf("saving image info")
	// We also save the fully qualified name to retrieve it later.
	return os.WriteFile(filepath.Join(targetDIR, "image_name"), []byte(image), 0o600)
}

// downloadLayer will download input layer into targetDIR.
// downloadLayer will first search existing images inside the ImageDir in order
// to find matching layers, and hardlink them in order to save disk space.
//
// Each layer download is verified in order to ensure no corrupted downloads occur.
func downloadLayer(targetDIR string, layer v1.Layer) (string, error) {
	// we use this as a path to download layers, in order to
	// verify them and ensure we do not leave broken files
	tmpdir := filepath.Join(targetDIR, ".temp")

	// always cleanup before
	_ = os.RemoveAll(tmpdir)

	if err := os.MkdirAll(tmpdir, 0o750); err != nil {
		return "", err
	}

	// and after
	defer func() { _ = os.RemoveAll(tmpdir) }()

	layerDigest, _ := layer.Digest()
	layerFileName := strings.Split(layerDigest.String(), ":")[1] + ".tar.gz"

	// If the layer already exists locally or in another image, reuse it.
	if reused := reuseExistingLayer(targetDIR, layerFileName, layerDigest.String()); reused != nil {
		return layerFileName, reused.err
	}

	// Else we proceed with the download of the layer.
	if err := writeLayer(filepath.Join(tmpdir, layerFileName), layer); err != nil {
		return "", err
	}

	// always verify if the download was correctly done by
	// checking the digest of the file
	if !CheckFileDigest(filepath.Join(tmpdir, layerFileName), layerDigest.String()) {
		return "", fmt.Errorf("error getting layer")
	}

	err := os.Rename(filepath.Join(tmpdir, layerFileName), filepath.Join(targetDIR, layerFileName))

	return layerFileName, err
}

// layerReuse signals that an existing layer was reused, carrying any error from
// the operation (e.g. creating a hardlink).
type layerReuse struct {
	err error
}

// reuseExistingLayer returns a non-nil result if the layer already exists in
// targetDIR or can be hard-linked from another image directory.
func reuseExistingLayer(targetDIR, layerFileName, digest string) *layerReuse {
	target := filepath.Join(targetDIR, layerFileName)

	// If a layer already exists and matches, reuse it as-is.
	if Exist(target) && CheckFileDigest(target, digest) {
		return &layerReuse{}
	}

	// But if a layer with the same name/digest exists in another directory
	// let's deduplicate the disk usage by using hardlinks.
	matchingLayers := findExistingLayer(filepath.Dir(targetDIR), layerFileName)
	if len(matchingLayers) > 0 && CheckFileDigest(matchingLayers[0], digest) {
		return &layerReuse{err: os.Link(matchingLayers[0], target)}
	}

	return nil
}

// writeLayer streams the compressed layer to path.
func writeLayer(path string, layer v1.Layer) error {
	//nolint:gosec // path is derived from provider config, not user input
	savedLayer, err := os.Create(path)
	if err != nil {
		return err
	}

	defer func() { _ = savedLayer.Close() }()

	tarLayer, err := layer.Compressed()
	if err != nil {
		return err
	}

	_, err = io.Copy(savedLayer, tarLayer)
	return err
}

// findExistingLayer is useful to find layers with matching name/digest in order to
// deduplicate disk usage by using hardlinks later.
func findExistingLayer(targetDIR, filename string) []string {
	var matchingFiles []string

	_ = filepath.WalkDir(targetDIR, func(name string, dirEntry fs.DirEntry, err error) error {
		if dirEntry.Name() == filename {
			matchingFiles = append(matchingFiles, name)
		}

		return nil
	})

	return matchingFiles
}
