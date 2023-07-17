// Copyright 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"net"
	"strings"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	corednsconstants "github.com/gardener/gardener/pkg/component/coredns/constants"
	nodelocaldnsconstants "github.com/gardener/gardener/pkg/component/nodelocaldns/constants"
	"github.com/gardener/gardener/pkg/controller/networkpolicy/helper"
	"github.com/gardener/gardener/pkg/controller/networkpolicy/hostnameresolver"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/extensions"
	"github.com/gardener/gardener/pkg/utils"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// Reconciler implements the reconcile.Reconcile interface for namespace reconciliation.
type Reconciler struct {
	RuntimeClient                     client.Client
	ConcurrentSyncs                   *int
	WatchRegisterers                  []func(controller.Controller) error
	Resolver                          hostnameresolver.HostResolver
	ResolverUpdate                    <-chan event.GenericEvent
	RuntimeNetworks                   RuntimeNetworkConfig
	AdditionalNamespaceSelectors      []metav1.LabelSelector
	additionalNamespaceLabelSelectors []labels.Selector
}

// RuntimeNetworkConfig is the configuration of the networks for the runtime cluster.
type RuntimeNetworkConfig struct {
	// Nodes is the CIDR of the node network.
	Nodes *string
	// Pods is the CIDR of the pod network.
	Pods string
	// Services is the CIDR of the service network.
	Services string
	// BlockCIDRs is a list of network addresses that should be blocked.
	BlockCIDRs []string
}

// Reconcile reconciles namespace in order to create some central network policies.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	ctx, cancel := controllerutils.GetMainReconciliationContext(ctx, controllerutils.DefaultReconciliationTimeout)
	defer cancel()

	namespace := &corev1.Namespace{}
	if err := r.RuntimeClient.Get(ctx, request.NamespacedName, namespace); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	if namespace.DeletionTimestamp != nil {
		log.V(1).Info("Skip NetworkPolicy reconciliation because namespace has a deletion timestamp")
		return reconcile.Result{}, nil
	}

	if namespace.Status.Phase != corev1.NamespaceActive {
		log.V(1).Info("Skip NetworkPolicy reconciliation because namespace is not in 'Active' phase")
		return reconcile.Result{}, nil
	}

	for _, policyConfig := range r.networkPolicyConfigs() {
		networkPolicy := &networkingv1.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      policyConfig.name,
				Namespace: request.Name,
			},
		}
		networkPolicyLogger := log.WithValues("networkPolicy", client.ObjectKeyFromObject(networkPolicy))

		if !labelsMatchAnySelector(namespace.Labels, policyConfig.namespaceSelectors) {
			networkPolicyLogger.Info("Deleting NetworkPolicy")
			if err := kubernetesutils.DeleteObject(ctx, r.RuntimeClient, networkPolicy); err != nil {
				return reconcile.Result{}, fmt.Errorf("failed to delete NetworkPolicy %s: %w", client.ObjectKeyFromObject(networkPolicy), err)
			}
			continue
		}

		networkPolicyLogger.V(1).Info("Reconciling NetworkPolicy")
		if err := policyConfig.reconcileFunc(ctx, log, networkPolicy); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to reconcile NetworkPolicy %s: %w", client.ObjectKeyFromObject(networkPolicy), err)
		}
		networkPolicyLogger.Info("Successfully reconciled NetworkPolicy")
	}

	return reconcile.Result{}, nil
}

func (r *Reconciler) reconcileNetworkPolicy(ctx context.Context, log logr.Logger, networkPolicy *networkingv1.NetworkPolicy, mutateFunc func(*networkingv1.NetworkPolicy)) error {
	if err := r.RuntimeClient.Get(ctx, client.ObjectKeyFromObject(networkPolicy), networkPolicy); client.IgnoreNotFound(err) != nil {
		return err
	}

	// avoid duplicative NetworkPolicy updates
	networkPolicyCopy := networkPolicy.DeepCopy()
	mutateFunc(networkPolicyCopy)
	if apiequality.Semantic.DeepEqual(networkPolicy, networkPolicyCopy) {
		log.V(1).Info("Skip NetworkPolicy reconciliation because it already is up-to-date")
		return nil
	}

	log.Info("Reconciling NetworkPolicy")

	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, r.RuntimeClient, networkPolicy, func() error {
		mutateFunc(networkPolicy)
		return nil
	})
	return err
}

