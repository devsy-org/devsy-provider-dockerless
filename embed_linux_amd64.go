//go:build linux && amd64

package main

import _ "embed"

//go:embed bin/rootlesskit-linux-amd64
var rootlesskit []byte

//go:embed bin/slirp4netns-linux-amd64
var slirp4netns []byte
