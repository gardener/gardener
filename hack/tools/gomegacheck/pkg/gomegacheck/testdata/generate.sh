#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

cd "$(dirname "$0")"

rm -rf src/github.com
mkdir -p src/github.com/onsi/gomega
cp ../../../../../../vendor/github.com/onsi/gomega/gomega_dsl.go src/github.com/onsi/gomega
cp -r ../../../../../../vendor/github.com/onsi/gomega/types src/github.com/onsi/gomega
cp -r ../../../../../../vendor/github.com/onsi/gomega/internal src/github.com/onsi/gomega
cp -r ../../../../../../vendor/github.com/onsi/gomega/format src/github.com/onsi/gomega

goimports -w .
