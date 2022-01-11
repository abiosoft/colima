
OS ?= $(shell uname)
ARCH ?= $(shell uname -m)

GOOS ?= $(shell echo "$(OS)" | tr '[:upper:]' '[:lower:]')
GOARCH_x86_64 = amd64
GOARCH_aarch64 = arm64
GOARCH_arm64 = arm64
GOARCH ?= $(shell echo "$(GOARCH_$(ARCH))")
GOLANGCI_LINT_VER = "1.43.0"

all: build 

clean:
	rm -rf _output

gopath:
	go get -v ./cmd/colima

fmt:
	go fmt ./...

build:
	GOOS=$(GOOS) GOARCH=$(GOARCH) OS=$(OS) ARCH=$(ARCH) sh scripts/build.sh

install:
	cp _output/binaries/colima-$(OS)-$(ARCH) /usr/local/bin/colima
	chmod +x /usr/local/bin/colima

.PHONY: lint
lint: ## Run golangci-lint
ifneq (${GOLANGCI_LINT_VER}, "$(shell ./bin/golangci-lint version --format short 2>&1)")
	@echo "golangci-lint missing or not version '${GOLANGCI_LINT_VER}', downloading..."
	curl -sSfL "https://raw.githubusercontent.com/golangci/golangci-lint/v${GOLANGCI_LINT_VER}/install.sh" | sh -s -- -b ./bin "v${GOLANGCI_LINT_VER}"
endif
	./bin/golangci-lint --timeout 3m run
