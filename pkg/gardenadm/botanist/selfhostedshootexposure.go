// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"fmt"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component"
)

// DeploySelfHostedShootExposure populates the SelfHostedShootExposure spec with the addresses of all control-plane
// nodes and deploys the resource. It waits for the LoadBalancer to be provisioned and its ingress to be
// reflected in `.status.ingress`.
func (b *GardenadmBotanist) DeploySelfHostedShootExposure(ctx context.Context) error {
	nodes, err := b.listControlPlaneNodes(ctx)
	if err != nil {
		return err
	}

	endpoints := make([]extensionsv1alpha1.ControlPlaneEndpoint, 0, len(nodes))
	for _, node := range nodes {
		if len(node.Status.Addresses) == 0 {
			return fmt.Errorf("node %q has no addresses", node.Name)
		}
		endpoints = append(endpoints, extensionsv1alpha1.ControlPlaneEndpoint{
			NodeName:  node.Name,
			Addresses: node.Status.Addresses,
		})
	}

	b.Shoot.Components.Extensions.SelfHostedShootExposure.SetEndpoints(endpoints)

	return component.OpWait(b.Shoot.Components.Extensions.SelfHostedShootExposure).Deploy(ctx)
}
