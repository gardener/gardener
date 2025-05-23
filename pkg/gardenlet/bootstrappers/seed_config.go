// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bootstrappers

import (
	"context"
	"fmt"
	"net"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
)

// SeedConfigChecker checks whether the seed networks in the specification of the provided SeedConfig are correctly
// configured. Note that this only works in case the seed cluster is a shoot cluster (i.e., if it has the `shoot-info`
// ConfigMap in the kube-system namespace).
type SeedConfigChecker struct {
	SeedClient client.Client
	SeedConfig *gardenletconfigv1alpha1.SeedConfig
}

// Start performs the check.
func (s *SeedConfigChecker) Start(ctx context.Context) error {
	if s.SeedConfig == nil {
		return nil
	}

	shootInfo := &corev1.ConfigMap{}
	if err := s.SeedClient.Get(ctx, client.ObjectKey{Namespace: metav1.NamespaceSystem, Name: v1beta1constants.ConfigMapNameShootInfo}, shootInfo); client.IgnoreNotFound(err) != nil {
		return err
	} else if errors.IsNotFound(err) {
		// Seed cluster does not seem to be managed by Gardener
		return checkSeedConfigHeuristically(ctx, s.SeedClient, s.SeedConfig)
	}

	if podNetwork := shootInfo.Data["podNetwork"]; podNetwork != s.SeedConfig.Spec.Networks.Pods {
		return fmt.Errorf("incorrect pod network specified in seed configuration (cluster=%q vs. config=%q)", podNetwork, s.SeedConfig.Spec.Networks.Pods)
	}

	if serviceNetwork := shootInfo.Data["serviceNetwork"]; serviceNetwork != s.SeedConfig.Spec.Networks.Services {
		return fmt.Errorf("incorrect service network specified in seed configuration (cluster=%q vs. config=%q)", serviceNetwork, s.SeedConfig.Spec.Networks.Services)
	}

	// Be graceful in case the (optional) node network is only available on one side
	if nodeNetwork, exists := shootInfo.Data["nodeNetwork"]; exists &&
		s.SeedConfig.Spec.Networks.Nodes != nil &&
		*s.SeedConfig.Spec.Networks.Nodes != nodeNetwork {
		return fmt.Errorf("incorrect node network specified in seed configuration (cluster=%q vs. config=%q)", nodeNetwork, *s.SeedConfig.Spec.Networks.Nodes)
	}

	return nil
}

func isSameIPFamily(ip net.IP, ipNet *net.IPNet) bool {
	return ip.To4() != nil && ipNet.IP.To4() != nil || ip.To16() != nil && ip.To4() == nil && ipNet.IP.To16() != nil && ipNet.IP.To4() == nil
}

// checkSeedConfigHeuristically validates the networking configuration of the seed configuration heuristically against the actual cluster.
func checkSeedConfigHeuristically(ctx context.Context, seedClient client.Client, seedConfig *gardenletconfigv1alpha1.SeedConfig) error {
	// Restrict the heuristic to a maximum of 100 entries to prevent initialization delays for big clusters
	limit := &client.ListOptions{Limit: 100}

	if seedConfig.Spec.Networks.Nodes != nil {
		nodeList := &corev1.NodeList{}
		if err := seedClient.List(ctx, nodeList, limit); err != nil {
			return err
		}

		_, seedConfigNodes, err := net.ParseCIDR(*seedConfig.Spec.Networks.Nodes)
		if err != nil {
			return err
		}

		for _, node := range nodeList.Items {
			for _, address := range node.Status.Addresses {
				if address.Type == corev1.NodeInternalIP {
					if ip := net.ParseIP(address.Address); ip != nil && !seedConfigNodes.Contains(ip) && isSameIPFamily(ip, seedConfigNodes) {
						return fmt.Errorf("incorrect node network specified in seed configuration (cluster node=%q vs. config=%q)", ip, *seedConfig.Spec.Networks.Nodes)
					}
				}
			}
		}
	}

	podList := &corev1.PodList{}
	if err := seedClient.List(ctx, podList, limit); err != nil {
		return err
	}

	_, seedConfigPods, err := net.ParseCIDR(seedConfig.Spec.Networks.Pods)
	if err != nil {
		return err
	}

	for _, pod := range podList.Items {
		if pod.Annotations["cni.projectcalico.org/ipv4pools"] != "" || pod.Annotations["cni.projectcalico.org/ipv6pools"] != "" {
			// machine-controller-manager-provider-local configures machine pods to use IPs from dedicated IPPools that
			// correlate with the configured shoot node CIDR. I.e., such pods will use IPs outside the configured seed pod
			// CIDR. Skip pods configuring non-default IPPools in this heuristic check accordingly.
			continue
		}

		if !pod.Spec.HostNetwork && pod.Status.PodIP != "" {
			if ip := net.ParseIP(pod.Status.PodIP); ip != nil && !seedConfigPods.Contains(ip) {
				return fmt.Errorf("incorrect pod network specified in seed configuration (cluster pod=%q vs. config=%q)", ip, seedConfig.Spec.Networks.Pods)
			}
		}
	}

	serviceList := &corev1.ServiceList{}
	if err := seedClient.List(ctx, serviceList, limit); err != nil {
		return err
	}

	_, seedConfigServices, err := net.ParseCIDR(seedConfig.Spec.Networks.Services)
	if err != nil {
		return err
	}

	for _, service := range serviceList.Items {
		if service.Spec.Type == corev1.ServiceTypeClusterIP && service.Spec.ClusterIP != "" && service.Spec.ClusterIP != corev1.ClusterIPNone {
			if ip := net.ParseIP(service.Spec.ClusterIP); ip != nil && !seedConfigServices.Contains(ip) {
				return fmt.Errorf("incorrect service network specified in seed configuration (cluster service=%q vs. config=%q)", ip, seedConfig.Spec.Networks.Services)
			}
		}
	}

	return nil
}
