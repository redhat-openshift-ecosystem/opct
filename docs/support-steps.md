# OPCT - Support Steps

This document describes tools used to explore the artifacts collected by
conformance workflow.

## Prerequisites

### Install Tools

- [omc](https://github.com/gmeghnag/omc)
~~~
OS=linux
curl -sL https://github.com/gmeghnag/omc/releases/latest/download/omc_${OS}_x86_64.tar.gz | tar xzf - omc
chmod +x ./omc
~~~

- [insights-core](https://github.com/RedHatInsights/insights-core)
~~~
$ pip install insights-core --upgrade
~~~

- [insights rules for OCP 4](https://gitlab.cee.redhat.com/ccx/ccx-rules-ocp)
~~~
pip install pip --upgrade
pip install git+https://gitlab.cee.redhat.com/ccx/ccx-ocp-core.git --upgrade
pip install git+https://gitlab.cee.redhat.com/ccx/ccx-rules-ocp.git --upgrade
~~~
- [opct](https://github.com/redhat-openshift-ecosystem/provider-certification-tool/releases/latest)


## Steps

### Step 1. Review valid artifact

#### Step 1A. Check artifact is valid

Goal: Ensure the artifact is valid
Description: 
Command:
Action on failed:

#### Step 1B. Check plugins has been finished - artifacts collector

Goal: Ensure the plugins have been finished
Description: 
Command:
~~~bash
$ tree plugins/99-openshift-artifacts-collector/results/global/
plugins/99-openshift-artifacts-collector/results/global/
├── artifacts_e2e-tests_kubernetes-conformance.txt
├── artifacts_e2e-tests_openshift-conformance.txt
├── artifacts_e2e-tests_openshift-upgrade.txt
└── artifacts_must-gather.tar.xz

$ tar xfJ plugins/99-openshift-artifacts-collector/results/global/artifacts_must-gather.tar.xz
~~~
Action when failed:

#### Step 1C. Check must-gather is present in artifacts

Goal: Ensure required artifacts are present
Description: 
Command:
Action on failed: ask to collect must gather

### Step 2. 


## Compliance

### Check image-registry is using a valid backend (non emptyDir)

> there is insight rule checking it

[FAIL] ccx_rules_ocp.external.rules.image_registry_storage.report
