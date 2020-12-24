#!/bin/bash
# The script does the following automatic checking on a Go package and its sub-packages:
# 1. go mod tidiness
# 2. unit tests
# 3. linting (github.com/golangci/golangci-lint)

set -ex

go version

# run `go mod download` and `go mod tidy` and fail if the git status of
# go.mod and/or go.sum changes
MOD_STATUS=$(git status --porcelain go.mod go.sum)
go mod download
go mod tidy
UPDATED_MOD_STATUS=$(git status --porcelain go.mod go.sum)
if [ "$UPDATED_MOD_STATUS" != "$MOD_STATUS" ]; then
    echo "Running `go mod tidy` modified go.mod and/or go.sum"
    exit 1
fi

# run tests
env GORACE="halt_on_error=1" go test -race ./...

# golangci-lint (github.com/golangci/golangci-lint) is used to run each each
# static checker.

# set output format
if [[ -v CI ]]; then
    OUT_FORMAT="github-actions"
else
    OUT_FORMAT="colored-line-number"
fi

# run linter
golangci-lint run --disable-all --deadline=10m \
  --out-format=$OUT_FORMAT \
  --enable=gofmt \
  --enable=golint \
  --enable=govet \
  --enable=gosimple \
  --enable=unconvert \
  --enable=ineffassign \
  --enable=structcheck \
  --enable=goimports \
  --enable=misspell \
  --enable=unparam \
  --enable=deadcode \
  --enable=unused \
  --enable=errcheck \
  --enable=asciicheck
  