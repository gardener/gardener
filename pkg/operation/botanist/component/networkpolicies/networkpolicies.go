// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package networkpolicies

import (
	"context"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/operation/botanist/component"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// New creates a new instance of DeployWaiter for the network policies.
// Deprecated.
// TODO(rfrankze): Remove this component in a future release.
func New(client client.Client, namespace string) component.Deployer {
	return &networkPolicies{
		client:    client,
		namespace: namespace,
	}
}

type networkPolicies struct {
	client    client.Client
	namespace string
}

func (n *networkPolicies) Deploy(ctx context.Context) error {
	return n.Destroy(ctx)
}

func (n *networkPolicies) Destroy(ctx context.Context) error {
	return kubernetesutils.DeleteObjects(ctx, n.client,
		&networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-to-aggregate-prometheus", Namespace: n.namespace}},
		&networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-to-seed-prometheus", Namespace: n.namespace}},
		&networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-to-all-shoot-apiservers", Namespace: n.namespace}},
	)
}
