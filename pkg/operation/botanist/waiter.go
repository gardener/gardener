// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package botanist

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/operation/botanist/component/konnectivity"
	"github.com/gardener/gardener/pkg/operation/common"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/retry"

	errorspkg "github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/pointer"
)

// WaitUntilNginxIngressServiceIsReady waits until the external load balancer of the nginx ingress controller has been created.
func (b *Botanist) WaitUntilNginxIngressServiceIsReady(ctx context.Context) error {
	const timeout = 10 * time.Minute

	loadBalancerIngress, err := kutil.WaitUntilLoadBalancerIsReady(ctx, b.K8sShootClient, metav1.NamespaceSystem, "addons-nginx-ingress-controller", timeout, b.Logger)
	if err != nil {
		return err
	}

	b.SetNginxIngressAddress(loadBalancerIngress, b.K8sSeedClient.DirectClient())
	return nil
}

// WaitUntilVpnShootServiceIsReady waits until the external load balancer of the VPN has been created.
func (b *Botanist) WaitUntilVpnShootServiceIsReady(ctx context.Context) error {
	const timeout = 10 * time.Minute

	_, err := kutil.WaitUntilLoadBalancerIsReady(ctx, b.K8sShootClient, metav1.NamespaceSystem, "vpn-shoot", timeout, b.Logger)
	return err
}

// WaitUntilKubeAPIServerIsDeleted waits until the kube-apiserver is deleted
func (b *Botanist) WaitUntilKubeAPIServerIsDeleted(ctx context.Context) error {
	return retry.UntilTimeout(ctx, 5*time.Second, 300*time.Second, func(ctx context.Context) (done bool, err error) {
		deploy := &appsv1.Deployment{}
		err = b.K8sSeedClient.Client().Get(ctx, kutil.Key(b.Shoot.SeedNamespace, v1beta1constants.DeploymentNameKubeAPIServer), deploy)
		switch {
		case apierrors.IsNotFound(err):
			return retry.Ok()
		case err == nil:
			return retry.MinorError(err)
		default:
			return retry.SevereError(err)
		}
	})
}

// WaitUntilKubeAPIServerReady waits until the kube-apiserver pod(s) indicate readiness in their statuses.
func (b *Botanist) WaitUntilKubeAPIServerReady(ctx context.Context) error {
	deployment := &appsv1.Deployment{}

	if err := retry.UntilTimeout(ctx, 5*time.Second, 300*time.Second, func(ctx context.Context) (done bool, err error) {
		if err := b.K8sSeedClient.APIReader().Get(ctx, kutil.Key(b.Shoot.SeedNamespace, v1beta1constants.DeploymentNameKubeAPIServer), deployment); err != nil {
			return retry.SevereError(err)
		}
		if deployment.Generation != deployment.Status.ObservedGeneration {
			return retry.MinorError(fmt.Errorf("kube-apiserver not observed at latest generation (%d/%d)",
				deployment.Status.ObservedGeneration, deployment.Generation))
		}

		replicas := int32(0)
		if deployment.Spec.Replicas != nil {
			replicas = *deployment.Spec.Replicas
		}
		if replicas != deployment.Status.UpdatedReplicas {
			return retry.MinorError(fmt.Errorf("kube-apiserver does not have enough updated replicas (%d/%d)",
				deployment.Status.UpdatedReplicas, replicas))
		}
		if replicas != deployment.Status.Replicas {
			return retry.MinorError(fmt.Errorf("kube-apiserver deployment has outdated replicas"))
		}
		if replicas != deployment.Status.AvailableReplicas {
			return retry.MinorError(fmt.Errorf("kube-apiserver does not have enough available replicas (%d/%d",
				deployment.Status.AvailableReplicas, replicas))
		}

		return retry.Ok()
	}); err != nil {
		var retryError *retry.Error
		if !errors.As(err, &retryError) {
			return err
		}

		newestPod, err2 := kutil.NewestPodForDeployment(ctx, b.K8sSeedClient.APIReader(), deployment)
		if err2 != nil {
			return errorspkg.Wrapf(err, "failure to find the newest pod for deployment to read the logs: %s", err2.Error())
		}
		if newestPod == nil {
			return err
		}

		logs, err2 := kutil.MostRecentCompleteLogs(ctx, b.K8sSeedClient.Kubernetes().CoreV1().Pods(newestPod.Namespace), newestPod, "kube-apiserver", pointer.Int64Ptr(10))
		if err2 != nil {
			return errorspkg.Wrapf(err, "failure to read the logs: %s", err2.Error())
		}

		errWithLogs := fmt.Errorf("%s, logs of newest pod:\n%s", err.Error(), logs)
		return gardencorev1beta1helper.DetermineError(errWithLogs, errWithLogs.Error())
	}

	return nil
}

// WaitUntilTunnelConnectionExists waits until a port forward connection to the tunnel pod (vpn-shoot or konnectivity-agent) in the kube-system
// namespace of the Shoot cluster can be established.
func (b *Botanist) WaitUntilTunnelConnectionExists(ctx context.Context) error {
	return retry.UntilTimeout(ctx, 5*time.Second, 900*time.Second, func(ctx context.Context) (done bool, err error) {
		tunnelName := common.VPNTunnel
		if b.Shoot.KonnectivityTunnelEnabled {
			tunnelName = konnectivity.AgentName
		}

		return b.CheckTunnelConnection(ctx, b.Logger, tunnelName)
	})
}

