#!/bin/sh
# Install the latest mctop release. Usage:
#   curl -fsSL https://raw.githubusercontent.com/aloki-alok/mctop/main/install.sh | sh
# Override the install directory with MCTOP_INSTALL_DIR (default /usr/local/bin,
# falling back to $HOME/.local/bin when that is not writable).
set -eu

REPO="aloki-alok/mctop"

err() {
	echo "install: $1" >&2
	exit 1
}

need() {
	command -v "$1" >/dev/null 2>&1 || err "missing required command: $1"
}

need uname
need tar
if command -v curl >/dev/null 2>&1; then
	fetch() { curl -fsSL "$1" -o "$2"; }
elif command -v wget >/dev/null 2>&1; then
	fetch() { wget -qO "$2" "$1"; }
else
	err "need curl or wget"
fi

os=$(uname -s)
case "$os" in
	Linux) os=linux ;;
	Darwin) os=darwin ;;
	*) err "unsupported OS: $os (use the prebuilt binaries on GitHub Releases)" ;;
esac

arch=$(uname -m)
case "$arch" in
	x86_64 | amd64) arch=amd64 ;;
	aarch64 | arm64) arch=arm64 ;;
	*) err "unsupported architecture: $arch" ;;
esac

archive="mctop_${os}_${arch}.tar.gz"
base="https://github.com/${REPO}/releases/latest/download"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

echo "install: downloading $archive"
fetch "${base}/${archive}" "${tmp}/${archive}" || err "download failed; no release for ${os}/${arch} yet?"

echo "install: verifying checksum"
fetch "${base}/checksums.txt" "${tmp}/checksums.txt" || err "could not fetch checksums"
if command -v sha256sum >/dev/null 2>&1; then
	sum=$(sha256sum "${tmp}/${archive}" | awk '{print $1}')
elif command -v shasum >/dev/null 2>&1; then
	sum=$(shasum -a 256 "${tmp}/${archive}" | awk '{print $1}')
else
	err "need sha256sum or shasum to verify the download"
fi
grep -q "$sum" "${tmp}/checksums.txt" || err "checksum mismatch, refusing to install"

tar -xzf "${tmp}/${archive}" -C "$tmp" mctop || err "could not extract mctop"

if [ -n "${MCTOP_INSTALL_DIR:-}" ]; then
	dir="$MCTOP_INSTALL_DIR"
	mkdir -p "$dir" || err "cannot create $dir"
else
	dir="/usr/local/bin"
	if [ ! -w "$dir" ]; then
		dir="${HOME}/.local/bin"
		mkdir -p "$dir"
	fi
fi
install -m 0755 "${tmp}/mctop" "${dir}/mctop"

echo "install: mctop installed to ${dir}/mctop"
case ":${PATH}:" in
	*":${dir}:"*) ;;
	*) echo "install: add ${dir} to your PATH to run mctop" ;;
esac
