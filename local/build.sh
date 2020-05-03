#!/usr/bin/env bash

ROOT="/Users/liufeihao/goWorkspace/src/github.com/l-f-h/video"
BIN="local"
OUTPUT=${ROOT}/local/output/

mkdir -p ${OUTPUT}
cp ${ROOT}/demo.mp4 ./demo.h264 ${OUTPUT}

export GO111MODULE=on
go build -o ${OUTPUT}/${BIN}