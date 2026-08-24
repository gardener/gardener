// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package peerexposure

import (
	"context"
	"fmt"

	istioapinetworkingv1beta1 "istio.io/api/networking/v1beta1"
	istionetworkingv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/component"
	etcdconstants "github.com/gardener/gardener/pkg/component/etcd/etcd/constants"
	"github.com/gardener/gardener/pkg/controllerutils"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// PeerMember holds the cross-seed routing info for one etcd member.
type PeerMember struct {
	// SNIHost is the hostname under which this member is reachable from other seeds via the Istio ingress gateway.
	SNIHost string
	// PodFQDN is the in-cluster fully-qualified pod subdomain through the headless peer Service
	// (e.g. etcd-main-0.etcd-main-peer.<namespace>.svc.cluster.local). Incoming traffic matching SNIHost is
	// forwarded to exactly this pod, ensuring peer messages reach the correct member.
	PodFQDN string
	// ExternalPort is the port on the Istio ingress gateway LB at which this member is reachable. Each member is
	// assigned a unique port so that etcd's URLStringsEqual, which falls back to DNS resolution
	// sees distinct IP:port tuples and cannot conflate members that share the same ingress IP.
	ExternalPort uint32
}

// Values are the configuration values for the etcd peer exposure component.
type Values struct {
	// Role is the role of the etcd.
	Role string
	// Members is the per-member routing table: each entry maps one SNI hostname to the pod it must reach.
	Members []PeerMember
	// ClientHost is the SNI host under which this seed's etcd client endpoint is reachable from other seeds. It is
	// routed to the etcd client Service. When empty, no client exposure is deployed.
	ClientHost string
	// IstioIngresGatewayNamespace is the namespace of the Istio ingress gateway that exposes the peer endpoints.
	IstioIngressGatewayNamespace string
	// IstioIngressGatewayLabels are the selector labels of the Istio ingress gateway that exposes the peer endpoints.
	IstioIngressGatewayLabels map[string]string
}

// New creates a new instance of DeployWaiter which exposes the peer endpoints of a shoot's etcd members across seeds.
func New(client client.Client, namespace string, values Values) component.DeployWaiter {
	return &peerExposure{
		client:    client,
		namespace: namespace,
		values:    values,
	}
}

type peerExposure struct {
	client    client.Client
	namespace string
	values    Values
}

func (p *peerExposure) name() string {
	return fmt.Sprintf("etcd-%s-peer", p.values.Role)
}

func (p *peerExposure) Deploy(ctx context.Context) error {
	var (
		gateway        = p.emptyGatewayFor(p.name())
		virtualService = p.emptyVirtualServiceFor(p.name())
		networkPolicy  = p.emptyNetworkPolicyFor(p.name(), networkingv1.PolicyTypeIngress)
		istioEgressNP  = p.emptyNetworkPolicyFor(p.name(), networkingv1.PolicyTypeEgress)
	)

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, p.client, gateway, gatewayWithPeerTLSPassthrough(gateway, getLabels(p.values.Role), p.values.IstioIngressGatewayLabels, p.values.Members)); err != nil {
		return fmt.Errorf("failed to reconcile Istio gateway for etcd peer exposure: %w", err)
	}

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, p.client, virtualService, virtualServiceWithPeerSNIMatch(virtualService, getLabels(p.values.Role), []string{p.values.IstioIngressGatewayNamespace}, p.values.Members, gateway.Name)); err != nil {
		return fmt.Errorf("failed to reconcile Istio virtual service for etcd peer exposure: %w", err)
	}

	// Gardener seeds set defaultServiceExportTo: ["~"] in the mesh config, so services are self-namespace only by
	// default. ServiceEntries with resolution: DNS and exportTo pointing at the istio-ingress namespace make each
	// pod's subdomain discoverable by the ingress proxy without touching the etcd-druid-owned Service (which the druid
	// admission webhook would reject).
	for i, m := range p.values.Members {
		se := p.emptyServiceEntryFor(fmt.Sprintf("%s-%d", p.name(), i))
		if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, p.client, se, serviceEntryForExport(se, getLabels(p.values.Role), m.PodFQDN, p.values.IstioIngressGatewayNamespace, uint32(etcdconstants.PortEtcdPeer), etcdconstants.ServicePortNameEtcdPeer)); err != nil { // #nosec G115 -- Port constants are positive values well within uint32 range.
			return fmt.Errorf("failed to reconcile Istio service entry for etcd peer member %d: %w", i, err)
		}
	}

	// Allow the Istio ingress gateway to reach the etcd pods on the peer port.
	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, p.client, networkPolicy, p.mutateNetworkPolicy(networkPolicy, networkingv1.PolicyTypeIngress, etcdconstants.PortEtcdPeer)); err != nil {
		return fmt.Errorf("failed to reconcile network policy for etcd peer exposure: %w", err)
	}

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, p.client, istioEgressNP, p.mutateNetworkPolicy(istioEgressNP, networkingv1.PolicyTypeEgress, etcdconstants.PortEtcdPeer)); err != nil {
		return fmt.Errorf("failed to reconcile Istio egress network policy for etcd peer exposure: %w", err)
	}

	if p.values.ClientHost != "" {
		if err := p.deployClientExposure(ctx); err != nil {
			return err
		}
	}

	return nil
}

