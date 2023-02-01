# Dev Guide - cluster provisioner (opct-runner)

> **NOTE**: This tool is used by Developers for development and OPCT QE/CI, it's not a recommendation to be used by the OpenShift Provider Certification workflow.

opct-runner is a collection of Ansible Playbooks used to provision the cluster*, run OPCT, and destroy the cluster.

> *the cluster provisioner is done by [okd-installer Ansible Collection](https://galaxy.ansible.com/mtulio/okd_installer), the playbooks and vars declared in this repo will set the required parameters to create all the stacks (Network, DNS, IAM, Compute, etc) and install OCP using agnostic installation (a.k.a `platform=None`).

The goal of those scripts are:

- decrease the total time to run the certification environment with non-goals, like managing infrastructure;
- decrease the manual steps like setting node labels required to the dedicated environment, setting up local-registry, waiting for COs, collecting artifacts (...)
- saving cloud infrastructure costs by destroying the cluster right after the OPCT has been finished
- saving cloud infrastructure costs by running different topologies (cheaper like single-AZ) in development environments
- allow developers to run locally instead of OCP-CI/cluster-bot (quickly provision OCP cluster to run OPCT)

The steps are:

- Build OPCT on the version desired to run
- Build the container image for opct-runner
- Set the Cloud providers and basic env vars for the OCP installer
- Run the OPCT runner targeting the desired OCP version

## Build

Clone the OPCT repo:

```bash
git clone https://github.com/redhat-openshift-ecosystem/provider-certification-tool.git
```

Build OPCT:

```bash
make linux-amd64
```

Build opct-runner container:

> check if needed to use a more recent version of okd-installer

```bash
podman build -t opct-runner:latest -f hack/opct-runner/Containerfile hack/opct-runner/
```

## Setup

Export and Create the environment var file:

```bash
export CLUSTER=opct22123102
cat <<EOF> ./.opct.env
CONFIG_PULL_SECRET_FILE=/pull-secret.json
AWS_ACCESS_KEY_ID=${AWS_ACCESS_KEY_ID}
AWS_SECRET_ACCESS_KEY=${AWS_SECRET_ACCESS_KEY}
AWS_DEFAULT_REGION=${AWS_DEFAULT_REGION}
EOF
```

Create the workdir (`./.opct`):

```bash
mkdir ./.opct
```

## Run

Run the opct-runner:

```bash
podman run \
    --env-file ${PWD}/.opct.env \
    -v ${PWD}/.opct:/root/.ansible/okd-installer:Z \
    -v ${HOME}/.ssh:/root/.ssh:Z \
    -v ${HOME}/.openshift/pull-secret-latest.json:/pull-secret.json \
    -v ${PWD}/openshift-provider-cert-linux-amd64:/openshift-provider-cert:Z \
    --rm opct-runner:latest \
        ansible-playbook opct-runner-all-aws.yaml \
        -e cluster_name=$CLUSTER \
        -e cluster_version=4.11.18 ;
```

The execution may take a while to run everything (expected 2-3 hours):

- ~20-30 minutes to provision the cluster
- ~2:30h to run the OPCT
- ~10 minutes to destroy the cluster

You can follow the cluster and execution in a new terminal:

- Checking the OPCT stdout/err on the log file:

```bash
tail -f ./.opct/clusters/$CLUSTER/opct/opct-runner.log
```

- Checking the pod logs:

```bash
oc --kubeconfig=./.opct/clusters/$CLUSTER/auth/kubeconfig get pods -n openshift-provider-certification
```

When the tasks have been finished, the results should be saved on the directory `./.opct/clusters/$CLUSTER/opct/`.

Check the results:

```bash
./openshift-provider-cert-linux-amd64 results .opct/clusters/${CLUSTER}/opct/*.tar.gz
```

Make sure the cluster has been destroyed.

## Run more

If you would like to run in parallel tests into different versions OPCT and/or OCP, you need to change only the args send to the playbook:

- `-e cluster_name`
- `-e cluster_version`

Example running the same OPCT version into two OCP clusters/versions (4.11.18 and 4.10.45):

```bash
# Cluster running in 4.11.18
podman run \
    --env-file ${PWD}/.opct.env \
    -v ${PWD}/.opct:/root/.ansible/okd-installer:Z \
    -v ${HOME}/.ssh:/root/.ssh:Z \
    -v ${HOME}/.openshift/pull-secret-latest.json:/pull-secret.json \
    -v ${PWD}/openshift-provider-cert-linux-amd64:/openshift-provider-cert:Z \
    --rm opct-runner:latest \
        ansible-playbook opct-runner-all-aws.yaml \
        -e cluster_name=opct-ocp41118 \
        -e cluster_version=4.11.18

# Cluster running in 4.10.45
podman run \
    --env-file ${PWD}/.opct.env \
    -v ${PWD}/.opct:/root/.ansible/okd-installer:Z \
    -v ${HOME}/.ssh:/root/.ssh:Z \
    -v ${HOME}/.openshift/pull-secret-latest.json:/pull-secret.json \
    -v ${PWD}/openshift-provider-cert-linux-amd64:/openshift-provider-cert:Z \
    --rm opct-runner:latest \
        ansible-playbook opct-runner-all-aws.yaml \
        -e cluster_name=opct-ocp41045 \
        -e cluster_version=4.10.45
```

Example running two OPCT versions (v0.1.0 and v0.2.1) into the same OCP version:

```bash
# Cluster running in OCP 4.11.18 and OPCT v0.2.1
podman run \
    --env-file ${PWD}/.opct.env \
    -v ${PWD}/.opct:/root/.ansible/okd-installer:Z \
    -v ${HOME}/.ssh:/root/.ssh:Z \
    -v ${HOME}/.openshift/pull-secret-latest.json:/pull-secret.json \
    -v ${PWD}/openshift-provider-cert-linux-amd64-v0.2.1:/openshift-provider-cert:Z \
    --rm opct-runner:latest \
        ansible-playbook opct-runner-all-aws.yaml \
        -e cluster_name=opct-v010 \
        -e cluster_version=4.11.18

# Cluster running in OCP 4.11.18 and OPCT v0.1.0
podman run \
    --env-file ${PWD}/.opct.env \
    -v ${PWD}/.opct:/root/.ansible/okd-installer:Z \
    -v ${HOME}/.ssh:/root/.ssh:Z \
    -v ${HOME}/.openshift/pull-secret-latest.json:/pull-secret.json \
    -v ${PWD}/openshift-provider-cert-linux-amd64-v0.1.0:/openshift-provider-cert:Z \
    --rm opct-runner:latest \
        ansible-playbook opct-runner-all-aws.yaml \
        -e cluster_name=opct-v021 \
        -e cluster_version=4.11.18
```


Example running OPCT running the upgrade feature:

```bash
CLI_PATH=${HOME}/go/src/github.com/mtulio/provider-certification-tool-cli
CLI_BIN_NAME=openshift-provider-cert-linux-amd64
CLI_VERSION=v0.3.0-alpha0
CLI_BIN_PATH=${CLI_PATH}/${CLI_BIN_NAME}-${CLI_VERSION}

CID=411to412
INSTALL_VERSION=4.11.22
UPGRADE_VERSION=4.12.0

CLUSTER=opct-${CID}
WORKPATH=${PWD}/.opct-${CID}
mkdir -p ${WORKPATH}

UPGRADE_IMG="$(oc adm release info ${UPGRADE_VERSION} -o jsonpath={.image})"

podman run \
    --env-file ${PWD}/.opct.env \
    -v ${WORKPATH}/:/root/.ansible/okd-installer:Z \
    -v ${HOME}/.ssh:/root/.ssh:Z \
    -v ${HOME}/.openshift/pull-secret-latest.json:/pull-secret.json \
    -v ${CLI_BIN_PATH}:/openshift-provider-cert:Z \
    --rm opct-runner:latest \
        ansible-playbook opct-runner-all-aws.yaml \
        -e cluster_name=$CLUSTER \
        -e cluster_version=${INSTALL_VERSION} \
        -e run_mode=upgrade \
        -e opct_run_mode="--mode=upgrade" \
        -e opct_run_args="--upgrade-to-image=\"${UPGRADE_IMG}\"" \
        -e skip_delete=true -e skip_run=true ;
```

Example running OPCT for a specific PR([#34](https://github.com/redhat-openshift-ecosystem/provider-certification-tool/pull/34)):

- Build the binary
```bash
CLI_PATH=${HOME}/go/src/github.com/mtulio/provider-certification-tool-cli
CLI_BIN_NAME=openshift-provider-cert-linux-amd64
CLI_VERSION=v0.3.0-dev3
CLI_BIN_PATH=${CLI_PATH}/${CLI_BIN_NAME}-${CLI_VERSION}

#cd ${CLI_PATH}
#git fetch origin pull/34/head:pr34
#git checkout pr34
make update
make linux-amd64
cp ${CLI_BIN_NAME} ${CLI_BIN_PATH}
```

- Run the cluster

```bash
CID=4120
INSTALL_VERSION=4.12.0
CLUSTER=opct-${CID}
WORKPATH=${PWD}/.opct-${CID}
mkdir -p ${WORKPATH}

podman run \
    --env-file ${PWD}/.opct.env \
    -e ENABLE_TURBO_MODE=1 \
    -v ${WORKPATH}:/root/.ansible/okd-installer:Z \
    -v ${HOME}/.ssh:/root/.ssh:Z \
    -v ${HOME}/.openshift/pull-secret-latest.json:/pull-secret.json \
    -v ${CLI_BIN_PATH}:/openshift-provider-cert:Z \
    --rm opct-runner:latest \
        ansible-playbook opct-runner-all-aws.yaml \
        -e cluster_name=$CLUSTER \
        -e cluster_version=${INSTALL_VERSION} \
        -e skip_delete=true;
```

Example for Create Cluster (only):

- Create the cluster
```bash
CID=4120
INSTALL_VERSION=4.12.0
CLUSTER=opct-${CID}
WORKPATH=${PWD}/.opct-${CID}
mkdir -p ${WORKPATH}

podman run \
    --env-file ${PWD}/.opct.env \
    -v ${WORKPATH}:/root/.ansible/okd-installer:Z \
    -v ${HOME}/.ssh:/root/.ssh:Z \
    -v ${HOME}/.openshift/pull-secret-latest.json:/pull-secret.json \
    --rm opct-runner:latest \
        ansible-playbook opct-cluster-create-aws.yaml \
        -e cluster_name=$CLUSTER \
        -e cluster_version=${INSTALL_VERSION} ;
```

## Alternative Playbooks

- Cluster destroy: Commonly used when the flag `keep_cluster` is set on `opct-runner-all-aws.yaml`

```bash
export CLUSTER=opct-v41046
podman run \
    --env-file ${PWD}/.opct.env \
    -v ${PWD}/.opct:/root/.ansible/okd-installer:Z \
    --rm opct-runner:latest \
        ansible-playbook opct-cluster-delete-aws.yaml \
        -e cluster_name=$CLUSTER
```

- When the execution has been finished but some errors to collect the artifacts, just run:

```bash
$ cd .opct/clusters/$CLUSTER/opct/
$ KUBECONFIG=${PWD}/../auth/kubeconfig ../../../../openshift-provider-cert-linux-amd64 retrieve
$ ../../../../openshift-provider-cert-linux-amd64 results *.tar.gz

# remember to destroy the cluster (opct-cluster-delete-aws.yaml)
```


## Run directly from your (py)env

If you would like to skip the container environment and run the playbooks directly from the host (hard way), you should install all the dependencies required by okd-installer on your environment.

Let's create the python virtual environment to isolate the packages and avoid breaking anything in your host:

~~~
python3 -m venv .venv
source ~/.venv/bin/activate
~~~

Install the requirements of okd-installer

~~~
pip install -r https://raw.githubusercontent.com/mtulio/ansible-collection-okd-installer/main/requirements.txt
ansible-galaxy collection install -r https://raw.githubusercontent.com/mtulio/ansible-collection-okd-installer/main/requirements.yml
~~~

Install the Collection

~~~
ansible-galaxy collection install mtulio.okd_installer
~~~

Run the playbook:

~~~
source ${PWD}/.opct.env
ansible-playbook opct-runner-all-aws.yaml \
        -e cluster_name=opct-41118 \
        -e cluster_version=4.11.18
~~~

The results should be saved at `${HOME}/.ansible/okd-installer/clusters/${CLUSTER}/opct`.
