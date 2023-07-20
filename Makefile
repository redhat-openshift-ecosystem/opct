# Ensure go modules are enabled:
export GO111MODULE=on

# Disable CGO so that we always generate static binaries:
export CGO_ENABLED=0

IMG ?= quay.io/ocp-cert/opct
VERSION=$(shell git rev-parse --short HEAD)
RELEASE_TAG ?= 0.0.0

GO_BUILD_FLAGS := -ldflags '-s -w -X github.com/redhat-openshift-ecosystem/provider-certification-tool/pkg/version.commit=$(VERSION) -X github.com/redhat-openshift-ecosystem/provider-certification-tool/pkg/version.version=$(RELEASE_TAG)'

# Unset GOFLAG for CI and ensure we've got nothing accidently set
unexport GOFLAGS

.PHONY: all
all: linux-amd64-container cross-build-windows-amd64 cross-build-darwin-amd64 cross-build-darwin-arm64

.PHONY: build
build:
	go build -o openshift-provider-cert $(GO_BUILD_FLAGS)

.PHONY: generate
update:
	./hack/update-generated-bindata.sh

.PHONY: verify-codegen
verify-codegen:
	./hack/verify-codegen.sh

.PHONY: linux-amd64
linux-amd64:
	GOOS=linux GOARCH=amd64 go build -o openshift-provider-cert-linux-amd64 $(GO_BUILD_FLAGS)
	cp openshift-provider-cert-linux-amd64 opct-linux-amd64

.PHONY: cross-build-windows-amd64
cross-build-windows-amd64:
	GOOS=windows GOARCH=amd64 go build -o openshift-provider-cert-windows.exe $(GO_BUILD_FLAGS)
	cp openshift-provider-cert-windows.exe opct-windows.exe

.PHONY: cross-build-darwin-amd64
cross-build-darwin-amd64:
	GOOS=darwin GOARCH=amd64 go build -o openshift-provider-cert-darwin-amd64 $(GO_BUILD_FLAGS)
	cp openshift-provider-cert-darwin-amd64 opct-darwin-amd64

.PHONY: cross-build-darwin-arm64
cross-build-darwin-arm64:
	GOOS=darwin GOARCH=arm64 go build -o openshift-provider-cert-darwin-arm64 $(GO_BUILD_FLAGS)
	cp openshift-provider-cert-darwin-arm64 opct-darwin-arm64

.PHONY: linux-amd64-container
linux-amd64-container: linux-amd64
	podman build -t $(IMG):latest -f hack/Containerfile --build-arg=RELEASE_TAG=$(RELEASE_TAG) .

.PHONY: test
test:
	go test ./...

.PHONY: vet
vet:
	go vet ./...

.PHONY: clean
clean:
	rm -rvf ./openshift-provider-cert-* ./opct-*

# For dependencies, see:
# .github/workflows/static-website.yml
# hack/docs-requirements.txt

.PHONY: build-changelog
build-changelog:
	./hack/generate-changelog.sh

.PHONY: build-docs
build-docs: build-changelog
	mkdocs build --site-dir ./site