type networkPolicyConfig struct {
	name               string
	reconcileFunc      func(context.Context, logr.Logger, *networkingv1.NetworkPolicy) error
	namespaceSelectors []labels.Selector
}

func (r *Reconciler) networkPolicyConfigs() []networkPolicyConfig {
	configs := []networkPolicyConfig{
		{
			name:          "deny-all",
			reconcileFunc: r.reconcileNetworkPolicyDenyAll,
			namespaceSelectors: append([]labels.Selector{
				labels.SelectorFromSet(labels.Set{corev1.LabelMetadataName: v1beta1constants.GardenNamespace}),
				labels.SelectorFromSet(labels.Set{v1beta1constants.GardenRole: v1beta1constants.GardenRoleShoot}),
				labels.SelectorFromSet(labels.Set{v1beta1constants.GardenRole: v1beta1constants.GardenRoleIstioSystem}),
				labels.SelectorFromSet(labels.Set{v1beta1constants.GardenRole: v1beta1constants.GardenRoleIstioIngress}),
				labels.SelectorFromSet(labels.Set{v1beta1constants.GardenRole: v1beta1constants.GardenRoleExtension}),
				labels.NewSelector().Add(utils.MustNewRequirement(v1beta1constants.LabelExposureClassHandlerName, selection.Exists)),
			}, r.additionalNamespaceLabelSelectors...),
		},
		{
			name: "allow-to-runtime-apiserver",
			reconcileFunc: func(ctx context.Context, log logr.Logger, networkPolicy *networkingv1.NetworkPolicy) error {
				return r.reconcileNetworkPolicyAllowToAPIServer(ctx, log, networkPolicy, v1beta1constants.LabelNetworkPolicyToRuntimeAPIServer)
			},
			namespaceSelectors: append([]labels.Selector{
				labels.SelectorFromSet(labels.Set{corev1.LabelMetadataName: v1beta1constants.GardenNamespace}),
				labels.SelectorFromSet(labels.Set{v1beta1constants.GardenRole: v1beta1constants.GardenRoleIstioSystem}),
				labels.SelectorFromSet(labels.Set{v1beta1constants.GardenRole: v1beta1constants.GardenRoleShoot}),
				labels.SelectorFromSet(labels.Set{v1beta1constants.GardenRole: v1beta1constants.GardenRoleExtension}),
			}, r.additionalNamespaceLabelSelectors...),
		},
		{
			name:          "allow-to-public-networks",
			reconcileFunc: r.reconcileNetworkPolicyAllowToPublicNetworks,
			namespaceSelectors: append([]labels.Selector{
				labels.SelectorFromSet(labels.Set{corev1.LabelMetadataName: v1beta1constants.GardenNamespace}),
				labels.SelectorFromSet(labels.Set{v1beta1constants.GardenRole: v1beta1constants.GardenRoleShoot}),
				labels.SelectorFromSet(labels.Set{v1beta1constants.GardenRole: v1beta1constants.GardenRoleExtension}),
			}, r.additionalNamespaceLabelSelectors...),
		},
		{
			name:          "allow-to-private-networks",
			reconcileFunc: r.reconcileNetworkPolicyAllowToPrivateNetworks,
			namespaceSelectors: append([]labels.Selector{
				labels.SelectorFromSet(labels.Set{corev1.LabelMetadataName: v1beta1constants.GardenNamespace}),
				labels.SelectorFromSet(labels.Set{v1beta1constants.GardenRole: v1beta1constants.GardenRoleShoot}),
				labels.SelectorFromSet(labels.Set{v1beta1constants.GardenRole: v1beta1constants.GardenRoleExtension}),
			}, r.additionalNamespaceLabelSelectors...),
		},
		{
			name:          "allow-to-blocked-cidrs",
			reconcileFunc: r.reconcileNetworkPolicyAllowToBlockedCIDRs,
			namespaceSelectors: append([]labels.Selector{
				labels.SelectorFromSet(labels.Set{corev1.LabelMetadataName: v1beta1constants.GardenNamespace}),
				labels.SelectorFromSet(labels.Set{v1beta1constants.GardenRole: v1beta1constants.GardenRoleShoot}),
				labels.SelectorFromSet(labels.Set{v1beta1constants.GardenRole: v1beta1constants.GardenRoleExtension}),
			}, r.additionalNamespaceLabelSelectors...),
		},
		{
			name:          "allow-to-dns",
			reconcileFunc: r.reconcileNetworkPolicyAllowToDNS,
			namespaceSelectors: append([]labels.Selector{
				labels.SelectorFromSet(labels.Set{corev1.LabelMetadataName: v1beta1constants.GardenNamespace}),
				labels.SelectorFromSet(labels.Set{v1beta1constants.GardenRole: v1beta1constants.GardenRoleShoot}),
				labels.SelectorFromSet(labels.Set{v1beta1constants.GardenRole: v1beta1constants.GardenRoleExtension}),
				labels.SelectorFromSet(labels.Set{v1beta1constants.GardenRole: v1beta1constants.GardenRoleIstioSystem}),
				labels.SelectorFromSet(labels.Set{v1beta1constants.GardenRole: v1beta1constants.GardenRoleIstioIngress}),
				labels.NewSelector().Add(utils.MustNewRequirement(v1beta1constants.LabelExposureClassHandlerName, selection.Exists)),
			}, r.additionalNamespaceLabelSelectors...),
		},
		{
			name:          "allow-to-shoot-networks",
			reconcileFunc: r.reconcileNetworkPolicyAllowToShootNetworks,
			namespaceSelectors: []labels.Selector{
				labels.SelectorFromSet(labels.Set{v1beta1constants.GardenRole: v1beta1constants.GardenRoleShoot}),
			},
		},
	}

	return configs
}

