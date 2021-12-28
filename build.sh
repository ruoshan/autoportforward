#!/bin/bash

set -xe

GOOS=linux go build --ldflags "-s" ./cmd/apf-agent
if command -v upx; then
    upx apf-agent
fi
tar -c --uid 0 --uname root --gid 0 --gname root -f agent.tar apf-agent
mv agent.tar bootstrap
rm -f apf-agent

go build ./cmd/apf

