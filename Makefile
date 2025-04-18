#!/bin/bash

COMMITID := $(shell git describe --always --long --dirty)
COMMIT := $(shell git rev-parse --short HEAD)
VERSION := $(shell git describe --tags --always)

GOOS := $(shell go env GOOS)
GOARCH := $(shell go env GOARCH)

all: build

build:
	protoc --proto_path=./icd --micro_out=. --go_out=. ./icd/audio.proto
	go build -v -ldflags="-X github.com/dh1tw/remoteAudio/cmd.commitHash=${COMMIT} \
		-X github.com/dh1tw/remoteAudio/cmd.version=${VERSION}"

# strip off dwraf table - used for travis CI
dist:
	protoc --proto_path=./icd --micro_out=. --go_out=. ./icd/audio.proto
	go build -v -ldflags="-w -s -X github.com/dh1tw/remoteAudio/cmd.commitHash=${COMMIT} \
		-X github.com/dh1tw/remoteAudio/cmd.version=${VERSION}"
	if [ "${GOOS}" = "windows" ]; \
		then upx remoteAudio.exe; \
	else \
		if [ "${GOOS}" = "darwin" ]; \
			then true; \
		else upx remoteAudio; \
		fi \
	fi

install:
	protoc --proto_path=./icd --micro_out=. --go_out=. ./icd/audio.proto
	go install -v -ldflags="-w -X github.com/dh1tw/remoteAudio/cmd.commitHash=${COMMIT} \
		-X github.com/dh1tw/remoteAudio/cmd.version=${VERSION}"

install-deps:
	go mod download
	go install github.com/golang/protobuf/protoc-gen-go@v1.5.2
	go install github.com/asim/go-micro/cmd/protoc-gen-micro/v3@v3.7.0

clean:
	-@rm remoteAudio remoteAudio-v*

.PHONY: build install vet lint clean install-deps