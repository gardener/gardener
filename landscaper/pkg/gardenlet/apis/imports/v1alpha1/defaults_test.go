// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package v1alpha1_test

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"
	"k8s.io/utils/pointer"

	. "github.com/gardener/gardener/landscaper/pkg/gardenlet/apis/imports/v1alpha1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	configv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Defaults", func() {
	Describe("#SetDefaults_Imports", func() {
		var obj *Imports

		BeforeEach(func() {
			obj = &Imports{}
		})

		It("should default the imports configuration", func() {
			SetDefaults_Imports(obj)

			Expect(obj).To(Equal(&Imports{
				DeploymentConfiguration: &seedmanagementv1alpha1.GardenletDeployment{
					ReplicaCount:         pointer.Int32(1),
					RevisionHistoryLimit: pointer.Int32(10),
					VPA:                  pointer.Bool(false),
				},
				ComponentConfiguration: runtime.RawExtension{Object: &configv1alpha1.GardenletConfiguration{
					TypeMeta: metav1.TypeMeta{
						APIVersion: configv1alpha1.SchemeGroupVersion.String(),
						Kind:       "GardenletConfiguration",
					},
					GardenClientConnection: &configv1alpha1.GardenClientConnection{
						ClientConnectionConfiguration: componentbaseconfigv1alpha1.ClientConnectionConfiguration{
							QPS:   100,
							Burst: 130,
						},
						BootstrapKubeconfig: &corev1.SecretReference{
							Name:      "gardenlet-kubeconfig-bootstrap",
							Namespace: "garden",
						},
						KubeconfigSecret: &corev1.SecretReference{
							Name:      "gardenlet-kubeconfig",
							Namespace: "garden",
						},
					},
				},
				},
			},
			))
		})

		It("should default the GardenletDeployment configuration", func() {
			config := &seedmanagementv1alpha1.GardenletDeployment{}
			SetDefaultsDeploymentConfiguration(config)

			Expect(config).To(Equal(&seedmanagementv1alpha1.GardenletDeployment{
				ReplicaCount:         pointer.Int32(1),
				RevisionHistoryLimit: pointer.Int32(10),
				VPA:                  pointer.Bool(false),
			}))
		})
	})
})
