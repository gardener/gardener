#!/usr/bin/env bash
#
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -e

repo_root="$(git rev-parse --show-toplevel)"
$repo_root/hack/check-skaffold-deps.sh update