func labelsMatchAnySelector(labelsToCheck map[string]string, selectors []labels.Selector) bool {
	for _, selector := range selectors {
		if selector.Matches(labels.Set(labelsToCheck)) {
			return true
		}
	}
	return false
}

func (r *Reconciler) reconcileNetworkPolicyDenyAll(ctx context.Context, log logr.Logger, networkPolicy *networkingv1.NetworkPolicy) error {
	return r.reconcileNetworkPolicy(ctx, log, networkPolicy, func(policy *networkingv1.NetworkPolicy) {
		metav1.SetMetaDataAnnotation(&policy.ObjectMeta, v1beta1constants.GardenerDescription, "Disables all ingress "+
			"and egress traffic into/from this namespace.")

		policy.Spec = networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress, networkingv1.PolicyTypeEgress},
		}
	})
}

func (r *Reconciler) reconcileNetworkPolicyAllowToAPIServer(ctx context.Context, log logr.Logger, networkPolicy *networkingv1.NetworkPolicy, labelKey string) error {
	kubernetesEndpoints := &corev1.Endpoints{}
	if err := r.RuntimeClient.Get(ctx, client.ObjectKey{Name: "kubernetes", Namespace: corev1.NamespaceDefault}, kubernetesEndpoints); err != nil {
		return err
	}

	return r.reconcileNetworkPolicy(ctx, log, networkPolicy, func(policy *networkingv1.NetworkPolicy) {
		metav1.SetMetaDataAnnotation(&policy.ObjectMeta, v1beta1constants.GardenerDescription, fmt.Sprintf("Allows "+
			"egress traffic from pods labeled with '%s=%s' to the endpoints in the default namespace of the kube-apiserver "+
			"of the runtime cluster.",
			labelKey, v1beta1constants.LabelNetworkPolicyAllowed))

		policy.Spec = networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{MatchLabels: map[string]string{labelKey: v1beta1constants.LabelNetworkPolicyAllowed}},
			Egress:      helper.GetEgressRules(append(kubernetesEndpoints.Subsets, r.Resolver.Subset()...)...),
			Ingress:     []networkingv1.NetworkPolicyIngressRule{},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
		}
	})
}

