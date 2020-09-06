#!/bin/bash

gitroot=$(git rev-parse --show-toplevel)

grpcurl -insecure \
  -import-path ${gitroot}/api \
  -import-path /usr/local/Cellar/protobuf/3.13.0/include \
  -import-path ${gitroot}/vendor \
  -import-path ${gitroot}/vendor/github.com/gogo/protobuf \
  -import-path ${gitroot}/vendor/github.com/gogo/googleapis \
  -import-path ${gitroot}/vendor/github.com/grpc-ecosystem/grpc-gateway \
  -emit-defaults \
  -proto controller.proto localhost:10000 "$@" | jq .
