// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"fmt"

	"github.com/Masterminds/semver/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/component-base/featuregate"
	"k8s.io/utils/pointer"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	. "github.com/gardener/gardener/pkg/apis/operator/v1alpha1/validation"
	"github.com/gardener/gardener/pkg/features"
)

var _ = Describe("Validation Tests", func() {
	Describe("#ValidateGarden", func() {
		var garden *operatorv1alpha1.Garden

		BeforeEach(func() {
			garden = &operatorv1alpha1.Garden{
				ObjectMeta: metav1.ObjectMeta{
					Name: "garden",
				},
				Spec: operatorv1alpha1.GardenSpec{
					RuntimeCluster: operatorv1alpha1.RuntimeCluster{
						Networking: operatorv1alpha1.RuntimeNetworking{
							Pods:     "10.1.0.0/16",
							Services: "10.2.0.0/16",
						},
					},
					VirtualCluster: operatorv1alpha1.VirtualCluster{
						DNS: operatorv1alpha1.DNS{
							Domains: []string{"foo.bar.com"},
						},
						Kubernetes: operatorv1alpha1.Kubernetes{
							Version: "1.26.3",
						},
						Networking: operatorv1alpha1.Networking{
							Services: "10.4.0.0/16",
						},
					},
				},
			}
		})

		Context("operation annotation", func() {
			It("should do nothing if the operation annotation is not set", func() {
				Expect(ValidateGarden(garden)).To(BeEmpty())
			})

			DescribeTable("should not allow starting/completing rotation when garden has deletion timestamp",
				func(operation string) {
					garden.DeletionTimestamp = &metav1.Time{}
					metav1.SetMetaDataAnnotation(&garden.ObjectMeta, "gardener.cloud/operation", operation)

					Expect(ValidateGarden(garden)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeForbidden),
						"Field": Equal("metadata.annotations[gardener.cloud/operation]"),
					}))))
				},

				Entry("start all rotations", "rotate-credentials-start"),
				Entry("complete all rotations", "rotate-credentials-complete"),
				Entry("start CA rotation", "rotate-ca-start"),
				Entry("complete CA rotation", "rotate-ca-complete"),
				Entry("start ServiceAccount key rotation", "rotate-serviceaccount-key-start"),
				Entry("complete ServiceAccount key rotation", "rotate-serviceaccount-key-complete"),
				Entry("start ETCD encryption key rotation", "rotate-etcd-encryption-key-start"),
				Entry("complete ETCD encryption key rotation", "rotate-etcd-encryption-key-complete"),
				Entry("start Observability key rotation", "rotate-observability-credentials"),
			)

			DescribeTable("starting rotation of all credentials",
				func(allowed bool, status operatorv1alpha1.GardenStatus) {
					metav1.SetMetaDataAnnotation(&garden.ObjectMeta, "gardener.cloud/operation", "rotate-credentials-start")
					garden.Status = status

					matcher := BeEmpty()
					if !allowed {
						matcher = ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeForbidden),
							"Field": Equal("metadata.annotations[gardener.cloud/operation]"),
						})))
					}

					Expect(ValidateGarden(garden)).To(matcher)
				},

				Entry("ca rotation phase is preparing", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							CertificateAuthorities: &gardencorev1beta1.CARotation{
								Phase: gardencorev1beta1.RotationPreparing,
							},
						},
					},
				}),
				Entry("sa rotation phase is preparing", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							ServiceAccountKey: &gardencorev1beta1.ServiceAccountKeyRotation{
								Phase: gardencorev1beta1.RotationPreparing,
							},
						},
					},
				}),
				Entry("etcd key rotation phase is preparing", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							ETCDEncryptionKey: &gardencorev1beta1.ETCDEncryptionKeyRotation{
								Phase: gardencorev1beta1.RotationPreparing,
							},
						},
					},
				}),
				Entry("ca rotation phase is prepared", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							CertificateAuthorities: &gardencorev1beta1.CARotation{
								Phase: gardencorev1beta1.RotationPrepared,
							},
						},
					},
				}),
				Entry("sa rotation phase is prepared", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							ServiceAccountKey: &gardencorev1beta1.ServiceAccountKeyRotation{
								Phase: gardencorev1beta1.RotationPrepared,
							},
						},
					},
				}),
				Entry("etcd key rotation phase is prepared", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							ETCDEncryptionKey: &gardencorev1beta1.ETCDEncryptionKeyRotation{
								Phase: gardencorev1beta1.RotationPrepared,
							},
						},
					},
				}),
				Entry("ca rotation phase is completing", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							CertificateAuthorities: &gardencorev1beta1.CARotation{
								Phase: gardencorev1beta1.RotationCompleting,
							},
						},
					},
				}),
				Entry("sa rotation phase is completing", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							ServiceAccountKey: &gardencorev1beta1.ServiceAccountKeyRotation{
								Phase: gardencorev1beta1.RotationCompleting,
							},
						},
					},
				}),
				Entry("etcd key rotation phase is completing", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							ETCDEncryptionKey: &gardencorev1beta1.ETCDEncryptionKeyRotation{
								Phase: gardencorev1beta1.RotationCompleting,
							},
						},
					},
				}),
				Entry("ca rotation phase is completed", true, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							CertificateAuthorities: &gardencorev1beta1.CARotation{
								Phase: gardencorev1beta1.RotationCompleted,
							},
						},
					},
				}),
				Entry("sa rotation phase is completed", true, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							ServiceAccountKey: &gardencorev1beta1.ServiceAccountKeyRotation{
								Phase: gardencorev1beta1.RotationCompleted,
							},
						},
					},
				}),
				Entry("etcd key rotation phase is completed", true, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							ETCDEncryptionKey: &gardencorev1beta1.ETCDEncryptionKeyRotation{
								Phase: gardencorev1beta1.RotationCompleted,
							},
						},
					},
				}),
			)

			DescribeTable("completing rotation of all credentials",
				func(allowed bool, status operatorv1alpha1.GardenStatus) {
					metav1.SetMetaDataAnnotation(&garden.ObjectMeta, "gardener.cloud/operation", "rotate-credentials-complete")
					garden.Status = status

					matcher := BeEmpty()
					if !allowed {
						matcher = ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeForbidden),
							"Field": Equal("metadata.annotations[gardener.cloud/operation]"),
						})))
					}

					Expect(ValidateGarden(garden)).To(matcher)
				},

				Entry("ca rotation phase is preparing", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							CertificateAuthorities: &gardencorev1beta1.CARotation{
								Phase: gardencorev1beta1.RotationPreparing,
							},
							ServiceAccountKey: &gardencorev1beta1.ServiceAccountKeyRotation{
								Phase: gardencorev1beta1.RotationPrepared,
							},
							ETCDEncryptionKey: &gardencorev1beta1.ETCDEncryptionKeyRotation{
								Phase: gardencorev1beta1.RotationPrepared,
							},
						},
					},
				}),
				Entry("sa rotation phase is preparing", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							CertificateAuthorities: &gardencorev1beta1.CARotation{
								Phase: gardencorev1beta1.RotationPrepared,
							},
							ServiceAccountKey: &gardencorev1beta1.ServiceAccountKeyRotation{
								Phase: gardencorev1beta1.RotationPreparing,
							},
							ETCDEncryptionKey: &gardencorev1beta1.ETCDEncryptionKeyRotation{
								Phase: gardencorev1beta1.RotationPrepared,
							},
						},
					},
				}),
				Entry("etcd key rotation phase is preparing", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							CertificateAuthorities: &gardencorev1beta1.CARotation{
								Phase: gardencorev1beta1.RotationPrepared,
							},
							ServiceAccountKey: &gardencorev1beta1.ServiceAccountKeyRotation{
								Phase: gardencorev1beta1.RotationPrepared,
							},
							ETCDEncryptionKey: &gardencorev1beta1.ETCDEncryptionKeyRotation{
								Phase: gardencorev1beta1.RotationPreparing,
							},
						},
					},
				}),
				Entry("all rotation phases are prepared", true, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							CertificateAuthorities: &gardencorev1beta1.CARotation{
								Phase: gardencorev1beta1.RotationPrepared,
							},
							ServiceAccountKey: &gardencorev1beta1.ServiceAccountKeyRotation{
								Phase: gardencorev1beta1.RotationPrepared,
							},
							ETCDEncryptionKey: &gardencorev1beta1.ETCDEncryptionKeyRotation{
								Phase: gardencorev1beta1.RotationPrepared,
							},
						},
					},
				}),
				Entry("ca rotation phase is completing", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							CertificateAuthorities: &gardencorev1beta1.CARotation{
								Phase: gardencorev1beta1.RotationCompleting,
							},
							ServiceAccountKey: &gardencorev1beta1.ServiceAccountKeyRotation{
								Phase: gardencorev1beta1.RotationPrepared,
							},
							ETCDEncryptionKey: &gardencorev1beta1.ETCDEncryptionKeyRotation{
								Phase: gardencorev1beta1.RotationPrepared,
							},
						},
					},
				}),
				Entry("sa rotation phase is completing", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							CertificateAuthorities: &gardencorev1beta1.CARotation{
								Phase: gardencorev1beta1.RotationPrepared,
							},
							ServiceAccountKey: &gardencorev1beta1.ServiceAccountKeyRotation{
								Phase: gardencorev1beta1.RotationCompleting,
							},
							ETCDEncryptionKey: &gardencorev1beta1.ETCDEncryptionKeyRotation{
								Phase: gardencorev1beta1.RotationPrepared,
							},
						},
					},
				}),
				Entry("etcd key rotation phase is completing", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							CertificateAuthorities: &gardencorev1beta1.CARotation{
								Phase: gardencorev1beta1.RotationPrepared,
							},
							ServiceAccountKey: &gardencorev1beta1.ServiceAccountKeyRotation{
								Phase: gardencorev1beta1.RotationPrepared,
							},
							ETCDEncryptionKey: &gardencorev1beta1.ETCDEncryptionKeyRotation{
								Phase: gardencorev1beta1.RotationCompleting,
							},
						},
					},
				}),
				Entry("ca rotation phase is completed", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							CertificateAuthorities: &gardencorev1beta1.CARotation{
								Phase: gardencorev1beta1.RotationCompleted,
							},
							ServiceAccountKey: &gardencorev1beta1.ServiceAccountKeyRotation{
								Phase: gardencorev1beta1.RotationPrepared,
							},
							ETCDEncryptionKey: &gardencorev1beta1.ETCDEncryptionKeyRotation{
								Phase: gardencorev1beta1.RotationPrepared,
							},
						},
					},
				}),
				Entry("sa rotation phase is completed", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							CertificateAuthorities: &gardencorev1beta1.CARotation{
								Phase: gardencorev1beta1.RotationPrepared,
							},
							ServiceAccountKey: &gardencorev1beta1.ServiceAccountKeyRotation{
								Phase: gardencorev1beta1.RotationCompleted,
							},
							ETCDEncryptionKey: &gardencorev1beta1.ETCDEncryptionKeyRotation{
								Phase: gardencorev1beta1.RotationPrepared,
							},
						},
					},
				}),
				Entry("etcd key rotation phase is completed", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							CertificateAuthorities: &gardencorev1beta1.CARotation{
								Phase: gardencorev1beta1.RotationPrepared,
							},
							ServiceAccountKey: &gardencorev1beta1.ServiceAccountKeyRotation{
								Phase: gardencorev1beta1.RotationPrepared,
							},
							ETCDEncryptionKey: &gardencorev1beta1.ETCDEncryptionKeyRotation{
								Phase: gardencorev1beta1.RotationCompleted,
							},
						},
					},
				}),
			)

			DescribeTable("starting CA rotation",
				func(allowed bool, status operatorv1alpha1.GardenStatus) {
					metav1.SetMetaDataAnnotation(&garden.ObjectMeta, "gardener.cloud/operation", "rotate-ca-start")
					garden.Status = status

					matcher := BeEmpty()
					if !allowed {
						matcher = ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeForbidden),
							"Field": Equal("metadata.annotations[gardener.cloud/operation]"),
						})))
					}

					Expect(ValidateGarden(garden)).To(matcher)
				},

				Entry("ca rotation phase is preparing", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							CertificateAuthorities: &gardencorev1beta1.CARotation{
								Phase: gardencorev1beta1.RotationPreparing,
							},
						},
					},
				}),
				Entry("ca rotation phase is prepared", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							CertificateAuthorities: &gardencorev1beta1.CARotation{
								Phase: gardencorev1beta1.RotationPrepared,
							},
						},
					},
				}),
				Entry("ca rotation phase is completing", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							CertificateAuthorities: &gardencorev1beta1.CARotation{
								Phase: gardencorev1beta1.RotationCompleting,
							},
						},
					},
				}),
				Entry("ca rotation phase is completed", true, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							CertificateAuthorities: &gardencorev1beta1.CARotation{
								Phase: gardencorev1beta1.RotationCompleted,
							},
						},
					},
				}),
			)

			DescribeTable("completing CA rotation",
				func(allowed bool, status operatorv1alpha1.GardenStatus) {
					metav1.SetMetaDataAnnotation(&garden.ObjectMeta, "gardener.cloud/operation", "rotate-ca-complete")
					garden.Status = status

					matcher := BeEmpty()
					if !allowed {
						matcher = ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeForbidden),
							"Field": Equal("metadata.annotations[gardener.cloud/operation]"),
						})))
					}

					Expect(ValidateGarden(garden)).To(matcher)
				},

				Entry("ca rotation phase is preparing", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							CertificateAuthorities: &gardencorev1beta1.CARotation{
								Phase: gardencorev1beta1.RotationPreparing,
							},
						},
					},
				}),
				Entry("ca rotation phase is prepared", true, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							CertificateAuthorities: &gardencorev1beta1.CARotation{
								Phase: gardencorev1beta1.RotationPrepared,
							},
						},
					},
				}),
				Entry("ca rotation phase is completing", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							CertificateAuthorities: &gardencorev1beta1.CARotation{
								Phase: gardencorev1beta1.RotationCompleting,
							},
						},
					},
				}),
				Entry("ca rotation phase is completed", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							CertificateAuthorities: &gardencorev1beta1.CARotation{
								Phase: gardencorev1beta1.RotationCompleted,
							},
						},
					},
				}),
			)

			DescribeTable("starting service account key rotation",
				func(allowed bool, status operatorv1alpha1.GardenStatus) {
					metav1.SetMetaDataAnnotation(&garden.ObjectMeta, "gardener.cloud/operation", "rotate-serviceaccount-key-start")
					garden.Status = status

					matcher := BeEmpty()
					if !allowed {
						matcher = ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeForbidden),
							"Field": Equal("metadata.annotations[gardener.cloud/operation]"),
						})))
					}

					Expect(ValidateGarden(garden)).To(matcher)
				},
				Entry("rotation phase is preparing", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							ServiceAccountKey: &gardencorev1beta1.ServiceAccountKeyRotation{
								Phase: gardencorev1beta1.RotationPreparing,
							},
						},
					},
				}),
				Entry("rotation phase is prepared", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							ServiceAccountKey: &gardencorev1beta1.ServiceAccountKeyRotation{
								Phase: gardencorev1beta1.RotationPrepared,
							},
						},
					},
				}),
				Entry("rotation phase is completing", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							ServiceAccountKey: &gardencorev1beta1.ServiceAccountKeyRotation{
								Phase: gardencorev1beta1.RotationCompleting,
							},
						},
					},
				}),
				Entry("rotation phase is completed", true, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							ServiceAccountKey: &gardencorev1beta1.ServiceAccountKeyRotation{
								Phase: gardencorev1beta1.RotationCompleted,
							},
						},
					},
				}),
			)

			DescribeTable("completing service account key rotation",
				func(allowed bool, status operatorv1alpha1.GardenStatus) {
					metav1.SetMetaDataAnnotation(&garden.ObjectMeta, "gardener.cloud/operation", "rotate-serviceaccount-key-complete")
					garden.Status = status

					matcher := BeEmpty()
					if !allowed {
						matcher = ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeForbidden),
							"Field": Equal("metadata.annotations[gardener.cloud/operation]"),
						})))
					}

					Expect(ValidateGarden(garden)).To(matcher)
				},

				Entry("rotation phase is preparing", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							ServiceAccountKey: &gardencorev1beta1.ServiceAccountKeyRotation{
								Phase: gardencorev1beta1.RotationPreparing,
							},
						},
					},
				}),
				Entry("rotation phase is prepared", true, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							ServiceAccountKey: &gardencorev1beta1.ServiceAccountKeyRotation{
								Phase: gardencorev1beta1.RotationPrepared,
							},
						},
					},
				}),
				Entry("rotation phase is completing", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							ServiceAccountKey: &gardencorev1beta1.ServiceAccountKeyRotation{
								Phase: gardencorev1beta1.RotationCompleting,
							},
						},
					},
				}),
				Entry("rotation phase is completed", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							ServiceAccountKey: &gardencorev1beta1.ServiceAccountKeyRotation{
								Phase: gardencorev1beta1.RotationCompleted,
							},
						},
					},
				}),
			)

			DescribeTable("starting ETCD encryption key rotation",
				func(allowed bool, status operatorv1alpha1.GardenStatus) {
					metav1.SetMetaDataAnnotation(&garden.ObjectMeta, "gardener.cloud/operation", "rotate-etcd-encryption-key-start")
					garden.Status = status

					matcher := BeEmpty()
					if !allowed {
						matcher = ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeForbidden),
							"Field": Equal("metadata.annotations[gardener.cloud/operation]"),
						})))
					}

					Expect(ValidateGarden(garden)).To(matcher)
				},

				Entry("rotation phase is prepare", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							ETCDEncryptionKey: &gardencorev1beta1.ETCDEncryptionKeyRotation{
								Phase: gardencorev1beta1.RotationPreparing,
							},
						},
					},
				}),
				Entry("rotation phase is prepared", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							ETCDEncryptionKey: &gardencorev1beta1.ETCDEncryptionKeyRotation{
								Phase: gardencorev1beta1.RotationPrepared,
							},
						},
					},
				}),
				Entry("rotation phase is complete", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							ETCDEncryptionKey: &gardencorev1beta1.ETCDEncryptionKeyRotation{
								Phase: gardencorev1beta1.RotationCompleting,
							},
						},
					},
				}),
				Entry("rotation phase is completed", true, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							ETCDEncryptionKey: &gardencorev1beta1.ETCDEncryptionKeyRotation{
								Phase: gardencorev1beta1.RotationCompleted,
							},
						},
					},
				}),
			)

			DescribeTable("completing ETCD encryption key rotation",
				func(allowed bool, status operatorv1alpha1.GardenStatus) {
					metav1.SetMetaDataAnnotation(&garden.ObjectMeta, "gardener.cloud/operation", "rotate-etcd-encryption-key-complete")
					garden.Status = status

					matcher := BeEmpty()
					if !allowed {
						matcher = ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeForbidden),
							"Field": Equal("metadata.annotations[gardener.cloud/operation]"),
						})))
					}

					Expect(ValidateGarden(garden)).To(matcher)
				},

				Entry("rotation phase is prepare", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							ETCDEncryptionKey: &gardencorev1beta1.ETCDEncryptionKeyRotation{
								Phase: gardencorev1beta1.RotationPreparing,
							},
						},
					},
				}),
				Entry("rotation phase is prepared", true, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							ETCDEncryptionKey: &gardencorev1beta1.ETCDEncryptionKeyRotation{
								Phase: gardencorev1beta1.RotationPrepared,
							},
						},
					},
				}),
				Entry("rotation phase is complete", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							ETCDEncryptionKey: &gardencorev1beta1.ETCDEncryptionKeyRotation{
								Phase: gardencorev1beta1.RotationCompleting,
							},
						},
					},
				}),
				Entry("rotation phase is completed", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							ETCDEncryptionKey: &gardencorev1beta1.ETCDEncryptionKeyRotation{
								Phase: gardencorev1beta1.RotationCompleted,
							},
						},
					},
				}),
			)
		})

		Context("runtime cluster", func() {
			Context("networking", func() {
				It("should complain when pod network of runtime cluster intersects with service network of runtime cluster", func() {
					garden.Spec.RuntimeCluster.Networking.Pods = garden.Spec.RuntimeCluster.Networking.Services

					Expect(ValidateGarden(garden)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.runtimeCluster.networking.services"),
					}))))
				})

				It("should complain when node network of runtime cluster intersects with pod network of runtime cluster", func() {
					garden.Spec.RuntimeCluster.Networking.Nodes = &garden.Spec.RuntimeCluster.Networking.Pods

					Expect(ValidateGarden(garden)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.runtimeCluster.networking.nodes"),
					}))))
				})

				It("should complain when node network of runtime cluster intersects with service network of runtime cluster", func() {
					garden.Spec.RuntimeCluster.Networking.Nodes = &garden.Spec.RuntimeCluster.Networking.Services

					Expect(ValidateGarden(garden)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.runtimeCluster.networking.nodes"),
					}))))
				})
			})

			Context("topology-aware routing field", func() {
				It("should prevent enabling topology-aware routing on single-zone cluster", func() {
					garden.Spec.RuntimeCluster.Provider.Zones = []string{"a"}
					garden.Spec.RuntimeCluster.Settings = &operatorv1alpha1.Settings{
						TopologyAwareRouting: &operatorv1alpha1.SettingTopologyAwareRouting{
							Enabled: true,
						},
					}
					garden.Spec.VirtualCluster.ControlPlane = &operatorv1alpha1.ControlPlane{HighAvailability: &operatorv1alpha1.HighAvailability{}}

					Expect(ValidateGarden(garden)).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(field.ErrorTypeForbidden),
							"Field":  Equal("spec.runtimeCluster.settings.topologyAwareRouting.enabled"),
							"Detail": Equal("topology-aware routing can only be enabled on multi-zone garden runtime cluster (with at least two zones in spec.provider.zones)"),
						})),
					))
				})

				It("should prevent enabling topology-aware routing when control-plane is not HA", func() {
					garden.Spec.RuntimeCluster.Provider.Zones = []string{"a", "b", "c"}
					garden.Spec.RuntimeCluster.Settings = &operatorv1alpha1.Settings{
						TopologyAwareRouting: &operatorv1alpha1.SettingTopologyAwareRouting{
							Enabled: true,
						},
					}
					garden.Spec.VirtualCluster.ControlPlane = nil

					Expect(ValidateGarden(garden)).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(field.ErrorTypeForbidden),
							"Field":  Equal("spec.runtimeCluster.settings.topologyAwareRouting.enabled"),
							"Detail": Equal("topology-aware routing can only be enabled when virtual cluster's high-availability is enabled"),
						})),
					))
				})

				It("should allow enabling topology-aware routing on multi-zone cluster with HA control-plane", func() {
					garden.Spec.RuntimeCluster.Provider.Zones = []string{"a", "b", "c"}
					garden.Spec.RuntimeCluster.Settings = &operatorv1alpha1.Settings{
						TopologyAwareRouting: &operatorv1alpha1.SettingTopologyAwareRouting{
							Enabled: true,
						},
					}
					garden.Spec.VirtualCluster.ControlPlane = &operatorv1alpha1.ControlPlane{HighAvailability: &operatorv1alpha1.HighAvailability{}}

					Expect(ValidateGarden(garden)).To(BeEmpty())
				})
			})
		})

		Context("virtual cluster", func() {
			Context("DNS", func() {
				It("should complain about invalid domain name in 'domain'", func() {
					garden.Spec.VirtualCluster.DNS.Domains = []string{",,,"}

					Expect(ValidateGarden(garden)).To(ContainElements(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeInvalid),
							"Field": Equal("spec.virtualCluster.dns.domains[0]"),
						})),
					))
				})

				It("should complain about duplicate domain names in 'domains'", func() {
					garden.Spec.VirtualCluster.DNS.Domains = []string{
						"example.com",
						"foo.bar",
						"example.com",
					}

					Expect(ValidateGarden(garden)).To(ContainElements(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeDuplicate),
							"Field": Equal("spec.virtualCluster.dns.domains[2]"),
						})),
					))
				})
			})

			Context("Networking", func() {
				It("should complain about an invalid service CIDR", func() {
					garden.Spec.VirtualCluster.Networking.Services = "not-parseable-cidr"

					Expect(ValidateGarden(garden)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.virtualCluster.networking.services"),
					}))))
				})

				It("should complain when pod network of runtime cluster intersects with service network of virtual cluster", func() {
					garden.Spec.RuntimeCluster.Networking.Pods = garden.Spec.VirtualCluster.Networking.Services

					Expect(ValidateGarden(garden)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.virtualCluster.networking.services"),
					}))))
				})

				It("should complain when service network of runtime cluster intersects with service network of virtual cluster", func() {
					garden.Spec.RuntimeCluster.Networking.Services = garden.Spec.VirtualCluster.Networking.Services

					Expect(ValidateGarden(garden)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.virtualCluster.networking.services"),
					}))))
				})

				It("should complain when node network of runtime cluster intersects with service network of virtual cluster", func() {
					garden.Spec.RuntimeCluster.Networking.Nodes = &garden.Spec.VirtualCluster.Networking.Services

					Expect(ValidateGarden(garden)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.virtualCluster.networking.services"),
					}))))
				})
			})

			Context("Gardener", func() {
				Context("APIServer", func() {
					BeforeEach(func() {
						garden.Spec.VirtualCluster.Gardener.APIServer = &operatorv1alpha1.GardenerAPIServerConfig{}
					})

					Context("Feature gates", func() {
						It("should complain when non-existing feature gates were configured", func() {
							garden.Spec.VirtualCluster.Gardener.APIServer.FeatureGates = map[string]bool{"Foo": true}

							Expect(ValidateGarden(garden)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
								"Type":  Equal(field.ErrorTypeForbidden),
								"Field": Equal("spec.virtualCluster.gardener.gardenerAPIServer.featureGates.Foo"),
							}))))
						})

						It("should complain when invalid feature gates were configured", func() {
							features.AllFeatureGates["Foo"] = featuregate.FeatureSpec{LockToDefault: true, Default: false}
							DeferCleanup(func() {
								delete(features.AllFeatureGates, "Foo")
							})

							garden.Spec.VirtualCluster.Gardener.APIServer = &operatorv1alpha1.GardenerAPIServerConfig{
								KubernetesConfig: gardencorev1beta1.KubernetesConfig{
									FeatureGates: map[string]bool{"Foo": true},
								},
							}

							Expect(ValidateGarden(garden)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
								"Type":  Equal(field.ErrorTypeForbidden),
								"Field": Equal("spec.virtualCluster.gardener.gardenerAPIServer.featureGates.Foo"),
							}))))
						})
					})

					Context("Admission plugins", func() {
						It("should allow not specifying admission plugins", func() {
							garden.Spec.VirtualCluster.Gardener.APIServer.AdmissionPlugins = nil

							Expect(ValidateGarden(garden)).To(BeEmpty())
						})

						It("should forbid specifying admission plugins without a name", func() {
							garden.Spec.VirtualCluster.Gardener.APIServer.AdmissionPlugins = []gardencorev1beta1.AdmissionPlugin{{Name: ""}}

							Expect(ValidateGarden(garden)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
								"Type":  Equal(field.ErrorTypeRequired),
								"Field": Equal("spec.virtualCluster.gardener.gardenerAPIServer.admissionPlugins[0].name"),
							}))))
						})

						It("should forbid specifying non-existing admission plugin", func() {
							garden.Spec.VirtualCluster.Gardener.APIServer.AdmissionPlugins = []gardencorev1beta1.AdmissionPlugin{{Name: "Foo"}}

							Expect(ValidateGarden(garden)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
								"Type":  Equal(field.ErrorTypeNotSupported),
								"Field": Equal("spec.virtualCluster.gardener.gardenerAPIServer.admissionPlugins[0].name"),
							}))))
						})
					})

					Context("AuditConfig", func() {
						It("should allow nil AuditConfig", func() {
							Expect(ValidateGarden(garden)).To(BeEmpty())
						})

						It("should forbid empty name", func() {
							garden.Spec.VirtualCluster.Gardener.APIServer.AuditConfig = &gardencorev1beta1.AuditConfig{
								AuditPolicy: &gardencorev1beta1.AuditPolicy{
									ConfigMapRef: &corev1.ObjectReference{},
								},
							}

							Expect(ValidateGarden(garden)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
								"Type":  Equal(field.ErrorTypeRequired),
								"Field": Equal("spec.virtualCluster.gardener.gardenerAPIServer.auditConfig.auditPolicy.configMapRef.name"),
							}))))
						})
					})

					Context("Watch cache sizes", func() {
						var negativeSize int32 = -1

						DescribeTable("watch cache size validation",
							func(sizes *gardencorev1beta1.WatchCacheSizes, matcher gomegatypes.GomegaMatcher) {
								garden.Spec.VirtualCluster.Gardener.APIServer.WatchCacheSizes = sizes
								Expect(ValidateGarden(garden)).To(matcher)
							},

							Entry("valid (unset)", nil, BeEmpty()),
							Entry("valid (fields unset)", &gardencorev1beta1.WatchCacheSizes{}, BeEmpty()),
							Entry("valid (default=0)", &gardencorev1beta1.WatchCacheSizes{
								Default: pointer.Int32(0),
							}, BeEmpty()),
							Entry("valid (default>0)", &gardencorev1beta1.WatchCacheSizes{
								Default: pointer.Int32(42),
							}, BeEmpty()),
							Entry("invalid (default<0)", &gardencorev1beta1.WatchCacheSizes{
								Default: pointer.Int32(negativeSize),
							}, ConsistOf(
								field.Invalid(field.NewPath("spec.virtualCluster.gardener.gardenerAPIServer.watchCacheSizes.default"), int64(negativeSize), apivalidation.IsNegativeErrorMsg),
							)),

							// APIGroup unset (core group)
							Entry("valid (core/secrets=0)", &gardencorev1beta1.WatchCacheSizes{
								Resources: []gardencorev1beta1.ResourceWatchCacheSize{{
									Resource:  "secrets",
									CacheSize: 0,
								}},
							}, BeEmpty()),
							Entry("valid (core/secrets=>0)", &gardencorev1beta1.WatchCacheSizes{
								Resources: []gardencorev1beta1.ResourceWatchCacheSize{{
									Resource:  "secrets",
									CacheSize: 42,
								}},
							}, BeEmpty()),
							Entry("invalid (core/secrets=<0)", &gardencorev1beta1.WatchCacheSizes{
								Resources: []gardencorev1beta1.ResourceWatchCacheSize{{
									Resource:  "secrets",
									CacheSize: negativeSize,
								}},
							}, ConsistOf(
								field.Invalid(field.NewPath("spec.virtualCluster.gardener.gardenerAPIServer.watchCacheSizes.resources[0].size"), int64(negativeSize), apivalidation.IsNegativeErrorMsg),
							)),
							Entry("invalid (core/resource empty)", &gardencorev1beta1.WatchCacheSizes{
								Resources: []gardencorev1beta1.ResourceWatchCacheSize{{
									Resource:  "",
									CacheSize: 42,
								}},
							}, ConsistOf(
								field.Required(field.NewPath("spec.virtualCluster.gardener.gardenerAPIServer.watchCacheSizes.resources[0].resource"), "must not be empty"),
							)),

							// APIGroup set
							Entry("valid (apps/deployments=0)", &gardencorev1beta1.WatchCacheSizes{
								Resources: []gardencorev1beta1.ResourceWatchCacheSize{{
									APIGroup:  pointer.String("apps"),
									Resource:  "deployments",
									CacheSize: 0,
								}},
							}, BeEmpty()),
							Entry("valid (apps/deployments=>0)", &gardencorev1beta1.WatchCacheSizes{
								Resources: []gardencorev1beta1.ResourceWatchCacheSize{{
									APIGroup:  pointer.String("apps"),
									Resource:  "deployments",
									CacheSize: 42,
								}},
							}, BeEmpty()),
							Entry("invalid (apps/deployments=<0)", &gardencorev1beta1.WatchCacheSizes{
								Resources: []gardencorev1beta1.ResourceWatchCacheSize{{
									APIGroup:  pointer.String("apps"),
									Resource:  "deployments",
									CacheSize: negativeSize,
								}},
							}, ConsistOf(
								field.Invalid(field.NewPath("spec.virtualCluster.gardener.gardenerAPIServer.watchCacheSizes.resources[0].size"), int64(negativeSize), apivalidation.IsNegativeErrorMsg),
							)),
							Entry("invalid (apps/resource empty)", &gardencorev1beta1.WatchCacheSizes{
								Resources: []gardencorev1beta1.ResourceWatchCacheSize{{
									Resource:  "",
									CacheSize: 42,
								}},
							}, ConsistOf(
								field.Required(field.NewPath("spec.virtualCluster.gardener.gardenerAPIServer.watchCacheSizes.resources[0].resource"), "must not be empty"),
							)),
						)
					})

					Context("Logging", func() {
						var negativeSize int32 = -1

						DescribeTable("Logging validation",
							func(loggingConfig *gardencorev1beta1.APIServerLogging, matcher gomegatypes.GomegaMatcher) {
								garden.Spec.VirtualCluster.Gardener.APIServer.Logging = loggingConfig
								Expect(ValidateGarden(garden)).To(matcher)
							},

							Entry("valid (unset)", nil, BeEmpty()),
							Entry("valid (fields unset)", &gardencorev1beta1.APIServerLogging{}, BeEmpty()),
							Entry("valid (verbosity=0)", &gardencorev1beta1.APIServerLogging{
								Verbosity: pointer.Int32(0),
							}, BeEmpty()),
							Entry("valid (httpAccessVerbosity=0)", &gardencorev1beta1.APIServerLogging{
								HTTPAccessVerbosity: pointer.Int32(0),
							}, BeEmpty()),
							Entry("valid (verbosity>0)", &gardencorev1beta1.APIServerLogging{
								Verbosity: pointer.Int32(3),
							}, BeEmpty()),
							Entry("valid (httpAccessVerbosity>0)", &gardencorev1beta1.APIServerLogging{
								HTTPAccessVerbosity: pointer.Int32(3),
							}, BeEmpty()),
							Entry("invalid (verbosity<0)", &gardencorev1beta1.APIServerLogging{
								Verbosity: pointer.Int32(negativeSize),
							}, ConsistOf(
								field.Invalid(field.NewPath("spec.virtualCluster.gardener.gardenerAPIServer.logging.verbosity"), int64(negativeSize), apivalidation.IsNegativeErrorMsg),
							)),
							Entry("invalid (httpAccessVerbosity<0)", &gardencorev1beta1.APIServerLogging{
								HTTPAccessVerbosity: pointer.Int32(negativeSize),
							}, ConsistOf(
								field.Invalid(field.NewPath("spec.virtualCluster.gardener.gardenerAPIServer.logging.httpAccessVerbosity"), int64(negativeSize), apivalidation.IsNegativeErrorMsg),
							)),
						)
					})

					Context("Requests", func() {
						It("should not allow too high values for max inflight requests fields", func() {
							garden.Spec.VirtualCluster.Gardener.APIServer.Requests = &gardencorev1beta1.APIServerRequests{
								MaxNonMutatingInflight: pointer.Int32(123123123),
								MaxMutatingInflight:    pointer.Int32(412412412),
							}

							Expect(ValidateGarden(garden)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
								"Type":  Equal(field.ErrorTypeInvalid),
								"Field": Equal("spec.virtualCluster.gardener.gardenerAPIServer.requests.maxNonMutatingInflight"),
							})), PointTo(MatchFields(IgnoreExtras, Fields{
								"Type":  Equal(field.ErrorTypeInvalid),
								"Field": Equal("spec.virtualCluster.gardener.gardenerAPIServer.requests.maxMutatingInflight"),
							}))))
						})

						It("should not allow negative values for max inflight requests fields", func() {
							garden.Spec.VirtualCluster.Gardener.APIServer.Requests = &gardencorev1beta1.APIServerRequests{
								MaxNonMutatingInflight: pointer.Int32(-1),
								MaxMutatingInflight:    pointer.Int32(-1),
							}

							Expect(ValidateGarden(garden)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
								"Type":  Equal(field.ErrorTypeInvalid),
								"Field": Equal("spec.virtualCluster.gardener.gardenerAPIServer.requests.maxNonMutatingInflight"),
							})), PointTo(MatchFields(IgnoreExtras, Fields{
								"Type":  Equal(field.ErrorTypeInvalid),
								"Field": Equal("spec.virtualCluster.gardener.gardenerAPIServer.requests.maxMutatingInflight"),
							}))))
						})
					})
				})

				Context("AdmissionController", func() {
					It("should allow the configuration being set to nil", func() {
						garden.Spec.VirtualCluster.Gardener.AdmissionController = nil

						Expect(ValidateGarden(garden)).To(BeEmpty())
					})

					It("should allow the configuration being empty", func() {
						garden.Spec.VirtualCluster.Gardener.AdmissionController = &operatorv1alpha1.GardenerAdmissionControllerConfig{}

						Expect(ValidateGarden(garden)).To(BeEmpty())
					})

					Context("Resource Admission Configuration", func() {
						Context("Operation mode", func() {
							test := func(mode string) field.ErrorList {
								var (
									admissionConfig *operatorv1alpha1.ResourceAdmissionConfiguration
									operationMode   = operatorv1alpha1.ResourceAdmissionWebhookMode(mode)
								)

								if mode != "" {
									admissionConfig = &operatorv1alpha1.ResourceAdmissionConfiguration{
										OperationMode: &operationMode,
									}
								}

								garden.Spec.VirtualCluster.Gardener.AdmissionController = &operatorv1alpha1.GardenerAdmissionControllerConfig{
									ResourceAdmissionConfiguration: admissionConfig,
								}

								return ValidateGarden(garden)
							}

							It("should allow no mode", func() {
								Expect(test("")).To(BeEmpty())
							})

							It("should allow blocking mode", func() {
								Expect(test("block")).To(BeEmpty())
							})

							It("should allow logging mode", func() {
								Expect(test("log")).To(BeEmpty())
							})

							It("should deny non existing mode", func() {
								Expect(test("foo")).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
									"Type":  Equal(field.ErrorTypeNotSupported),
									"Field": Equal("spec.virtualCluster.gardener.gardenerAdmissionController.resourceAdmissionConfiguration.mode"),
								}))))
							})
						})

						Context("Limits validation", func() {
							var (
								apiGroups = []string{"core.gardener.cloud"}
								versions  = []string{"v1beta1", "v1alpha1"}
								resources = []string{"shoot"}
								size      = "1Ki"

								test = func(apiGroups []string, versions []string, resources []string, size string) field.ErrorList {
									s, err := resource.ParseQuantity(size)
									utilruntime.Must(err)

									garden.Spec.VirtualCluster.Gardener.AdmissionController = &operatorv1alpha1.GardenerAdmissionControllerConfig{
										ResourceAdmissionConfiguration: &operatorv1alpha1.ResourceAdmissionConfiguration{
											Limits: []operatorv1alpha1.ResourceLimit{
												{
													APIGroups:   apiGroups,
													APIVersions: versions,
													Resources:   resources,
													Size:        s,
												},
											},
										},
									}

									return ValidateGarden(garden)
								}
							)

							It("should allow request", func() {
								Expect(test(apiGroups, versions, resources, size)).To(BeEmpty())
							})

							It("should deny empty apiGroup", func() {
								Expect(test(nil, versions, resources, size)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
									"Type":  Equal(field.ErrorTypeInvalid),
									"Field": Equal("spec.virtualCluster.gardener.gardenerAdmissionController.resourceAdmissionConfiguration.limits[0].apiGroups"),
								}))))
							})

							It("should allow apiGroup w/ zero length", func() {
								Expect(test([]string{""}, versions, resources, size)).To(BeEmpty())
							})

							It("should deny empty versions", func() {
								Expect(test(apiGroups, nil, resources, size)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
									"Type":  Equal(field.ErrorTypeInvalid),
									"Field": Equal("spec.virtualCluster.gardener.gardenerAdmissionController.resourceAdmissionConfiguration.limits[0].versions"),
								}))))
							})

							It("should deny versions w/ zero length", func() {
								Expect(test(apiGroups, []string{""}, resources, size)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
									"Type":  Equal(field.ErrorTypeInvalid),
									"Field": Equal("spec.virtualCluster.gardener.gardenerAdmissionController.resourceAdmissionConfiguration.limits[0].versions[0]"),
								}))))
							})

							It("should deny empty resources", func() {
								Expect(test(apiGroups, versions, nil, size)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
									"Type":  Equal(field.ErrorTypeInvalid),
									"Field": Equal("spec.virtualCluster.gardener.gardenerAdmissionController.resourceAdmissionConfiguration.limits[0].resources"),
								}))))
							})

							It("should deny resources w/ zero length", func() {
								Expect(test(apiGroups, versions, []string{""}, size)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
									"Type":  Equal(field.ErrorTypeInvalid),
									"Field": Equal("spec.virtualCluster.gardener.gardenerAdmissionController.resourceAdmissionConfiguration.limits[0].resources[0]"),
								}))))
							})

							It("should deny invalid size", func() {
								Expect(test(apiGroups, versions, resources, "-1k")).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
									"Type":  Equal(field.ErrorTypeInvalid),
									"Field": Equal("spec.virtualCluster.gardener.gardenerAdmissionController.resourceAdmissionConfiguration.limits[0].size"),
								}))))
							})

							It("should deny invalid size and resources w/ zero length", func() {
								Expect(test(apiGroups, versions, []string{resources[0], ""}, "-1k")).To(ConsistOf(
									PointTo(MatchFields(IgnoreExtras, Fields{
										"Type":  Equal(field.ErrorTypeInvalid),
										"Field": Equal("spec.virtualCluster.gardener.gardenerAdmissionController.resourceAdmissionConfiguration.limits[0].size"),
									})),
									PointTo(MatchFields(IgnoreExtras, Fields{
										"Type":  Equal(field.ErrorTypeInvalid),
										"Field": Equal("spec.virtualCluster.gardener.gardenerAdmissionController.resourceAdmissionConfiguration.limits[0].resources[1]"),
									}))))
							})
						})

						Context("User configuration validation", func() {
							var (
								userName       = "admin"
								namespace      = "default"
								emptyNamespace = ""

								test = func(kind string, name string, namespace string, apiGroup string) field.ErrorList {
									garden.Spec.VirtualCluster.Gardener.AdmissionController = &operatorv1alpha1.GardenerAdmissionControllerConfig{
										ResourceAdmissionConfiguration: &operatorv1alpha1.ResourceAdmissionConfiguration{
											UnrestrictedSubjects: []rbacv1.Subject{
												{
													Kind:      kind,
													Name:      name,
													Namespace: namespace,
													APIGroup:  apiGroup,
												},
											},
										},
									}

									return ValidateGarden(garden)
								}
							)

							It("should allow request for user", func() {
								Expect(test(rbacv1.UserKind, userName, emptyNamespace, rbacv1.GroupName)).To(BeEmpty())
							})

							It("should allow request for group", func() {
								Expect(test(rbacv1.GroupKind, userName, emptyNamespace, rbacv1.GroupName)).To(BeEmpty())
							})

							It("should allow request for service account", func() {
								Expect(test(rbacv1.ServiceAccountKind, userName, namespace, "")).To(BeEmpty())
							})

							It("should deny invalid apiGroup for user", func() {
								Expect(test(rbacv1.UserKind, userName, emptyNamespace, "invalid")).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
									"Type":  Equal(field.ErrorTypeNotSupported),
									"Field": Equal("spec.virtualCluster.gardener.gardenerAdmissionController.resourceAdmissionConfiguration.unrestrictedSubjects[0].apiGroup"),
								}))))
							})

							It("should deny invalid apiGroup for group", func() {
								Expect(test(rbacv1.GroupKind, userName, emptyNamespace, "invalid")).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
									"Type":  Equal(field.ErrorTypeNotSupported),
									"Field": Equal("spec.virtualCluster.gardener.gardenerAdmissionController.resourceAdmissionConfiguration.unrestrictedSubjects[0].apiGroup"),
								}))))
							})

							It("should deny invalid apiGroup for service account", func() {
								Expect(test(rbacv1.ServiceAccountKind, userName, namespace, "invalid")).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
									"Type":  Equal(field.ErrorTypeInvalid),
									"Field": Equal("spec.virtualCluster.gardener.gardenerAdmissionController.resourceAdmissionConfiguration.unrestrictedSubjects[0].apiGroup"),
								}))))
							})

							It("should deny invalid namespace setting for user", func() {
								Expect(test(rbacv1.UserKind, userName, namespace, rbacv1.GroupName)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
									"Type":  Equal(field.ErrorTypeInvalid),
									"Field": Equal("spec.virtualCluster.gardener.gardenerAdmissionController.resourceAdmissionConfiguration.unrestrictedSubjects[0].namespace"),
								}))))
							})

							It("should deny invalid namespace setting for group", func() {
								Expect(test(rbacv1.GroupKind, userName, namespace, rbacv1.GroupName)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
									"Type":  Equal(field.ErrorTypeInvalid),
									"Field": Equal("spec.virtualCluster.gardener.gardenerAdmissionController.resourceAdmissionConfiguration.unrestrictedSubjects[0].namespace"),
								}))))
							})

							It("should deny invalid kind", func() {
								Expect(test("invalidKind", userName, emptyNamespace, rbacv1.GroupName)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
									"Type":  Equal(field.ErrorTypeNotSupported),
									"Field": Equal("spec.virtualCluster.gardener.gardenerAdmissionController.resourceAdmissionConfiguration.unrestrictedSubjects[0].kind"),
								}))))
							})

							It("should deny empty name", func() {
								Expect(test(rbacv1.UserKind, "", emptyNamespace, rbacv1.GroupName)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
									"Field": Equal("spec.virtualCluster.gardener.gardenerAdmissionController.resourceAdmissionConfiguration.unrestrictedSubjects[0].name"),
								}))))
							})
						})
					})
				})

				Context("ControllerManager", func() {
					Context("Feature gates", func() {
						It("should complain when non-existing feature gates were configured", func() {
							garden.Spec.VirtualCluster.Gardener.ControllerManager = &operatorv1alpha1.GardenerControllerManagerConfig{
								KubernetesConfig: gardencorev1beta1.KubernetesConfig{
									FeatureGates: map[string]bool{"Foo": true},
								},
							}

							Expect(ValidateGarden(garden)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
								"Type":  Equal(field.ErrorTypeForbidden),
								"Field": Equal("spec.virtualCluster.gardener.gardenerControllerManager.featureGates.Foo"),
							}))))
						})

						It("should complain when invalid feature gates were configured", func() {
							features.AllFeatureGates["Foo"] = featuregate.FeatureSpec{LockToDefault: true, Default: false}
							DeferCleanup(func() {
								delete(features.AllFeatureGates, "Foo")
							})

							garden.Spec.VirtualCluster.Gardener.ControllerManager = &operatorv1alpha1.GardenerControllerManagerConfig{
								KubernetesConfig: gardencorev1beta1.KubernetesConfig{
									FeatureGates: map[string]bool{"Foo": true},
								},
							}

							Expect(ValidateGarden(garden)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
								"Type":  Equal(field.ErrorTypeForbidden),
								"Field": Equal("spec.virtualCluster.gardener.gardenerControllerManager.featureGates.Foo"),
							}))))
						})
					})

					Context("Default Project Quotas", func() {
						It("should complain when invalid label selectors were specified", func() {
							garden.Spec.VirtualCluster.Gardener.ControllerManager = &operatorv1alpha1.GardenerControllerManagerConfig{
								DefaultProjectQuotas: []operatorv1alpha1.ProjectQuotaConfiguration{{
									ProjectSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"-": "!"}},
								}},
							}

							Expect(ValidateGarden(garden)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
								"Type":  Equal(field.ErrorTypeInvalid),
								"Field": Equal("spec.virtualCluster.gardener.gardenerControllerManager.defaultProjectQuotas[0].projectSelector.matchLabels"),
							}))))
						})
					})
				})

				Context("Scheduler", func() {
					Context("Feature gates", func() {
						It("should complain when non-existing feature gates were configured", func() {
							garden.Spec.VirtualCluster.Gardener.Scheduler = &operatorv1alpha1.GardenerSchedulerConfig{
								KubernetesConfig: gardencorev1beta1.KubernetesConfig{
									FeatureGates: map[string]bool{"Foo": true},
								},
							}

							Expect(ValidateGarden(garden)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
								"Type":  Equal(field.ErrorTypeForbidden),
								"Field": Equal("spec.virtualCluster.gardener.gardenerScheduler.featureGates.Foo"),
							}))))
						})

						It("should complain when invalid feature gates were configured", func() {
							features.AllFeatureGates["Foo"] = featuregate.FeatureSpec{LockToDefault: true, Default: false}
							DeferCleanup(func() {
								delete(features.AllFeatureGates, "Foo")
							})

							garden.Spec.VirtualCluster.Gardener.Scheduler = &operatorv1alpha1.GardenerSchedulerConfig{
								KubernetesConfig: gardencorev1beta1.KubernetesConfig{
									FeatureGates: map[string]bool{"Foo": true},
								},
							}

							Expect(ValidateGarden(garden)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
								"Type":  Equal(field.ErrorTypeForbidden),
								"Field": Equal("spec.virtualCluster.gardener.gardenerScheduler.featureGates.Foo"),
							}))))
						})
					})
				})
			})
		})
	})

	Describe("#ValidateGardenUpdate", func() {
		var oldGarden, newGarden *operatorv1alpha1.Garden

		BeforeEach(func() {
			oldGarden = &operatorv1alpha1.Garden{
				ObjectMeta: metav1.ObjectMeta{
					Name: "garden",
				},
				Spec: operatorv1alpha1.GardenSpec{
					VirtualCluster: operatorv1alpha1.VirtualCluster{
						Kubernetes: operatorv1alpha1.Kubernetes{
							Version: "1.27.0",
						},
					},
				},
			}
			newGarden = oldGarden.DeepCopy()
		})

		Context("virtual cluster", func() {
			Context("dns", func() {
				Context("when domains are modified", func() {
					It("should allow update if nothing changes", func() {
						oldGarden.Spec.VirtualCluster.DNS.Domains = []string{"example.com"}
						newGarden.Spec.VirtualCluster.DNS.Domains = []string{"example.com"}

						Expect(ValidateGardenUpdate(oldGarden, newGarden)).NotTo(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
							"Field": ContainSubstring("domain"),
						}))))
					})

					It("should allow adding a domain", func() {
						oldGarden.Spec.VirtualCluster.DNS.Domains = []string{"example.com"}
						newGarden.Spec.VirtualCluster.DNS.Domains = []string{"example.com", "foo.bar"}

						Expect(ValidateGardenUpdate(oldGarden, newGarden)).NotTo(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
							"Field": ContainSubstring("domain"),
						}))))
					})

					It("should allow removing any domain but first entry", func() {
						oldGarden.Spec.VirtualCluster.DNS.Domains = []string{"example.com", "foo.bar", "bar.foo"}
						newGarden.Spec.VirtualCluster.DNS.Domains = []string{"example.com", "bar.foo"}

						Expect(ValidateGardenUpdate(oldGarden, newGarden)).NotTo(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
							"Field": ContainSubstring("domain"),
						}))))
					})

					It("should forbid removing the first entry", func() {
						oldGarden.Spec.VirtualCluster.DNS.Domains = []string{"example.com", "foo.bar", "bar.foo"}
						newGarden.Spec.VirtualCluster.DNS.Domains = []string{"bar.foo"}

						Expect(ValidateGardenUpdate(oldGarden, newGarden)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeInvalid),
							"Field": Equal("spec.virtualCluster.dns.domains[0]"),
						}))))
					})

					It("should forbid changing the first entry", func() {
						oldGarden.Spec.VirtualCluster.DNS.Domains = []string{"example.com", "foo.bar", "bar.foo"}
						newGarden.Spec.VirtualCluster.DNS.Domains = []string{"example2.com", "foo.bar", "bar.foo"}

						Expect(ValidateGardenUpdate(oldGarden, newGarden)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeInvalid),
							"Field": Equal("spec.virtualCluster.dns.domains[0]"),
						}))))
					})
				})
			})

			Context("control plane", func() {
				It("should not be possible to remove the high availability setting once set", func() {
					oldGarden.Spec.VirtualCluster.ControlPlane = &operatorv1alpha1.ControlPlane{HighAvailability: &operatorv1alpha1.HighAvailability{}}

					Expect(ValidateGardenUpdate(oldGarden, newGarden)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.virtualCluster.controlPlane.highAvailability"),
					}))))
				})
			})

			Context("kubernetes", func() {
				It("should not not allow version downgrade", func() {
					version := semver.MustParse(newGarden.Spec.VirtualCluster.Kubernetes.Version)
					previousMinor := semver.MustParse(fmt.Sprintf("%d.%d.%d", version.Major(), version.Minor()-1, version.Patch()))

					newGarden.Spec.VirtualCluster.Kubernetes.Version = previousMinor.String()

					Expect(ValidateGardenUpdate(oldGarden, newGarden)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeForbidden),
						"Field": Equal("spec.virtualCluster.kubernetes.version"),
					}))))
				})
			})
		})
	})
})
