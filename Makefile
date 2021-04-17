#!/bin/bash

PKG := github.com/dh1tw/remoteAudio
COMMITID := $(shell git describe --always --long --dirty)
COMMIT := $(shell git rev-parse --short HEAD)
VERSION := $(shell git describe --tags --always)

PKG_LIST := $(shell go list ${PKG}/... | grep -v /vendor/)
GO_FILES := $(shell find . -name '*.go' | grep -v /vendor/)
all: build

build:
	protoc --proto_path=./icd --micro_out=. --go_out=. audio.proto	cd webserver; \
	rice embed-go
	go build -v -ldflags="-X github.com/dh1tw/remoteAudio/cmd.commitHash=${COMMIT} \
		-X github.com/dh1tw/remoteAudio/cmd.version=${VERSION}"

# strip off dwraf table - used for travis CI
dist:
	protoc --proto_path=./icd --micro_out=. --go_out=. audio.proto
	cd webserver; \
	rice embed-go
	go build -v -ldflags="-w -s -X github.com/dh1tw/remoteAudio/cmd.commitHash=${COMMIT} \
		-X github.com/dh1tw/remoteAudio/cmd.version=${VERSION}"
	@if [ ${GOOS} = "windows" ]; \
		then upx ./remoteAudio.exe; \
	else \
		upx ./remoteAudio; \
	fi

# test:
# 	@go test -short ${PKG_LIST}

vet:
	@go vet ${PKG_LIST}

lint:
	@for file in ${GO_FILES} ;  do \
		golint $$file ; \
	done

install:
	protoc --proto_path=./icd --micro_out=. --go_out=. audio.proto
	cd webserver; \
	rice embed-go
	go install -v -ldflags="-w -X github.com/dh1tw/remoteAudio/cmd.commitHash=${COMMIT} \
		-X github.com/dh1tw/remoteAudio/cmd.version=${VERSION}"

install-deps:
	go get github.com/golang/protobuf/protoc-gen-go
	go get github.com/GeertJohan/go.rice/rice
	go get github.com/asim/go-micro/cmd/protoc-gen-micro/v3

# static: vet lint
# 	go build -i -v -o ${OUT}-v${VERSION} -tags netgo -ldflags="-extldflags \"-static\" -w -s -X main.version=${VERSION}" ${PKG}

clean:
	-@rm remoteAudio remoteAudio-v*

.PHONY: build install vet lint clean install-deps