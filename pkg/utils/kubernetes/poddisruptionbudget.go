// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
