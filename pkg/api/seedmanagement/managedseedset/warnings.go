// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package managedseedset

import (
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/gardener/gardener/pkg/api/core/shoot"
	"github.com/gardener/gardener/pkg/apis/seedmanagement"
)

// GetWarnings returns warnings for the given ManagedSeedSet.
func GetWarnings(managedseedset *seedmanagement.ManagedSeedSet) []string {
	if managedseedset == nil {
		return nil
	}

	var warnings []string

	if kubeAPIServer := managedseedset.Spec.ShootTemplate.Spec.Kubernetes.KubeAPIServer; kubeAPIServer != nil {
		path := field.NewPath("spec", "shootTemplate", "spec", "kubernetes", "kubeAPIServer")

		warnings = append(warnings, shoot.GetKubeAPIServerWarnings(kubeAPIServer, path)...)
	}

	return warnings
}
