// SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

//go:generate ../../../../../hack/generate-crds.sh -p crd- autoscaling.gardener.cloud

package assets

import (
	_ "github.com/gardener/pvc-autoscaler/api/autoscaling/v1alpha1" // Import to register the types for CRD generation.
)
