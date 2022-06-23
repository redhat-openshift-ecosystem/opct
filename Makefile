# Ensure go modules are enabled:
export GO111MODULE=on

# Disable CGO so that we always generate static binaries:
export CGO_ENABLED=0

VERSION=$(shell git rev-parse --short HEAD)
RELEASE_TAG ?= 0.0.0

GO_BUILD_FLAGS := -ldflags '-X github.com/redhat-openshift-ecosystem/provider-certification-tool/pkg/version.commit=$(VERSION) -X github.com/redhat-openshift-ecosystem/provider-certification-tool/pkg/version.version=$(RELEASE_TAG)'

# Unset GOFLAG for CI and ensure we've got nothing accidently set
unexport GOFLAGS

.PHONY: build
build:
	go build -o openshift-provider-cert $(GO_BUILD_FLAGS)

.PHONY: generate
update:
	./hack/update-generated-bindata.sh

.PHONY: verify-codegen
verify-codegen:
	./hack/verify-codegen.sh

.PHONY: cross-build-windows-amd64
cross-build-windows-amd64:
	GOOS=windows GOARCH=amd64 go build -o openshift-provider-cert.exe $(GO_BUILD_FLAGS)

.PHONY: cross-build-darwin-amd64
cross-build-darwin-amd64:
	GOOS=darwin GOARCH=amd64 go build -o openshift-provider-cert $(GO_BUILD_FLAGS)

.PHONY: cross-build-darwin-arm64
cross-build-darwin-arm64:
	GOOS=darwin GOARCH=arm64 go build -o openshift-provider-cert $(GO_BUILD_FLAGS)


.PHONY: test
test:
	go test ./...

.PHONY: vet
vet:
	go vet ./...


.PHONY: clean
clean:
	rm -rf \
	  openshift-provider-cert
