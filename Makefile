
OS ?= $(shell uname)
ARCH ?= $(shell uname -m)

GOOS ?= $(shell echo "$(OS)" | tr '[:upper:]' '[:lower:]')
GOARCH_x86_64 = amd64
GOARCH_aarch64 = arm64
GOARCH_arm64 = arm64
GOARCH ?= $(shell echo "$(GOARCH_$(ARCH))")

VERSION := $(shell git describe --tags --always)
REVISION := $(shell git rev-parse HEAD)
PACKAGE := github.com/abiosoft/colima/config
VERSION_VARIABLES := -X $(PACKAGE).appVersion=$(VERSION) -X $(PACKAGE).revision=$(REVISION)

OUTPUT_DIR := _output/binaries
OUTPUT_BIN := colima-$(OS)-$(ARCH)
INSTALL_DIR := /usr/local/bin
BIN_NAME := colima

LDFLAGS := $(VERSION_VARIABLES)

.PHONY: all
all: build

.PHONY: clean
clean:
	rm -rf _output _build

.PHONY: gopath
gopath:
	go get -v ./cmd/colima

.PHONY: fmt
fmt:
	go fmt ./...
	goimports -w .

.PHONY: build
build:
	GOOS=$(GOOS) GOARCH=$(GOARCH) go build -ldflags="$(LDFLAGS)" -o $(OUTPUT_DIR)/$(OUTPUT_BIN) ./cmd/colima
ifeq ($(GOOS),darwin)
	codesign -s - $(OUTPUT_DIR)/$(OUTPUT_BIN)
endif
	cd $(OUTPUT_DIR) && openssl sha256 -r -out $(OUTPUT_BIN).sha256sum $(OUTPUT_BIN)

.PHONY: test
test:
	go test -v -ldflags="$(LD_FLAGS)" ./...

.PHONY: vmnet
vmnet:
	sh scripts/build_vmnet.sh

.PHONY: install
install:
	mkdir -p $(INSTALL_DIR)
	rm -f $(INSTALL_DIR)/$(BIN_NAME)
	cp $(OUTPUT_DIR)/colima-$(OS)-$(ARCH) $(INSTALL_DIR)/$(BIN_NAME)
	chmod +x $(INSTALL_DIR)/$(BIN_NAME)

.PHONY: lint
lint: ## Assumes that golangci-lint is installed and in the path.  To install: https://golangci-lint.run/usage/install/
	golangci-lint --timeout 3m run

.PHONY: print-binary-name
print-binary-name:
	@echo $(OUTPUT_DIR)/$(OUTPUT_BIN)

.PHONY: nix-derivation-shell
nix-derivation-shell:
	$(eval DERIVATION=$(shell nix-build))
	echo $(DERIVATION) | grep ^/nix
	nix-shell -p $(DERIVATION)

.PHONY: integration
integration: build
	GOARCH=$(GOARCH) COLIMA_BINARY=$(OUTPUT_DIR)/$(OUTPUT_BIN) scripts/integration.sh

.PHONY: images-sha
images-sha:
	bash embedded/images/images_sha.sh
