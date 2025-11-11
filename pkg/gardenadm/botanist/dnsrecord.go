// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
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

// RestoreExternalDNSRecord restores the external DNSRecord pointing to the control plane nodes based on the extension
// state from the bootstrap cluster stored by `gardenadm bootstrap` in the ShootState.
func (b *GardenadmBotanist) RestoreExternalDNSRecord(ctx context.Context) error {
	// The DNSRecord values are not persisted in the ShootState, so we recalculate them from the Node objects.
	// This is the same logic as in DeployBootstrapDNSRecord, but we fetch the control plane Node objects using a label
	// selector instead of fetching the Machine objects.
	// We expect that any bootstrapped control plane node accepts traffic on its preferred address.
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

	// Collect preferred addresses of all control plane nodes and ensure they are of the same type.
	var (
		values     []string
		recordType extensionsv1alpha1.DNSRecordType
	)

	for _, node := range nodeList.Items {
		nodeAddr, err := PreferredAddress(node.Status.Addresses)
		if err != nil {
			return fmt.Errorf("failed getting preferred address for node %q: %w", node.Name, err)
		}
		values = append(values, nodeAddr)

		currentRecordType := extensionsv1alpha1helper.GetDNSRecordType(nodeAddr)
		if recordType == "" {
			recordType = currentRecordType
		} else if recordType != currentRecordType {
			return fmt.Errorf("inconsistent address types for control plane nodes: found both %q and %q: %v", recordType, currentRecordType, values)
		}
	}

	b.Shoot.Components.Extensions.ExternalDNSRecord.SetRecordType(recordType)
	b.Shoot.Components.Extensions.ExternalDNSRecord.SetValues(values)

	if err := b.Shoot.Components.Extensions.ExternalDNSRecord.Restore(ctx, b.Shoot.GetShootState()); err != nil {
		return err
	}
	return b.Shoot.Components.Extensions.ExternalDNSRecord.Wait(ctx)
}
