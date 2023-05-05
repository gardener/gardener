// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/extensions/pkg/controller/extension"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	kubernetesclient "github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/extensions"
	"github.com/gardener/gardener/pkg/provider-local/local"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

const (
	// ApplicationName is the name of the application.
	ApplicationName string = "provider-local-networkpolicy"
	// ManagedResourceName is the name for the managedResource.
	ManagedResourceName string = ApplicationName
)

type actuator struct {
	client client.Client
}

// NewActuator returns an actuator responsible for Extension resources.
func NewActuator() extension.Actuator {
	return &actuator{}
}

// InjectClient injects the controller runtime client into the reconciler.
func (a *actuator) InjectClient(client client.Client) error {
	a.client = client
	return nil
}

// Reconcile the extension resource.
func (a *actuator) Reconcile(ctx context.Context, log logr.Logger, ex *extensionsv1alpha1.Extension) error {
	var namespace = ex.Namespace

	cluster, err := extensions.GetCluster(ctx, a.client, namespace)
	if err != nil {
		return err
	}

	seedResources, err := getSeedResources(cluster, namespace)
	if err != nil {
		return err
	}

	if err := managedresources.CreateForSeed(ctx, a.client, namespace, ManagedResourceName, false, seedResources); err != nil {
		return err
	}

	twoMinutes := 2 * time.Minute
	timeoutSeedCtx, cancelSeedCtx := context.WithTimeout(ctx, twoMinutes)
	defer cancelSeedCtx()
	return managedresources.WaitUntilHealthy(timeoutSeedCtx, a.client, namespace, ManagedResourceName)
}

// Delete the extension resource.
func (a *actuator) Delete(ctx context.Context, log logr.Logger, ex *extensionsv1alpha1.Extension) error {
	namespace := ex.GetNamespace()
	twoMinutes := 2 * time.Minute

	timeoutSeedCtx, cancelSeedCtx := context.WithTimeout(ctx, twoMinutes)
	defer cancelSeedCtx()

	if err := managedresources.DeleteForSeed(ctx, a.client, namespace, ManagedResourceName); err != nil {
		return err
	}

	return managedresources.WaitUntilDeleted(timeoutSeedCtx, a.client, namespace, ManagedResourceName)
}

// Migrate the extension resource.
func (a *actuator) Migrate(ctx context.Context, log logr.Logger, ex *extensionsv1alpha1.Extension) error {
	return a.Delete(ctx, log, ex)
}

// Restore the extension resource.
func (a *actuator) Restore(ctx context.Context, log logr.Logger, ex *extensionsv1alpha1.Extension) error {
	return a.Reconcile(ctx, log, ex)
}

func getSeedResources(cluster *extensions.Cluster, namespace string) (map[string][]byte, error) {
	var (
		registry    = managedresources.NewRegistry(kubernetesclient.SeedScheme, kubernetesclient.SeedCodec, kubernetesclient.SeedSerializer)
		protocolTCP = corev1.ProtocolTCP
		protocolUDP = corev1.ProtocolUDP
	)

	networkPolicyAllowToProviderLocalCoreDNS := emptyNetworkPolicy("allow-to-provider-local-coredns", namespace)
	networkPolicyAllowToProviderLocalCoreDNS.Spec = networkingv1.NetworkPolicySpec{
		Egress: []networkingv1.NetworkPolicyEgressRule{{
			To: []networkingv1.NetworkPolicyPeer{{
				NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"kubernetes.io/metadata.name": "gardener-extension-provider-local-coredns"}},
				PodSelector:       &metav1.LabelSelector{MatchLabels: map[string]string{"app": "coredns"}},
			}},
			Ports: []networkingv1.NetworkPolicyPort{
				{Port: utils.IntStrPtrFromInt(9053), Protocol: &protocolTCP},
				{Port: utils.IntStrPtrFromInt(9053), Protocol: &protocolUDP},
			},
		}},
		PodSelector: metav1.LabelSelector{
			MatchLabels: map[string]string{v1beta1constants.LabelNetworkPolicyToDNS: v1beta1constants.LabelNetworkPolicyAllowed},
		},
		PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
	}

	networkPolicyAllowToIstioIngressGateway := emptyNetworkPolicy("allow-to-istio-ingress-gateway", namespace)
	networkPolicyAllowToIstioIngressGateway.Spec = networkingv1.NetworkPolicySpec{
		Egress: []networkingv1.NetworkPolicyEgressRule{{
			To: []networkingv1.NetworkPolicyPeer{{
				NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"kubernetes.io/metadata.name": "istio-ingress"}},
				PodSelector: &metav1.LabelSelector{MatchLabels: map[string]string{
					"app":   "istio-ingressgateway",
					"istio": "ingressgateway",
				}},
			}},
			Ports: []networkingv1.NetworkPolicyPort{
				{Port: utils.IntStrPtrFromInt(8132), Protocol: &protocolTCP},
				{Port: utils.IntStrPtrFromInt(8443), Protocol: &protocolTCP},
				{Port: utils.IntStrPtrFromInt(9443), Protocol: &protocolTCP},
			},
		}},
		PodSelector: metav1.LabelSelector{
			MatchLabels: map[string]string{local.LabelNetworkPolicyToIstioIngressGateway: v1beta1constants.LabelNetworkPolicyAllowed},
		},
		PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
	}
	if len(cluster.Seed.Spec.Provider.Zones) > 1 {
		for _, zone := range cluster.Seed.Spec.Provider.Zones {
			networkPolicyAllowToIstioIngressGateway.Spec.Egress[0].To = append(networkPolicyAllowToIstioIngressGateway.Spec.Egress[0].To, networkingv1.NetworkPolicyPeer{
				NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"kubernetes.io/metadata.name": "istio-ingress--" + zone}},
				PodSelector: &metav1.LabelSelector{MatchLabels: map[string]string{
					"app":   "istio-ingressgateway",
					"istio": "ingressgateway--zone--" + zone,
				}},
			})
		}
	}

	return registry.AddAllAndSerialize(
		networkPolicyAllowToProviderLocalCoreDNS,
		networkPolicyAllowToIstioIngressGateway,
	)
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
