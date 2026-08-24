// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package peerexposure

import (
	"context"
	"fmt"

	istionetworkingv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/component"
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

func (p *peerExposure) Deploy(_ context.Context) error { return nil }

func (p *peerExposure) Destroy(_ context.Context) error { return nil }

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
