// Copyright 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package csimigration_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	. "github.com/gardener/gardener/extensions/pkg/controller/csimigration"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

var _ = Describe("utils", func() {
	var (
		k8s118 = "1.18"

		objectMetaWithNeedsCompleteAnnotation = metav1.ObjectMeta{
			Annotations: map[string]string{
				AnnotationKeyNeedsComplete: "true",
			},
		}

		clusterBrokenVersion         = &extensionscontroller.Cluster{Shoot: shootWithVersion("foo")}
		clusterK8sLessThan118        = &extensionscontroller.Cluster{Shoot: shootWithVersion("1.17.0")}
		clusterK8s1180               = &extensionscontroller.Cluster{Shoot: shootWithVersion("1.18.0")}
		clusterK8s1180WithAnnotation = &extensionscontroller.Cluster{Shoot: shootWithVersion("1.18.0"), ObjectMeta: objectMetaWithNeedsCompleteAnnotation}
		clusterK8s1185               = &extensionscontroller.Cluster{Shoot: shootWithVersion("1.18.5")}
		clusterK8s1185WithAnnotation = &extensionscontroller.Cluster{Shoot: shootWithVersion("1.18.5"), ObjectMeta: objectMetaWithNeedsCompleteAnnotation}
		clusterK8sMoreThan118        = &extensionscontroller.Cluster{Shoot: shootWithVersion("1.19.0")}
	)

	DescribeTable("#CheckCSIConditions",
		func(cluster *extensionscontroller.Cluster, csiMigrationVersion string, expectedCSIEnabled bool, expectedCSIMigrationComplete bool, expectErr bool) {
			csiEnabled, csiMigrationComplete, err := CheckCSIConditions(cluster, csiMigrationVersion)
			if expectErr {
				Expect(err).NotTo(Succeed())
			} else {
				Expect(err).To(Succeed())
				Expect(csiEnabled).To(Equal(expectedCSIEnabled))
				Expect(csiMigrationComplete).To(Equal(expectedCSIMigrationComplete))
			}
		},

		Entry("unparseable version", clusterBrokenVersion, k8s118, false, false, true),
		Entry("shoot version with higher major minor", clusterK8sMoreThan118, k8s118, true, true, false),
		Entry("shoot version with lower major minor", clusterK8sLessThan118, k8s118, false, false, false),
		Entry("shoot version exactly minimum (w/o annotation)", clusterK8s1180, k8s118, true, false, false),
		Entry("shoot version exactly minimum (w/ annotation)", clusterK8s1180WithAnnotation, k8s118, true, true, false),
		Entry("shoot version with same major minor (w/o annotation)", clusterK8s1185, k8s118, true, false, false),
		Entry("shoot version with same major minor (w/ annotation)", clusterK8s1185WithAnnotation, k8s118, true, true, false),
	)
})

func shootWithVersion(v string) *gardencorev1beta1.Shoot {
	return &gardencorev1beta1.Shoot{
		Spec: gardencorev1beta1.ShootSpec{
			Kubernetes: gardencorev1beta1.Kubernetes{
				Version: v,
			},
		},
	}
}
