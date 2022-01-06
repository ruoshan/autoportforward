#!/bin/bash

set -xe

# Build a static agent binary
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build --ldflags "-s" ./cmd/apf-agent

if command -v upx; then
    upx apf-agent
fi
if [[ "$OSTYPE" == "darwin"* ]]; then
    tar -c --uid 0 --uname root --gid 0 --gname root -f agent.tar apf-agent
else
    tar -c --group=root:0 --owner=root:0 -f agent.tar apf-agent
fi
mv agent.tar bootstrap
rm -f apf-agent

go build -ldflags="-X main.version=${VERSION:-dev}" ./cmd/apf
