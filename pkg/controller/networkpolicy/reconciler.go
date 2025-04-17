// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	corednsconstants "github.com/gardener/gardener/pkg/component/networking/coredns/constants"
	nodelocaldnsconstants "github.com/gardener/gardener/pkg/component/networking/nodelocaldns/constants"
	"github.com/gardener/gardener/pkg/controller/networkpolicy/helper"
	"github.com/gardener/gardener/pkg/controller/networkpolicy/hostnameresolver"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/extensions"
	"github.com/gardener/gardener/pkg/utils"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	netutils "github.com/gardener/gardener/pkg/utils/net"
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
	// IPFamilies specifies the IP protocol versions used in the runtime cluster.
	IPFamilies []gardencorev1beta1.IPFamily
	// Nodes are the CIDRs of the node network.
	Nodes []string
	// Pods are the CIDRs of the pod network.
	Pods []string
	// Services are the CIDRs of the service network.
	Services []string
	// BlockCIDRs is a list of network addresses that should be blocked.
	BlockCIDRs []string
}

// getBlockedNetworkPeers returns a list of CIDRs to exclude from a NetworkPolicy IPBlock. `ipFamily` should match the
// IP family of the IPBlock.CIDR value. The resulting list still needs to be filtered for subsets of IPBlock.CIDR.
func (r RuntimeNetworkConfig) getBlockedNetworkPeers(ipFamily gardencorev1beta1.IPFamily) []string {
	// NB: BlockCIDRs can contain both IPv4 and IPv6 CIDRs.
	var peers = append([]string(nil), r.BlockCIDRs...)

	if len(r.IPFamilies) == 1 && r.IPFamilies[0] == ipFamily {
		peers = append(peers, r.Pods...)
		peers = append(peers, r.Services...)
		peers = append(peers, r.Nodes...)
	}

	return peers
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

	log.Info("Reconciling NetworkPolicy", "networkPolicy", client.ObjectKeyFromObject(networkPolicy))

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
				labels.SelectorFromSet(labels.Set{v1beta1constants.GardenRole: v1beta1constants.GardenRoleIstioIngress}),
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

	if !r.Resolver.HasSynced() {
		log.Info("Resolver has not synced yet - skipping update of NetworkPolicy", "networkPolicyName", "allow-to-runtime-apiserver")
		// The resolver triggers an event after it has been synced, which starts a new reconciliation.
		// No need to raise an error here.
		return nil
	}

	egressRules, err := helper.GetEgressRules(append(kubernetesEndpoints.Subsets, r.Resolver.Subset()...)...)
	if err != nil {
		return err
	}

	return r.reconcileNetworkPolicy(ctx, log, networkPolicy, func(policy *networkingv1.NetworkPolicy) {
		metav1.SetMetaDataAnnotation(&policy.ObjectMeta, v1beta1constants.GardenerDescription, fmt.Sprintf("Allows "+
			"egress traffic from pods labeled with '%s=%s' to the endpoints in the default namespace of the kube-apiserver "+
			"of the runtime cluster.",
			labelKey, v1beta1constants.LabelNetworkPolicyAllowed))

		policy.Spec = networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{MatchLabels: map[string]string{labelKey: v1beta1constants.LabelNetworkPolicyAllowed}},
			Egress:      egressRules,
			Ingress:     []networkingv1.NetworkPolicyIngressRule{},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
		}
	})
}

