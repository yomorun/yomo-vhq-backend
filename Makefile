GO ?= go
GOFMT ?= gofmt "-s"
GOFILES := $(shell find . -name "*.go")
VETPACKAGES ?= $(shell $(GO) list ./... | grep -v /examples/)
VER ?= $(shell cat VERSION)

.PHONY: fmt
fmt:
	$(GOFMT) -w $(GOFILES)

vet:
	$(GO) vet $(VETPACKAGES)

build:
	$(GO) build -o bin/vhqd -ldflags "-s -w ${GO_LDFLAGS}" ./cmd/main.go

build-arm:
	GOARCH=arm64 GOOS=linux $(GO) build -o bin/vhqd-aarch64-linux -ldflags "-s -w ${GO_LDFLAGS}" ./cmd/main.go

build-linux:
	GOOS=linux $(GO) build -o bin/vhqd-x86_64-linux -ldflags "-s -w ${GO_LDFLAGS}" ./cmd/main.go
