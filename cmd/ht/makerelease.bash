#! /bin/bash

set -e

version=$(git describe)
GO15VENDOREXPERIMENT="1" go build -v -ldflags "-X main.version=$version"
# Pack binaries with github.com/pwaller/goupx
goupx --strip-binary ht
./ht version
