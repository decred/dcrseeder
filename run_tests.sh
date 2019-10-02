#!/usr/bin/env bash

# usage:
# ./run_tests.sh                         # local, go 1.12
# GOVERSION=1.11 ./run_tests.sh          # local, go 1.11

set -ex

[[ ! "$GOVERSION" ]] && GOVERSION=1.12
REPO=dcrseeder

go version
env CC=gcc GOTESTFLAGS='-short' go test -v
