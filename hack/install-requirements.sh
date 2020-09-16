#!/bin/bash
#
# SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -e

echo "> Installing requirements"

GO111MODULE=off go get golang.org/x/tools/cmd/goimports

export GO111MODULE=on
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.27.0
curl -s "https://raw.githubusercontent.com/helm/helm/v2.16.9/scripts/get" | bash -s -- --version 'v2.13.1'

if [[ "$(uname -s)" == *"Darwin"* ]]; then
  cat <<EOM
You are running in a MAC OS environment!

Please make sure you have installed the following requirements:

- GNU Core Utils
- GNU Tar
- GNU Sed

Brew command:
$ brew install coreutils gnu-tar gnu-sed

Please allow them to be used without their "g" prefix:
$ export PATH=/usr/local/opt/coreutils/libexec/gnubin:\$PATH
$ export PATH=/usr/local/opt/gnu-tar/libexec/gnubin:\$PATH
$ export PATH=/usr/local/opt/gnu-sed/libexec/gnubin:\$PATH
EOM
fi
