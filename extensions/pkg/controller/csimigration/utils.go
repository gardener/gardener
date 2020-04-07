// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	isHigherThanCSIMigrationVersion, err := version.CompareVersions(cluster.Shoot.Spec.Kubernetes.Version, ">", csiMigrationVersion)
	if err != nil {
		return false, false, err
	}

	if isHigherThanCSIMigrationVersion {
		return true, true, nil
	}

	isCSIMigrationVersion, err := version.CompareVersions(cluster.Shoot.Spec.Kubernetes.Version, "~", csiMigrationVersion)
	if err != nil {
		return false, false, err
	}

	if !isCSIMigrationVersion {
		return false, false, nil
	}

	return true, metav1.HasAnnotation(cluster.ObjectMeta, AnnotationKeyNeedsComplete), nil
}
