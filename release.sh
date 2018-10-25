#!/bin/bash -e

go_version=1.10.3

docker run \
  -i -t --rm \
  -v $(pwd):/go/$(realpath --relative-to=$(go env GOPATH) $(git rev-parse --show-toplevel)) \
  -w /go/$(realpath --relative-to=$(go env GOPATH) $(pwd)) \
  golang:${go_version} \
  ./build.sh


