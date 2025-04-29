// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorehelper "github.com/gardener/gardener/pkg/apis/core/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/apis/seedmanagement"
	. "github.com/gardener/gardener/pkg/apis/seedmanagement/validation"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
)

var _ = Describe("Gardenlet Validation Tests", func() {
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
					Region:   ptr.To("some-region"),
					CredentialsRef: &corev1.ObjectReference{
						APIVersion: "v1",
						Kind:       "Secret",
						Name:       "backup-test",
						Namespace:  "garden",
					},
					SecretRef: corev1.SecretReference{
						Name:      "backup-test",
						Namespace: "garden",
					},
				},
				DNS: core.SeedDNS{
					Provider: &core.SeedDNSProvider{
						Type: "foo",
						SecretRef: corev1.SecretReference{
							Name:      "secret",
							Namespace: "namespace",
						},
					},
				},
				Ingress: &core.Ingress{
					Domain: "ingress.test.example.com",
					Controller: core.IngressController{
						Kind: "nginx",
					},
				},
				Networks: core.SeedNetworks{
					Nodes:    ptr.To("10.250.0.0/16"),
					Pods:     "100.96.0.0/11",
					Services: "100.64.0.0/13",
					ShootDefaults: &core.ShootNetworks{
						Pods:     ptr.To("10.240.0.0/16"),
						Services: ptr.To("10.241.0.0/16"),
					},
				},
				Provider: core.SeedProvider{
					Type:   "foo",
					Region: "some-region",
				},
				Taints: []core.SeedTaint{
					{Key: "foo"},
				},
			},
		}

		gardenlet *seedmanagement.Gardenlet
	)

	BeforeEach(func() {
		gardenlet = &seedmanagement.Gardenlet{
			ObjectMeta: metav1.ObjectMeta{
				Name:       name,
				Namespace:  namespace,
				Generation: 1,
			},
			Spec: seedmanagement.GardenletSpec{
				Deployment: seedmanagement.GardenletSelfDeployment{
					Helm: seedmanagement.GardenletHelm{
						OCIRepository: core.OCIRepository{
							Ref: ptr.To("some-ref:v0.0.0"),
						},
					},
				},
			},
			Status: seedmanagement.GardenletStatus{
				ObservedGeneration: 1,
			},
		}
	})

	Describe("#ValidateGardenlet", func() {
		DescribeTable("Gardenlet metadata",
			func(objectMeta metav1.ObjectMeta, matcher gomegatypes.GomegaMatcher) {
				gardenlet.ObjectMeta = objectMeta

				Expect(ValidateGardenlet(gardenlet)).To(matcher)
			},

			Entry("should forbid Gardenlet with empty metadata",
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
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeForbidden),
						"Field":  Equal("metadata.namespace"),
						"Detail": Equal("namespace must be garden"),
					})),
				),
			),
			Entry("should forbid Gardenlet with empty name",
				metav1.ObjectMeta{Name: "", Namespace: namespace},
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("metadata.name"),
				}))),
			),
			Entry("should forbid Gardenlet with '.' in the name (not a DNS-1123 label compliant name)",
				metav1.ObjectMeta{Name: "gardenlet.test", Namespace: namespace},
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("metadata.name"),
				}))),
			),
			Entry("should forbid Gardenlet with '_' in the name (not a DNS-1123 subdomain)",
				metav1.ObjectMeta{Name: "gardenlet_test", Namespace: namespace},
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("metadata.name"),
				}))),
			),
		)

		Context("operation annotation", func() {
			It("should do nothing if the operation annotation is not set", func() {
				Expect(ValidateGardenlet(gardenlet)).To(BeEmpty())
			})

			It("should return an error if the operation annotation is invalid", func() {
				metav1.SetMetaDataAnnotation(&gardenlet.ObjectMeta, "gardener.cloud/operation", "foo-bar")
				Expect(ValidateGardenlet(gardenlet)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeNotSupported),
					"Field": Equal("metadata.annotations[gardener.cloud/operation]"),
				}))))
			})

			DescribeTable("should return nothing if the operation annotations is valid", func(operation string) {
				metav1.SetMetaDataAnnotation(&gardenlet.ObjectMeta, "gardener.cloud/operation", operation)
				Expect(ValidateGardenlet(gardenlet)).To(BeEmpty())
			},
				Entry("reconcile", "reconcile"),
				Entry("renew-kubeconfig", "renew-kubeconfig"),
				Entry("force-redeploy", "force-redeploy"),
			)
		})

		Context("gardenlet", func() {
			var (
				seedx *gardencorev1beta1.Seed
				err   error
			)

			BeforeEach(func() {
				seedx, err = gardencorehelper.ConvertSeedExternal(seed)
				Expect(err).NotTo(HaveOccurred())

				gardenlet.Spec = seedmanagement.GardenletSpec{
					Deployment: seedmanagement.GardenletSelfDeployment{
						GardenletDeployment: seedmanagement.GardenletDeployment{
							Image: &seedmanagement.Image{
								PullPolicy: ptr.To(corev1.PullIfNotPresent),
							},
						},
						Helm: seedmanagement.GardenletHelm{
							OCIRepository: core.OCIRepository{
								Ref: ptr.To("some-ref:v0.0.0"),
							},
						},
					},
					Config: gardenletConfiguration(seedx, nil),
				}
			})

			It("should allow valid resources", func() {
				Expect(ValidateGardenlet(gardenlet)).To(BeEmpty())
			})

			It("should forbid empty or invalid fields in gardenlet", func() {
				seedx.Name = "foo"
				seedx.Spec.Networks.Nodes = ptr.To("")

				gardenlet.Spec.Deployment.GardenletDeployment = seedmanagement.GardenletDeployment{
					ReplicaCount:         ptr.To(int32(-1)),
					RevisionHistoryLimit: ptr.To(int32(-1)),
					ServiceAccountName:   ptr.To(""),
					Image: &seedmanagement.Image{
						Repository: ptr.To(""),
						Tag:        ptr.To(""),
						PullPolicy: ptr.To[corev1.PullPolicy]("foo"),
					},
					PodLabels:      map[string]string{"foo!": "bar"},
					PodAnnotations: map[string]string{"bar@": "baz"},
				}
				gardenlet.Spec.Config = gardenletConfiguration(seedx, nil)

				Expect(ValidateGardenlet(gardenlet)).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.deployment.replicaCount"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.deployment.revisionHistoryLimit"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.deployment.serviceAccountName"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.deployment.image.repository"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.deployment.image.tag"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeNotSupported),
						"Field": Equal("spec.deployment.image.pullPolicy"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.deployment.podLabels"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.deployment.podAnnotations"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeForbidden),
						"Field": Equal("spec.config.seedConfig.metadata.name"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.config.seedConfig.spec.networks.nodes"),
					})),
				))
			})

			It("should forbid garden client connection kubeconfig if bootstrap is specified", func() {
				gardenlet.Spec.Config = gardenletConfiguration(seedx,
					&gardenletconfigv1alpha1.GardenClientConnection{
						ClientConnectionConfiguration: v1alpha1.ClientConnectionConfiguration{
							Kubeconfig: "foo",
						},
					},
				)

				Expect(ValidateGardenlet(gardenlet)).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeForbidden),
						"Field": Equal("spec.config.gardenClientConnection.kubeconfig"),
					})),
				))
			})
		})
	})

	Describe("#ValidateGardenletUpdate", func() {
		var (
			newGardenlet *seedmanagement.Gardenlet
		)

		BeforeEach(func() {
			newGardenlet = gardenlet.DeepCopy()
			newGardenlet.ResourceVersion = "1"
		})

		Context("operation annotation", func() {
			DescribeTable("should do nothing if a valid operation annotation is added", func(operation string) {
				metav1.SetMetaDataAnnotation(&newGardenlet.ObjectMeta, "gardener.cloud/operation", operation)
				Expect(ValidateGardenletUpdate(newGardenlet, gardenlet)).To(BeEmpty())
			},
				Entry("reconcile", "reconcile"),
				Entry("renew-kubeconfig", "renew-kubeconfig"),
			)

			DescribeTable("should do nothing if a valid operation annotation is removed", func(operation string) {
				metav1.SetMetaDataAnnotation(&gardenlet.ObjectMeta, "gardener.cloud/operation", operation)
				Expect(ValidateGardenletUpdate(newGardenlet, gardenlet)).To(BeEmpty())
			},
				Entry("reconcile", "reconcile"),
				Entry("renew-kubeconfig", "renew-kubeconfig"),
			)

			DescribeTable("should do nothing if a valid operation annotation does not change during an update", func(operation string) {
				metav1.SetMetaDataAnnotation(&gardenlet.ObjectMeta, "gardener.cloud/operation", operation)
				metav1.SetMetaDataAnnotation(&newGardenlet.ObjectMeta, "gardener.cloud/operation", operation)
				Expect(ValidateGardenletUpdate(newGardenlet, gardenlet)).To(BeEmpty())
			},
				Entry("reconcile", "reconcile"),
				Entry("renew-kubeconfig", "renew-kubeconfig"),
			)

			It("should return an error if a valid operation should be overwritten with a different valid operation", func() {
				metav1.SetMetaDataAnnotation(&gardenlet.ObjectMeta, "gardener.cloud/operation", "reconcile")
				metav1.SetMetaDataAnnotation(&newGardenlet.ObjectMeta, "gardener.cloud/operation", "renew-kubeconfig")
				Expect(ValidateGardenletUpdate(newGardenlet, gardenlet)).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeForbidden),
						"Field":  Equal("metadata.annotations[gardener.cloud/operation]"),
						"Detail": Equal("must not overwrite operation \"reconcile\" with \"renew-kubeconfig\""),
					}))))
			})
		})

		It("should forbid changes to immutable metadata fields", func() {
			newGardenlet.Name = name + "x"

			errorList := ValidateGardenletUpdate(newGardenlet, gardenlet)

			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("metadata.name"),
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

				gardenlet.Spec.Config = gardenletConfiguration(seedx, nil)

				newGardenlet = gardenlet.DeepCopy()
				newGardenlet.ResourceVersion = "1"
			})

			It("should allow valid updates", func() {
				Expect(ValidateGardenletUpdate(newGardenlet, gardenlet)).To(BeEmpty())
			})

			It("should forbid changes to immutable fields in gardenlet", func() {
				seedxCopy := seedx.DeepCopy()
				seedxCopy.Spec.Backup.Provider = "bar"

				newGardenlet.Spec.Config = gardenletConfiguration(seedxCopy, nil)

				Expect(ValidateGardenletUpdate(newGardenlet, gardenlet)).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.config.seedConfig.spec.backup.provider"),
						"Detail": Equal("field is immutable"),
					})),
				))
			})
		})
	})

	Describe("#ValidateGardenletStatusUpdate", func() {
		var (
			newGardenlet *seedmanagement.Gardenlet
		)

		BeforeEach(func() {
			newGardenlet = gardenlet.DeepCopy()
			newGardenlet.ResourceVersion = "1"
		})

		It("should allow valid status updates", func() {
			Expect(ValidateGardenletStatusUpdate(newGardenlet, gardenlet)).To(BeEmpty())
		})

		It("should forbid negative observed generation", func() {
			newGardenlet.Status.ObservedGeneration = -1

			Expect(ValidateGardenletStatusUpdate(newGardenlet, gardenlet)).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("status.observedGeneration"),
				})),
			))
		})
	})
})
