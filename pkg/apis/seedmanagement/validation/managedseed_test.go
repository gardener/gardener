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
	"k8s.io/component-base/config/v1alpha1"

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/apis/seedmanagement"
	"github.com/gardener/gardener/pkg/apis/seedmanagement/helper"
	. "github.com/gardener/gardener/pkg/apis/seedmanagement/validation"
	configv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/pointer"
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
					Region:   pointer.StringPtr("some-region"),
					SecretRef: corev1.SecretReference{
						Name:      "backup-test",
						Namespace: "garden",
					},
				},
				DNS: core.SeedDNS{
					IngressDomain: pointer.StringPtr("ingress.test.example.com"),
				},
				Networks: core.SeedNetworks{
					Nodes:    pointer.StringPtr("10.250.0.0/16"),
					Pods:     "100.96.0.0/11",
					Services: "100.64.0.0/13",
					ShootDefaults: &core.ShootNetworks{
						Pods:     pointer.StringPtr("10.240.0.0/16"),
						Services: pointer.StringPtr("10.241.0.0/16"),
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
				Name:      name,
				Namespace: namespace,
			},
			Spec: seedmanagement.ManagedSeedSpec{
				Shoot: seedmanagement.Shoot{
					Name: name,
				},
				SeedTemplate: &seedmanagement.SeedTemplate{
					ObjectMeta: seed.ObjectMeta,
					Spec:       seed.Spec,
				},
			},
		}
	})

	Describe("#ValidateManagedSeed", func() {
		It("should forbid empty metadata", func() {
			managedSeed.ObjectMeta = metav1.ObjectMeta{}

			errorList := ValidateManagedSeed(managedSeed)

			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("metadata.name"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("metadata.namespace"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("metadata.namespace"),
				})),
			))
		})

		It("should forbid namespace different from garden", func() {
			managedSeed.Namespace = "foo"

			errorList := ValidateManagedSeed(managedSeed)

			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("metadata.namespace"),
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

		It("should forbid both seed template and gardenlet", func() {
			managedSeed.Spec.Gardenlet = &seedmanagement.Gardenlet{}

			errorList := ValidateManagedSeed(managedSeed)

			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec"),
				})),
			))
		})

		Context("seed template", func() {
			It("should allow valid resources", func() {
				errorList := ValidateManagedSeed(managedSeed)

				Expect(errorList).To(HaveLen(0))
			})

			It("should forbid empty or invalid fields in seed template", func() {
				seedCopy := seed.DeepCopy()
				seedCopy.Spec.Provider.Type = ""
				managedSeed.Spec.SeedTemplate.Spec = seedCopy.Spec

				errorList := ValidateManagedSeed(managedSeed)

				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("spec.seedTemplate.spec.provider.type"),
					})),
				))
			})
		})

		Context("gardenlet", func() {
			var (
				seedx *gardencorev1beta1.Seed
				err   error
			)

			BeforeEach(func() {
				seedx, err = helper.ConvertSeedExternal(seed)
				Expect(err).NotTo(HaveOccurred())

				managedSeed.Spec.SeedTemplate = nil
				managedSeed.Spec.Gardenlet = &seedmanagement.Gardenlet{
					Deployment: &seedmanagement.GardenletDeployment{
						Image: &seedmanagement.Image{
							PullPolicy: pullPolicyPtr(corev1.PullIfNotPresent),
						},
					},
					Config:          gardenletConfiguration(seedx, nil),
					Bootstrap:       bootstrapPtr(seedmanagement.BootstrapToken),
					MergeWithParent: pointer.BoolPtr(true),
				}
			})

			It("should allow valid resources", func() {
				errorList := ValidateManagedSeed(managedSeed)

				Expect(errorList).To(HaveLen(0))
			})

			It("should forbid empty or invalid fields in gardenlet", func() {
				seedxCopy := seedx.DeepCopy()
				seedxCopy.Name = "foo"
				seedxCopy.Spec.Provider.Type = ""

				managedSeed.Spec.Gardenlet.Deployment = &seedmanagement.GardenletDeployment{
					ReplicaCount:         pointer.Int32Ptr(-1),
					RevisionHistoryLimit: pointer.Int32Ptr(-1),
					Image: &seedmanagement.Image{
						PullPolicy: pullPolicyPtr("foo"),
					},
					PodLabels:      map[string]string{"foo!": "bar"},
					PodAnnotations: map[string]string{"bar@": "baz"},
				}
				managedSeed.Spec.Gardenlet.Config = gardenletConfiguration(seedxCopy, nil)
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
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("spec.gardenlet.config.seedConfig.spec.provider.type"),
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
				managedSeed.Spec.Gardenlet.MergeWithParent = pointer.BoolPtr(false)

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
					"Field":  Equal("spec.shoot.name"),
					"Detail": Equal("field is immutable"),
				})),
			))
		})

		It("should forbid changing from seed template to gardenlet", func() {
			newManagedSeed.Spec.SeedTemplate = nil
			newManagedSeed.Spec.Gardenlet = &seedmanagement.Gardenlet{}

			errorList := ValidateManagedSeedUpdate(newManagedSeed, managedSeed)

			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec"),
					"Detail": Equal("changing from seed template to gardenlet and vice versa is not allowed"),
				})),
			))
		})

		Context("seed template", func() {
			It("should allow valid updates", func() {
				errorList := ValidateManagedSeedUpdate(newManagedSeed, managedSeed)

				Expect(errorList).To(HaveLen(0))
			})

			It("should forbid changes to immutable fields in seed template", func() {
				seedCopy := seed.DeepCopy()
				seedCopy.Spec.Backup.Provider = "bar"
				newManagedSeed.Spec.SeedTemplate.Spec = seedCopy.Spec

				errorList := ValidateManagedSeedUpdate(newManagedSeed, managedSeed)

				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.seedTemplate.spec.backup.provider"),
						"Detail": Equal("field is immutable"),
					})),
				))
			})
		})

		Context("gardenlet", func() {
			var (
				seedx *gardencorev1beta1.Seed
				err   error
			)

			BeforeEach(func() {
				seedx, err = helper.ConvertSeedExternal(seed)
				Expect(err).NotTo(HaveOccurred())

				managedSeed.Spec.SeedTemplate = nil
				managedSeed.Spec.Gardenlet = &seedmanagement.Gardenlet{
					Config:          gardenletConfiguration(seedx, nil),
					Bootstrap:       bootstrapPtr(seedmanagement.BootstrapToken),
					MergeWithParent: pointer.BoolPtr(true),
				}

				newManagedSeed = managedSeed.DeepCopy()
				newManagedSeed.ResourceVersion = "1"
			})

			It("should allow valid updates", func() {
				errorList := ValidateManagedSeedUpdate(newManagedSeed, managedSeed)

				Expect(errorList).To(HaveLen(0))
			})

			It("should forbid changes to immutable fields in gardenlet", func() {
				seedxCopy := seedx.DeepCopy()
				seedxCopy.Spec.Backup.Provider = "bar"

				newManagedSeed.Spec.Gardenlet.Config = gardenletConfiguration(seedxCopy, nil)
				newManagedSeed.Spec.Gardenlet.Bootstrap = bootstrapPtr(seedmanagement.BootstrapServiceAccount)
				newManagedSeed.Spec.Gardenlet.MergeWithParent = pointer.BoolPtr(false)

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
})

func gardenletConfiguration(seed *gardencorev1beta1.Seed, gcc *configv1alpha1.GardenClientConnection) *configv1alpha1.GardenletConfiguration {
	return &configv1alpha1.GardenletConfiguration{
		TypeMeta: metav1.TypeMeta{
			APIVersion: configv1alpha1.SchemeGroupVersion.String(),
			Kind:       "GardenletConfiguration",
		},
		SeedConfig: &configv1alpha1.SeedConfig{
			Seed: *seed,
		},
		GardenClientConnection: gcc,
	}
}

func pullPolicyPtr(v corev1.PullPolicy) *corev1.PullPolicy { return &v }

func bootstrapPtr(v seedmanagement.Bootstrap) *seedmanagement.Bootstrap { return &v }
