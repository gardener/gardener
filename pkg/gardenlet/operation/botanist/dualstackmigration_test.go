// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/clock/testing"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	. "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
)

var _ = Describe("DualStackMigration", func() {

	var (
		botanist  *Botanist
		mockClock *testing.FakeClock
	)

	BeforeEach(func() {
		mockClock = testing.NewFakeClock(time.Date(2025, 3, 30, 3, 33, 33, 33, time.UTC))

		botanist = &Botanist{Operation: &operation.Operation{
			Shoot: &shootpkg.Shoot{
				Components: &shootpkg.Components{
					Extensions: &shootpkg.Extensions{},
				},
			},
			Clock: mockClock,
		}}

		// Initialize the shoot with proper IP families for dual-stack tests
		botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-shoot",
				Namespace: "test-namespace",
			},
			Spec: gardencorev1beta1.ShootSpec{
				Networking: &gardencorev1beta1.Networking{
					IPFamilies: []gardencorev1beta1.IPFamily{
						gardencorev1beta1.IPFamilyIPv4,
						gardencorev1beta1.IPFamilyIPv6,
					},
				},
			},
		})
	})

	Describe("#DetermineUpdateFunction", func() {
		It("Removes the constraint when network is ready for dual-stack migration and adds DNSServiceMigrationReady", func() {
			nodeList := &corev1.NodeList{
				Items: []corev1.Node{
					{Spec: corev1.NodeSpec{PodCIDRs: []string{"10.1.0.0/24", "fd01::/64"}}},
				},
			}

			shoot := &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-shoot",
					Namespace:   "test-namespace",
					Annotations: map[string]string{},
				},
				Spec: gardencorev1beta1.ShootSpec{
					Networking: &gardencorev1beta1.Networking{
						IPFamilies: []gardencorev1beta1.IPFamily{
							gardencorev1beta1.IPFamilyIPv4,
							gardencorev1beta1.IPFamilyIPv6,
						},
					},
				},
				Status: gardencorev1beta1.ShootStatus{
					Constraints: []gardencorev1beta1.Condition{
						v1beta1helper.InitConditionWithClock(mockClock, gardencorev1beta1.ShootDualStackNodesMigrationReady),
					},
				},
			}

			updateFunc := botanist.DetermineUpdateFunction(true, nodeList)
			err := updateFunc(shoot)
			Expect(err).NotTo(HaveOccurred())
			condition := v1beta1helper.GetCondition(shoot.Status.Constraints, gardencorev1beta1.ShootDualStackNodesMigrationReady)
			Expect(condition).To(BeNil())

			condition = v1beta1helper.GetCondition(shoot.Status.Constraints, gardencorev1beta1.ShootDNSServiceMigrationReady)
			Expect(condition.Status).To(Equal(gardencorev1beta1.ConditionProgressing))
		})

		It("Updates the constraint to ConditionTrue when all nodes are dual-stack", func() {
			nodeList := &corev1.NodeList{
				Items: []corev1.Node{
					{Spec: corev1.NodeSpec{PodCIDRs: []string{"10.1.0.0/24", "fd01::/64"}}},
					{Spec: corev1.NodeSpec{PodCIDRs: []string{"10.1.0.0/24", "fd01::/64"}}},
				},
			}

			shoot := &gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Networking: &gardencorev1beta1.Networking{
						IPFamilies: []gardencorev1beta1.IPFamily{
							gardencorev1beta1.IPFamilyIPv4,
							gardencorev1beta1.IPFamilyIPv6,
						},
					},
				},
				Status: gardencorev1beta1.ShootStatus{
					Constraints: []gardencorev1beta1.Condition{},
				},
			}

			updateFunc := botanist.DetermineUpdateFunction(false, nodeList)
			err := updateFunc(shoot)
			Expect(err).NotTo(HaveOccurred())

			condition := v1beta1helper.GetCondition(shoot.Status.Constraints, gardencorev1beta1.ShootDualStackNodesMigrationReady)
			Expect(condition).NotTo(BeNil())
			Expect(condition.Status).To(Equal(gardencorev1beta1.ConditionTrue))
			Expect(condition.Reason).To(Equal("NodesMigrated"))
			Expect(condition.Message).To(Equal("All node pod CIDRs migrated to target network stack.")) // Updated message
		})

		It("Updates the constraint to ConditionFalse when not all nodes are dual-stack", func() {
			nodeList := &corev1.NodeList{
				Items: []corev1.Node{
					{Spec: corev1.NodeSpec{PodCIDRs: []string{"10.1.0.0/24"}}},
					{Spec: corev1.NodeSpec{PodCIDRs: []string{"10.1.0.0/24", "fd01::/64"}}},
				},
			}

			shoot := &gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Networking: &gardencorev1beta1.Networking{
						IPFamilies: []gardencorev1beta1.IPFamily{
							gardencorev1beta1.IPFamilyIPv4,
							gardencorev1beta1.IPFamilyIPv6,
						},
					},
				},
				Status: gardencorev1beta1.ShootStatus{
					Constraints: []gardencorev1beta1.Condition{},
				},
			}

			updateFunc := botanist.DetermineUpdateFunction(false, nodeList)
			err := updateFunc(shoot)
			Expect(err).NotTo(HaveOccurred())

			condition := v1beta1helper.GetCondition(shoot.Status.Constraints, gardencorev1beta1.ShootDualStackNodesMigrationReady)
			Expect(condition).NotTo(BeNil())
			Expect(condition.Status).To(Equal(gardencorev1beta1.ConditionProgressing))
			Expect(condition.Reason).To(Equal("NodesNotMigrated"))
			Expect(condition.Message).To(Equal("Migrating node pod CIDRs to match target network stack.")) // Updated message
		})
		It("Updates the constraint to ConditionProgressing if coredns pods not are migrated", func() {

			shoot := &gardencorev1beta1.Shoot{
				Status: gardencorev1beta1.ShootStatus{
					Constraints: []gardencorev1beta1.Condition{},
				},
			}

			updateFunc := botanist.DetermineUpdateFunctionDNS(false, false)
			err := updateFunc(shoot)
			Expect(err).NotTo(HaveOccurred())

			condition := v1beta1helper.GetCondition(shoot.Status.Constraints, gardencorev1beta1.ShootDNSServiceMigrationReady)
			Expect(condition).NotTo(BeNil())
			Expect(condition.Status).To(Equal(gardencorev1beta1.ConditionProgressing))
		})

		It("Updates the constraint to ConditionTrue if all coredns pods are migrated", func() {

			shoot := &gardencorev1beta1.Shoot{
				Status: gardencorev1beta1.ShootStatus{
					Constraints: []gardencorev1beta1.Condition{
						v1beta1helper.InitConditionWithClock(mockClock, gardencorev1beta1.ShootDNSServiceMigrationReady),
					},
				},
			}

			updateFunc := botanist.DetermineUpdateFunctionDNS(false, true)
			err := updateFunc(shoot)
			Expect(err).NotTo(HaveOccurred())

			condition := v1beta1helper.GetCondition(shoot.Status.Constraints, gardencorev1beta1.ShootDNSServiceMigrationReady)
			Expect(condition).NotTo(BeNil())
			Expect(condition.Status).To(Equal(gardencorev1beta1.ConditionTrue))
		})
		It("Removes the constraint if svc is migrated", func() {

			shoot := &gardencorev1beta1.Shoot{
				Status: gardencorev1beta1.ShootStatus{
					Constraints: []gardencorev1beta1.Condition{
						v1beta1helper.InitConditionWithClock(mockClock, gardencorev1beta1.ShootDNSServiceMigrationReady),
					},
				},
			}

			updateFunc := botanist.DetermineUpdateFunctionDNS(true, true)
			err := updateFunc(shoot)
			Expect(err).NotTo(HaveOccurred())

			condition := v1beta1helper.GetCondition(shoot.Status.Constraints, gardencorev1beta1.ShootDNSServiceMigrationReady)
			Expect(condition).To(BeNil())
		})

	})
})
