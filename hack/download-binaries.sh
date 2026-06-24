#!/usr/bin/env bash
# Downloads the rootless networking helpers (rootlesskit, slirp4netns) for every
# supported linux architecture into ./bin so they can be embedded into the
# provider binary by goreleaser. Invoked from .goreleaser.yml's before hook.
set -euo pipefail

ROOTLESSKIT_VERSION="${ROOTLESSKIT_VERSION:-1.1.1}"
SLIRP4NETNS_VERSION="${SLIRP4NETNS_VERSION:-1.2.2}"

REPO_ROOT="$(git rev-parse --show-toplevel)"
BIN_DIR="${REPO_ROOT}/bin"
mkdir -p "${BIN_DIR}"

# Maps go arch -> upstream release arch suffix.
declare -A ARCHES=(
    ["amd64"]="x86_64"
    ["arm64"]="aarch64"
)

for goarch in "${!ARCHES[@]}"; do
    upstream="${ARCHES[$goarch]}"

    rootlesskit_out="${BIN_DIR}/rootlesskit-linux-${goarch}"
    slirp4netns_out="${BIN_DIR}/slirp4netns-linux-${goarch}"

    if [[ ! -f "${rootlesskit_out}" ]]; then
        echo "Downloading rootlesskit ${ROOTLESSKIT_VERSION} (${goarch})"
        tmp="$(mktemp -d)"
        curl -fsSL \
            "https://github.com/rootless-containers/rootlesskit/releases/download/v${ROOTLESSKIT_VERSION}/rootlesskit-${upstream}.tar.gz" \
            -o "${tmp}/rootlesskit.tar.gz"
        tar -C "${tmp}" -zxf "${tmp}/rootlesskit.tar.gz" rootlesskit
        mv "${tmp}/rootlesskit" "${rootlesskit_out}"
        chmod +x "${rootlesskit_out}"
        rm -rf "${tmp}"
    fi

    if [[ ! -f "${slirp4netns_out}" ]]; then
        echo "Downloading slirp4netns ${SLIRP4NETNS_VERSION} (${goarch})"
        curl -fsSL \
            "https://github.com/rootless-containers/slirp4netns/releases/download/v${SLIRP4NETNS_VERSION}/slirp4netns-${upstream}" \
            -o "${slirp4netns_out}"
        chmod +x "${slirp4netns_out}"
    fi
done

echo "Rootless helpers ready in ${BIN_DIR}"
