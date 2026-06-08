// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

//go:generate ../../../../../../hack/generate-crds.sh -p crd- dashboard.gardener.cloud

package assets

import (
	_ "github.com/gardener/terminal-controller-manager/api/v1alpha1" // required for generating CRDs
)
