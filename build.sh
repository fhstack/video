#!/usr/bin/env bash

BIN="video"

mkdir -p output/
cp ./demo.mp4 output/

export GO111MODULE=on
go build -o output/${BIN}