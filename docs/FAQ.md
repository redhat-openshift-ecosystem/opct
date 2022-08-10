# FAQ for OpenShift Provider Certification Tool

**Q: What is the OpenShift Provider Certification Tool (OPCT) ?**

OPCT was designed to validate the custom OpenShift installations in
in non-integrated providers, **running existing e2e tests** from Kubernetes
and OpenShift ecosystem.

OPCT was build on top of [Sonobuoy](https://sonobuoy.io/) and [openshift-tests](https://github.com/openshift/origin#end-to-end-e2e-and-extended-tests) utility. These utilities
are commonly used and integrated with kubernetes test framework, used, for example
for conformance tests suites and [certification programs](https://www.cncf.io/certification/software-conformance/).

* It is created to run e2e OpenShift conformance tests on providers installation

**Q: What is OPCT not created?**

It's not created to:
* replace OpenShift CI tests/clusters
* implement component specific e2e tests

**Q: Is it possible to write custom e2e tests from my custom component?**

Possible yes, but it's not recommended and not supported by OPCT. The
OPCT was designed to implement the requirements of VSCP. The e2e tests
written as plugin on OPCT are specific for the certification, or evaluate
specific install requirements on OpenShift cluster installed external of
OpenShift CI environment.

You have those options to implement e2e tests that should be tested on OpenShift installation:

* add the tests on the [openshift-tests utility](https://github.com/openshift/origin#end-to-end-e2e-and-extended-tests)
* if you would like to create a Sonobuoy plugin to keep compability with the existing certification tools, you can start looking at the [example repository](https://github.com/vmware-tanzu/sonobuoy-plugins/tree/main/examples/e2e-skeleton)
