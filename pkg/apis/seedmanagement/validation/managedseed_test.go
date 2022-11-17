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

package validation_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/component-base/config/v1alpha1"
	"k8s.io/utils/pointer"

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorehelper "github.com/gardener/gardener/pkg/apis/core/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/apis/seedmanagement"
	. "github.com/gardener/gardener/pkg/apis/seedmanagement/validation"
	configv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
)

const (
	name      = "test"
	namespace = "garden"
)

var _ = Describe("ManagedSeed Validation Tests", func() {
	var (
		seed = &core.Seed{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"foo": "bar",
				},
			},
			Spec: core.SeedSpec{
				Backup: &core.SeedBackup{
					Provider: "foo",
					Region:   pointer.String("some-region"),
					SecretRef: corev1.SecretReference{
						Name:      "backup-test",
						Namespace: "garden",
					},
				},
				DNS: core.SeedDNS{
					IngressDomain: pointer.String("ingress.test.example.com"),
				},
				Networks: core.SeedNetworks{
					Nodes:    pointer.String("10.250.0.0/16"),
					Pods:     "100.96.0.0/11",
					Services: "100.64.0.0/13",
					ShootDefaults: &core.ShootNetworks{
						Pods:     pointer.String("10.240.0.0/16"),
						Services: pointer.String("10.241.0.0/16"),
					},
				},
				Provider: core.SeedProvider{
					Type:   "foo",
					Region: "some-region",
				},
				SecretRef: &corev1.SecretReference{
					Name:      "seed-test",
					Namespace: "garden",
				},
				Taints: []core.SeedTaint{
					{Key: "foo"},
				},
			},
		}

		managedSeed *seedmanagement.ManagedSeed
	)

	BeforeEach(func() {
		managedSeed = &seedmanagement.ManagedSeed{
			ObjectMeta: metav1.ObjectMeta{
				Name:       name,
				Namespace:  namespace,
				Generation: 1,
			},
			Spec: seedmanagement.ManagedSeedSpec{
				Shoot: &seedmanagement.Shoot{
					Name: name,
				},
				Gardenlet: &seedmanagement.Gardenlet{},
			},
			Status: seedmanagement.ManagedSeedStatus{
				ObservedGeneration: 1,
			},
		}
	})

	Describe("#ValidateManagedSeed", func() {
		DescribeTable("ManagedSeed metadata",
			func(objectMeta metav1.ObjectMeta, matcher gomegatypes.GomegaMatcher) {
				managedSeed.ObjectMeta = objectMeta

				errorList := ValidateManagedSeed(managedSeed)

				Expect(errorList).To(matcher)
			},

			Entry("should forbid ManagedSeed with empty metadata",
				metav1.ObjectMeta{},
				ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("metadata.name"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("metadata.namespace"),
					})),
				),
			),
			Entry("should forbid ManagedSeed with empty name",
				metav1.ObjectMeta{Name: "", Namespace: namespace},
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("metadata.name"),
				}))),
			),
			Entry("should forbid ManagedSeed with '.' in the name (not a DNS-1123 label compliant name)",
				metav1.ObjectMeta{Name: "managedseed.test", Namespace: namespace},
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("metadata.name"),
				}))),
			),
			Entry("should forbid ManagedSeed with '_' in the name (not a DNS-1123 subdomain)",
				metav1.ObjectMeta{Name: "managedseed_test", Namespace: namespace},
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("metadata.name"),
				}))),
			),
		)

		It("should forbid nil shoot", func() {
			managedSeed.Spec.Shoot = nil

			errorList := ValidateManagedSeed(managedSeed)

			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.shoot"),
				})),
			))
		})

		It("should forbid empty shoot name", func() {
			managedSeed.Spec.Shoot.Name = ""

			errorList := ValidateManagedSeed(managedSeed)

			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.shoot.name"),
				})),
			))
		})

		It("should forbid nil gardenlet", func() {
			managedSeed.Spec.Gardenlet = nil

			errorList := ValidateManagedSeed(managedSeed)

			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.gardenlet"),
				})),
			))
		})

		Context("gardenlet", func() {
			var (
				seedx *gardencorev1beta1.Seed
				err   error
			)

			BeforeEach(func() {
				seedx, err = gardencorehelper.ConvertSeedExternal(seed)
				Expect(err).NotTo(HaveOccurred())

				managedSeed.Spec.Gardenlet = &seedmanagement.Gardenlet{
					Deployment: &seedmanagement.GardenletDeployment{
						Image: &seedmanagement.Image{
							PullPolicy: pullPolicyPtr(corev1.PullIfNotPresent),
						},
					},
					Config:          gardenletConfiguration(seedx, nil),
					Bootstrap:       bootstrapPtr(seedmanagement.BootstrapToken),
					MergeWithParent: pointer.Bool(true),
				}
			})

			It("should allow valid resources", func() {
				errorList := ValidateManagedSeed(managedSeed)

				Expect(errorList).To(BeEmpty())
			})

			It("should forbid empty or invalid fields in gardenlet", func() {
				seedx.Name = "foo"
				seedx.Spec.Networks.Nodes = pointer.String("")

				managedSeed.Spec.Gardenlet.Deployment = &seedmanagement.GardenletDeployment{
					ReplicaCount:         pointer.Int32(-1),
					RevisionHistoryLimit: pointer.Int32(-1),
					ServiceAccountName:   pointer.String(""),
					Image: &seedmanagement.Image{
						Repository: pointer.String(""),
						Tag:        pointer.String(""),
						PullPolicy: pullPolicyPtr("foo"),
					},
					PodLabels:      map[string]string{"foo!": "bar"},
					PodAnnotations: map[string]string{"bar@": "baz"},
				}
				managedSeed.Spec.Gardenlet.Config = gardenletConfiguration(seedx, nil)
				managedSeed.Spec.Gardenlet.Bootstrap = bootstrapPtr("foo")

				errorList := ValidateManagedSeed(managedSeed)

				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.gardenlet.deployment.replicaCount"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.gardenlet.deployment.revisionHistoryLimit"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.gardenlet.deployment.serviceAccountName"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.gardenlet.deployment.image.repository"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.gardenlet.deployment.image.tag"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeNotSupported),
						"Field": Equal("spec.gardenlet.deployment.image.pullPolicy"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.gardenlet.deployment.podLabels"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.gardenlet.deployment.podAnnotations"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeForbidden),
						"Field": Equal("spec.gardenlet.config.seedConfig.metadata.name"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.gardenlet.config.seedConfig.spec.networks.nodes"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeNotSupported),
						"Field": Equal("spec.gardenlet.bootstrap"),
					})),
				))
			})

			It("should forbid garden client connection kubeconfig if bootstrap is specified", func() {
				managedSeed.Spec.Gardenlet.Config = gardenletConfiguration(seedx,
					&configv1alpha1.GardenClientConnection{
						ClientConnectionConfiguration: v1alpha1.ClientConnectionConfiguration{
							Kubeconfig: "foo",
						},
					})

				errorList := ValidateManagedSeed(managedSeed)

				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeForbidden),
						"Field": Equal("spec.gardenlet.config.gardenClientConnection.kubeconfig"),
					})),
				))
			})

			It("should forbid garden client connection bootstrap kubeconfig and kubeconfig secret if bootstrap is not specified", func() {
				managedSeed.Spec.Gardenlet.Config = gardenletConfiguration(seedx,
					&configv1alpha1.GardenClientConnection{
						BootstrapKubeconfig: &corev1.SecretReference{
							Name:      name,
							Namespace: namespace,
						},
						KubeconfigSecret: &corev1.SecretReference{
							Name:      name,
							Namespace: namespace,
						},
					})
				managedSeed.Spec.Gardenlet.Bootstrap = bootstrapPtr(seedmanagement.BootstrapNone)
				managedSeed.Spec.Gardenlet.MergeWithParent = pointer.Bool(false)

				errorList := ValidateManagedSeed(managedSeed)

				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("spec.gardenlet.config.gardenClientConnection.kubeconfig"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeForbidden),
						"Field": Equal("spec.gardenlet.config.gardenClientConnection.bootstrapKubeconfig"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeForbidden),
						"Field": Equal("spec.gardenlet.config.gardenClientConnection.kubeconfigSecret"),
					})),
				))
			})
		})
	})

	Describe("#ValidateManagedSeedUpdate", func() {
		var (
			newManagedSeed *seedmanagement.ManagedSeed
		)

		BeforeEach(func() {
			newManagedSeed = managedSeed.DeepCopy()
			newManagedSeed.ResourceVersion = "1"
		})

		It("should forbid changes to immutable metadata fields", func() {
			newManagedSeed.Name = name + "x"

			errorList := ValidateManagedSeedUpdate(newManagedSeed, managedSeed)

			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("metadata.name"),
					"Detail": Equal("field is immutable"),
				})),
			))
		})

		It("should forbid changing the shoot name", func() {
			newManagedSeed.Spec.Shoot.Name = name + "x"

			errorList := ValidateManagedSeedUpdate(newManagedSeed, managedSeed)

			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.shoot"),
					"Detail": Equal("field is immutable"),
				})),
			))
		})

		Context("gardenlet", func() {
			var (
				seedx *gardencorev1beta1.Seed
				err   error
			)

			BeforeEach(func() {
				seedx, err = gardencorehelper.ConvertSeedExternal(seed)
				Expect(err).NotTo(HaveOccurred())

				managedSeed.Spec.Gardenlet = &seedmanagement.Gardenlet{
					Config:          gardenletConfiguration(seedx, nil),
					Bootstrap:       bootstrapPtr(seedmanagement.BootstrapToken),
					MergeWithParent: pointer.Bool(true),
				}

				newManagedSeed = managedSeed.DeepCopy()
				newManagedSeed.ResourceVersion = "1"
			})

			It("should allow valid updates", func() {
				errorList := ValidateManagedSeedUpdate(newManagedSeed, managedSeed)

				Expect(errorList).To(BeEmpty())
			})

			It("should forbid changes to immutable fields in gardenlet", func() {
				seedxCopy := seedx.DeepCopy()
				seedxCopy.Spec.Backup.Provider = "bar"

				newManagedSeed.Spec.Gardenlet.Config = gardenletConfiguration(seedxCopy, nil)
				newManagedSeed.Spec.Gardenlet.Bootstrap = bootstrapPtr(seedmanagement.BootstrapServiceAccount)
				newManagedSeed.Spec.Gardenlet.MergeWithParent = pointer.Bool(false)

				errorList := ValidateManagedSeedUpdate(newManagedSeed, managedSeed)

				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.gardenlet.config.seedConfig.spec.backup.provider"),
						"Detail": Equal("field is immutable"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.gardenlet.bootstrap"),
						"Detail": Equal("field is immutable"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.gardenlet.mergeWithParent"),
						"Detail": Equal("field is immutable"),
					})),
				))
			})
		})
	})

	Describe("#ValidateManagedSeedStatusUpdate", func() {
		var (
			newManagedSeed *seedmanagement.ManagedSeed
		)

		BeforeEach(func() {
			newManagedSeed = managedSeed.DeepCopy()
			newManagedSeed.ResourceVersion = "1"
		})

		It("should allow valid status updates", func() {
			errorList := ValidateManagedSeedStatusUpdate(newManagedSeed, managedSeed)

			Expect(errorList).To(BeEmpty())
		})

		It("should forbid negative observed generation", func() {
			newManagedSeed.Status.ObservedGeneration = -1

			errorList := ValidateManagedSeedStatusUpdate(newManagedSeed, managedSeed)

			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("status.observedGeneration"),
				})),
			))
		})
	})
})

func gardenletConfiguration(seed *gardencorev1beta1.Seed, gcc *configv1alpha1.GardenClientConnection) *configv1alpha1.GardenletConfiguration {
	return &configv1alpha1.GardenletConfiguration{
		TypeMeta: metav1.TypeMeta{
			APIVersion: configv1alpha1.SchemeGroupVersion.String(),
			Kind:       "GardenletConfiguration",
		},
		SeedConfig: &configv1alpha1.SeedConfig{
			SeedTemplate: gardencorev1beta1.SeedTemplate{
				ObjectMeta: seed.ObjectMeta,
				Spec:       seed.Spec,
			},
		},
		GardenClientConnection: gcc,
	}
}

func pullPolicyPtr(v corev1.PullPolicy) *corev1.PullPolicy { return &v }

func bootstrapPtr(v seedmanagement.Bootstrap) *seedmanagement.Bootstrap { return &v }
