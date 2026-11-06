// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package kubernetes

import (
	"context"
	"fmt"
	"net"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	kubeletapis "k8s.io/kubelet/pkg/apis"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/utils"
)

// HasMoreThanOneNode returns true if the cluster has more than one node. It uses a metadata-only list with limit 2 to
// minimize the amount of data transferred.
func HasMoreThanOneNode(ctx context.Context, reader client.Reader) (bool, error) {
	nodeList := &metav1.PartialObjectMetadataList{}
	nodeList.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("NodeList"))
	if err := reader.List(ctx, nodeList, client.Limit(2)); err != nil {
		return false, fmt.Errorf("failed to list nodes: %w", err)
	}
	return len(nodeList.Items) > 1, nil
}

// AvailableZones returns the set of distinct `topology.kubernetes.io/zone` values of all schedulable nodes matching
// the given list options. Cordoned (`.spec.unschedulable`) nodes are excluded, but node readiness is not considered on
// purpose: readiness flaps on reboots and kubelet restarts, which must not influence the zone set that drives hard node
// affinities.
func AvailableZones(ctx context.Context, reader client.Reader, opts ...client.ListOption) (sets.Set[string], error) {
	nodeList := &corev1.NodeList{}
	if err := reader.List(ctx, nodeList, opts...); err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}

	zones := sets.New[string]()
	for _, node := range nodeList.Items {
		if node.Spec.Unschedulable {
			continue
		}
		if zone := node.Labels[corev1.LabelTopologyZone]; zone != "" {
			zones.Insert(zone)
		}
	}

	return zones, nil
}

// IsNodeLabelAllowedForKubelet determines whether kubelet is allowed by the NodeRestriction admission plugin to set a
// label on its own Node object with the given key.
// See https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/#noderestriction.
func IsNodeLabelAllowedForKubelet(key string) bool {
	namespace := getLabelNamespace(key)

	// kubelets are forbidden to set node-restriction labels
	if namespace == corev1.LabelNamespaceNodeRestriction || strings.HasSuffix(namespace, "."+corev1.LabelNamespaceNodeRestriction) {
		return false
	}

	// kubelets are forbidden to set unknown kubernetes.io and k8s.io labels
	if isKubernetesLabelNamespace(namespace) && !kubeletapis.IsKubeletLabel(key) {
		return false
	}

	return true
}

// same logic as in kube-apiserver and kubelet code
func getLabelNamespace(key string) string {
	if parts := strings.SplitN(key, "/", 2); len(parts) == 2 {
		return parts[0]
	}
	return ""
}

func isKubernetesLabelNamespace(namespace string) bool {
	if namespace == "kubernetes.io" || strings.HasSuffix(namespace, ".kubernetes.io") {
		return true
	}
	if namespace == "k8s.io" || strings.HasSuffix(namespace, ".k8s.io") {
		return true
	}
	return false
}

// NodeInternalIP returns the internal IP of the given node, preferring the IP family indicated by preferIPv6.
// Falls back to any NodeInternalIP address if no address of the preferred family is found.
func NodeInternalIP(node corev1.Node, preferIPv6 bool) (net.IP, error) {
	var addrs []net.IP

	for _, address := range node.Status.Addresses {
		if address.Type != corev1.NodeInternalIP {
			continue
		}
		if ip := net.ParseIP(address.Address); ip != nil {
			addrs = append(addrs, ip)
		}
	}

	if ip := utils.PreferredIPAddress(preferIPv6, addrs...); ip != nil {
		return ip, nil
	}

	return nil, fmt.Errorf("no %s address found in node %s", corev1.NodeInternalIP, node.Name)
}
