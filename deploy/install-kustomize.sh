#!/bin/bash

set -o nounset
set -o errexit

readonly INSTALL_DIR="$( dirname -- "$( readlink -f -- "$0"; )"; )/../bin"

if [ ! -f "${INSTALL_DIR}" ]; then
  mkdir -p "${INSTALL_DIR}"
fi
if [ -f "${INSTALL_DIR}/kustomize" ]; then
  rm ${INSTALL_DIR}/kustomize
fi

echo "installing kustomize"

tmpDir=`mktemp -d`
if [[ ! "$tmpDir" || ! -d "$tmpDir" ]]; then
  echo "Could not create temp dir."
  exit 1
fi

function cleanup {
  rm -rf "$tmpDir"
}

trap cleanup EXIT

pushd $tmpDir >& /dev/null

opsys=windows
arch=amd64
if [[ "$OSTYPE" == linux* ]]; then
  opsys=linux
elif [[ "$OSTYPE" == darwin* ]]; then
  opsys=darwin
fi

# As github has a limit on what stored in releases/, and kustomize has many different package
# versions, we just point directly at the version we want. See
# github.com/kubernetes-sigs/kustomize/master/hack/install_kustomize.sh.

version=v5.0.3
url_base=https://api.github.com/repos/kubernetes-sigs/kustomize/releases/tags/kustomize%2F
curl -s ${url_base}${version} |\
  grep browser_download.*${opsys}_${arch} |\
  cut -d '"' -f 4 |\
  sort -V | tail -n 1 |\
  xargs curl -s -O -L

tar xzf ./kustomize_v*_${opsys}_${arch}.tar.gz

cp ./kustomize ${INSTALL_DIR}/kustomize

popd >& /dev/null

${INSTALL_DIR}/kustomize version