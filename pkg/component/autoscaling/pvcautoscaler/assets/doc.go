// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

//go:generate ../../../../../hack/generate-crds.sh -p crd- --allow-dangerous-types autoscaling.gardener.cloud

package assets

import (
	_ "github.com/gardener/pvc-autoscaler/api/autoscaling/v1alpha1"
)
