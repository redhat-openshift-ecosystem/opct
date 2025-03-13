# Validating OpenShift installation with ARM

This guide describes how to install an OpenShift cluster on AWS on instances with ARM64 architecture and run the validation tests with OPCT.

This is a reference guide, not an official documentation of installing OpenShift on ARM64 architecture.

Please refer to [OpenShift documentation][openshift-docs] for more information.

## Install a cluster

- Download the installer binary:

```bash
wget -O openshift-install.tar.gz https://mirror.openshift.com/pub/openshift-v4/amd64/clients/ocp/4.14.0-rc.6/openshift-install-linux.tar.gz
tar xfz openshift-install.tar.gz
```

- Export the variables used to create a cluster:

```bash
export INSTALL_DIR=install-dir1
export BASE_DOMAIN=devcluster.openshift.com
export CLUSTER_NAME=arm-opct01
export CLUSTER_REGION=us-east-1
export SSH_PUB_KEY_FILE=$HOME/.ssh/id_rsa.pub
export PULL_SECRET_FILE=$HOME/.openshift/pull-secret-latest.json

mkdir -p $INSTALL_DIR
```

- Pick the release image in the [release controller][release-controller] (valid only for experimental environments):

```bash
export OPENSHIFT_INSTALL_RELEASE_IMAGE_OVERRIDE="quay.io/openshift-release-dev/ocp-release:4.14.0-rc.6-aarch64
```

- Create installer configuration:

```bash
cat <<EOF > ${INSTALL_DIR}/install-config.yaml
apiVersion: v1
publish: External
baseDomain: ${BASE_DOMAIN}
metadata:
  name: "${CLUSTER_NAME}"
controlPlane:
  name: master
  architecture: arm64
  replicas: 3
compute:
- name: worker
  architecture: arm64
  replicas: 3
platform:
  aws:
    region: ${CLUSTER_REGION}
pullSecret: '$(cat ${PULL_SECRET_FILE} |awk -v ORS= -v OFS= '{$1=$1}1')'
sshKey: |
  $(cat ${SSH_PUB_KEY_FILE})
EOF
```

- Install the cluster:

```bash
./openshift-install create cluster --dir ${INSTALL_DIR} --log-level debug
```

- Export kubeconfig:

```bash
export KUBECONFIG=${INSTALL_DIR}/auth/kubeconfig
```

## Run Conformance workflow and explore the results

This section describes how to rnu OPCT in an OpenShift cluster.

### Prerequisites

- OpenShift cluster installed on ARM
- OPCT command line interface installed
- KUBECONFIG environment variable exported
- An OpenShift user with cluster-admin privileges

### Steps

- Setup test node:

```bash
opct adm e2e-dedicated taint-node --yes
```

- Run OPCT and retrieve results when finished:

```bash
./opct run --wait
```

- Collect the results:

```bash
./opct retrieve
```

- Explore the results:

```bash
./opct report $(date +%Y%m)*.tar.gz --save-to ./report --loglevel debug
```

Explore the results.

## Destroy

Destroy the conformance test environment:

```bash
./opct destroy
```

- Destroy a cluster:

```bash
./openshift-install destroy cluster --dir ${INSTALL_DIR}
```

[openshift-docs]: https://docs.openshift.com/container-platform/latest
[release-controller]: https://arm64.ocp.releases.ci.openshift.org/
