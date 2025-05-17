// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
)

var _ = Describe("Defaults", func() {
	var obj *Gardenlet

	BeforeEach(func() {
		obj = &Gardenlet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
		}
	})

	Describe("Gardenlet defaulting", func() {
		It("should default gardenlet configuration", func() {
			SetObjectDefaults_Gardenlet(obj)

			Expect(obj.Spec.Deployment).NotTo(BeNil())
			Expect(obj.Spec.Config).To(Equal(runtime.RawExtension{
				Object: &gardenletconfigv1alpha1.GardenletConfiguration{
					TypeMeta: metav1.TypeMeta{
						APIVersion: gardenletconfigv1alpha1.SchemeGroupVersion.String(),
						Kind:       "GardenletConfiguration",
					},
					Resources: &gardenletconfigv1alpha1.ResourcesConfiguration{
						Capacity: corev1.ResourceList{
							gardencorev1beta1.ResourceShoots: resource.MustParse("250"),
						},
					},
					SeedConfig: &gardenletconfigv1alpha1.SeedConfig{},
				}}))
		})

		It("should default gardenlet configuration, and backup secret reference if backup is specified", func() {
			obj.Spec.Config = runtime.RawExtension{
				Raw: encode(&gardenletconfigv1alpha1.GardenletConfiguration{
					TypeMeta: metav1.TypeMeta{
						APIVersion: gardenletconfigv1alpha1.SchemeGroupVersion.String(),
						Kind:       "GardenletConfiguration",
					},
					SeedConfig: &gardenletconfigv1alpha1.SeedConfig{
						SeedTemplate: gardencorev1beta1.SeedTemplate{
							Spec: gardencorev1beta1.SeedSpec{
								Backup: &gardencorev1beta1.Backup{},
							},
						},
					},
				}),
			}

			SetObjectDefaults_Gardenlet(obj)

			Expect(obj.Spec.Config).To(Equal(
				runtime.RawExtension{
					Object: &gardenletconfigv1alpha1.GardenletConfiguration{
						TypeMeta: metav1.TypeMeta{
							APIVersion: gardenletconfigv1alpha1.SchemeGroupVersion.String(),
							Kind:       "GardenletConfiguration",
						},
						Resources: &gardenletconfigv1alpha1.ResourcesConfiguration{
							Capacity: corev1.ResourceList{
								gardencorev1beta1.ResourceShoots: resource.MustParse("250"),
							},
						},
						SeedConfig: &gardenletconfigv1alpha1.SeedConfig{
							SeedTemplate: gardencorev1beta1.SeedTemplate{
								Spec: gardencorev1beta1.SeedSpec{
									Backup: &gardencorev1beta1.Backup{
										SecretRef: corev1.SecretReference{
											Name:      "backup-" + name,
											Namespace: namespace,
										},
										CredentialsRef: &corev1.ObjectReference{
											APIVersion: "v1",
											Kind:       "Secret",
											Name:       "backup-" + name,
											Namespace:  namespace,
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
			obj.Spec.Config = runtime.RawExtension{
				Raw: encode(&gardenletconfigv1alpha1.GardenletConfiguration{
					TypeMeta: metav1.TypeMeta{
						APIVersion: gardenletconfigv1alpha1.SchemeGroupVersion.String(),
						Kind:       "GardenletConfiguration",
					},
					Resources: &gardenletconfigv1alpha1.ResourcesConfiguration{
						Capacity: corev1.ResourceList{
							gardencorev1beta1.ResourceShoots: resource.MustParse("300"),
						},
					},
					SeedConfig: &gardenletconfigv1alpha1.SeedConfig{
						SeedTemplate: gardencorev1beta1.SeedTemplate{
							Spec: gardencorev1beta1.SeedSpec{
								Backup: &gardencorev1beta1.Backup{
									CredentialsRef: &corev1.ObjectReference{
										APIVersion: "v1",
										Kind:       "Secret",
										Name:       "foo",
										Namespace:  "bar",
									},
								},
								Ingress: &gardencorev1beta1.Ingress{
									Controller: gardencorev1beta1.IngressController{
										Kind: "foobar",
									},
								},
							},
						},
					},
				}),
			}

			SetObjectDefaults_Gardenlet(obj)

			Expect(obj.Spec.Config).To(Equal(
				runtime.RawExtension{
					Object: &gardenletconfigv1alpha1.GardenletConfiguration{
						TypeMeta: metav1.TypeMeta{
							APIVersion: gardenletconfigv1alpha1.SchemeGroupVersion.String(),
							Kind:       "GardenletConfiguration",
						},
						Resources: &gardenletconfigv1alpha1.ResourcesConfiguration{
							Capacity: corev1.ResourceList{
								gardencorev1beta1.ResourceShoots: resource.MustParse("300"),
							},
						},
						SeedConfig: &gardenletconfigv1alpha1.SeedConfig{
							SeedTemplate: gardencorev1beta1.SeedTemplate{
								Spec: gardencorev1beta1.SeedSpec{
									Backup: &gardencorev1beta1.Backup{
										CredentialsRef: &corev1.ObjectReference{
											APIVersion: "v1",
											Kind:       "Secret",
											Name:       "foo",
											Namespace:  "bar",
										},
									},
									Ingress: &gardencorev1beta1.Ingress{
										Controller: gardencorev1beta1.IngressController{
											Kind: "foobar",
										},
									},
								},
							},
						},
					},
				},
			))
		})
	})

	Describe("GardenletDeployment defaulting", func() {
		It("should default GardenletDeployment field", func() {
			SetObjectDefaults_Gardenlet(obj)

			Expect(obj.Spec.Deployment.ReplicaCount).To(Equal(ptr.To[int32](2)))
			Expect(obj.Spec.Deployment.RevisionHistoryLimit).To(Equal(ptr.To[int32](2)))
			Expect(obj.Spec.Deployment.Image).NotTo(BeNil())
		})

		It("should not overwrite the already set values for GardenletDeployment field", func() {
			obj.Spec.Deployment = GardenletSelfDeployment{GardenletDeployment: GardenletDeployment{
				ReplicaCount:         ptr.To[int32](3),
				RevisionHistoryLimit: ptr.To[int32](3),
			}}
			SetObjectDefaults_Gardenlet(obj)

			Expect(obj.Spec.Deployment.ReplicaCount).To(Equal(ptr.To[int32](3)))
			Expect(obj.Spec.Deployment.RevisionHistoryLimit).To(Equal(ptr.To[int32](3)))
			Expect(obj.Spec.Deployment.Image).NotTo(BeNil())
		})
	})

	Describe("Image defaulting", func() {
		It("should default pull policy to IfNotPresent", func() {
			SetObjectDefaults_Gardenlet(obj)

			Expect(obj.Spec.Deployment.Image).To(Equal(&Image{
				PullPolicy: ptr.To(corev1.PullIfNotPresent),
			}))
		})

		It("should default pull policy to Always if tag is latest", func() {
			obj.Spec.Deployment = GardenletSelfDeployment{GardenletDeployment: GardenletDeployment{
				Image: &Image{Tag: ptr.To("latest")},
			}}

			SetObjectDefaults_Gardenlet(obj)

			Expect(obj.Spec.Deployment.Image).To(Equal(&Image{
				Tag:        ptr.To("latest"),
				PullPolicy: ptr.To(corev1.PullAlways),
			}))
		})

		It("should not overwrite pull policy if tag is not latest", func() {
			obj.Spec.Deployment = GardenletSelfDeployment{GardenletDeployment: GardenletDeployment{
				Image: &Image{
					Tag:        ptr.To("foo"),
					PullPolicy: ptr.To(corev1.PullNever),
				},
			}}

			SetObjectDefaults_Gardenlet(obj)

			Expect(obj.Spec.Deployment.Image).To(Equal(&Image{
				Tag:        ptr.To("foo"),
				PullPolicy: ptr.To(corev1.PullNever),
			}))
		})
	})
})
