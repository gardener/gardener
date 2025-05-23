// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
)

// DetermineUpdateFunction determines the update function for the shoot's status based on dual-stack migration readiness.
func (b *Botanist) DetermineUpdateFunction(
	networkReadyForDualStackMigration bool,
	nodeList *corev1.NodeList,
) func(*gardencorev1beta1.Shoot) error {
	if networkReadyForDualStackMigration {
		return func(shoot *gardencorev1beta1.Shoot) error {
			shoot.Status.Constraints = v1beta1helper.RemoveConditions(shoot.Status.Constraints, gardencorev1beta1.ShootDualStackNodesMigrationReady)
			return nil
		}
	}

	allNodesDualStack := true
	conditionStatus := gardencorev1beta1.ConditionFalse
	conditionReason := "NodesNotMigrated"
	conditionMessage := "Not all nodes were migrated to dual-stack networking."
	for _, node := range nodeList.Items {
		allNodesDualStack = allNodesDualStack && len(node.Spec.PodCIDRs) == 2
	}
	if allNodesDualStack {
		conditionStatus = gardencorev1beta1.ConditionTrue
		conditionReason = "NodesMigrated"
		conditionMessage = "All nodes were migrated to dual-stack networking."
	}

	return func(shoot *gardencorev1beta1.Shoot) error {
		constraint := v1beta1helper.GetOrInitConditionWithClock(b.Clock, shoot.Status.Constraints, gardencorev1beta1.ShootDualStackNodesMigrationReady)
		constraint = v1beta1helper.UpdatedConditionWithClock(b.Clock, constraint, conditionStatus, conditionReason, conditionMessage)
		shoot.Status.Constraints = v1beta1helper.MergeConditions(shoot.Status.Constraints, constraint)
		return nil
	}
}

// CheckPodCIDRsInNodes verifies the pod CIDRs in the nodes during dual-stack migration and updates the shoot's status accordingly.
func (b *Botanist) CheckPodCIDRsInNodes(ctx context.Context) error {
	if condition := v1beta1helper.GetCondition(b.Shoot.GetInfo().Status.Constraints, gardencorev1beta1.ShootDualStackNodesMigrationReady); condition == nil {
		return nil
	}

	infrastructure, err := b.Shoot.Components.Extensions.Infrastructure.Get(ctx)
	if err != nil {
		return fmt.Errorf("failed getting Infrastructure resource: %w", err)
	}

	infrastructureReadyForDualStackMigration := len(infrastructure.Status.Networking.Nodes) == 2
	if !infrastructureReadyForDualStackMigration {
		return nil
	}

	network, err := b.Shoot.Components.Extensions.Network.Get(ctx)
	if err != nil {
		return fmt.Errorf("failed to get network resource: %w", err)
	}

	nodeList := &corev1.NodeList{}
	if err := b.ShootClientSet.Client().List(ctx, nodeList); err != nil {
		return fmt.Errorf("failed to list nodes during dual-stack migration: %w", err)
	}

	networkReadyForDualStackMigration := len(network.Status.IPFamilies) == 2
	updateFunction := b.DetermineUpdateFunction(networkReadyForDualStackMigration, nodeList)
	if err := b.Shoot.UpdateInfoStatus(ctx, b.GardenClient, true, false, updateFunction); err != nil {
		return fmt.Errorf("failed to update shoot info status during dual-stack migration: %w", err)
	}
	return nil
}

// UpdateDualStackMigrationConditionIfNeeded checks if the shoot should be migrated to dual-stack networking and sets the shoot status accordingly.
func (b *Botanist) UpdateDualStackMigrationConditionIfNeeded(ctx context.Context) error {
	shoot := b.Shoot.GetInfo()

	constraint := v1beta1helper.GetCondition(shoot.Status.Constraints, gardencorev1beta1.ShootDualStackNodesMigrationReady)
	if constraint == nil && len(shoot.Spec.Networking.IPFamilies) == 2 && shoot.Status.Networking != nil && len(shoot.Status.Networking.Nodes) == 1 {
		if err := b.Shoot.UpdateInfoStatus(ctx, b.GardenClient, true, false, func(shoot *gardencorev1beta1.Shoot) error {
			constraint := v1beta1helper.GetOrInitConditionWithClock(b.Clock, shoot.Status.Constraints, gardencorev1beta1.ShootDualStackNodesMigrationReady)
			constraint = v1beta1helper.UpdatedConditionWithClock(b.Clock, constraint, gardencorev1beta1.ConditionFalse, "DualStackMigration", "The shoot is migrating to dual-stack networking.")
			shoot.Status.Constraints = v1beta1helper.MergeConditions(shoot.Status.Constraints, constraint)
			return nil
		}); err != nil {
			return fmt.Errorf("failed updating %s constraint in shoot status: %w", gardencorev1beta1.ShootDualStackNodesMigrationReady, err)
		}
	}
	return nil
}
