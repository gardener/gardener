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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	kubernetesclient "github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	etcdconstants "github.com/gardener/gardener/pkg/component/etcd/etcd/constants"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	managedresourcesutils "github.com/gardener/gardener/pkg/utils/managedresources"
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

func (p *peerExposure) managedResourceName() string {
	return fmt.Sprintf("etcd-%s-peer-exposure", p.values.Role)
}

func (p *peerExposure) Deploy(ctx context.Context) error {
	var (
		err      error
		registry = managedresourcesutils.NewRegistry(kubernetesclient.SeedScheme, kubernetesclient.SeedCodec, kubernetesclient.SeedSerializer)
	)

	gateway := p.emptyGatewayFor(p.name())
	if err = gatewayWithPeerTLSPassthrough(gateway, getLabels(p.values.Role), p.values.IstioIngressGatewayLabels, p.values.Members)(); err != nil {
		return err
	}

	virtualService := p.emptyVirtualServiceFor(p.name())
	if err = virtualServiceWithPeerSNIMatch(virtualService, getLabels(p.values.Role), []string{p.values.IstioIngressGatewayNamespace}, p.values.Members, gateway.Name)(); err != nil {
		return err
	}

	networkPolicyTriggerService := p.emptyServiceFor(p.npServiceName())
	if err = p.mutateNetworkPolicyTriggerService(networkPolicyTriggerService)(); err != nil {
		return err
	}

	resources := []client.Object{gateway, virtualService, networkPolicyTriggerService}

	// Gardener seeds set defaultServiceExportTo: ["~"] in the mesh config, so services are self-namespace only by
	// default. ServiceEntries with resolution: DNS and exportTo pointing at the istio-ingress namespace make each
	// pod's subdomain discoverable by the ingress proxy without touching the etcd-druid-owned Service (which the druid
	// admission webhook would reject).
	for i, m := range p.values.Members {
		serviceEntry := p.emptyServiceEntryFor(fmt.Sprintf("%s-%d", p.name(), i))
		if err := serviceEntryForExport(serviceEntry, getLabels(p.values.Role), m.PodFQDN, p.values.IstioIngressGatewayNamespace, uint32(etcdconstants.PortEtcdPeer), etcdconstants.ServicePortNameEtcdPeer)(); err != nil { // #nosec G115 -- Port constants are positive values well within uint32 range.
			return err
		}
		resources = append(resources, serviceEntry)
	}

	if p.values.ClientHost != "" {
		clientGateway := p.emptyGatewayFor(p.clientName())
		if err = gatewayWithClientTLSPassthrough(clientGateway, getLabels(p.values.Role), p.values.IstioIngressGatewayLabels, []string{p.values.ClientHost})(); err != nil {
			return err
		}

		clientVirtualService := p.emptyVirtualServiceFor(p.clientName())
		if err = virtualServiceWithClientSNIMatch(clientVirtualService, getLabels(p.values.Role), []string{p.values.IstioIngressGatewayNamespace}, []string{p.values.ClientHost}, clientGateway.Name, p.clientServiceHost())(); err != nil {
			return err
		}

		clientServiceEntry := p.emptyServiceEntryFor(p.clientName())
		if err = serviceEntryForExport(clientServiceEntry, getLabels(p.values.Role), p.clientServiceHost(), p.values.IstioIngressGatewayNamespace, uint32(etcdconstants.PortEtcdClient), etcdconstants.ServicePortNameEtcdClient)(); err != nil { // #nosec G115 -- Port constants are positive values well within uint32 range.
			return err
		}

		resources = append(resources, clientGateway, clientVirtualService, clientServiceEntry)
	}

	data, err := registry.AddAllAndSerialize(resources...)
	if err != nil {
		return err
	}

	return managedresourcesutils.CreateForSeed(ctx, p.client, p.namespace, p.managedResourceName(), false, data)
}

func (p *peerExposure) Destroy(ctx context.Context) error {
	return managedresourcesutils.DeleteForSeed(ctx, p.client, p.namespace, p.managedResourceName())
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

func (p *peerExposure) emptyServiceFor(name string) *corev1.Service {
	return &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: p.namespace}}
}

func (p *peerExposure) npServiceName() string {
	return fmt.Sprintf("etcd-%s-np", p.values.Role)
}

func (p *peerExposure) mutateNetworkPolicyTriggerService(svc *corev1.Service) func() error {
	return func() error {
		svc.Labels = getLabels(p.values.Role)
		svc.Spec.Selector = map[string]string{
			v1beta1constants.LabelApp:  etcdconstants.LabelAppValue,
			v1beta1constants.LabelRole: p.values.Role,
		}
		svc.Spec.Ports = []corev1.ServicePort{
			{Name: fmt.Sprintf("tcp-%d", etcdconstants.PortEtcdPeer), Port: etcdconstants.PortEtcdPeer, Protocol: corev1.ProtocolTCP},
			{Name: fmt.Sprintf("tcp-%d", etcdconstants.PortEtcdClient), Port: etcdconstants.PortEtcdClient, Protocol: corev1.ProtocolTCP},
		}
		utilruntime.Must(gardenerutils.InjectNetworkPolicyNamespaceSelectors(svc,
			metav1.LabelSelector{MatchLabels: map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleIstioIngress}},
		))
		metav1.SetMetaDataAnnotation(&svc.ObjectMeta, resourcesv1alpha1.NetworkingPodLabelSelectorNamespaceAlias, v1beta1constants.LabelNetworkPolicyShootNamespaceAlias)
		return nil
	}
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
