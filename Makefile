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
all: linux-amd64-container build-windows-amd64 build-darwin-amd64 build-darwin-arm64

.PHONY: build-dep
build-dep:
	@mkdir -p $(BUILD_DIR)

.PHONY: build
build: build-dep
	GOOS=$(GOOS) GOARCH=$(GOARCH) go build -o $(BUILD_DIR)/opct-$(GOOS)-$(GOARCH) $(GO_BUILD_FLAGS)
	@cd $(BUILD_DIR); md5sum $(BUILD_DIR)/opct-$(GOOS)-$(GOARCH) > $(BUILD_DIR)/opct-$(GOOS)-$(GOARCH).sum; cd -

.PHONY: build-linux-amd64
build-linux-amd64: GOOS = linux
build-linux-amd64: GOARCH = amd64
build-linux-amd64: build

.PHONY: build-windows-amd64
build-windows-amd64: build-dep
	GOOS=windows GOARCH=amd64 go build -o $(BUILD_DIR)/opct-windows.exe $(GO_BUILD_FLAGS)
	@cd $(BUILD_DIR); md5sum $(BUILD_DIR)/opct-windows-amd64 > $(BUILD_DIR)/opct-windows-amd64.sum; cd -

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

# Publish devel binaries (non-official). Must be used only for troubleshooting in development/support.
.PHONY: publish-amd64-devel
publish-amd64-devel: build-linux-amd64
	aws s3 cp $(BUILD_DIR)/opct-linux-amd64 s3://openshift-provider-certification/bin/opct-linux-amd64-devel

.PHONY: publish-darwin-arm64-devel
publish-darwin-arm64-devel: build-darwin-arm64
	aws s3 cp $(BUILD_DIR)/opct-darwin-arm64 s3://openshift-provider-certification/bin/opct-darwin-arm64-devel

.PHONY: publish-devel
publish-devel: publish-amd64-devel
publish-devel: publish-darwin-arm64-devel

.PHONY: test
test:
	go test ./...

.PHONY: vet
vet:
	go vet ./...

.PHONY: clean
clean:
	rm -rvf ./build/ ./openshift-provider-cert-* ./opct-*

# For dependencies, see:
# .github/workflows/static-website.yml
# hack/docs-requirements.txt

.PHONY: build-changelog
build-changelog:
	./hack/generate-changelog.sh

.PHONY: build-docs
build-docs: build-changelog
	mkdocs build --site-dir ./site
