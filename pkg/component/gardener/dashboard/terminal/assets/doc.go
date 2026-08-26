// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

//go:generate ../../../../../../hack/generate-crds.sh -p crd- dashboard.gardener.cloud

package assets

import (
	_ "github.com/gardener/terminal-controller-manager/api/v1alpha1" // Import to register the types for CRD generation.
)
