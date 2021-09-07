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
	"github.com/gardener/gardener/pkg/utils/retry"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// SetupPortForwarder is an alias for kubernetes.SetupPortForwarder, exposed for testing
var SetupPortForwarder = kubernetes.SetupPortForwarder

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

	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	fw, err := SetupPortForwarder(timeoutCtx, shootClient.RESTConfig(), tunnelPod.Namespace, tunnelPod.Name, 0, 22)
	if err != nil {
		return retry.MinorError(fmt.Errorf("could not setup pod port forwarding: %w", err))
	}

	if err := kubernetes.CheckForwardPodPort(fw); err != nil {
		logger.Info("Waiting until the tunnel connection has been established...")
		return retry.MinorError(fmt.Errorf("could not forward to %s pod (timeout after 5 seconds): %v", tunnelName, err))
	}

	logger.Info("Tunnel connection has been established.")
	return retry.Ok()
}
