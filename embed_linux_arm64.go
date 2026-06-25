//go:build linux && arm64

package main

import _ "embed"

//go:embed bin/rootlesskit-linux-arm64
var rootlesskit []byte

//go:embed bin/slirp4netns-linux-arm64
var slirp4netns []byte
