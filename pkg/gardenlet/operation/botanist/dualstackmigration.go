// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	corednsconstants "github.com/gardener/gardener/pkg/component/networking/coredns/constants"
)

func nodesMigrated(nodeList *corev1.NodeList, ipFamiliesLen int) bool {
	if nodeList == nil || len(nodeList.Items) == 0 {
		return false
	}
	for _, node := range nodeList.Items {
		if len(node.Spec.PodCIDRs) != ipFamiliesLen {
			return false
		}
	}
	return true
}

// DetermineUpdateFunction determines the update function for the shoot's status based on dual-stack migration readiness.
func (b *Botanist) DetermineUpdateFunction(networkReadyForDualStackMigration bool, nodeList *corev1.NodeList) func(*gardencorev1beta1.Shoot) error {
	shootIPFamilies := b.Shoot.GetInfo().Spec.Networking.IPFamilies
	isSingleStack := len(shootIPFamilies) == 1
	allNodesMigrated := nodesMigrated(nodeList, len(shootIPFamilies))

	if networkReadyForDualStackMigration {
		return b.createConstraintRemovalFunction(isSingleStack, allNodesMigrated)
	}
	return b.createNodeMigrationFunction(isSingleStack, allNodesMigrated)
}

func (b *Botanist) createConstraintRemovalFunction(isSingleStack, allNodesMigrated bool) func(*gardencorev1beta1.Shoot) error {
	return func(shoot *gardencorev1beta1.Shoot) error {
		if isSingleStack && !allNodesMigrated {
			return nil // Don't remove constraint yet
		}
		if v1beta1helper.GetCondition(shoot.Status.Constraints, gardencorev1beta1.ShootDualStackNodesMigrationReady) == nil {
			return nil // Constraint already removed
		}
		shoot.Status.Constraints = v1beta1helper.RemoveConditions(shoot.Status.Constraints, gardencorev1beta1.ShootDualStackNodesMigrationReady)
		if !isSingleStack && !b.hasDNSMigrationConstraint(shoot) {
			b.addDNSMigrationConstraint(shoot)
		}
		return nil
	}
}

func (b *Botanist) createNodeMigrationFunction(isSingleStack, allNodesMigrated bool) func(*gardencorev1beta1.Shoot) error {
	status := gardencorev1beta1.ConditionProgressing
	reason := "NodesNotMigrated"
	message := "Migrating node pod CIDRs to match target network stack."

	if allNodesMigrated {
		status = gardencorev1beta1.ConditionTrue
		reason = "NodesMigrated"
		message = "All node pod CIDRs migrated to target network stack."
	} else if isSingleStack {
		status = gardencorev1beta1.ConditionTrue
		reason = "NodesMigrating"
		message = "Node pod CIDRs are currently migrated to target network stack."
	}

	return func(shoot *gardencorev1beta1.Shoot) error {
		constraint := v1beta1helper.GetOrInitConditionWithClock(b.Clock, shoot.Status.Constraints, gardencorev1beta1.ShootDualStackNodesMigrationReady)
		constraint = v1beta1helper.UpdatedConditionWithClock(b.Clock, constraint, status, reason, message)
		shoot.Status.Constraints = v1beta1helper.MergeConditions(shoot.Status.Constraints, constraint)
		return nil
	}
}

func (b *Botanist) hasDNSMigrationConstraint(shoot *gardencorev1beta1.Shoot) bool {
	return v1beta1helper.GetCondition(shoot.Status.Constraints, gardencorev1beta1.ShootDNSServiceMigrationReady) != nil
}

func (b *Botanist) addDNSMigrationConstraint(shoot *gardencorev1beta1.Shoot) {
	constraint := v1beta1helper.GetOrInitConditionWithClock(b.Clock, shoot.Status.Constraints, gardencorev1beta1.ShootDNSServiceMigrationReady)
	constraint = v1beta1helper.UpdatedConditionWithClock(b.Clock, constraint, gardencorev1beta1.ConditionProgressing, "DNSServiceMigration", "The shoot is migrating the kube-dns service.")
	shoot.Status.Constraints = v1beta1helper.MergeConditions(shoot.Status.Constraints, constraint)
}

