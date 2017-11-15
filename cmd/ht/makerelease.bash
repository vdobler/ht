#! /bin/bash

set -e

# Build using a released version of the compiler.
GO=/usr/local/go/bin/go
echo "### Using $($GO version)"

echo
echo "### Generating data"
$GO generate -x 

echo
echo "### Build"
$GO build

echo
echo "### Smoketests"
./test-all.bash

version=$(git describe)
rm -f ht*

LDFLAGS="-X main.version=$version -s -w"

echo
echo "### Linux version"
GOOS=linux GOARCH=amd64 $GO build -o ht_linux -ldflags "$LDFLAGS"
# Pack binaries with goupx (github.com/pwaller/goupx) which
# uses upx (http://upx.sourceforge.net)
goupx --strip-binary ht_linux

echo
echo "### Mac OS X version"
GOOS=darwin GOARCH=amd64 $GO build -o ht_darwin -ldflags "$LDFLAGS"

echo
echo "### Windows version"
GOOS=windows GOARCH=amd64 $GO build -o ht_windows.exe -ldflags "$LDFLAGS"

echo
echo "### Documentation"
(cd ../../ht/; ./generate-doc.bash;)
mv ../../ht/Documentation.{html,pdf} .
ls -l Documentation.{html,pdf}

source <($GO env)
echo
echo "Successfully built $(./ht_$GOHOSTOS version)"
echo "Source $(grep -F "version = " version.go)"

if ! git diff-index --quiet HEAD --; then
    echo
    echo "Uncommited files present. Should not release."
fi

