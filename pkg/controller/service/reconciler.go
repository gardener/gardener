// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"math/rand/v2"
	"strings"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
	keyIstioIngressGateway      = client.ObjectKey{Namespace: "istio-ingress", Name: "istio-ingressgateway"}
	keyIstioIngressGatewayZone0 = client.ObjectKey{Namespace: "istio-ingress--0", Name: "istio-ingressgateway"}
	keyIstioIngressGatewayZone1 = client.ObjectKey{Namespace: "istio-ingress--1", Name: "istio-ingressgateway"}
	keyIstioIngressGatewayZone2 = client.ObjectKey{Namespace: "istio-ingress--2", Name: "istio-ingressgateway"}

	keyVirtualGardenIstioIngressGateway = client.ObjectKey{Namespace: "virtual-garden-istio-ingress", Name: "istio-ingressgateway"}
)

const (
	nodePortIstioIngressGateway      int32 = 30443
	nodePortIstioIngressGatewayZone0 int32 = 30444
	nodePortIstioIngressGatewayZone1 int32 = 30445
	nodePortIstioIngressGatewayZone2 int32 = 30446

	nodePortVirtualGardenKubeAPIServer int32 = 31443

	nodePortHTTPProxyIstioIngressGateway      int32 = 32443
	nodePortHTTPProxyIstioIngressGatewayZone0 int32 = 32444
	nodePortHTTPProxyIstioIngressGatewayZone1 int32 = 32445
	nodePortHTTPProxyIstioIngressGatewayZone2 int32 = 32446

	nodePortBastion int32 = 30022
)

// TODO(hown3d): Drop with RemoveHTTPProxyLegacyPort feature gate
const (
	nodePortTunnelIstioIngressGateway      int32 = 32132
	nodePortTunnelIstioIngressGatewayZone0 int32 = 32133
	nodePortTunnelIstioIngressGatewayZone1 int32 = 32134
	nodePortTunnelIstioIngressGatewayZone2 int32 = 32135
)

// Reconciler is a reconciler for Service resources.
type Reconciler struct {
	Client      client.Client
	HostIP      string
	Zone0IP     string
	Zone1IP     string
	Zone2IP     string
	IsMultiZone bool
	BastionIP   string
}

