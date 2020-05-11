// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package v1beta1_test

import (
	. "github.com/gardener/gardener/pkg/apis/core/v1beta1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/utils/pointer"
)

var _ = Describe("Defaults", func() {
	Describe("#SetDefaults_Project", func() {
		Context("api group defaulting", func() {
			DescribeTable(
				"should default the owner api groups",
				func(owner *rbacv1.Subject, kind string, expectedAPIGroup string) {
					if owner != nil {
						owner.Kind = kind
					}

					obj := &Project{
						Spec: ProjectSpec{
							Owner: owner,
						},
					}

					SetDefaults_Project(obj)

					if owner != nil {
						Expect(obj.Spec.Owner.APIGroup).To(Equal(expectedAPIGroup))
					} else {
						Expect(obj.Spec.Owner).To(BeNil())
					}
				},
				Entry("do nothing because owner is nil", nil, "", ""),
				Entry("kind serviceaccount", &rbacv1.Subject{}, rbacv1.ServiceAccountKind, ""),
				Entry("kind user", &rbacv1.Subject{}, rbacv1.UserKind, rbacv1.GroupName),
				Entry("kind group", &rbacv1.Subject{}, rbacv1.GroupKind, rbacv1.GroupName),
			)

			It("should default the api groups of members", func() {
				member1 := ProjectMember{
					Subject: rbacv1.Subject{
						APIGroup: "group",
						Kind:     "kind",
						Name:     "member1",
					},
					Roles: []string{"role"},
				}
				member2 := ProjectMember{
					Subject: rbacv1.Subject{
						Kind: rbacv1.ServiceAccountKind,
						Name: "member2",
					},
					Roles: []string{"role"},
				}
				member3 := ProjectMember{
					Subject: rbacv1.Subject{
						Kind: rbacv1.UserKind,
						Name: "member3",
					},
					Roles: []string{"role"},
				}
				member4 := ProjectMember{
					Subject: rbacv1.Subject{
						Kind: rbacv1.GroupKind,
						Name: "member4",
					},
					Roles: []string{"role"},
				}

				obj := &Project{
					Spec: ProjectSpec{
						Members: []ProjectMember{member1, member2, member3, member4},
					},
				}

				SetDefaults_Project(obj)

				Expect(obj.Spec.Members[0].APIGroup).To(Equal(member1.Subject.APIGroup))
				Expect(obj.Spec.Members[1].APIGroup).To(BeEmpty())
				Expect(obj.Spec.Members[2].APIGroup).To(Equal(rbacv1.GroupName))
				Expect(obj.Spec.Members[3].APIGroup).To(Equal(rbacv1.GroupName))
			})
		})

		It("should default the roles of members", func() {
			member1 := ProjectMember{
				Subject: rbacv1.Subject{
					APIGroup: "group",
					Kind:     "kind",
					Name:     "member1",
				},
			}
			member2 := ProjectMember{
				Subject: rbacv1.Subject{
					APIGroup: "group",
					Kind:     "kind",
					Name:     "member2",
				},
			}

			obj := &Project{
				Spec: ProjectSpec{
					Members: []ProjectMember{member1, member2},
				},
			}

			SetDefaults_Project(obj)

			for _, m := range obj.Spec.Members {
				Expect(m.Role).NotTo(HaveLen(0))
				Expect(m.Role).To(Equal(ProjectMemberViewer))
			}
		})
	})

	Describe("#SetDefaults_ControllerResource", func() {
		It("should default the primary field", func() {
			resource := ControllerResource{}

			SetDefaults_ControllerResource(&resource)

			Expect(resource.Primary).To(PointTo(BeTrue()))
		})

		It("should not default the primary field", func() {
			resource := ControllerResource{Primary: pointer.BoolPtr(false)}
			resourceCopy := resource.DeepCopy()

			SetDefaults_ControllerResource(&resource)

			Expect(resource.Primary).To(Equal(resourceCopy.Primary))
		})
	})

	Describe("#SetDefaults_ControllerDeployment", func() {
		var (
			ondemand = ControllerDeploymentPolicyOnDemand
			always   = ControllerDeploymentPolicyAlways
		)

		It("should default the policy field", func() {
			deployment := ControllerDeployment{}

			SetDefaults_ControllerDeployment(&deployment)

			Expect(deployment.Policy).To(PointTo(Equal(ondemand)))
		})

		It("should not default the policy field", func() {
			deployment := ControllerDeployment{Policy: &always}
			deploymentCopy := deployment.DeepCopy()

			SetDefaults_ControllerDeployment(&deployment)

			Expect(deployment.Policy).To(Equal(deploymentCopy.Policy))
		})
	})

	Describe("#SetDefaults_Seed", func() {
		var obj *Seed

		BeforeEach(func() {
			obj = &Seed{}
		})

		It("should default the seed settings (w/o taints)", func() {
			SetDefaults_Seed(obj)

			Expect(obj.Spec.Settings.ExcessCapacityReservation.Enabled).To(BeTrue())
			Expect(obj.Spec.Settings.Scheduling.Visible).To(BeTrue())
			Expect(obj.Spec.Settings.ShootDNS.Enabled).To(BeTrue())
		})

		It("should default the seed settings (w/ taints)", func() {
			obj.Spec.Taints = []SeedTaint{
				{Key: DeprecatedSeedTaintDisableCapacityReservation},
				{Key: DeprecatedSeedTaintInvisible},
				{Key: DeprecatedSeedTaintDisableDNS},
			}

			SetDefaults_Seed(obj)

			Expect(obj.Spec.Settings.ExcessCapacityReservation.Enabled).To(BeFalse())
			Expect(obj.Spec.Settings.Scheduling.Visible).To(BeFalse())
			Expect(obj.Spec.Settings.ShootDNS.Enabled).To(BeFalse())
		})

		It("should not default the seed settings because they were provided", func() {
			var (
				excessCapacityReservation = false
				scheduling                = true
				shootDNS                  = false
			)

			obj.Spec.Settings = &SeedSettings{
				ExcessCapacityReservation: &SeedSettingExcessCapacityReservation{Enabled: excessCapacityReservation},
				Scheduling:                &SeedSettingScheduling{Visible: scheduling},
				ShootDNS:                  &SeedSettingShootDNS{Enabled: shootDNS},
			}

			SetDefaults_Seed(obj)

			Expect(obj.Spec.Settings.ExcessCapacityReservation.Enabled).To(Equal(excessCapacityReservation))
			Expect(obj.Spec.Settings.Scheduling.Visible).To(Equal(scheduling))
			Expect(obj.Spec.Settings.ShootDNS.Enabled).To(Equal(shootDNS))
		})
	})
})
