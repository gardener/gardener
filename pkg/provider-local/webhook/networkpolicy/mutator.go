// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package networkpolicy

import (
	"context"
	"fmt"

	networkingv1 "k8s.io/api/networking/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
)

type mutator struct {
	client client.Client
}

func (m *mutator) Mutate(ctx context.Context, newObj, _ client.Object) error {
	if newObj.GetName() != "allow-to-private-networks" {
		return nil
	}

	networkPolicy, ok := newObj.(*networkingv1.NetworkPolicy)
	if !ok {
		return fmt.Errorf("unexpected object, got %T wanted *networkingv1.NetworkPolicy", newObj)
	}

	cluster, err := extensionscontroller.GetCluster(ctx, m.client, networkPolicy.Namespace)
	if err != nil {
		return err
	}

	if cluster.Seed.Spec.Networks.Nodes == nil {
		return nil
	}

	for i, egress := range networkPolicy.Spec.Egress {
		for j, to := range egress.To {
			if to.IPBlock == nil {
				continue
			}

			for k, except := range to.IPBlock.Except {
				if except == *cluster.Seed.Spec.Networks.Nodes {
					networkPolicy.Spec.Egress[i].To[j].IPBlock.Except = append(networkPolicy.Spec.Egress[i].To[j].IPBlock.Except[:k], networkPolicy.Spec.Egress[i].To[j].IPBlock.Except[k+1:]...)
				}
			}
		}
	}

	return nil
}