func (r *Reconciler) reconcileNetworkPolicyAllowToPublicNetworks(ctx context.Context, log logr.Logger, networkPolicy *networkingv1.NetworkPolicy) error {
	peersV4, err := networkPolicyPeersWithExceptions([]string{"0.0.0.0/0"}, append(
		toCIDRStrings(allPrivateNetworkBlocksV4()...),
		r.RuntimeNetworks.BlockCIDRs...,
	)...)
	if err != nil {
		return err
	}

	peersV6, err := networkPolicyPeersWithExceptions([]string{"::/0"}, append(
		toCIDRStrings(allPrivateNetworkBlocksV6()...),
		// In IPv4, all cluster networks are contained in the private IPv4 blocks.
		// In IPv6 however, cluster networks might be "public" (e.g., if using prefix delegation from provider).
		// As this NetworkPolicy should only allow communication with public networks *outside* the cluster,
		// we exclude the cluster networks.
		r.RuntimeNetworks.getBlockedNetworkPeers(gardencorev1beta1.IPFamilyIPv6)...,
	)...)
	if err != nil {
		return err
	}

	return r.reconcileNetworkPolicy(ctx, log, networkPolicy, func(policy *networkingv1.NetworkPolicy) {
		metav1.SetMetaDataAnnotation(&policy.ObjectMeta, v1beta1constants.GardenerDescription, fmt.Sprintf("Allows "+
			"egress from pods labeled with '%s=%s' to all public network IPs, except for private networks (RFC1918), "+
			"carrier-grade NAT (RFC6598), and explicitly blocked addresses configured by human operators. In practice, "+
			"this blocks egress traffic to all networks in the cluster and only allows egress traffic to public IPv4 "+
			"addresses.", v1beta1constants.LabelNetworkPolicyToPublicNetworks, v1beta1constants.LabelNetworkPolicyAllowed))

		policy.Spec = networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{MatchLabels: map[string]string{v1beta1constants.LabelNetworkPolicyToPublicNetworks: v1beta1constants.LabelNetworkPolicyAllowed}},
			Egress: []networkingv1.NetworkPolicyEgressRule{{
				To: append(peersV4, peersV6...),
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
	blockedNetworkPeersV4 := r.RuntimeNetworks.getBlockedNetworkPeers(gardencorev1beta1.IPFamilyIPv4)
	blockedNetworkPeersV6 := r.RuntimeNetworks.getBlockedNetworkPeers(gardencorev1beta1.IPFamilyIPv6)

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
			var shootNetworks []string
			if v := shoot.Spec.Networking.Nodes; v != nil {
				shootNetworks = append(shootNetworks, *v)
			}
			if v := shoot.Spec.Networking.Pods; v != nil {
				shootNetworks = append(shootNetworks, *v)
			}
			if v := shoot.Spec.Networking.Services; v != nil {
				shootNetworks = append(shootNetworks, *v)
			}

			if shoot.Status.Networking != nil {
				networks := sets.New(shootNetworks...)
				networks.Insert(shoot.Status.Networking.Nodes...)
				networks.Insert(shoot.Status.Networking.Pods...)
				networks.Insert(shoot.Status.Networking.Services...)
				shootNetworks = networks.UnsortedList()
			}

			if gardencorev1beta1.IsIPv4SingleStack(shoot.Spec.Networking.IPFamilies) {
				blockedNetworkPeersV4 = append(blockedNetworkPeersV4, shootNetworks...)
			} else {
				blockedNetworkPeersV6 = append(blockedNetworkPeersV6, shootNetworks...)
			}
		}
	}

	privateNetworkPeersV4, err := toNetworkPolicyPeersWithExceptions(allPrivateNetworkBlocksV4(), blockedNetworkPeersV4...)
	if err != nil {
		return err
	}

	privateNetworkPeersV6, err := toNetworkPolicyPeersWithExceptions(allPrivateNetworkBlocksV6(), blockedNetworkPeersV6...)
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
			Egress: []networkingv1.NetworkPolicyEgressRule{
				{To: privateNetworkPeersV4},
				{To: privateNetworkPeersV6},
			},
			Ingress:     []networkingv1.NetworkPolicyIngressRule{},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
		}
	})
}

func (r *Reconciler) reconcileNetworkPolicyAllowToDNS(ctx context.Context, log logr.Logger, networkPolicy *networkingv1.NetworkPolicy) error {
	if len(r.RuntimeNetworks.Services) == 0 {
		return fmt.Errorf("no service range configured")
	}

	var dnsServerAddressCIDRs []string
	for _, s := range r.RuntimeNetworks.Services {
		_, runtimeServiceCIDR, err := net.ParseCIDR(s)
		if err != nil {
			return err
		}

		runtimeDNSServerAddress, err := utils.ComputeOffsetIP(runtimeServiceCIDR, 10)
		if err != nil {
			return fmt.Errorf("cannot calculate CoreDNS ClusterIP: %w", err)
		}
		runtimeDNSServerAddressBitLen, err := netutils.GetBitLen(runtimeDNSServerAddress.String())
		if err != nil {
			return fmt.Errorf("cannot get bit len: %w", err)
		}

		dnsServerAddressCIDRs = append(dnsServerAddressCIDRs, fmt.Sprintf("%s/%d", runtimeDNSServerAddress, runtimeDNSServerAddressBitLen))
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
					// required for node local dns feature, allows egress traffic to node local dns cache
					{
						IPBlock: &networkingv1.IPBlock{
							// node local dns feature is only supported for shoots with IPv4 or IPv6 single-stack networking
							CIDR: fmt.Sprintf("%s/32", nodelocaldnsconstants.IPVSAddress),
						},
					},
					{
						IPBlock: &networkingv1.IPBlock{
							// node local dns feature is only supported for shoots with IPv4 or IPv6 single-stack networking
							CIDR: fmt.Sprintf("%s/128", nodelocaldnsconstants.IPVSIPv6Address),
						},
					},
				},
				Ports: []networkingv1.NetworkPolicyPort{
					{Protocol: ptr.To(corev1.ProtocolUDP), Port: ptr.To(intstr.FromInt32(corednsconstants.PortServiceServer))},
					{Protocol: ptr.To(corev1.ProtocolTCP), Port: ptr.To(intstr.FromInt32(corednsconstants.PortServiceServer))},
					{Protocol: ptr.To(corev1.ProtocolUDP), Port: ptr.To(intstr.FromInt32(corednsconstants.PortServer))},
					{Protocol: ptr.To(corev1.ProtocolTCP), Port: ptr.To(intstr.FromInt32(corednsconstants.PortServer))},
				},
			}},
			Ingress:     []networkingv1.NetworkPolicyIngressRule{},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
		}

		for _, dnsServer := range dnsServerAddressCIDRs {
			// required for node local dns feature, allows egress traffic to CoreDNS
			policy.Spec.Egress[0].To = append(policy.Spec.Egress[0].To, networkingv1.NetworkPolicyPeer{
				IPBlock: &networkingv1.IPBlock{
					CIDR: dnsServer,
				},
			})
		}
	})
}
