// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	vpnseedserver "github.com/gardener/gardener/pkg/component/networking/vpn/seedserver"
	"github.com/gardener/gardener/pkg/utils"
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

// CheckVPNSeedServerClientsConnected scrapes the vpn-seed-server exporter(s) in the given namespace and returns the set
// of client common names currently connected across all running vpn-seed-server pods. It is only meaningful for HA VPN,
// where the exporter exists.
func CheckVPNSeedServerClientsConnected(ctx context.Context, restConfig *rest.Config, seedClient client.Client, namespace string) (sets.Set[string], error) {
	podList := &corev1.PodList{}
	if err := seedClient.List(ctx, podList, client.InNamespace(namespace), client.MatchingLabels{v1beta1constants.LabelApp: v1beta1constants.DeploymentNameVPNSeedServer}); err != nil {
		return nil, err
	}

	connected := sets.New[string]()
	for _, pod := range podList.Items {
		if pod.Status.Phase != corev1.PodRunning {
			continue
		}
		metrics, err := scrapeVPNSeedServerMetrics(ctx, restConfig, namespace, pod.Name)
		if err != nil {
			return nil, err
		}
		connected = connected.Union(parseConnectedVPNClients(metrics))
	}
	return connected, nil
}

// scrapeVPNSeedServerMetrics port-forwards to the openvpn exporter of the given vpn-seed-server pod and returns the raw
// /metrics response body.
func scrapeVPNSeedServerMetrics(ctx context.Context, restConfig *rest.Config, namespace, podName string) (string, error) {
	fwCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	local, err := utils.FindFreePort()
	if err != nil {
		return "", err
	}

	fw, err := SetupPortForwarder(fwCtx, restConfig, namespace, podName, local, vpnseedserver.MetricsPort)
	if err != nil {
		return "", err
	}

	if err := kubernetes.CheckForwardPodPort(fw); err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(fwCtx, http.MethodGet, fmt.Sprintf("http://localhost:%d/metrics", local), nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code %d scraping vpn-seed-server metrics of pod %s", resp.StatusCode, podName)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// parseConnectedVPNClients extracts the set of connected client common names from openvpn exporter /metrics output. A
// client is considered connected when it has at least one per-client series (openvpn_server_client_*) carrying its
// common_name label.
func parseConnectedVPNClients(metrics string) sets.Set[string] {
	result := sets.New[string]()
	for line := range strings.SplitSeq(metrics, "\n") {
		if !strings.HasPrefix(line, "openvpn_server_client_") {
			continue
		}
		_, afterLabel, ok := strings.Cut(line, `common_name="`)
		if !ok {
			continue
		}
		if cn, _, ok := strings.Cut(afterLabel, `"`); ok {
			result.Insert(cn)
		}
	}
	return result
}

// expectedVPNClientCNs returns the HA client common names for the permanent (nameSuffix "") or temporary
// (nameSuffix "temp") vpn-shoot, matching the naming produced by the vpn-shoot component.
func expectedVPNClientCNs(nameSuffix string, haShootClients int) sets.Set[string] {
	base := "vpn-shoot-client"
	if nameSuffix != "" {
		base = base + "-" + nameSuffix
	}

	result := sets.New[string]()
	for i := range haShootClients {
		result.Insert(fmt.Sprintf("%s-%d", base, i))
	}
	return result
}
