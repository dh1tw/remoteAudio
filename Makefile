PKG := github.com/dh1tw/remoteAudio
COMMITID := $(shell git describe --always --long --dirty)
COMMIT := $(shell git rev-parse --short HEAD)
VERSION := $(shell git describe --tags)

PKG_LIST := $(shell go list ${PKG}/... | grep -v /vendor/)
GO_FILES := $(shell find . -name '*.go' | grep -v /vendor/)
all: build

build:
	protoc --proto_path=./icd --gofast_out=./sb_audio ./icd/audio.proto
	cd webserver; \
	rice embed-go 
	go build -v -ldflags="-X github.com/dh1tw/remoteAudio/cmd.commitHash=${COMMIT} \
		-X github.com/dh1tw/remoteAudio/cmd.version=${VERSION}"

# strip off dwraf table - used for travis CI
dist: 
	protoc --proto_path=./icd --gofast_out=./sb_audio ./icd/audio.proto
	cd webserver; \
	rice embed-go 
	go build -v -ldflags="-w -X github.com/dh1tw/remoteAudio/cmd.commitHash=${COMMIT} \
		-X github.com/dh1tw/remoteAudio/cmd.version=${VERSION}"

# test:
# 	@go test -short ${PKG_LIST}

vet:
	@go vet ${PKG_LIST}

lint:
	@for file in ${GO_FILES} ;  do \
		golint $$file ; \
	done

install: 
	protoc --proto_path=./icd --gofast_out=./sb_audio ./icd/audio.proto
	cd webserver; \
	rice embed-go 
	go install -v -ldflags="-w -X github.com/dh1tw/remoteAudio/cmd.commitHash=${COMMIT} \
		-X github.com/dh1tw/remoteAudio/cmd.version=${VERSION}"

# static: vet lint
# 	go build -i -v -o ${OUT}-v${VERSION} -tags netgo -ldflags="-extldflags \"-static\" -w -s -X main.version=${VERSION}" ${PKG}

client: build
	./remoteAudio client mqtt

server: build
	./remoteAudio server mqtt

clean:
	-@rm remoteAudio remoteAudio-v*

.PHONY: build client server install vet lint clean