// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"fmt"
	"slices"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1helper "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1/helper"
	"github.com/gardener/gardener/pkg/component"
)

// DeployBootstrapDNSRecord deploys the external DNSRecord pointing to the first control plane machine for bootstrapping
// the self-hosted shoot cluster. The DNSRecord might be publicly resolvable but not publicly accessible, depending on
// shoot's infrastructure provider and network setup. It should be resolvable and accessible from within the shoot
// cluster's network including the machines, so `gardenadm init` can use it to bootstrap the cluster.
func (b *GardenadmBotanist) DeployBootstrapDNSRecord(ctx context.Context) error {
	machine, err := b.GetMachineByIndex(0)
	if err != nil {
		return err
	}

	machineAddr, err := PreferredAddress(machine.Status.Addresses)
	if err != nil {
		return fmt.Errorf("failed getting preferred address for machine %q: %w", machine.Name, err)
	}

	b.Shoot.Components.Extensions.ExternalDNSRecord.SetRecordType(extensionsv1alpha1helper.GetDNSRecordType(machineAddr))
	b.Shoot.Components.Extensions.ExternalDNSRecord.SetValues([]string{machineAddr})

	return component.OpWait(b.Shoot.Components.Extensions.ExternalDNSRecord).Deploy(ctx)
}

// RestoreBootstrapDNSRecord restores the external DNSRecord pointing to the first control plane node for bootstrapping
// the self-hosted shoot cluster.
func (b *GardenadmBotanist) RestoreBootstrapDNSRecord(ctx context.Context) error {
	// The DNSRecord values are not persisted in the ShootState, so we need to recalculate them from the Node objects.
	// This is the same logic as in DeployBootstrapDNSRecord, but we fetch the control plane Node objects using a label
	// selector instead of fetching the Machine objects.
	// Also, there might be more than one control plane node, so we just pick the oldest one, which should be the one
	// provisioned by `gardenadm bootstrap`. We expect that any bootstrapped control plane node accepts traffic on its
	// internal address, so picking one should be sufficient during the bootstrap procedure.
	// Putting all control plane nodes in the DNSRecord would require consistent address types across all nodes, so we
	// avoid that handling for now. After all, the DNSRecord is only temporary until the LoadBalancer is ready.
	controlPlaneWorkerPool := v1beta1helper.ControlPlaneWorkerPoolForShoot(b.Shoot.GetInfo().Spec.Provider.Workers)
	if controlPlaneWorkerPool == nil {
		return fmt.Errorf("failed fetching the control plane worker pool for the shoot")
	}

	nodeList := &corev1.NodeList{}
	if err := b.SeedClientSet.Client().List(ctx, nodeList, client.MatchingLabels{v1beta1constants.LabelWorkerPool: controlPlaneWorkerPool.Name}); err != nil {
		return fmt.Errorf("failed to list machines: %w", err)
	}
	if len(nodeList.Items) == 0 {
		return fmt.Errorf("no control plane nodes founds")
	}

	// pick the oldest node
	slices.SortStableFunc(nodeList.Items, func(a, b corev1.Node) int {
		return a.CreationTimestamp.Compare(b.CreationTimestamp.Time)
	})
	node := &nodeList.Items[0]

	nodeAddr, err := PreferredAddress(node.Status.Addresses)
	if err != nil {
		return fmt.Errorf("failed getting preferred address for node %q: %w", node.Name, err)
	}

	b.Shoot.Components.Extensions.ExternalDNSRecord.SetRecordType(extensionsv1alpha1helper.GetDNSRecordType(nodeAddr))
	b.Shoot.Components.Extensions.ExternalDNSRecord.SetValues([]string{nodeAddr})

	if err := b.Shoot.Components.Extensions.ExternalDNSRecord.Restore(ctx, b.Shoot.GetShootState()); err != nil {
		return err
	}
	return b.Shoot.Components.Extensions.ExternalDNSRecord.Wait(ctx)
}
