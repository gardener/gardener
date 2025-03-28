// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
)

func (b *Botanist) checkInfraStatus(ctx context.Context) (bool, error) {
	infra, err := b.Shoot.Components.Extensions.Infrastructure.Get(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to get infra resource: %w", err)
	}
	result := len(infra.Status.Networking.Nodes) == 2
	return result, nil
}

func (b *Botanist) checkNetworkStatusIPFamilies(ctx context.Context) (bool, error) {
	network, err := b.Shoot.Components.Extensions.Network.Get(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to get network resource: %w", err)
	}
	providerStatus := network.Status.GetProviderStatus()
	if providerStatus.Raw == nil {
		return false, fmt.Errorf("network providerStatus is nil")
	}
	var networkStatus map[string]any
	if err := json.Unmarshal(providerStatus.Raw, &networkStatus); err != nil {
		return false, fmt.Errorf("failed to unmarshal network providerStatus: %w", err)
	}
	ipFamilies, ok := networkStatus["ipFamilies"]

	return ok && len(ipFamilies.([]any)) == 2, nil
}

// CheckPodCIDRsInNodes verifies the pod CIDRs in the nodes during dual-stack migration and updates the shoot's status accordingly.
func (b *Botanist) CheckPodCIDRsInNodes(ctx context.Context) error {
	if condition := v1beta1helper.GetCondition(b.Shoot.GetInfo().Status.Constraints, gardencorev1beta1.ShootDualStackNodesMigrationReady); condition == nil {
		return nil
	}

	infraReady, err := b.checkInfraStatus(ctx)
	if err != nil {
		return err
	}
	if !infraReady {
		return nil
	}

	networkReady, err := b.checkNetworkStatusIPFamilies(ctx)
	if err != nil {
		return err
	}
	if !networkReady {
		if b.ShootClientSet != nil {
			nodeList := &corev1.NodeList{}
			if err := b.ShootClientSet.Client().List(ctx, nodeList); err != nil {
				return fmt.Errorf("failed to list nodes during dual-stack migration: %w", err)
			}
			allNodesDualStack := true
			conditionStatus := gardencorev1beta1.ConditionFalse
			conditionReason := "NodesNotMigrated"
			conditionMessage := "Nodes are not migrated to dual-stack networking."
			for _, node := range nodeList.Items {
				allNodesDualStack = allNodesDualStack && len(node.Spec.PodCIDRs) == 2
			}
			if allNodesDualStack {
				conditionStatus = gardencorev1beta1.ConditionTrue
				conditionReason = "NodesMigrated"
				conditionMessage = "Nodes are migrated to dual-stack networking."
			}
			if err := b.Shoot.UpdateInfoStatus(ctx, b.GardenClient, true, func(shoot *gardencorev1beta1.Shoot) error {
				condition := v1beta1helper.GetOrInitConditionWithClock(b.Clock, shoot.Status.Constraints, gardencorev1beta1.ShootDualStackNodesMigrationReady)
				condition = v1beta1helper.UpdatedConditionWithClock(b.Clock, condition, conditionStatus, conditionReason, conditionMessage)
				shoot.Status.Constraints = v1beta1helper.MergeConditions(shoot.Status.Constraints, condition)
				return nil
			}); err != nil {
				return fmt.Errorf("failed to update shoot info status during dual-stack migration: %w", err)
			}
		}
	} else {
		if err := b.Shoot.UpdateInfoStatus(ctx, b.GardenClient, true, func(shoot *gardencorev1beta1.Shoot) error {
			shoot.Status.Constraints = v1beta1helper.RemoveConditions(shoot.Status.Constraints, gardencorev1beta1.ShootDualStackNodesMigrationReady)
			return nil
		}); err != nil {
			return fmt.Errorf("failed to update shoot info status during dual-stack migration: %w", err)
		}
	}
	return nil
}

// UpdateDualStackMigrationConditionIfNeeded checks if the shoot should be migrated to dual-stack networking and sets the shoot status accordingly.
func (b *Botanist) UpdateDualStackMigrationConditionIfNeeded(ctx context.Context) error {
	shoot := b.Shoot.GetInfo()

	condition := v1beta1helper.GetCondition(shoot.Status.Constraints, gardencorev1beta1.ShootDualStackNodesMigrationReady)
	if condition == nil && len(shoot.Spec.Networking.IPFamilies) == 2 && shoot.Status.Networking != nil && len(shoot.Status.Networking.Nodes) == 1 {
		err := b.Shoot.UpdateInfoStatus(ctx, b.GardenClient, true, func(shoot *gardencorev1beta1.Shoot) error {
			condition := v1beta1helper.GetOrInitConditionWithClock(b.Clock, shoot.Status.Constraints, gardencorev1beta1.ShootDualStackNodesMigrationReady)
			condition = v1beta1helper.UpdatedConditionWithClock(b.Clock, condition, gardencorev1beta1.ConditionFalse, "DualStackMigration", "The shoot is migrating to dual-stack networking.")
			shoot.Status.Constraints = v1beta1helper.MergeConditions(shoot.Status.Constraints, condition)
			return nil
		})
		if err != nil {
			return fmt.Errorf("error while updating shoot status in UpdateDualStackMigrationConditionIfNeeded: %w", err)
		}
	}
	return nil
}
