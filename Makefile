GOOS ?= $(shell uname | tr '[:upper:]' '[:lower:]')

ARCH ?= $(shell uname -m)
GOARCH_x86_64 = amd64
GOARCH_aarch64 = arm64
GOARCH ?= $(shell echo "$(GOARCH_$(ARCH))")

all: build 

clean:
	rm -rf _output

gopath:
	go get -v ./cmd/colima

fmt:
	go fmt ./...

build:
	GOOS=$(GOOS) GOARCH=$(GOARCH) sh scripts/build.sh

install:
	cp _output/binaries/colima-$(GOOS)-$(GOARCH) /usr/local/bin/colima
	chmod +x /usr/local/bin/colima