func (p *peerExposure) deployClientExposure(ctx context.Context) error {
	var (
		clientGateway        = p.emptyGatewayFor(p.clientName())
		clientVirtualService = p.emptyVirtualServiceFor(p.clientName())
		clientNetworkPolicy  = p.emptyNetworkPolicyFor(p.clientName(), networkingv1.PolicyTypeIngress)
		clientIstioEgressNP  = p.emptyNetworkPolicyFor(p.clientName(), networkingv1.PolicyTypeEgress)
	)

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, p.client, clientGateway, gatewayWithClientTLSPassthrough(clientGateway, getLabels(p.values.Role), p.values.IstioIngressGatewayLabels, []string{p.values.ClientHost})); err != nil {
		return fmt.Errorf("failed to reconcile Istio gateway for etcd client exposure: %w", err)
	}

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, p.client, clientVirtualService, virtualServiceWithClientSNIMatch(clientVirtualService, getLabels(p.values.Role), []string{p.values.IstioIngressGatewayNamespace}, []string{p.values.ClientHost}, clientGateway.Name, p.clientServiceHost())); err != nil {
		return fmt.Errorf("failed to reconcile Istio virtual service for etcd client exposure: %w", err)
	}

	clientServiceEntry := p.emptyServiceEntryFor(p.clientName())
	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, p.client, clientServiceEntry, serviceEntryForExport(clientServiceEntry, getLabels(p.values.Role), p.clientServiceHost(), p.values.IstioIngressGatewayNamespace, uint32(etcdconstants.PortEtcdClient), etcdconstants.ServicePortNameEtcdClient)); err != nil { // #nosec G115 -- Port constants are positive values well within uint32 range.
		return fmt.Errorf("failed to reconcile Istio service entry for etcd client exposure: %w", err)
	}

	// Allow the Istio ingress gateway to reach the etcd pods on the client port.
	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, p.client, clientNetworkPolicy, p.mutateNetworkPolicy(clientNetworkPolicy, networkingv1.PolicyTypeIngress, etcdconstants.PortEtcdClient)); err != nil {
		return fmt.Errorf("failed to reconcile network policy for etcd client exposure: %w", err)
	}

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, p.client, clientIstioEgressNP, p.mutateNetworkPolicy(clientIstioEgressNP, networkingv1.PolicyTypeEgress, etcdconstants.PortEtcdClient)); err != nil {
		return fmt.Errorf("failed to reconcile Istio egress network policy for etcd client exposure: %w", err)
	}

	return nil
}

func (p *peerExposure) Destroy(ctx context.Context) error {
	objects := []client.Object{p.emptyGatewayFor(p.name()), p.emptyVirtualServiceFor(p.name()), p.emptyNetworkPolicyFor(p.name(), networkingv1.PolicyTypeIngress), p.emptyNetworkPolicyFor(p.name(), networkingv1.PolicyTypeEgress)}
	for i := range p.values.Members {
		objects = append(objects, p.emptyServiceEntryFor(fmt.Sprintf("%s-%d", p.name(), i)))
	}
	if p.values.ClientHost != "" {
		objects = append(objects, p.emptyGatewayFor(p.clientName()), p.emptyVirtualServiceFor(p.clientName()), p.emptyServiceEntryFor(p.clientName()), p.emptyNetworkPolicyFor(p.clientName(), networkingv1.PolicyTypeIngress), p.emptyNetworkPolicyFor(p.clientName(), networkingv1.PolicyTypeEgress))
	}
	return kubernetesutils.DeleteObjects(ctx, p.client, objects...)
}

