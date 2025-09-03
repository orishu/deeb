#!/bin/bash

docker run --rm -v $PWD:/src -w /src --platform=linux/arm64 golang:1.21 \
  sh -c 'apt-get update && apt-get install -y gcc libc6-dev && \
         GOARCH=arm64 GOOS=linux CGO_ENABLED=1 go build ./cmd/controller'

