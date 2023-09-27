# User Installation Guide - Disconnected Installations

## Prerequisites

- Disconnected Mirror Image Registry created
- [Private cluster Installed](https://docs.openshift.com/container-platform/latest/installing/installing_bare_metal/installing-restricted-networks-bare-metal.html)
- [You created a registry on your mirror host](https://docs.openshift.com/container-platform/latest/installing/disconnected_install/installing-mirroring-installation-images.html#installing-mirroring-installation-images)

## Configuring the Disconnected Mirror Registry

1. Extract the `openshift-tests` executable associated with the version of OpenShift you are installing.
_Note:_ The pull secret must contain both your OpenShift pull secret as well credentials for the disconnected
mirror registry.

~~~sh
PULL_SECRET=/path/to/pull-secret
OPENSHIFT_TESTS_IMAGE=$(oc get is -n openshift tests -o=jsonpath='{.spec.tags[0].from.name}')
oc image extract -a ${PULL_SECRET} "${OPENSHIFT_TESTS_IMAGE}" --file="/usr/bin/openshift-tests"
chmod +x openshift-tests
~~~

2. Extract the images and the location to where they are to be mirrored from the `openshift-tests` executable.  

~~~sh
TARGET_REPO=target-registry.net/ocp-cert
./openshift-tests images --to-repository ${TARGET_REPO} > images-to-mirror
~~~

3. Append images used in OPCT to the `images-to-mirror` list

~~~sh
./opct get images --to-repository ${TARGET_REPO} >> images-to-mirror
~~~

5. Mirror the images to the disconnected mirror registry

~~~sh
oc image mirror -a ${PULL_SECRET} -f images-to-mirror
~~~

## Preparing Your Cluster

- The Insights operator must be disabled prior to to running tests.  See [Disabling insights operator](https://docs.openshift.com/container-platform/latest/support/remote_health_monitoring/opting-out-of-remote-health-reporting.html)
- The [Image Registry Operator](https://docs.openshift.com/container-platform/latest/registry/index.html) must be configured and available


For additional details and configuration options, see [User Guide](./user.md).