// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package csimigration

import (
	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"

	"github.com/gardener/gardener/pkg/utils/version"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CheckCSIConditions takes the `Cluster` object and the Kubernetes version that shall be used for CSI migration. It
// returns two booleans - the first one indicates whether CSI shall be used at all (this may help the provider extension
// to decide whether to enable CSIMigration feature gates), and the second one indicates whether the CSI migration has
// been completed (this may help the provider extension to decide whether to enable the CSIMigration<Provider>Complete
// feature gate). If the shoot cluster version is higher than the CSI migration version then it always returns true for
// both variables. If it's lower than the CSI migration version then it always returns false for both variables. If it's
// the exact CSI migration (minor) version then it returns true for the first value (CSI migration shall be enabled),
// and true or false based on whether the "needs-complete-feature-gates" annotation is set on the Cluster object.
func CheckCSIConditions(cluster *extensionscontroller.Cluster, csiMigrationVersion string) (useCSI bool, csiMigrationComplete bool, err error) {
	isCSIMigrationVersion, err := version.CompareVersions(cluster.Shoot.Spec.Kubernetes.Version, "~", csiMigrationVersion)
	if err != nil {
		return false, false, err
	}

	if isCSIMigrationVersion {
		return true, metav1.HasAnnotation(cluster.ObjectMeta, AnnotationKeyNeedsComplete), nil
	}

	isHigherThanCSIMigrationVersion, err := version.CompareVersions(cluster.Shoot.Spec.Kubernetes.Version, ">", csiMigrationVersion)
	if err != nil {
		return false, false, err
	}

	if isHigherThanCSIMigrationVersion {
		return true, true, nil
	}

	return false, false, nil
}