func (r *Reconciler) reconcileNetworkPolicyAllowToPublicNetworks(ctx context.Context, log logr.Logger, networkPolicy *networkingv1.NetworkPolicy) error {
	return r.reconcileNetworkPolicy(ctx, log, networkPolicy, func(policy *networkingv1.NetworkPolicy) {
		metav1.SetMetaDataAnnotation(&policy.ObjectMeta, v1beta1constants.GardenerDescription, fmt.Sprintf("Allows "+
			"egress from pods labeled with '%s=%s' to all public network IPs, except for private networks (RFC1918), "+
			"carrier-grade NAT (RFC6598), and explicitly blocked addresses configured by human operators. In practice, "+
			"this blocks egress traffic to all networks in the cluster and only allows egress traffic to public IPv4 "+
			"addresses.", v1beta1constants.LabelNetworkPolicyToPublicNetworks, v1beta1constants.LabelNetworkPolicyAllowed))

		policy.Spec = networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{MatchLabels: map[string]string{v1beta1constants.LabelNetworkPolicyToPublicNetworks: v1beta1constants.LabelNetworkPolicyAllowed}},
			Egress: []networkingv1.NetworkPolicyEgressRule{{
				To: []networkingv1.NetworkPolicyPeer{{
					IPBlock: &networkingv1.IPBlock{
						CIDR: "0.0.0.0/0",
						Except: append([]string{
							private8BitBlock().String(),
							private12BitBlock().String(),
							private16BitBlock().String(),
							carrierGradeNATBlock().String(),
						}, r.RuntimeNetworks.BlockCIDRs...),
					},
				}},
			}},
			Ingress:     []networkingv1.NetworkPolicyIngressRule{},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
		}
	})
}

func (r *Reconciler) reconcileNetworkPolicyAllowToBlockedCIDRs(ctx context.Context, log logr.Logger, networkPolicy *networkingv1.NetworkPolicy) error {
	return r.reconcileNetworkPolicy(ctx, log, networkPolicy, func(policy *networkingv1.NetworkPolicy) {
		metav1.SetMetaDataAnnotation(&policy.ObjectMeta, v1beta1constants.GardenerDescription, fmt.Sprintf("Allows "+
			"egress from pods labeled with '%s=%s' to explicitly blocked addresses configured by human operators.",
			v1beta1constants.LabelNetworkPolicyToBlockedCIDRs, v1beta1constants.LabelNetworkPolicyAllowed))

		policy.Spec = networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{MatchLabels: map[string]string{v1beta1constants.LabelNetworkPolicyToBlockedCIDRs: v1beta1constants.LabelNetworkPolicyAllowed}},
			Egress:      []networkingv1.NetworkPolicyEgressRule{},
			Ingress:     []networkingv1.NetworkPolicyIngressRule{},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
		}

		for _, cidr := range r.RuntimeNetworks.BlockCIDRs {
			policy.Spec.Egress = append(policy.Spec.Egress, networkingv1.NetworkPolicyEgressRule{
				To: []networkingv1.NetworkPolicyPeer{{
					IPBlock: &networkingv1.IPBlock{
						CIDR: cidr,
					},
				}},
			})
		}
	})
}

