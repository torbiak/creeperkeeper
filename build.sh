#!/usr/bin/env bash

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
    zip -q crkr-v$version-$os-$arch.zip crkr
    rm crkr
}

build_all "$@"
