// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

//go:generate ../../../../../hack/generate-crds.sh -p crd- -k --allow-dangerous-types autoscaling.k8s.io

package templates

import (
	_ "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1beta2"
)
