// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

//go:generate ../../../../../hack/generate-crds.sh -p crd- autoscaling.gardener.cloud

package assets

import (
	// Import to register the types for CRD generation.
	_ "github.com/gardener/pvc-autoscaler/api/autoscaling/v1alpha1"
)
