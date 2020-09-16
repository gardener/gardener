// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

// Bridge package to expose internal functions to tests in the botanist_test package.

package secrets

var (
	ExportGenerateKubeconfig = generateKubeconfig
)
