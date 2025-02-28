# User Installation Guide - Disconnected Installations

## Prerequisites

- Disconnected Mirror Image Registry created
- [Private cluster installed][ocp-private]
- [You created a registry on your mirror host][ocp-registry]

## Configuring the Disconnected Mirror Registry

### Mirror images used by conformance suite

1. Extract the `openshift-tests` executable associated with the version of OpenShift you are installing.
_Note:_ The pull secret must contain both your OpenShift pull secret as well as credentials for the disconnected mirror registry.

~~~sh
PULL_SECRET=/path/to/pull-secret
OPENSHIFT_TESTS_IMAGE=$(oc get is -n openshift tests -o=jsonpath='{.spec.tags[0].from.name}')
oc image extract -a ${PULL_SECRET} "${OPENSHIFT_TESTS_IMAGE}" --file="/usr/bin/openshift-tests"
chmod +x openshift-tests
~~~

2. Extract the images and the location to where they are to be mirrored from the `openshift-tests` executable.

~~~sh
TARGET_REPO=target-registry.net/opct
./openshift-tests images --to-repository ${TARGET_REPO} > images-to-mirror
~~~

### Mirror images used by test environment

1. Append images used by OPCT to the `images-to-mirror` list:

~~~sh
./opct get images --to-repository ${TARGET_REPO} >> images-to-mirror
~~~

### Mirror the images

1. Mirror the images to the disconnected mirror registry:

~~~sh
oc image mirror -a ${PULL_SECRET} -f images-to-mirror
~~~

## Preparing Your Cluster

- The Insights operator must be disabled prior to running tests. See [Disabling insights operator][disable-insights]
- The [Image Registry Operator][registry-operator] must be configured and available

For additional details and configuration options, see the [User Guide][index.md].

[index.md]: ./index.md
[ocp-private]: https://docs.openshift.com/container-platform/latest/installing/installing_bare_metal/installing-restricted-networks-bare-metal.html
[ocp-registry]: https://docs.openshift.com/container-platform/latest/installing/disconnected_install/installing-mirroring-installation-images.html#installing-mirroring-installation-images
[disable-insights]: https://docs.openshift.com/container-platform/latest/support/remote_health_monitoring/opting-out-of-remote-health-reporting.html
[registry-operator]: https://docs.openshift.com/container-platform/latest/registry/index.html
