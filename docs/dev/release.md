# Dev Guide - Release components

This guides describes how to release a new version of OPCT considering all the project dependencies.

## Creating container images for components

### Sonobuoy

Steps to check if Sonobuoy provides images to the target platform in the version used by OPCT:

1) Check the sonobuoy version used by OPCT
```bash
$ go list -m github.com/vmware-tanzu/sonobuoy
github.com/vmware-tanzu/sonobuoy v0.56.10

# OR
$ curl -s https://raw.githubusercontent.com/redhat-openshift-ecosystem/provider-certification-tool/main/go.mod | grep 'github.com/vmware-tanzu/sonobuoy'
    github.com/vmware-tanzu/sonobuoy v0.56.10
```

2) Check the Sonobuoy images built for the version required by OPCT
```bash
$ skopeo list-tags docker://docker.io/sonobuoy/sonobuoy | jq .Tags | grep -i v0.56.10
  "amd64-v0.56.10",
  "arm64-v0.56.10",
  "ppc64le-v0.56.10",
  "s390x-v0.56.10",
  "v0.56.10",
  "win-amd64-1809-v0.56.10",
  "win-amd64-1903-v0.56.10",
  "win-amd64-1909-v0.56.10",
  "win-amd64-2004-v0.56.10",
  "win-amd64-20H2-v0.56.10",
```

3) Pull the desired image and push it to the mirrored repo with the standard name: `<version>-<platform>`
```bash
SB_REPO=docker.io/sonobuoy/sonobuoy
SB_VERSION=v0.56.10
TARGET_REPO=quay.io/opct/sonobuoy
TARGET_IMG=${TARGET_REPO}:$SB_VERSION
declare -A platforms=()
platforms+=( [linux-amd64]=amd64-${SB_VERSION} )
platforms+=( [linux-arm64]=arm64-${SB_VERSION} )
platforms+=( [linux-ppc64le]=ppc64le-${SB_VERSION} )
platforms+=( [linux-s390x]=s390x-${SB_VERSION} )
containers=""
for arch ${!platforms[*]}; do
    echo "syncing $SB_VERSION-$arch"
    src_img=$SB_REPO:${platforms[$arch]}-$SB_VERSION
    dest_image=$TARGET_IMG-$arch
    podman pull $src_img
    podman tag $src_img $dest_image
    podman push $dest_image
    containers+=" $dest_image"
done
```
4) Create the manifest for each image
```bash
podman manifest create $TARGET_IMG $containers
podman manifest push $TARGET_IMG docker://$TARGET_IMG
```

The following images must be created:

```bash
$ skopeo list-tags docker://quay.io/opct/sonobuoy | jq .Tags | grep -i v0.56.10
  "v0.56.10",
  "v0.56.10-linux-amd64",
  "v0.56.10-linux-arm64",
  "v0.56.10-linux-ppc64le",
  "v0.56.10-linux-s390x",
```

### Plugins images

#### Development builds

Create images for development tests (PRs):

```bash
make build-push-arch-amd64
make build-push-arch-arm64
make build-manifests && make push-manifests
```

#### Production builds

Create the images for each component and arch:

- Tools
- Plugin openshift-tests
- Must-gather-monitoring

amd64:

```bash
TOOLS_VERSION="v0.3.0" \
    PLUGIN_TESTS_VERSION="v0.5.0-alpha.3" \
    MGM_VERSION="v0.2.0" \
    make prod-build-push-arch-amd64
```

arm64:

```bash
TOOLS_VERSION="v0.3.0" \
    PLUGIN_TESTS_VERSION="v0.5.0-alpha.3" \
    MGM_VERSION="v0.2.0" \
    make prod-build-push-arch-arm64
```

The following images must be created:

```bash
$ podman images | grep quay.io/opct
quay.io/opct/must-gather-monitoring             v0.2.0-linux-amd64                163337b90d21  2 days ago     300 MB
quay.io/opct/plugin-openshift-tests             v0.5.0-alpha.3-linux-amd64        dcad3b42447f  2 days ago     254 MB
quay.io/opct/tools                              v0.3.0-linux-amd64                ef6d90ac44cb  2 days ago     254 MB
quay.io/opct/must-gather-monitoring             v0.2.0-linux-arm64                b105537c3414  2 days ago     374 MB
quay.io/opct/plugin-openshift-tests             v0.5.0-alpha.3-linux-arm64        a34e547c1ac0  2 days ago     300 MB
quay.io/opct/tools                              v0.3.0-linux-arm64                c9c55203dad5  2 days ago     300 MB
```


#### Create manifests

Step to create the manifest with container images by platform:

```bash
TOOLS_VERSION="v0.3.0" \
    PLUGIN_TESTS_VERSION="v0.5.0-alpha.3" \
    MGM_VERSION="v0.2.0" \
    make prod-build-push-manifests
```

The following manifests should be created:

```bash
$ podman images | grep quay.io/opct
quay.io/opct/must-gather-monitoring             v0.2.0                            fe82f161a8f4  2 days ago     844 B
quay.io/opct/plugin-openshift-tests             v0.5.0-alpha.3                    b323006580af  2 days ago     860 B
quay.io/opct/tools                              v0.3.0                            faa9f430f3e8  2 days ago     810 B
```