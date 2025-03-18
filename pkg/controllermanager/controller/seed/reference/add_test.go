// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package reference_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/controllermanager/controller/shoot/reference"
)

var _ = Describe("Add", func() {
	Describe("#Predicate", func() {
		var shoot *gardencorev1beta1.Shoot

		BeforeEach(func() {
			shoot = &gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Kubernetes: gardencorev1beta1.Kubernetes{
						KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{},
					},
				},
			}
		})

		It("should return false because new object is no shoot", func() {
			Expect(Predicate(nil, nil)).To(BeFalse())
		})

		It("should return false because old object is no shoot", func() {
			Expect(Predicate(nil, shoot)).To(BeFalse())
		})

		It("should return false because there is no ref change", func() {
			Expect(Predicate(shoot, shoot)).To(BeFalse())
		})

		It("should return true because the DNS fields changed", func() {
			oldShoot := shoot.DeepCopy()
			shoot.Spec.DNS = &gardencorev1beta1.DNS{
				Providers: []gardencorev1beta1.DNSProvider{{
					SecretName: ptr.To("secret"),
				}},
			}
			Expect(Predicate(oldShoot, shoot)).To(BeTrue())
		})

		It("should return true because the admission plugins field changed", func() {
			oldShoot := shoot.DeepCopy()
			shoot.Spec.Kubernetes.KubeAPIServer.AdmissionPlugins = []gardencorev1beta1.AdmissionPlugin{{}}
			Expect(Predicate(oldShoot, shoot)).To(BeTrue())
		})

		It("should return true because the audit policy field changed", func() {
			oldShoot := shoot.DeepCopy()
			shoot.Spec.Kubernetes.KubeAPIServer.AuditConfig = &gardencorev1beta1.AuditConfig{
				AuditPolicy: &gardencorev1beta1.AuditPolicy{
					ConfigMapRef: &corev1.ObjectReference{
						Name: "audit-policy",
					},
				},
			}
			Expect(Predicate(oldShoot, shoot)).To(BeTrue())
		})

		It("should return true because the structured authentication field changed", func() {
			oldShoot := shoot.DeepCopy()
			shoot.Spec.Kubernetes.KubeAPIServer.StructuredAuthentication = &gardencorev1beta1.StructuredAuthentication{}
			Expect(Predicate(oldShoot, shoot)).To(BeTrue())
		})

		It("should return true because the structured authorization field changed", func() {
			oldShoot := shoot.DeepCopy()
			shoot.Spec.Kubernetes.KubeAPIServer.StructuredAuthorization = &gardencorev1beta1.StructuredAuthorization{}
			Expect(Predicate(oldShoot, shoot)).To(BeTrue())
		})

		It("should return true because the resources field changed", func() {
			oldShoot := shoot.DeepCopy()
			shoot.Spec.Resources = []gardencorev1beta1.NamedResourceReference{{
				Name: "resource-1",
				ResourceRef: autoscalingv1.CrossVersionObjectReference{
					APIVersion: "v1",
					Kind:       "Secret",
					Name:       "test",
				},
			}}
			Expect(Predicate(oldShoot, shoot)).To(BeTrue())
		})
	})
})