// WaitUntilNodesDeleted waits until no nodes exist in the shoot cluster anymore.
func (b *Botanist) WaitUntilNodesDeleted(ctx context.Context) error {
	return retry.Until(ctx, 5*time.Second, func(ctx context.Context) (done bool, err error) {
		nodesList := &corev1.NodeList{}
		if err := b.K8sShootClient.Client().List(ctx, nodesList); err != nil {
			return retry.SevereError(err)
		}

		if len(nodesList.Items) == 0 {
			return retry.Ok()
		}

		b.Logger.Infof("Waiting until all nodes have been deleted in the shoot cluster...")
		return retry.MinorError(fmt.Errorf("not all nodes have been deleted in the shoot cluster"))
	})
}

// WaitUntilNoPodRunning waits until there is no running Pod in the shoot cluster.
func (b *Botanist) WaitUntilNoPodRunning(ctx context.Context) error {
	b.Logger.Info("waiting until there are no running Pods in the shoot cluster...")

	return retry.Until(ctx, 5*time.Second, func(ctx context.Context) (done bool, err error) {
		podList := &corev1.PodList{}
		if err := b.K8sShootClient.Client().List(ctx, podList); err != nil {
			return retry.SevereError(err)
		}

		for _, pod := range podList.Items {
			if pod.Status.Phase == corev1.PodRunning {
				msg := fmt.Sprintf("waiting until there are no running Pods in the shoot cluster... "+
					"there is still at least one running Pod in the shoot cluster: %s/%s", pod.Namespace, pod.Name)
				b.Logger.Info(msg)
				return retry.MinorError(fmt.Errorf(msg))
			}
		}

		return retry.Ok()
	})
}

// WaitUntilEndpointsDoNotContainPodIPs waits until all endpoints in the shoot cluster to not contain any IPs from the Shoot's PodCIDR.
func (b *Botanist) WaitUntilEndpointsDoNotContainPodIPs(ctx context.Context) error {
	b.Logger.Info("waiting until there are no Endpoints containing Pod IPs in the shoot cluster...")

	var podsNetwork *net.IPNet
	if val := b.Shoot.Info.Spec.Networking.Pods; val != nil {
		var err error
		_, podsNetwork, err = net.ParseCIDR(*val)
		if err != nil {
			return fmt.Errorf("unable to check if there are still Endpoints containing Pod IPs in the shoot cluster. Shoots's Pods network could not be parsed: %+v", err)
		}
	} else {
		return fmt.Errorf("unable to check if there are still Endpoints containing Pod IPs in the shoot cluster. Shoot's Pods network is empty")
	}

	return retry.Until(ctx, 5*time.Second, func(ctx context.Context) (done bool, err error) {
		endpointsList := &corev1.EndpointsList{}
		if err := b.K8sShootClient.Client().List(ctx, endpointsList); err != nil {
			return retry.SevereError(err)
		}

		serviceList := &corev1.ServiceList{}
		if err := b.K8sShootClient.Client().List(ctx, serviceList); err != nil {
			return retry.SevereError(err)
		}

		epsNotReconciledByKCM := sets.NewString()
		for _, service := range serviceList.Items {
			// if service.Spec.Selector is empty or nil, kube-controller-manager will not reconcile Endpoints for this Service
			if len(service.Spec.Selector) == 0 {
				epsNotReconciledByKCM.Insert(fmt.Sprintf("%s/%s", service.Namespace, service.Name))
			}
		}

		for _, endpoints := range endpointsList.Items {
			if epsNotReconciledByKCM.Has(fmt.Sprintf("%s/%s", endpoints.Namespace, endpoints.Name)) {
				continue
			}

			for _, subset := range endpoints.Subsets {
				for _, address := range subset.Addresses {
					if podsNetwork.Contains(net.ParseIP(address.IP)) {
						msg := fmt.Sprintf("waiting until there are no Endpoints containing Pod IPs in the shoot cluster... "+
							"There is still at least one Endpoints object containing a Pod's IP: %s/%s, IP: %s", endpoints.Namespace, endpoints.Name, address.IP)
						b.Logger.Info(msg)
						return retry.MinorError(fmt.Errorf(msg))
					}
				}
			}
		}

		return retry.Ok()
	})
}

// WaitUntilRequiredExtensionsReady waits until all the extensions required for a shoot reconciliation are ready
func (b *Botanist) WaitUntilRequiredExtensionsReady(ctx context.Context) error {
	return retry.UntilTimeout(ctx, 5*time.Second, time.Minute, func(ctx context.Context) (done bool, err error) {
		if err := b.RequiredExtensionsReady(ctx); err != nil {
			b.Logger.Infof("Waiting until all the required extension controllers are ready (%+v)", err)
			return retry.MinorError(err)
		}
		return retry.Ok()
	})
}
