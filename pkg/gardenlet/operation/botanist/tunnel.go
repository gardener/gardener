// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils/retry"
)

// SetupPortForwarder is an alias for kubernetes.SetupPortForwarder, exposed for testing
var SetupPortForwarder = kubernetes.SetupPortForwarder

// CheckTunnelConnection checks if the tunnel connection between the control plane and the shoot networks
// is established.
func CheckTunnelConnection(ctx context.Context, log logr.Logger, shootClient kubernetes.Interface, tunnelName string) (bool, error) {
	podList := &corev1.PodList{}
	if err := shootClient.Client().List(ctx, podList, client.InNamespace(metav1.NamespaceSystem), client.MatchingLabels{"app": tunnelName}); err != nil {
		return retry.SevereError(err)
	}

	var tunnelPod *corev1.Pod
	for _, p := range podList.Items {
		pod := p
		if pod.Status.Phase == corev1.PodRunning {
			tunnelPod = &pod
			break
		}
	}

	if tunnelPod == nil {
		log.Info("Waiting until a running pod exists in the Shoot cluster", "tunnelName", tunnelName)
		return retry.MinorError(fmt.Errorf("no running %s pod found yet in the shoot cluster", tunnelName))
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	fw, err := SetupPortForwarder(timeoutCtx, shootClient.RESTConfig(), tunnelPod.Namespace, tunnelPod.Name, 0, 22)
	if err != nil {
		return retry.MinorError(fmt.Errorf("could not setup pod port forwarding: %w", err))
	}

	if err := kubernetes.CheckForwardPodPort(fw); err != nil {
		log.Info("Waiting until the tunnel connection has been established")
		return retry.MinorError(fmt.Errorf("could not forward to %s pod (timeout after 5 seconds): %v", tunnelName, err))
	}

	log.Info("Tunnel connection has been established")
	return retry.Ok()
}
