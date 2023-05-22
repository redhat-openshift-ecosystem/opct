# FAQ for OpenShift Provider Certification Tool

## OPCT Project

**Q: What is the OpenShift Provider Certification Tool (OPCT) ?**

OPCT was designed to validate the custom OpenShift installations in non-integrated providers (a.k.a [agnostic installation](https://docs.openshift.com/container-platform/4.11/installing/installing_platform_agnostic/installing-platform-agnostic.html) with `platform.none:{}` option for `install-config.yaml`), **running existing suite of e2e tests** from Kubernetes and OpenShift ecosystem.

OPCT was build on top of [Sonobuoy](https://sonobuoy.io/) and [openshift-tests](https://github.com/openshift/origin#end-to-end-e2e-and-extended-tests) utilities. The `openshift-tests` tool
is used to run conformance test suites across the CI Jobs, and also the [Kubernetes Certification Program](https://www.cncf.io/certification/software-conformance/). The `sonobuoy` tool is used to automate the certification environment, running many custom plugins implemented to meet the requirements of OpenShift Provider Certification Program.

**Q: What was OPCT not created for?**

It's not created to:
* replace OpenShift CI tests/clusters tooling
* implement OpenShift's component-specific e2e tests
* implement Provider's specific e2e test

**Q: Is it possible to write custom e2e tests from my custom component?**

Possibly yes, but it's not recommended and not supported by OPCT. The
OPCT was designed to implement the requirements of VSCP/OpenShift Provider Certification Program. The e2e tests
written as a plugin on OPCT are specific for the certification, or evaluate
specific install requirements on OpenShift cluster installed on external/non-supported platforms.

You have those options to implement e2e tests that should be tested on OpenShift installation:

* add the tests on the [openshift-tests utility](https://github.com/openshift/origin#end-to-end-e2e-and-extended-tests)
* if you would like to create a Sonobuoy plugin to keep compatibility with the existing certification tools, you can start looking at the [example repository](https://github.com/vmware-tanzu/sonobuoy-plugins/tree/main/examples/e2e-skeleton)

## Cluster Troubleshoot

**Q: OPCT provides automation for end users?**

No, the goal for OPCT is to provide the automation to run OpenShift/Kubernetes Conformance tests in custom OpenShift installations.

**Q: How can I extract the details of failed tests?**

OPCT collect the details only for failed e2e tests. There are many ways to read the details, but the quickest is using `--save-to` flag of `report` command:

~~~
./opct artifact.tar.gz --save-to results-data
~~~

All the tests details is saved, by text file inside the `results-data` directory.

See more how to Explore the [failures on the docs](https://redhat-openshift-ecosystem.github.io/provider-certification-tool/support-guide/#exploring-the-failures)