// Reconcile reconciles Service resources.
func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	var (
		log     = logf.FromContext(ctx)
		key     = req.NamespacedName
		service = &corev1.Service{}
	)

	if err := r.Client.Get(ctx, key, service); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	if service.Spec.Type != corev1.ServiceTypeLoadBalancer {
		return reconcile.Result{}, nil
	}

	log.Info("Reconciling service")

	var (
		ips      []string
		nodePort int32
		// TODO(hown3d): Drop with RemoveHTTPProxyLegacyPort feature gate
		nodePortTunnel    int32
		nodePortHTTPProxy int32
		isBastion         = service.Labels["app"] == "bastion"
		patch             = client.MergeFrom(service.DeepCopy())
	)

	switch key {
	case keyIstioIngressGateway:
		nodePort = nodePortIstioIngressGateway
		nodePortTunnel = nodePortTunnelIstioIngressGateway
		nodePortHTTPProxy = nodePortHTTPProxyIstioIngressGateway
		if r.IsMultiZone {
			// Docker desktop for mac v4.23 breaks traffic going through a port mapping to a different docker container.
			// Setting external traffic policy to local mitigates the issue for multi-node setups, e.g. for gardener-operator.
			service.Spec.ExternalTrafficPolicy = corev1.ServiceExternalTrafficPolicyLocal
		}
		// Internal IPs of the nodes are required to reach the virtual garden from inside the kind cluster.
		// If there is a virtual-garden-istio-ingress istio-ingressgateway created by operator, the IPs should be added to its LB service (see `keyVirtualGardenIstioIngressGateway` case).
		// If there is no such ingress, the IPs should be added to the LB service of istio-ingress istio-ingress-gateway (this case).
		// This handling assumes that virtual-garden-istio-ingress istio-ingressgateway would be created first.
		if err := r.Client.Get(ctx, keyVirtualGardenIstioIngressGateway, ptr.To(corev1.Service{})); apierrors.IsNotFound(err) {
			nodes, err := r.getNodesInternalIPs(ctx)
			if err != nil {
				return reconcile.Result{}, err
			}
			ips = append(ips, nodes...)
		} else if err != nil {
			return reconcile.Result{}, err
		}

		// We append this IP behind the nodeIPs because the gardenlet puts the last ingress IP into the DNSRecords
		// and we need a change in the DNSRecords if the control-plane moved from one zone to another to update the coredns-custom configMap.
		ips = append(ips, r.HostIP)
	case keyIstioIngressGatewayZone0:
		nodePort = nodePortIstioIngressGatewayZone0
		nodePortTunnel = nodePortTunnelIstioIngressGatewayZone0
		nodePortHTTPProxy = nodePortHTTPProxyIstioIngressGatewayZone0
		ips = append(ips, r.Zone0IP)
	case keyIstioIngressGatewayZone1:
		nodePort = nodePortIstioIngressGatewayZone1
		nodePortTunnel = nodePortTunnelIstioIngressGatewayZone1
		nodePortHTTPProxy = nodePortHTTPProxyIstioIngressGatewayZone1
		ips = append(ips, r.Zone1IP)
	case keyIstioIngressGatewayZone2:
		nodePort = nodePortIstioIngressGatewayZone2
		nodePortTunnel = nodePortTunnelIstioIngressGatewayZone2
		nodePortHTTPProxy = nodePortHTTPProxyIstioIngressGatewayZone2
		ips = append(ips, r.Zone2IP)
	case keyVirtualGardenIstioIngressGateway:
		nodePort = nodePortVirtualGardenKubeAPIServer
		if r.IsMultiZone {
			// Docker desktop for mac v4.23 breaks traffic going through a port mapping to a different docker container.
			// Setting external traffic policy to local mitigates the issue for multi-node setups, e.g. for gardener-operator.
			service.Spec.ExternalTrafficPolicy = corev1.ServiceExternalTrafficPolicyLocal
		}
		nodes, err := r.getNodesInternalIPs(ctx, client.MatchingLabels{"node-role.kubernetes.io/control-plane": ""})
		if err != nil {
			return reconcile.Result{}, err
		}
		ips = append(ips, nodes...)
	}

	if isBastion {
		// We only allocate and port-forward a single IP/nodePort for bastion services in the local setup.
		// Multiple bastion services are not supported.
		serviceList := &corev1.ServiceList{}
		if err := r.Client.List(ctx, serviceList, client.MatchingLabels{"app": "bastion"}); err != nil {
			return reconcile.Result{}, fmt.Errorf("error listing bastion services: %w", err)
		}
		if len(serviceList.Items) > 1 {
			return reconcile.Result{}, fmt.Errorf("only one bastion service is supported in the local setup")
		}

		nodePort = nodePortBastion
		ips = append(ips, r.BastionIP)
	}

	for i, servicePort := range service.Spec.Ports {
		if servicePort.Name == "tcp" || servicePort.Name == "ssh" {
			service.Spec.Ports[i].NodePort = nodePort
		}
		if servicePort.Name == "tls-tunnel" {
			service.Spec.Ports[i].NodePort = nodePortTunnel
		}
		if servicePort.Name == "http-proxy" {
			service.Spec.Ports[i].NodePort = nodePortHTTPProxy
		}
	}

	if err := r.Client.Patch(ctx, service, patch); err != nil {
		if (apierrors.IsInvalid(err) && strings.Contains(err.Error(), "port is already allocated")) ||
			// for some reason this error is not of type "Invalid"
			strings.Contains(err.Error(), "duplicate nodePort") {
			log.Info("Patching nodePort failed because it is already allocated, enabling auto-remediation", "err", err.Error())
			return reconcile.Result{RequeueAfter: 2 * time.Second}, r.remediateAllocatedNodePorts(ctx, log, key, nodePort, nodePortTunnel)
		}
		return reconcile.Result{}, err
	}

	patch = client.MergeFrom(service.DeepCopy())
	service.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{}

	for _, ip := range ips {
		service.Status.LoadBalancer.Ingress = append(service.Status.LoadBalancer.Ingress, corev1.LoadBalancerIngress{IP: ip})
	}

	return reconcile.Result{}, r.Client.Status().Patch(ctx, service, patch)
}

func (r *Reconciler) getNodesInternalIPs(ctx context.Context, opts ...client.ListOption) ([]string, error) {
	nodes := &corev1.NodeList{}
	if err := r.Client.List(ctx, nodes, opts...); err != nil {
		return nil, err
	}

	var ips []string
	for _, node := range nodes.Items {
		for _, address := range node.Status.Addresses {
			if address.Type == corev1.NodeInternalIP {
				ips = append(ips, address.Address)
			}
		}
	}
	return ips, nil
}

func (r *Reconciler) remediateAllocatedNodePorts(ctx context.Context, log logr.Logger, keyService client.ObjectKey, nodePorts ...int32) error {
	serviceList := &corev1.ServiceList{}
	if err := r.Client.List(ctx, serviceList); err != nil {
		return err
	}

	for _, service := range serviceList.Items {
		if client.ObjectKeyFromObject(&service) == keyService {
			continue
		}

		var (
			mustUpdate    bool
			patch         = client.StrategicMergeFrom(service.DeepCopy())
			requiredPorts = sets.New(nodePorts...)
		)

		for i, port := range service.Spec.Ports {
			if port.NodePort != 0 {
				log.Info("Found service with nodePort", "serviceCheckedForAllocation", client.ObjectKeyFromObject(&service), "nodePort", port.NodePort)
			}

			if requiredPorts.Has(port.NodePort) {
				var (
					min, max    = 30000, 32767
					newNodePort = int32(rand.N(max-min) + min) // #nosec: G115 G404 -- Value range limited in previous line, no cryptographic context.
				)

				log.Info("Assigning new nodePort to service which already allocates the nodePort",
					"service", client.ObjectKeyFromObject(&service),
					"oldNodePort", port.NodePort,
					"newNodePort", newNodePort,
				)

				service.Spec.Ports[i].NodePort = newNodePort
				mustUpdate = true
			}
		}

		if mustUpdate {
			if err := r.Client.Patch(ctx, &service, patch); err != nil {
				return err
			}
		}
	}

	return nil
}
