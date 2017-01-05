#!/usr/bin/env bash
# Create archives containing the crkr binary for the target platforms. Don't
# run on cygwin, since it's difficult to get the extension right for the
# archives for windows.
set -e

function build_all {
    local version=${1:?No version given}; shift
    for os in darwin windows linux; do
        for arch in 386 amd64; do
            build $version $os $arch
        done
    done
}

function build {
    local version=${1:?No version given}; shift
    local os=${1:?No os given}; shift
    local arch=${1:?No arch given}; shift
    GOOS=$os GOARCH=$arch go build -ldflags "-X main.version=$version" ./cmd/crkr
    local bin=crkr
    if test $os = windows; then
        bin=${bin}.exe
    fi
    zip -q crkr-v$version-$os-$arch.zip $bin
    rm $bin
}

build_all "$@"
