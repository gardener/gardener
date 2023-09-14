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

package reference_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"

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
					SecretName: pointer.String("secret"),
				}},
			}
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
