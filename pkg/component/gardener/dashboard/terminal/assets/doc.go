// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

//go:generate ../../../../../../hack/generate-crds.sh -p crd- dashboard.gardener.cloud

package assets

import (
	// Import to register the types for CRD generation.
	_ "github.com/gardener/terminal-controller-manager/api/v1alpha1"
)
