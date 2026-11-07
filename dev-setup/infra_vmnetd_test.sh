#!/usr/bin/env bash
# SPDX-FileCopyrightText: Contributors to the Gardener project
#
# SPDX-License-Identifier: Apache-2.0

set -o errexit
set -o pipefail

source "$(dirname "${BASH_SOURCE[0]}")/infra.sh"

installer_help=$'Commands:\n  vmnetd\n  remove-vmnetd'
docker_desktop_should_install_vmnetd darwin24 'Docker Desktop' absent "$installer_help"

if docker_desktop_should_install_vmnetd darwin24 'Docker Desktop' absent $'Commands:\n  remove-vmnetd'; then
  echo 'Error: remove-vmnetd must not enable installation.' >&2
  exit 1
fi

if docker_desktop_should_install_vmnetd darwin24 'Docker Desktop' present "$installer_help" ||
   docker_desktop_should_install_vmnetd linux-gnu 'Docker Desktop' absent "$installer_help" ||
   docker_desktop_should_install_vmnetd darwin24 'Docker Engine' absent "$installer_help"; then
  echo 'Error: installation requires macOS, Docker Desktop, and an absent socket.' >&2
  exit 1
fi

echo 'Docker Desktop vmnetd capability checks passed.'
