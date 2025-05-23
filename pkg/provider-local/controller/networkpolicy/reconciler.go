// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package networkpolicy

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/extensions"
	"github.com/gardener/gardener/pkg/provider-local/local"
)

// Reconciler creates the required networkpolicies in the shoot namespace.
type Reconciler struct {
	Client client.Client
}

// Reconcile creates the required networkpolicies in the shoot namespace.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	namespace := &corev1.Namespace{}
	if err := r.Client.Get(ctx, request.NamespacedName, namespace); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	cluster, err := extensions.GetCluster(ctx, r.Client, namespace.Name)
	if err != nil {
		return reconcile.Result{}, err
	}

	var (
		protocolTCP = corev1.ProtocolTCP
		protocolUDP = corev1.ProtocolUDP
	)

	networkPolicyAllowToProviderLocalCoreDNS := emptyNetworkPolicy("allow-to-provider-local-coredns", namespace.Name)
	networkPolicyAllowToProviderLocalCoreDNS.Spec = networkingv1.NetworkPolicySpec{
		Egress: []networkingv1.NetworkPolicyEgressRule{{
			To: []networkingv1.NetworkPolicyPeer{{
				NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"kubernetes.io/metadata.name": "gardener-extension-provider-local-coredns"}},
				PodSelector:       &metav1.LabelSelector{MatchLabels: map[string]string{"app": "coredns"}},
			}},
			Ports: []networkingv1.NetworkPolicyPort{
				{Port: ptr.To(intstr.FromInt32(9053)), Protocol: &protocolTCP},
				{Port: ptr.To(intstr.FromInt32(9053)), Protocol: &protocolUDP},
			},
		}},
		PodSelector: metav1.LabelSelector{
			MatchLabels: map[string]string{v1beta1constants.LabelNetworkPolicyToDNS: v1beta1constants.LabelNetworkPolicyAllowed},
		},
		PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
	}

	networkPolicyAllowToIstioIngressGateway := emptyNetworkPolicy("allow-to-istio-ingress-gateway", namespace.Name)
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
				{Port: ptr.To(intstr.FromInt32(8132)), Protocol: &protocolTCP},
				{Port: ptr.To(intstr.FromInt32(8443)), Protocol: &protocolTCP},
				{Port: ptr.To(intstr.FromInt32(9443)), Protocol: &protocolTCP},
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

	for _, obj := range []client.Object{
		networkPolicyAllowToProviderLocalCoreDNS,
		networkPolicyAllowToIstioIngressGateway,
	} {
		if err := r.Client.Patch(ctx, obj, client.Apply, local.FieldOwner, client.ForceOwnership); err != nil {
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{}, nil
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
