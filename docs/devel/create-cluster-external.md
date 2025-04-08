# Create OpenShift Cluster Platform Type External

!!! warn "Development Reference"
    This guide is intended for developers/builders who want to quickly install OpenShift clusters using a configuration commonly employed for provider validation: the platform-agnostic installation method.

This document explains how to create a cluster using platform type `External`
for your development environment to quickly test OPCT with OpenShift
clusters using the platform-agnostic installation method.

OpenShift cluster installations with platform type `External`
are the primary use case for the OPCT tool, as it validates standalone
installations to ensure that non-integrated providers
are correctly setting up the infrastructure required for OpenShift/OKD.

The platform type `External` is based on platform-agnostic installations.
You can find more details in the
[Installing a cluster with Platform External](https://docs.providers.openshift.org/platform-external/installing/) documentation.

Platform-agnostic installations require the user to provide the infrastructure,
with no infrastructure automation by the `openshift-install` program. The
steps in this guide reuse the infrastructure automation created for OpenShift CI (hosted in the `release` repository).
The `release-devenv` project reproduces workflows from the `release` repository locally,
saving time during the installation process.

This guide uses AWS as the provider to install an OpenShift cluster with platform
type `External`, leveraging existing automation described in the docs.providers documentation to prepare the cluster.

Here is how this document is organized:

- Section 1: Install OpenShift cluster with platform type `External`
- Section 2: Run OPCT

## Section 1: Install OpenShift Cluster

### Prerequisites

Create the AWS credentials to `${HOME}/.aws/opct`. Make sure it will use the default profile, such as:

```sh
$ cat ${HOME}/.aws/opct
[opct]
aws_access_key_id = <access id>
aws_secret_access_key = <access key>
```

Setup release-devenv project:

> See the docs for more information: https://github.com/openshift-splat-team/release-devenv/blob/main/docs/examples/openshift-e2e-external-aws.md

```sh
export AWS_REGION='us-west-2'
export CLUSTER_BASE_DOMAIN="opct.devcluster.openshift.com"

export WORKDIR="${HOME}/openshift-labs/external-opct"
export RELEASE_REPO_PATH="${WORKDIR}/release"
export RELEASE_DEV_REPO_PATH="${WORKDIR}/release-devenv"

export OCP_RELEASE=4.18
export OCP_VERSION=4.18.5

# Clone required repos
git clone git@github.com:openshift/release ${RELEASE_REPO_PATH}
git clone git@github.com:openshift-splat-team/release-devenv ${RELEASE_DEV_REPO_PATH}

# Install devenv dependencies
python -m venv ${WORKDIR}/venv && source ${WORKDIR}/venv/bin/activate
pip install -r ${RELEASE_DEV_REPO_PATH}/requirements.txt

# Create environment variables for release-devenv
cat << EOF > ${RELEASE_DEV_REPO_PATH}/.env
# Required by runner
PULL_SECRET_FILE    = ${PULL_SECRET_FILE}
RELEASE_REPO_PATH   = ${RELEASE_REPO_PATH}
SSH_PRIVATE_KEY     = '${HOME}/.ssh/id_rsa'
SSH_PUBLIC_KEY      = '${HOME}/.ssh/id_rsa.pub'
AWS_CREDENTIAL_PATH = '${HOME}/.aws/opct'

SKIP_STEPS=ipi-install-rbac,openshift-cluster-bot-rbac

## Required by runner to simulate workflows
CLUSTER_NAME          = ${USER}-ext-opct2
BASE_DOMAIN           = ${CLUSTER_BASE_DOMAIN}
LEASED_RESOURCE       = $AWS_REGION
JOB_NAME              = 'periodic-ci-openshift-release-master-nightly-${OCP_RELEASE}-e2e-external-aws-ccm'
CLUSTER_TYPE          = 'aws'
PERSISTENT_MONITORING = 'no'
OCP_ARCH              = 'amd64'

# Override globals
RELEASE_IMAGE                            = 'quay.io/openshift-release-dev/ocp-release:${OCP_VERSION}-x86_64'
RELEASE_IMAGE_LATEST                     = 'quay.io/openshift-release-dev/ocp-release:${OCP_VERSION}-x86_64'
OPENSHIFT_INSTALL_RELEASE_IMAGE_OVERRIDE = 'quay.io/openshift-release-dev/ocp-release:${OCP_VERSION}-x86_64'

# Required by workflow/chain/step
AWS_REGION                    = ${AWS_REGION}
PROVIDER_NAME                 = 'aws'
PLATFORM_EXTERNAL_CCM_ENABLED = 'yes'

REGISTRY_AUTH_FILE = ${RELEASE_DEV_REPO_PATH}/volumes/__runtime__/shared/pull-secret-with-ci
AWS_SHARED_CREDENTIALS_FILE = ${RELEASE_DEV_REPO_PATH}/volumes/cluster-profile/.awscred
EOF

# ensure .env is created with all variables
```

### Run the Installation steps

The installation steps are defined on [platform-external-cluster-aws-pre](https://steps.ci.openshift.org/chain/platform-external-cluster-aws-pre)

```sh
cd ${RELEASE_DEV_REPO_PATH}
./ci-runner.py --init --run-chain platform-external-cluster-aws-pre
```

## Section 2: Explore conformance test tooling

Prerequisites:

- OpenShift cluster installed
- opct installed


### Explore OPCT

Sample/simple steps to schedule conformance workflows/jobs to the target cluster using OPCT:

```sh
opct adm dedicated-node setup
opct run -w
opct retrieve
opct report -s ./results *.tar.gz
```

### Explore openshift-tests

Steps to perform individual tests using the `openshift-tests` utility:

- Set up a dedicated node
- Extract `openshift-tests`
- Create the test list
- Run the test

```sh
# Extract openshift-tests https://docs.providers.openshift.org/platform-external/e2e-testing/#openshift-tests-utility

# Run
test_id=OCPBUGS
variant=ext-ctrl
./openshift-tests run \
-f test-${test_id}.txt \
--monitor="etcd-log-analyzer" \
--junit-dir=./test-${test_id}_${variant} \
| tee -a test-${test_id}_${variant}.txt
```
