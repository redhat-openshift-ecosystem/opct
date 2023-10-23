# OPCT - OpenShift Provider Compatibility Tool

OpenShift Provider Compatibility Tool is used to evaluate an OKD/OpenShift installation
on a cloud provider by orchestrating conformance suites, and collecting cluster data to
further analisys.

## Documentation

- [User Guide](https://redhat-openshift-ecosystem.github.io/provider-certification-tool/)
- [Development Guide](https://redhat-openshift-ecosystem.github.io/provider-certification-tool/dev)

## Getting started

- Download OPCT

```bash
VERSION=v0.4.1
BINARY=opct-linux-amd64
wget -O /usr/local/bin/opct "https://github.com/redhat-openshift-ecosystem/provider-certification-tool/releases/download/${VERSION}/${BINARY}"
chmod u+x /usr/local/bin/opct
```

- Setup a dedicated node

```bash
test_node=$(oc get nodes -l node-role.kubernetes.io/worker='') -o jsonpath='{.items[0].metadata.name}'
oc label node $test_node node-role.kubernetes.io/tests=""
oc adm taint node $test_node node-role.kubernetes.io/tests="":NoSchedule
```

- Run regular conformance tests

```bash
opct run --wait
```

- Check the status

```bash
opct status --wait
```

- Collcet the results

```bash
opct retrieve
```

- Read the report

```bash
opct report *.tar.gz
```

- Destroy the environment

```bash
opct destroy
```

## See also

- [User Guide](https://redhat-openshift-ecosystem.github.io/provider-certification-tool/user/)
- [Contributor guidelines](CONTRIBUTING)
