// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubernetes

import (
	"github.com/Masterminds/semver/v3"
	policyv1 "k8s.io/api/policy/v1"

	versionutils "github.com/gardener/gardener/pkg/utils/version"
)

// SetAlwaysAllowEviction sets the UnhealthyPodEvictionPolicy field to AlwaysAllow if the kubernetes version is >= 1.26.
func SetAlwaysAllowEviction(pdb *policyv1.PodDisruptionBudget, kubernetesVersion *semver.Version) {
	var unhealthyPodEvictionPolicyAlwaysAllow = policyv1.AlwaysAllow

	if versionutils.ConstraintK8sGreaterEqual126.Check(kubernetesVersion) {
		pdb.Spec.UnhealthyPodEvictionPolicy = &unhealthyPodEvictionPolicyAlwaysAllow
	}
}