// DetermineUpdateFunctionDNS determines the update function for the shoot's status based on DNS service and pod readiness.
func (b *Botanist) DetermineUpdateFunctionDNS(svcReady, podsReady bool) func(*gardencorev1beta1.Shoot) error {
	if svcReady {
		return func(shoot *gardencorev1beta1.Shoot) error {
			shoot.Status.Constraints = v1beta1helper.RemoveConditions(shoot.Status.Constraints, gardencorev1beta1.ShootDNSServiceMigrationReady)
			return nil
		}
	}

	if podsReady {
		return func(shoot *gardencorev1beta1.Shoot) error {
			constraint := v1beta1helper.GetOrInitConditionWithClock(b.Clock, shoot.Status.Constraints, gardencorev1beta1.ShootDNSServiceMigrationReady)
			constraint = v1beta1helper.UpdatedConditionWithClock(b.Clock, constraint, gardencorev1beta1.ConditionTrue, "DNSServiceMigration", "The coreDNS pods are ready for DNS service migration.")
			shoot.Status.Constraints = v1beta1helper.MergeConditions(shoot.Status.Constraints, constraint)
			return nil
		}
	}
	return func(shoot *gardencorev1beta1.Shoot) error {
		constraint := v1beta1helper.GetCondition(shoot.Status.Constraints, gardencorev1beta1.ShootDNSServiceMigrationReady)
		if constraint == nil {
			constraint := v1beta1helper.GetOrInitConditionWithClock(b.Clock, shoot.Status.Constraints, gardencorev1beta1.ShootDNSServiceMigrationReady)
			constraint = v1beta1helper.UpdatedConditionWithClock(b.Clock, constraint, gardencorev1beta1.ConditionProgressing, "DNSServiceMigration", "The shoot is migrating the kube-dns service.")
			shoot.Status.Constraints = v1beta1helper.MergeConditions(shoot.Status.Constraints, constraint)
		}
		return nil
	}
}

// CheckDNSServiceMigration checks the DNS service migration status.
func (b *Botanist) CheckDNSServiceMigration(ctx context.Context) error {
	if condition := v1beta1helper.GetCondition(b.Shoot.GetInfo().Status.Constraints, gardencorev1beta1.ShootDNSServiceMigrationReady); condition == nil {
		return nil
	}

	service := &corev1.Service{}
	if err := b.ShootClientSet.Client().Get(ctx, client.ObjectKey{Namespace: metav1.NamespaceSystem, Name: corednsconstants.LabelValue}, service); err != nil {
		return err
	}

	svcReady := len(service.Spec.ClusterIPs) == len(b.Shoot.GetInfo().Spec.Networking.IPFamilies)

	podList := &corev1.PodList{}
	if err := b.ShootClientSet.Client().List(ctx, podList, client.InNamespace(metav1.NamespaceSystem), client.MatchingLabels{corednsconstants.LabelKey: corednsconstants.LabelValue}); err != nil {
		return err
	}
	if len(podList.Items) == 0 {
		return nil
	}

	podsReady := len(podList.Items) != 0

	for _, pod := range podList.Items {
		podsReady = podsReady && len(pod.Status.PodIPs) == len(b.Shoot.GetInfo().Spec.Networking.IPFamilies)
	}

	updateFunction := b.DetermineUpdateFunctionDNS(svcReady, podsReady)
	if err := b.Shoot.UpdateInfoStatus(ctx, b.GardenClient, true, false, updateFunction); err != nil {
		return fmt.Errorf("failed to update shoot info status during network stack migration: %w", err)
	}
	return nil
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
		return fmt.Errorf("failed to list nodes during network stack migration: %w", err)
	}

	networkReadyForDualStackMigration := len(network.Status.IPFamilies) == len(b.Shoot.GetInfo().Spec.Networking.IPFamilies)
	updateFunction := b.DetermineUpdateFunction(networkReadyForDualStackMigration, nodeList)
	if err := b.Shoot.UpdateInfoStatus(ctx, b.GardenClient, true, false, updateFunction); err != nil {
		return fmt.Errorf("failed to update shoot info status during network stack migration: %w", err)
	}

	return nil
}

// UpdateDualStackMigrationConditionIfNeeded checks if the shoot should be migrated to dual-stack networking and sets the shoot status accordingly.
func (b *Botanist) UpdateDualStackMigrationConditionIfNeeded(ctx context.Context) error {
	shoot := b.Shoot.GetInfo()

	nodeList := &corev1.NodeList{}
	if b.ShootClientSet != nil {
		if err := b.ShootClientSet.Client().List(ctx, nodeList); err != nil {
			return fmt.Errorf("failed to list nodes during network stack migration: %w", err)
		}
	}

	if constraint := v1beta1helper.GetCondition(shoot.Status.Constraints, gardencorev1beta1.ShootDualStackNodesMigrationReady); constraint == nil {
		network, err := b.Shoot.Components.Extensions.Network.Get(ctx)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil // Network not yet created, nothing to do
			}
			return fmt.Errorf("failed to get network resource: %w", err)
		}

		if network == nil || network.Status.IPFamilies == nil {
			return nil
		}

		networkReadyForDualStackMigration := len(network.Status.IPFamilies) == len(b.Shoot.GetInfo().Spec.Networking.IPFamilies)
		updateFunction := b.DetermineUpdateFunction(networkReadyForDualStackMigration, nodeList)
		if err := b.Shoot.UpdateInfoStatus(ctx, b.GardenClient, true, false, updateFunction); err != nil {
			return fmt.Errorf("failed updating %s constraint in shoot status: %w", gardencorev1beta1.ShootDualStackNodesMigrationReady, err)
		}
	}
	return nil
}
