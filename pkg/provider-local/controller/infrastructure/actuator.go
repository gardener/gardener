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

package infrastructure

import (
	"context"

	"github.com/go-logr/logr"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/infrastructure"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/provider-local/local"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

type actuator struct {
	client client.Client
}

// NewActuator creates a new Actuator that updates the status of the handled Infrastructure resources.
func NewActuator(mgr manager.Manager) infrastructure.Actuator {
	return &actuator{
		client: mgr.GetClient(),
	}
}

func (a *actuator) Reconcile(ctx context.Context, _ logr.Logger, infrastructure *extensionsv1alpha1.Infrastructure, cluster *extensionscontroller.Cluster) error {
	networkPolicyAllowToMachinePods := emptyNetworkPolicy("allow-to-machine-pods", infrastructure.Namespace)
	networkPolicyAllowToMachinePods.Spec = networkingv1.NetworkPolicySpec{
		Egress: []networkingv1.NetworkPolicyEgressRule{{
			To: []networkingv1.NetworkPolicyPeer{{
				PodSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "machine"}},
			}},
		}},
		PodSelector: metav1.LabelSelector{
			MatchLabels: map[string]string{v1beta1constants.LabelNetworkPolicyToShootNetworks: v1beta1constants.LabelNetworkPolicyAllowed},
		},
		PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
	}

	networkPolicyAllowMachinePods := emptyNetworkPolicy("allow-machine-pods", infrastructure.Namespace)
	networkPolicyAllowMachinePods.Spec = networkingv1.NetworkPolicySpec{
		Ingress: []networkingv1.NetworkPolicyIngressRule{{
			From: []networkingv1.NetworkPolicyPeer{
				{PodSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "machine"}}},
				{PodSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "vpn-seed-server"}}},
			},
		}},
		Egress: []networkingv1.NetworkPolicyEgressRule{{
			To: []networkingv1.NetworkPolicyPeer{
				{
					PodSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "machine"}},
				},
				{
					NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"role": "garden"}},
					PodSelector: &metav1.LabelSelector{MatchLabels: map[string]string{
						"app":       "nginx-ingress",
						"component": "controller",
					}},
				},
				{
					NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "registry"}},
					PodSelector:       &metav1.LabelSelector{MatchLabels: map[string]string{"app": "registry"}},
				},
			},
		}},
		PodSelector: metav1.LabelSelector{
			MatchLabels: map[string]string{"app": "machine"},
		},
		PolicyTypes: []networkingv1.PolicyType{
			networkingv1.PolicyTypeIngress,
			networkingv1.PolicyTypeEgress,
		},
	}

	for _, obj := range []client.Object{
		networkPolicyAllowToMachinePods,
		networkPolicyAllowMachinePods,
	} {
		if err := a.client.Patch(ctx, obj, client.Apply, local.FieldOwner, client.ForceOwnership); err != nil {
			return err
		}
	}

	return nil
}

func (a *actuator) Delete(ctx context.Context, _ logr.Logger, infrastructure *extensionsv1alpha1.Infrastructure, _ *extensionscontroller.Cluster) error {
	return kubernetesutils.DeleteObjects(ctx, a.client,
		emptyNetworkPolicy("allow-machine-pods", infrastructure.Namespace),
		emptyNetworkPolicy("allow-to-istio-ingress-gateway", infrastructure.Namespace),
		emptyNetworkPolicy("allow-to-provider-local-coredns", infrastructure.Namespace),
		emptyNetworkPolicy("allow-to-machine-pods", infrastructure.Namespace),
	)
}

func (a *actuator) Migrate(ctx context.Context, log logr.Logger, infrastructure *extensionsv1alpha1.Infrastructure, cluster *extensionscontroller.Cluster) error {
	return a.Delete(ctx, log, infrastructure, cluster)
}

func (a *actuator) Restore(ctx context.Context, log logr.Logger, infrastructure *extensionsv1alpha1.Infrastructure, cluster *extensionscontroller.Cluster) error {
	return a.Reconcile(ctx, log, infrastructure, cluster)
}

func emptyNetworkPolicy(name, namespace string) *networkingv1.NetworkPolicy {
	return &networkingv1.NetworkPolicy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: networkingv1.SchemeGroupVersion.String(),
			Kind:       "NetworkPolicy",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}
