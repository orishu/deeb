#!/bin/bash

root=$(git rev-parse --show-toplevel)

cd ${root}/api

protoc --gofast_out=plugins=grpc:. controller.proto

cd - >/dev/null 2>&1
