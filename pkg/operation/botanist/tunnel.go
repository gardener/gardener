// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"fmt"
	"time"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/operation/shoot"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/retry"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CheckTunnelConnection checks if the tunnel connection between the control plane and the shoot networks
// is established.
func CheckTunnelConnection(ctx context.Context, shootClient kubernetes.Interface, logger logrus.FieldLogger, tunnelName string) (bool, error) {
	podList := &corev1.PodList{}
	if err := shootClient.Client().List(ctx, podList, client.InNamespace(metav1.NamespaceSystem), client.MatchingLabels{"app": tunnelName}); err != nil {
		return retry.SevereError(err)
	}

	var tunnelPod *corev1.Pod
	for _, pod := range podList.Items {
		if pod.Status.Phase == corev1.PodRunning {
			tunnelPod = &pod
			break
		}
	}

	if tunnelPod == nil {
		logger.Infof("Waiting until a running %s pod exists in the Shoot cluster...", tunnelName)
		return retry.MinorError(fmt.Errorf("no running %s pod found yet in the shoot cluster", tunnelName))
	}
	if err := shootClient.CheckForwardPodPort(tunnelPod.Namespace, tunnelPod.Name, 0, 22); err != nil {
		logger.Info("Waiting until the tunnel connection has been established...")
		return retry.MinorError(fmt.Errorf("could not forward to %s pod: %v", tunnelName, err))
	}

	logger.Info("Tunnel connection has been established.")
	return retry.Ok()
}

// CheckAndWaitForTunnelConnection checks until the tunnel connection between the control plane and the shoot networks
// is established, or until the configured timeout.
func CheckAndWaitForTunnelConnection(ctx context.Context, shootClient kubernetes.Interface, shoot *shoot.Shoot, logger logrus.FieldLogger, tunnelName string, timeout time.Duration) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	return retry.Until(timeoutCtx, 5*time.Second, func(ctx context.Context) (bool, error) {
		done, err := CheckTunnelConnection(ctx, shootClient, logger, common.VPNTunnel)

		// If the tunnel connection check failed but is not yet "done" (i.e., will be retried, hence, it didn't fail
		// with a severe error), and if the classic VPN solution is used for the shoot cluster then let's try to fetch
		// the last events of the vpn-shoot service (potentially indicating an error with the load balancer service).
		if err != nil &&
			!done &&
			!shoot.ReversedVPNEnabled {

			logger.Errorf("error %v occurred while checking the tunnel connection", err)

			service := &corev1.Service{
				TypeMeta: metav1.TypeMeta{
					APIVersion: corev1.SchemeGroupVersion.String(),
					Kind:       "Service",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "vpn-shoot",
					Namespace: metav1.NamespaceSystem,
				},
			}

			eventsErrorMessage, err2 := kutil.FetchEventMessages(ctx, shootClient.Client().Scheme(), shootClient.Client(), service, corev1.EventTypeWarning, 2)
			if err2 != nil {
				logger.Errorf("error %v occurred while fetching events for VPN load balancer service", err2)
				return retry.SevereError(fmt.Errorf("'%w' occurred but could not fetch events for more information", err))
			}

			if eventsErrorMessage != "" {
				return retry.SevereError(fmt.Errorf("%s\n\n%s", err.Error(), eventsErrorMessage))
			}
		}

		return done, err
	})
}