func (r *Reconciler) reconcileNetworkPolicyAllowToPrivateNetworks(ctx context.Context, log logr.Logger, networkPolicy *networkingv1.NetworkPolicy) error {
	blockedNetworkPeers := append([]string{
		r.RuntimeNetworks.Pods,
		r.RuntimeNetworks.Services,
	}, r.RuntimeNetworks.BlockCIDRs...)

	if v := r.RuntimeNetworks.Nodes; v != nil {
		blockedNetworkPeers = append(blockedNetworkPeers, *v)
	}

	if strings.HasPrefix(networkPolicy.Namespace, v1beta1constants.TechnicalIDPrefix) {
		cluster := &extensionsv1alpha1.Cluster{}
		if err := r.RuntimeClient.Get(ctx, client.ObjectKey{Name: networkPolicy.Namespace}, cluster); err != nil {
			return err
		}

		shoot, err := extensions.ShootFromCluster(cluster)
		if err != nil {
			return err
		}

		if shoot.Spec.Networking != nil {
			if v := shoot.Spec.Networking.Nodes; v != nil {
				blockedNetworkPeers = append(blockedNetworkPeers, *v)
			}
			if v := shoot.Spec.Networking.Pods; v != nil {
				blockedNetworkPeers = append(blockedNetworkPeers, *v)
			}
			if v := shoot.Spec.Networking.Services; v != nil {
				blockedNetworkPeers = append(blockedNetworkPeers, *v)
			}
		}
	}

	privateNetworkPeers, err := toNetworkPolicyPeersWithExceptions(allPrivateNetworkBlocks(), blockedNetworkPeers...)
	if err != nil {
		return err
	}

	return r.reconcileNetworkPolicy(ctx, log, networkPolicy, func(policy *networkingv1.NetworkPolicy) {
		metav1.SetMetaDataAnnotation(&policy.ObjectMeta, v1beta1constants.GardenerDescription, fmt.Sprintf("Allows "+
			"egress from pods labeled with '%s=%s' to the private networks (RFC1918) and carrier-grade NAT (RFC6598), "+
			"except for cluster-specific networks.", v1beta1constants.LabelNetworkPolicyToPrivateNetworks,
			v1beta1constants.LabelNetworkPolicyAllowed))

		policy.Spec = networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{MatchLabels: map[string]string{v1beta1constants.LabelNetworkPolicyToPrivateNetworks: v1beta1constants.LabelNetworkPolicyAllowed}},
			Egress:      []networkingv1.NetworkPolicyEgressRule{{To: privateNetworkPeers}},
			Ingress:     []networkingv1.NetworkPolicyIngressRule{},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
		}
	})
}

func (r *Reconciler) reconcileNetworkPolicyAllowToShootNetworks(ctx context.Context, log logr.Logger, networkPolicy *networkingv1.NetworkPolicy) error {
	cluster := &extensionsv1alpha1.Cluster{}
	if err := r.RuntimeClient.Get(ctx, client.ObjectKey{Name: networkPolicy.Namespace}, cluster); err != nil {
		return err
	}

	shoot, err := extensions.ShootFromCluster(cluster)
	if err != nil {
		return err
	}

	var shootNetworks []string
	if shoot.Spec.Networking != nil {
		if v := shoot.Spec.Networking.Nodes; v != nil {
			shootNetworks = append(shootNetworks, *v)
		}
		if v := shoot.Spec.Networking.Pods; v != nil {
			shootNetworks = append(shootNetworks, *v)
		}
		if v := shoot.Spec.Networking.Services; v != nil {
			shootNetworks = append(shootNetworks, *v)
		}
	}

	shootNetworkPeers, err := networkPolicyPeersWithExceptions(shootNetworks, r.RuntimeNetworks.BlockCIDRs...)
	if err != nil {
		return err
	}

	return r.reconcileNetworkPolicy(ctx, log, networkPolicy, func(policy *networkingv1.NetworkPolicy) {
		metav1.SetMetaDataAnnotation(&policy.ObjectMeta, v1beta1constants.GardenerDescription, fmt.Sprintf("Allows "+
			"egress from pods labeled with '%s=%s' to IPv4 blocks belonging to the shoot networks. In practice, this "+
			"should be used by components which use VPN tunnel to communicate to pods in the shoot cluster.",
			v1beta1constants.LabelNetworkPolicyToShootNetworks, v1beta1constants.LabelNetworkPolicyAllowed))

		policy.Spec = networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{MatchLabels: map[string]string{v1beta1constants.LabelNetworkPolicyToShootNetworks: v1beta1constants.LabelNetworkPolicyAllowed}},
			Egress:      []networkingv1.NetworkPolicyEgressRule{{To: shootNetworkPeers}},
			Ingress:     []networkingv1.NetworkPolicyIngressRule{},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
		}
	})
}

