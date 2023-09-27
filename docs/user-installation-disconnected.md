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

# Extra images
echo "registry.k8s.io/pause:3.8 ${TARGET_REPO}/ocp-cert:e2e-28-registry-k8s-io-pause-3-8-aP7uYsw5XCmoDy5W" >> images-to-mirror
~~~

3. Append Sonobuoy to the `images-to-mirror` list

~~~sh
SONOBUOY_TAG=$(./openshift-provider-cert-linux-amd64 version | grep "Sonobuoy Version:" | cut -d' ' -f 3)
SONOBUOY_TARGET=${TARGET_REPO}/sonobuoy:${SONOBUOY_TAG}
echo "quay.io/ocp-cert/sonobuoy:${SONOBUOY_TAG} ${SONOBUOY_TARGET}" >> images-to-mirror
~~~

4. Append the OPCT images to the `images-to-mirror` list

~~~sh
# Plugins
OPCT_VERSION=v0.4.1  # <-- Adjust to the OPCT release you are using
OPCT_TARGET=${TARGET_REPO}/openshift-tests-provider-cert:${OPCT_VERSION}
echo "quay.io/ocp-cert/openshift-tests-provider-cert:${OPCT_VERSION} ${OPCT_TARGET}" >> images-to-mirror

# must-gather monitoring
MGM_VERSION=v0.1.0
MGM_TARGET=${TARGET_REPO}/must-gather-monitoring:${MGM_VERSION}
echo "quay.io/opct/must-gather-monitoring:${MGM_VERSION} ${MGM_TARGET}" >> images-to-mirror
~~~

5. Mirror the images to the disconnected mirror registry

~~~sh
oc image mirror -a ${PULL_SECRET} -f images-to-mirror
~~~

## Preparing Your Cluster

- The Insights operator must be disabled prior to to running tests.  See [Disabling insights operator](https://docs.openshift.com/container-platform/latest/support/remote_health_monitoring/opting-out-of-remote-health-reporting.html)
- The [Image Registry Operator](https://docs.openshift.com/container-platform/latest/registry/index.html) must be configured and available


For additional details and configuration options, see [User Guide](./user.md).