func (p *peerExposure) Wait(_ context.Context) error        { return nil }
func (p *peerExposure) WaitCleanup(_ context.Context) error { return nil }

func (p *peerExposure) emptyGatewayFor(name string) *istionetworkingv1beta1.Gateway {
	return &istionetworkingv1beta1.Gateway{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: p.namespace}}
}

func (p *peerExposure) emptyVirtualServiceFor(name string) *istionetworkingv1beta1.VirtualService {
	return &istionetworkingv1beta1.VirtualService{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: p.namespace}}
}

func (p *peerExposure) emptyServiceEntryFor(name string) *istionetworkingv1beta1.ServiceEntry {
	return &istionetworkingv1beta1.ServiceEntry{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: p.namespace}}
}

func (p *peerExposure) emptyNetworkPolicyFor(baseName string, policyType networkingv1.PolicyType) *networkingv1.NetworkPolicy {
	if policyType == networkingv1.PolicyTypeIngress {
		return &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-istio-ingress-to-" + baseName, Namespace: p.namespace}}
	}
	return &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-istio-ingress-to-" + baseName + "-" + p.namespace, Namespace: p.values.IstioIngressGatewayNamespace}}
}

func (p *peerExposure) clientName() string {
	return fmt.Sprintf("etcd-%s-client", p.values.Role)
}

func (p *peerExposure) clientServiceHost() string {
	return kubernetesutils.FQDNForService(fmt.Sprintf("etcd-%s-client", p.values.Role), p.namespace)
}

func getLabels(role string) map[string]string {
	return map[string]string{
		v1beta1constants.LabelApp:  "etcd-peer-exposure",
		v1beta1constants.LabelRole: role,
	}
}

func (p *peerExposure) mutateNetworkPolicy(networkPolicy *networkingv1.NetworkPolicy, policyType networkingv1.PolicyType, port int32) func() error {
	return func() error {
		networkPolicy.Labels = getLabels(p.values.Role)
		etcdLabels := map[string]string{
			v1beta1constants.LabelApp:  etcdconstants.LabelAppValue,
			v1beta1constants.LabelRole: p.values.Role,
		}
		networkPolicyPort := networkingv1.NetworkPolicyPort{
			Port:     new(intstr.FromInt32(port)),
			Protocol: new(corev1.ProtocolTCP),
		}

		networkPolicy.Spec = networkingv1.NetworkPolicySpec{PolicyTypes: []networkingv1.PolicyType{policyType}}
		if policyType == networkingv1.PolicyTypeIngress {
			networkPolicy.Spec.PodSelector = metav1.LabelSelector{MatchLabels: etcdLabels}
			networkPolicy.Spec.Ingress = []networkingv1.NetworkPolicyIngressRule{{
				From: []networkingv1.NetworkPolicyPeer{{
					NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{corev1.LabelMetadataName: p.values.IstioIngressGatewayNamespace}},
					PodSelector:       &metav1.LabelSelector{MatchLabels: p.values.IstioIngressGatewayLabels},
				}},
				Ports: []networkingv1.NetworkPolicyPort{networkPolicyPort},
			}}
		} else {
			networkPolicy.Spec.PodSelector = metav1.LabelSelector{MatchLabels: p.values.IstioIngressGatewayLabels}
			networkPolicy.Spec.Egress = []networkingv1.NetworkPolicyEgressRule{{
				To: []networkingv1.NetworkPolicyPeer{{
					NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{corev1.LabelMetadataName: p.namespace}},
					PodSelector:       &metav1.LabelSelector{MatchLabels: etcdLabels},
				}},
				Ports: []networkingv1.NetworkPolicyPort{networkPolicyPort},
			}}
		}
		return nil
	}
}

func serviceEntryForExport(serviceEntry *istionetworkingv1beta1.ServiceEntry, labels map[string]string, host, ingressNamespace string, port uint32, portName string) func() error {
	return func() error {
		serviceEntry.Labels = labels
		serviceEntry.Spec = istioapinetworkingv1beta1.ServiceEntry{
			Hosts:    []string{host},
			ExportTo: []string{ingressNamespace},
			Ports: []*istioapinetworkingv1beta1.ServicePort{{
				Number:   port,
				Name:     portName,
				Protocol: "TLS",
			}},
			Resolution: istioapinetworkingv1beta1.ServiceEntry_DNS,
		}
		return nil
	}
}