func (r *Reconciler) reconcileNetworkPolicyAllowToDNS(ctx context.Context, log logr.Logger, networkPolicy *networkingv1.NetworkPolicy) error {
	_, runtimeServiceCIDR, err := net.ParseCIDR(r.RuntimeNetworks.Services)
	if err != nil {
		return err
	}

	runtimeDNSServerAddress, err := utils.ComputeOffsetIP(runtimeServiceCIDR, 10)
	if err != nil {
		return fmt.Errorf("cannot calculate CoreDNS ClusterIP: %w", err)
	}

	return r.reconcileNetworkPolicy(ctx, log, networkPolicy, func(policy *networkingv1.NetworkPolicy) {
		metav1.SetMetaDataAnnotation(&policy.ObjectMeta, v1beta1constants.GardenerDescription, fmt.Sprintf("Allows "+
			"egress from pods labeled with '%s=%s' to DNS running in '%s'. In practice, most of the pods which require "+
			"network egress need this label.", v1beta1constants.LabelNetworkPolicyToDNS, v1beta1constants.LabelNetworkPolicyAllowed,
			metav1.NamespaceSystem))

		policy.Spec = networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{MatchLabels: map[string]string{v1beta1constants.LabelNetworkPolicyToDNS: v1beta1constants.LabelNetworkPolicyAllowed}},
			Egress: []networkingv1.NetworkPolicyEgressRule{{
				To: []networkingv1.NetworkPolicyPeer{
					{
						NamespaceSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								corev1.LabelMetadataName: metav1.NamespaceSystem,
							},
						},
						PodSelector: &metav1.LabelSelector{
							MatchExpressions: []metav1.LabelSelectorRequirement{{
								Key:      corednsconstants.LabelKey,
								Operator: metav1.LabelSelectorOpIn,
								Values:   []string{corednsconstants.LabelValue},
							}},
						},
					},
					{
						NamespaceSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								corev1.LabelMetadataName: metav1.NamespaceSystem,
							},
						},
						PodSelector: &metav1.LabelSelector{
							MatchExpressions: []metav1.LabelSelectorRequirement{{
								Key:      corednsconstants.LabelKey,
								Operator: metav1.LabelSelectorOpIn,
								Values:   []string{nodelocaldnsconstants.LabelValue},
							}},
						},
					},
					// required for node local dns feature, allows egress traffic to CoreDNS
					{
						IPBlock: &networkingv1.IPBlock{
							CIDR: fmt.Sprintf("%s/32", runtimeDNSServerAddress),
						},
					},
					// required for node local dns feature, allows egress traffic to node local dns cache
					{
						IPBlock: &networkingv1.IPBlock{
							CIDR: fmt.Sprintf("%s/32", nodelocaldnsconstants.IPVSAddress),
						},
					},
				},
				Ports: []networkingv1.NetworkPolicyPort{
					{Protocol: utils.ProtocolPtr(corev1.ProtocolUDP), Port: utils.IntStrPtrFromInt(corednsconstants.PortServiceServer)},
					{Protocol: utils.ProtocolPtr(corev1.ProtocolTCP), Port: utils.IntStrPtrFromInt(corednsconstants.PortServiceServer)},
					{Protocol: utils.ProtocolPtr(corev1.ProtocolUDP), Port: utils.IntStrPtrFromInt(corednsconstants.PortServer)},
					{Protocol: utils.ProtocolPtr(corev1.ProtocolTCP), Port: utils.IntStrPtrFromInt(corednsconstants.PortServer)},
				},
			}},
			Ingress:     []networkingv1.NetworkPolicyIngressRule{},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
		}
	})
}
