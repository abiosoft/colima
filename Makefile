GOOS ?= $(shell uname | tr '[:upper:]' '[:lower:]')

ARCH ?= $(shell uname -m)
GOARCH_x86_64 = amd64
GOARCH_aarch64 = arm64
GOARCH ?= $(shell echo "$(GOARCH_$(ARCH))")

all: release

clean:
	rm -rf _output

gopath:
	go get -v ./cmd/colima

fmt:
	go fmt ./...

release:
	GOOS=$(GOOS) GOARCH=$(GOARCH) sh scripts/release.sh ${VERSION}

gh_release: gh_release-$(GOOS)-$(GOARCH)

gh_release-$(GOOS)-$(GOARCH):
	GOOS=$(GOOS) GOARCH=$(GOARCH) GITHUB=1 sh scripts/release.sh ${VERSION} -F CHANGELOG.md

install:
	cp _output/binaries/colima-$(GOOS)-$(GOARCH) /usr/local/bin/colima
	chmod +x /usr/local/bin/colima
