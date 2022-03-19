
OS ?= $(shell uname)
ARCH ?= $(shell uname -m)

GOOS ?= $(shell echo "$(OS)" | tr '[:upper:]' '[:lower:]')
GOARCH_x86_64 = amd64
GOARCH_aarch64 = arm64
GOARCH_arm64 = arm64
GOARCH ?= $(shell echo "$(GOARCH_$(ARCH))")

all: build 

clean:
	rm -rf _output _build

gopath:
	go get -v ./cmd/colima

fmt:
	go fmt ./...

build:
	GOOS=$(GOOS) GOARCH=$(GOARCH) OS=$(OS) ARCH=$(ARCH) sh scripts/build.sh

vmnet:
	sh scripts/build_vmnet.sh

install:
	cp _output/binaries/colima-$(OS)-$(ARCH) /usr/local/bin/colima
	chmod +x /usr/local/bin/colima

.PHONY: lint
lint: ## Assumes that golangci-lint is installed and in the path.  To install: https://golangci-lint.run/usage/install/
	golangci-lint --timeout 3m run
