// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"context"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// GetNetworkPolicyMeta returns the network policy object with filled meta data.
func GetNetworkPolicyMeta(namespace, providerName string) *networkingv1.NetworkPolicy {
	return &networkingv1.NetworkPolicy{ObjectMeta: kutil.ObjectMeta(namespace, "gardener-extension-"+providerName)}
}

// EnsureNetworkPolicy ensures that the required network policy that allows the kube-apiserver running in the given namespace
// to talk to the extension webhook is installed.
func EnsureNetworkPolicy(ctx context.Context, c client.Client, namespace, providerName string, port int) error {
	networkPolicy := GetNetworkPolicyMeta(namespace, providerName)

	policyPort := intstr.FromInt(port)
	policyProtocol := corev1.ProtocolTCP

	_, err := controllerutil.CreateOrUpdate(ctx, c, networkPolicy, func() error {
		networkPolicy.Spec = networkingv1.NetworkPolicySpec{
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
			Egress: []networkingv1.NetworkPolicyEgressRule{
				{
					Ports: []networkingv1.NetworkPolicyPort{
						{
							Port:     &policyPort,
							Protocol: &policyProtocol,
						},
					},
					To: []networkingv1.NetworkPolicyPeer{
						{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									v1beta1constants.LabelControllerRegistrationName: providerName,
									v1beta1constants.GardenRole:                      v1beta1constants.GardenRoleExtension,
								},
							},
							PodSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"app.kubernetes.io/name": "gardener-extension-" + providerName,
								},
							},
						},
					},
				},
			},
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					v1beta1constants.LabelApp:  v1beta1constants.LabelKubernetes,
					v1beta1constants.LabelRole: v1beta1constants.LabelAPIServer,
				},
			},
		}
		return nil
	})
	return err
}
