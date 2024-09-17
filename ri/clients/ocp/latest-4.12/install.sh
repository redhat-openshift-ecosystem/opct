#!/bin/bash
set -ex

OCP_VERSION="latest-4.12"
URL_BASE="https://mirror.openshift.com/pub/openshift-v4/x86_64/clients/ocp/$OCP_VERSION"
echo "Downloading OCP [$OCP_VERSION] Clients from $URL_BASE"

URL_INSTALLER="$URL_BASE/openshift-install-linux.tar.gz"
URL_CLIENT="$URL_BASE/openshift-client-linux.tar.gz"
URL_CCOCTL="$URL_BASE/ccoctl-linux.tar.gz"

curl -s "$URL_INSTALLER" | tar -xz 'openshift-install'
curl -s "$URL_CLIENT" | tar -xz 'oc'
curl -s "$URL_CCOCTL" | tar -xz 'ccoctl'

ls -l

echo "export PATH=$PATH:$HOME/src/provider-certification-tool//clients/ocp/latest-4.12"
echo "OCP [$OCP_VERSION] Clients installed"