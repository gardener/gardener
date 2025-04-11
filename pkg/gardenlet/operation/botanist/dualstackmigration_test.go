// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
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
	})

	Describe("#DetermineUpdateFunction", func() {
		It("Removes the constraint when network is ready for dual-stack migration", func() {
			nodeList := &corev1.NodeList{
				Items: []corev1.Node{
					{Spec: corev1.NodeSpec{PodCIDRs: []string{"10.1.0.0/24", "fd01::/64"}}},
				},
			}

			shoot := &gardencorev1beta1.Shoot{
				Status: gardencorev1beta1.ShootStatus{
					Constraints: []gardencorev1beta1.Condition{
						v1beta1helper.InitConditionWithClock(mockClock, gardencorev1beta1.ShootDualStackNodesMigrationReady),
					},
				},
			}

			updateFunc := botanist.DetermineUpdateFunction(true, nodeList)
			err := updateFunc(shoot)
			Expect(err).NotTo(HaveOccurred())
			Expect(shoot.Status.Constraints).To(BeEmpty())
		})

		It("Updates the constraint to ConditionTrue when all nodes are dual-stack", func() {
			nodeList := &corev1.NodeList{
				Items: []corev1.Node{
					{Spec: corev1.NodeSpec{PodCIDRs: []string{"10.1.0.0/24", "fd01::/64"}}},
					{Spec: corev1.NodeSpec{PodCIDRs: []string{"10.1.0.0/24", "fd01::/64"}}},
				},
			}

			shoot := &gardencorev1beta1.Shoot{
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
			Expect(condition.Message).To(Equal("All nodes were migrated to dual-stack networking."))
		})

		It("Updates the constraint to ConditionFalse when not all nodes are dual-stack", func() {
			nodeList := &corev1.NodeList{
				Items: []corev1.Node{
					{Spec: corev1.NodeSpec{PodCIDRs: []string{"10.1.0.0/24"}}},
					{Spec: corev1.NodeSpec{PodCIDRs: []string{"10.1.0.0/24", "fd01::/64"}}},
				},
			}

			shoot := &gardencorev1beta1.Shoot{
				Status: gardencorev1beta1.ShootStatus{
					Constraints: []gardencorev1beta1.Condition{},
				},
			}

			updateFunc := botanist.DetermineUpdateFunction(false, nodeList)
			err := updateFunc(shoot)
			Expect(err).NotTo(HaveOccurred())

			condition := v1beta1helper.GetCondition(shoot.Status.Constraints, gardencorev1beta1.ShootDualStackNodesMigrationReady)
			Expect(condition).NotTo(BeNil())
			Expect(condition.Status).To(Equal(gardencorev1beta1.ConditionFalse))
			Expect(condition.Reason).To(Equal("NodesNotMigrated"))
			Expect(condition.Message).To(Equal("Not all nodes were migrated to dual-stack networking."))
		})
	})
})