func gatewayWithPeerTLSPassthrough(gateway *istionetworkingv1beta1.Gateway, labels, istioLabels map[string]string, members []PeerMember) func() error {
	return func() error {
		gateway.Labels = labels
		servers := make([]*istioapinetworkingv1beta1.Server, len(members))

		for i, m := range members {
			servers[i] = &istioapinetworkingv1beta1.Server{
				Hosts: []string{m.SNIHost},
				Port: &istioapinetworkingv1beta1.Port{
					Number:   m.ExternalPort,
					Name:     fmt.Sprintf("%s-%d", etcdconstants.ServicePortNameEtcdPeer, i),
					Protocol: "TLS",
				},
				Tls: &istioapinetworkingv1beta1.ServerTLSSettings{
					Mode: istioapinetworkingv1beta1.ServerTLSSettings_PASSTHROUGH,
				},
			}
		}
		gateway.Spec = istioapinetworkingv1beta1.Gateway{
			Selector: istioLabels,
			Servers:  servers,
		}
		return nil
	}
}

func virtualServiceWithPeerSNIMatch(virtualService *istionetworkingv1beta1.VirtualService, labels map[string]string, exportTo []string, members []PeerMember, gatewayName string) func() error {
	return func() error {
		virtualService.Labels = labels
		routes := make([]*istioapinetworkingv1beta1.TLSRoute, len(members))
		allHosts := make([]string, len(members))

		for i, m := range members {
			allHosts[i] = m.SNIHost
			routes[i] = &istioapinetworkingv1beta1.TLSRoute{
				Match: []*istioapinetworkingv1beta1.TLSMatchAttributes{{
					Port:     m.ExternalPort,
					SniHosts: []string{m.SNIHost},
				}},
				Route: []*istioapinetworkingv1beta1.RouteDestination{{
					Destination: &istioapinetworkingv1beta1.Destination{
						Host: m.PodFQDN,
						Port: &istioapinetworkingv1beta1.PortSelector{Number: uint32(etcdconstants.PortEtcdPeer)}, // #nosec G115 -- Port constants are positive values well within uint32 range.
					},
				}},
			}
		}

		virtualService.Spec = istioapinetworkingv1beta1.VirtualService{
			ExportTo: exportTo,
			Hosts:    allHosts,
			Gateways: []string{gatewayName},
			Tls:      routes,
		}
		return nil
	}
}

func gatewayWithClientTLSPassthrough(gateway *istionetworkingv1beta1.Gateway, labels, istioLabels map[string]string, hosts []string) func() error {
	return func() error {
		gateway.Labels = labels
		gateway.Spec = istioapinetworkingv1beta1.Gateway{
			Selector: istioLabels,
			Servers: []*istioapinetworkingv1beta1.Server{{
				Hosts: hosts,
				Port: &istioapinetworkingv1beta1.Port{
					Number:   uint32(etcdconstants.PortEtcdClientExternal), // #nosec G115 -- Port constants are positive values well within uint32 range.
					Name:     etcdconstants.ServicePortNameEtcdClient,
					Protocol: "TLS",
				},
				Tls: &istioapinetworkingv1beta1.ServerTLSSettings{
					Mode: istioapinetworkingv1beta1.ServerTLSSettings_PASSTHROUGH,
				},
			}},
		}
		return nil
	}
}

func virtualServiceWithClientSNIMatch(virtualService *istionetworkingv1beta1.VirtualService, labels map[string]string, exportTo, hosts []string, gatewayName, destinationHost string) func() error {
	return func() error {
		virtualService.Labels = labels
		virtualService.Spec = istioapinetworkingv1beta1.VirtualService{
			ExportTo: exportTo,
			Hosts:    hosts,
			Gateways: []string{gatewayName},
			Tls: []*istioapinetworkingv1beta1.TLSRoute{{
				Match: []*istioapinetworkingv1beta1.TLSMatchAttributes{{
					Port:     uint32(etcdconstants.PortEtcdClientExternal), // #nosec G115 -- Port constants are positive values well within uint32 range.
					SniHosts: hosts,
				}},
				Route: []*istioapinetworkingv1beta1.RouteDestination{{
					Destination: &istioapinetworkingv1beta1.Destination{
						Host: destinationHost,
						Port: &istioapinetworkingv1beta1.PortSelector{Number: uint32(etcdconstants.PortEtcdClient)}, // #nosec G115 -- Port constants are positive values well within uint32 range.
					},
				}},
			}},
		}
		return nil
	}
}
