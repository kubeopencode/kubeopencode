#!/bin/bash

# Copyright Contributors to the KubeOpenCode project

set -o errexit
set -o nounset
set -o pipefail

SCRIPT_ROOT=$(dirname "${BASH_SOURCE}")/..
cd "${SCRIPT_ROOT}"

# Use go tool controller-gen for deepcopy generation
echo "Generating deepcopy functions..."
go tool controller-gen object:headerFile="${SCRIPT_ROOT}/hack/boilerplate.txt" paths="./api/..."

echo "Deepcopy generation complete"
