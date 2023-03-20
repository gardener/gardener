// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

func (m *mutator) InjectClient(client client.Client) error {
	m.client = client
	return nil
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
