//go:build !linux

package main

// The rootless networking helpers (rootlesskit, slirp4netns) are only embedded
// on linux builds, where the provider actually runs containers. On other
// platforms these are empty so the binary still compiles for development and
// tooling (e.g. generating provider.yaml).
var (
	rootlesskit []byte
	slirp4netns []byte
)
