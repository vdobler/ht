#! /bin/bash

set -e

version=$(git describe)
GO15VENDOREXPERIMENT="1" go build -v -ldflags "-X main.version=$version"
# Pack binaries with github.com/pwaller/goupx
goupx --strip-binary ht
(cd ../../ht/; ./list-checks.bash;) > CheckDocumentation.txt
./ht version

