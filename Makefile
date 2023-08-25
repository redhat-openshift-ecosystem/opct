# Ensure go modules are enabled:
export GO111MODULE=on

# Disable CGO so that we always generate static binaries:
export CGO_ENABLED=0

BUILD_DIR ?= $(PWD)/build
IMG ?= quay.io/ocp-cert/opct
VERSION=$(shell git rev-parse --short HEAD)
RELEASE_TAG ?= 0.0.0
BIN_NAME ?= opct

GO_BUILD_FLAGS := -ldflags '-s -w -X github.com/redhat-openshift-ecosystem/provider-certification-tool/pkg/version.commit=$(VERSION) -X github.com/redhat-openshift-ecosystem/provider-certification-tool/pkg/version.version=$(RELEASE_TAG)'
GOOS ?= linux
GOARCH ?= amd64

# Unset GOFLAG for CI and ensure we've got nothing accidently set
unexport GOFLAGS

.PHONY: all
all: build-linux-amd64
all: build-windows-amd64
all: build-darwin-amd64
all: build-darwin-arm64

.PHONY: build-dep
build-dep:
	@mkdir -p $(BUILD_DIR)

.PHONY: build
build: build-dep
	GOOS=$(GOOS) GOARCH=$(GOARCH) go build -o $(BUILD_DIR)/opct-$(GOOS)-$(GOARCH)$(GOEXT) $(GO_BUILD_FLAGS)
	@cd $(BUILD_DIR); md5sum $(BUILD_DIR)/opct-$(GOOS)-$(GOARCH)$(GOEXT) > $(BUILD_DIR)/opct-$(GOOS)-$(GOARCH)$(GOEXT).sum; cd -

.PHONY: build-linux-amd64
build-linux-amd64: GOOS = linux
build-linux-amd64: GOARCH = amd64
build-linux-amd64: build

.PHONY: build-windows-amd64
build-windows-amd64: GOOS = windows
build-windows-amd64: GOARCH = amd64
build-windows-amd64: GOEXT = .exe
build-windows-amd64: build

.PHONY: build-darwin-amd64
build-darwin-amd64: GOOS = darwin
build-darwin-amd64: GOARCH = amd64
build-darwin-amd64: build

.PHONY: build-darwin-arm64
build-darwin-arm64: GOOS = darwin
build-darwin-arm64: GOARCH = arm64
build-darwin-arm64: build

.PHONY: linux-amd64-container
linux-amd64-container: build-linux-amd64
	podman build -t $(IMG):latest -f hack/Containerfile --build-arg=RELEASE_TAG=$(RELEASE_TAG) .

# Utils dev
.PHONY: update-go
update-go:
	go get -u
	go mod tidy

.PHONY: test
test:
	go test ./...

.PHONY: vet
vet:
	go vet ./...

.PHONY: clean
clean:
	rm -rvf ./build/ ./openshift-provider-cert-* ./opct-*
