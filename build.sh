#!/bin/bash -e

package=$(basename $(pwd))

go install tawesoft.co.uk/gopkg/gocomply@latest
gocomply > cmd/credits/CREDITS

go install github.com/mitchellh/gox@master
mkdir -p build
gox -arch "amd64 arm64" -os "linux darwin windows" -ldflags "-s -w" -output "build/${package}_{{.OS}}_{{.Arch}}/${package}"

cd build
for dir in $(find -maxdepth 1 -type d -name "${package}*"); do
   tar czf $dir.tar.gz $dir
done
