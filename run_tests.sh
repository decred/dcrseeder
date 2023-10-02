#!/bin/bash
# The script does the following automatic checking on a Go package and its sub-packages:
# 1. go mod tidiness
# 2. unit tests
# 3. linting (github.com/golangci/golangci-lint)

set -ex

go version

# run tests
env GORACE="halt_on_error=1" go test -race ./...

# golangci-lint (github.com/golangci/golangci-lint) is used to run each each
# static checker.

# set output format
if [[ -n $CI ]]; then
    OUT_FORMAT="github-actions"
else
    OUT_FORMAT="colored-line-number"
fi

# run linter
golangci-lint run --disable-all --deadline=10m \
  --out-format=$OUT_FORMAT \
  --enable=gofmt \
  --enable=revive \
  --enable=govet \
  --enable=gosimple \
  --enable=unconvert \
  --enable=ineffassign \
  --enable=goimports \
  --enable=misspell \
  --enable=unparam \
  --enable=unused \
  --enable=errcheck \
  --enable=asciicheck \
  --enable=noctx

