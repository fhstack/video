#!/usr/bin/env bash

ROOT="/Users/liufeihao/goWorkspace/src/github.com/l-f-h/video/net"

mkdir -p output

cd ${ROOT}/client
go build -o ${ROOT}/output/client

cd ${ROOT}/server
go build -o ${ROOT}/output/server


