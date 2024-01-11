// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"encoding/json"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	gardenletv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
)

const (
	name      = "test"
	namespace = "garden"
)

var _ = Describe("Defaults", func() {
	var obj *ManagedSeed

	BeforeEach(func() {
		obj = &ManagedSeed{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
		}
	})

	Describe("ManagedSeed defaulting", func() {
		It("should default gardenlet configuration", func() {
			obj.Spec.Gardenlet = &Gardenlet{}

			SetObjectDefaults_ManagedSeed(obj)

			Expect(obj.Spec.Gardenlet.Deployment).NotTo(BeNil())
			Expect(obj.Spec.Gardenlet.Config).To(Equal(runtime.RawExtension{
				Object: &gardenletv1alpha1.GardenletConfiguration{
					TypeMeta: metav1.TypeMeta{
						APIVersion: gardenletv1alpha1.SchemeGroupVersion.String(),
						Kind:       "GardenletConfiguration",
					},
					Resources: &gardenletv1alpha1.ResourcesConfiguration{
						Capacity: corev1.ResourceList{
							gardencorev1beta1.ResourceShoots: resource.MustParse("250"),
						},
					},
					SeedConfig: &gardenletv1alpha1.SeedConfig{},
				}}))
			Expect(obj.Spec.Gardenlet.Bootstrap).To(PointTo(Equal(BootstrapToken)))
			Expect(obj.Spec.Gardenlet.MergeWithParent).To(PointTo(Equal(true)))
		})

		It("should default gardenlet configuration, and backup secret reference if backup is specified", func() {
			obj.Spec.Gardenlet = &Gardenlet{
				Config: runtime.RawExtension{
					Raw: encode(&gardenletv1alpha1.GardenletConfiguration{
						TypeMeta: metav1.TypeMeta{
							APIVersion: gardenletv1alpha1.SchemeGroupVersion.String(),
							Kind:       "GardenletConfiguration",
						},
						SeedConfig: &gardenletv1alpha1.SeedConfig{
							SeedTemplate: gardencorev1beta1.SeedTemplate{
								Spec: gardencorev1beta1.SeedSpec{
									Backup: &gardencorev1beta1.SeedBackup{},
								},
							},
						},
					}),
				},
			}

			SetObjectDefaults_ManagedSeed(obj)

			Expect(obj.Spec.Gardenlet.Config).To(Equal(
				runtime.RawExtension{
					Object: &gardenletv1alpha1.GardenletConfiguration{
						TypeMeta: metav1.TypeMeta{
							APIVersion: gardenletv1alpha1.SchemeGroupVersion.String(),
							Kind:       "GardenletConfiguration",
						},
						Resources: &gardenletv1alpha1.ResourcesConfiguration{
							Capacity: corev1.ResourceList{
								gardencorev1beta1.ResourceShoots: resource.MustParse("250"),
							},
						},
						SeedConfig: &gardenletv1alpha1.SeedConfig{
							SeedTemplate: gardencorev1beta1.SeedTemplate{
								Spec: gardencorev1beta1.SeedSpec{
									Backup: &gardencorev1beta1.SeedBackup{
										SecretRef: corev1.SecretReference{
											Name:      fmt.Sprintf("backup-%s", name),
											Namespace: namespace,
										},
									},
								},
							},
						},
					},
				},
			))
		})

		It("should not overwrite already set values for GardenletConfiguration", func() {
			obj.Spec.Gardenlet = &Gardenlet{
				Config: runtime.RawExtension{
					Raw: encode(&gardenletv1alpha1.GardenletConfiguration{
						TypeMeta: metav1.TypeMeta{
							APIVersion: gardenletv1alpha1.SchemeGroupVersion.String(),
							Kind:       "GardenletConfiguration",
						},
						Resources: &gardenletv1alpha1.ResourcesConfiguration{
							Capacity: corev1.ResourceList{
								gardencorev1beta1.ResourceShoots: resource.MustParse("300"),
							},
						},
						SeedConfig: &gardenletv1alpha1.SeedConfig{
							SeedTemplate: gardencorev1beta1.SeedTemplate{
								Spec: gardencorev1beta1.SeedSpec{
									Backup: &gardencorev1beta1.SeedBackup{
										SecretRef: corev1.SecretReference{
											Name:      "foo",
											Namespace: "bar",
										},
									},
								},
							},
						},
					}),
				},
				Bootstrap:       bootstrapPtr(BootstrapServiceAccount),
				MergeWithParent: pointer.Bool(false),
			}

			SetObjectDefaults_ManagedSeed(obj)

			Expect(obj.Spec.Gardenlet.Config).To(Equal(
				runtime.RawExtension{
					Object: &gardenletv1alpha1.GardenletConfiguration{
						TypeMeta: metav1.TypeMeta{
							APIVersion: gardenletv1alpha1.SchemeGroupVersion.String(),
							Kind:       "GardenletConfiguration",
						},
						Resources: &gardenletv1alpha1.ResourcesConfiguration{
							Capacity: corev1.ResourceList{
								gardencorev1beta1.ResourceShoots: resource.MustParse("300"),
							},
						},
						SeedConfig: &gardenletv1alpha1.SeedConfig{
							SeedTemplate: gardencorev1beta1.SeedTemplate{
								Spec: gardencorev1beta1.SeedSpec{
									Backup: &gardencorev1beta1.SeedBackup{
										SecretRef: corev1.SecretReference{
											Name:      "foo",
											Namespace: "bar",
										},
									},
								},
							},
						},
					},
				},
			))
			Expect(obj.Spec.Gardenlet.Bootstrap).To(PointTo(Equal(BootstrapServiceAccount)))
			Expect(obj.Spec.Gardenlet.MergeWithParent).To(PointTo(Equal(false)))
		})
	})

	Describe("GardenletDeployment defaulting", func() {
		It("should default GardenletDeployment field", func() {
			obj.Spec.Gardenlet = &Gardenlet{}
			SetObjectDefaults_ManagedSeed(obj)

			Expect(obj.Spec.Gardenlet.Deployment.ReplicaCount).To(Equal(pointer.Int32(2)))
			Expect(obj.Spec.Gardenlet.Deployment.RevisionHistoryLimit).To(Equal(pointer.Int32(2)))
			Expect(obj.Spec.Gardenlet.Deployment.Image).NotTo(BeNil())
			Expect(obj.Spec.Gardenlet.Deployment.VPA).To(Equal(pointer.Bool(true)))
		})

		It("should not overwrite the already set values for GardenletDeployment field", func() {
			obj.Spec.Gardenlet = &Gardenlet{
				Deployment: &GardenletDeployment{
					ReplicaCount:         pointer.Int32(3),
					RevisionHistoryLimit: pointer.Int32(3),
					VPA:                  pointer.Bool(false),
				},
			}
			SetObjectDefaults_ManagedSeed(obj)

			Expect(obj.Spec.Gardenlet.Deployment.ReplicaCount).To(Equal(pointer.Int32(3)))
			Expect(obj.Spec.Gardenlet.Deployment.RevisionHistoryLimit).To(Equal(pointer.Int32(3)))
			Expect(obj.Spec.Gardenlet.Deployment.Image).NotTo(BeNil())
			Expect(obj.Spec.Gardenlet.Deployment.VPA).To(Equal(pointer.Bool(false)))
		})
	})

	Describe("Image defaulting", func() {
		It("should default pull policy to IfNotPresent", func() {
			obj.Spec.Gardenlet = &Gardenlet{}
			SetObjectDefaults_ManagedSeed(obj)

			Expect(obj.Spec.Gardenlet.Deployment.Image).To(Equal(&Image{
				PullPolicy: pullPolicyPtr(corev1.PullIfNotPresent),
			}))
		})

		It("should default pull policy to Always if tag is latest", func() {
			obj.Spec.Gardenlet = &Gardenlet{
				Deployment: &GardenletDeployment{
					Image: &Image{Tag: pointer.String("latest")},
				}}

			SetObjectDefaults_ManagedSeed(obj)

			Expect(obj.Spec.Gardenlet.Deployment.Image).To(Equal(&Image{
				Tag:        pointer.String("latest"),
				PullPolicy: pullPolicyPtr(corev1.PullAlways),
			}))
		})

		It("should not overwrite pull policy if tag is not latest", func() {
			obj.Spec.Gardenlet = &Gardenlet{
				Deployment: &GardenletDeployment{
					Image: &Image{
						Tag:        pointer.String("foo"),
						PullPolicy: pullPolicyPtr(corev1.PullNever),
					},
				}}

			SetObjectDefaults_ManagedSeed(obj)

			Expect(obj.Spec.Gardenlet.Deployment.Image).To(Equal(&Image{
				Tag:        pointer.String("foo"),
				PullPolicy: pullPolicyPtr(corev1.PullNever),
			}))
		})
	})
})

func encode(obj runtime.Object) []byte {
	data, _ := json.Marshal(obj)
	return data
}

func pullPolicyPtr(v corev1.PullPolicy) *corev1.PullPolicy { return &v }

func bootstrapPtr(v Bootstrap) *Bootstrap { return &v }
