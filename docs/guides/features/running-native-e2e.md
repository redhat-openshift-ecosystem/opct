# OPCT | Running Native Kubernetes e2e with opct and openshift-tests

## Prerequisites

- install OPCT

- grant permissions to the test environment

```bash
oc adm policy add-scc-to-group privileged system:authenticated system:serviceaccounts
oc adm policy add-scc-to-group anyuid system:authenticated system:serviceaccounts
```

- install yq

```bash
VERSION="v4.2.0"
BINARY=yq_linux_amd64
wget https://github.com/mikefarah/yq/releases/download/${VERSION}/${BINARY} -O $HOME/bin/yq &&\
    chmod +x $HOME/bin/yq
wget https://github.com/mikefarah/yq/releases/download/${VERSION}/${BINARY}.tar.gz -O - |\
  tar xz && mv ${BINARY} $HOME/bin
```

## Running the tests

To beging with, you need to define the group of tests it will run.

Sonobuoy provides a rich documentation guiding how to explore it, take a look at: https://sonobuoy.io/docs/main/e2eplugin/

In this example, we'll trigger the test with 'loadbalancer' in the name.

Steps:

- Run the tool, focusing in 'loadbalancer':

```sh
./opct sonobuoy run --e2e-focus='LoadBalancers' --dns-namespace=openshift-dns --dns-pod-labels=dns.operator.openshift.io/daemonset-dns=default
```

- Check the status:

```sh
$ /home/mtulio/opct/bin/opct-devel sonobuoy status
```

- Check if the environment was created

```sh
oc get pods -n sonobuoy -w
```

- Check the logs:

```sh
$ oc logs -l sonobuoy-plugin=e2e -n sonobuoy -c e2e
```

- Collect the results:

```sh
RESULT_FILE=$(./opct sonobuoy retrieve)
```

- Explore the results

```sh
./opct sonobuoy results $RESULT_FILE -mode full
```

- Exploring more:

```sh
./opct sonobuoy results $RESULT_FILE -p e2e

./opct sonobuoy results $RESULT_FILE -p e2e -m dump | yq e '.items[].items[].items[] | select(.status=="passed")' - 

```


## Run directly

- Run
```sh
export PULL_SECRET=$HOME/.openshift/pull-secret-latest.json
OPENSHIFT_TESTS_IMAGE=$(oc get is -n openshift tests -o=jsonpath='{.spec.tags[0].from.name}')
oc image extract -a ${PULL_SECRET} "${OPENSHIFT_TESTS_IMAGE}" --file="/usr/bin/openshift-tests"

./openshift-tests run all --dry-run  |grep '\[sig-network\] LoadBalancers' > ./tests-lb.txt

$ wc -l ./tests-lb.txt
20 ./tests-lb.txt
```


### Parallel execution (default)

```sh
./openshift-tests run --junit-dir ./junits -f ./tests-lb.txt | tee -a tests-lb-run.txt

grep -E ^'(passed|skipped|failed)' ./tests-lb-run.txt
grep ^passed ./tests-lb-run.txt
grep ^failed ./tests-lb-run.txt
grep ^skipped ./tests-lb-run.txt
```

### Serial mode (Parallel==1)

- Serial execution:

```sh
./openshift-tests run --junit-dir ./junits-serial -f ./tests-lb.txt --max-parallel-tests 1 | tee -a tests-lb-run-serial.txt

grep ^passed ./tests-lb-run-serial.txt
grep ^failed ./tests-lb-run-serial.txt
grep ^skipped ./tests-lb-run-serial.txt
```