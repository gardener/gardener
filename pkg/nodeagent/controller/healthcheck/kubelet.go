// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package healthcheck

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/nodeagent/dbus"
)

// DefaultKubeletHealthEndpoint is the health endpoint of the kubelet.
var DefaultKubeletHealthEndpoint = "http://127.0.0.1:10248/healthz"

const (
	// maxToggles defines how often the kubelet can change the readiness during toggleTimeSpan until the node will be rebooted.
	maxToggles = 5
	// toggleTimeSpan is a floating time window where the kubelet readiness toggles are considered harmful.
	toggleTimeSpan = 10 * time.Minute
)

// KubeletHealthChecker configures the kubelet healthcheck.
type KubeletHealthChecker struct {
	// Clock exported for testing.
	Clock clock.Clock
	// KubeletReadinessToggles contains an entry for every toggle between NotReady->Ready state.
	KubeletReadinessToggles []time.Time
	// NodeReady indicates if the node is ready. Exported for testing.
	NodeReady bool

	client                client.Client
	httpClient            *http.Client
	firstFailure          *time.Time
	dbus                  dbus.DBus
	recorder              record.EventRecorder
	lastInternalIP        netip.Addr
	getAddresses          func() ([]net.Addr, error)
	kubeletHealthEndpoint string
}

// NewKubeletHealthChecker create an instance of a kubelet health check.
func NewKubeletHealthChecker(client client.Client, clock clock.Clock, dbus dbus.DBus, recorder record.EventRecorder, getAddresses func() ([]net.Addr, error)) *KubeletHealthChecker {
	return &KubeletHealthChecker{
		client:                  client,
		httpClient:              &http.Client{Timeout: 10 * time.Second},
		dbus:                    dbus,
		Clock:                   clock,
		recorder:                recorder,
		getAddresses:            getAddresses,
		KubeletReadinessToggles: []time.Time{},
		kubeletHealthEndpoint:   DefaultKubeletHealthEndpoint,
	}
}

// Name returns the name of this health check.
func (*KubeletHealthChecker) Name() string {
	return "kubelet"
}

// HasLastInternalIP returns true if the node.InternalIP was stored.
// Exported for testing.
func (k *KubeletHealthChecker) HasLastInternalIP() bool {
	return k.lastInternalIP.IsValid()
}

// SetKubeletHealthEndpoint set the kubeletHealthEndpoint.
// Exported for testing.
func (k *KubeletHealthChecker) SetKubeletHealthEndpoint(kubeletHealthEndpoint string) {
	k.kubeletHealthEndpoint = kubeletHealthEndpoint
}

// Check performs the actual health check for the kubelet.
func (k *KubeletHealthChecker) Check(ctx context.Context, node *corev1.Node) error {
	log := logf.FromContext(ctx).WithName(k.Name())

	if err := k.verifyNodeReady(log, node); err != nil {
		return err
	}

	if err := k.ensureNodeInternalIP(ctx, node); err != nil {
		return err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, k.kubeletHealthEndpoint, nil)
	if err != nil {
		log.Error(err, "Creating request to kubelet health endpoint failed")
		return err
	}
	response, err := k.httpClient.Do(request)
	if err != nil {
		log.Error(err, "HTTP request to kubelet health endpoint failed")
	}
	if err == nil && response.StatusCode == http.StatusOK {
		if k.firstFailure != nil {
			log.Info("Kubelet is healthy again", "statusCode", response.StatusCode)
			k.recorder.Event(node, corev1.EventTypeNormal, "kubelet", "Kubelet is healthy")
			k.firstFailure = nil
		}
		return nil
	}
	if k.firstFailure == nil {
		now := k.Clock.Now()
		k.firstFailure = &now

		log.Error(err, "Kubelet is unhealthy")
		k.recorder.Eventf(node, corev1.EventTypeWarning, "kubelet", "Kubelet is unhealthy, health check error: %s", err.Error())
	}

	if time.Since(*k.firstFailure).Abs() < maxFailureDuration {
		return nil
	}

	log.Error(err, "Kubelet is unhealthy, restarting it", "failureDuration", maxFailureDuration)
	k.recorder.Eventf(node, corev1.EventTypeWarning, "kubelet", "Kubelet is unhealthy for more than %s, restarting it. Health check error: %s", maxFailureDuration, err.Error())
	err = k.dbus.Restart(ctx, k.recorder, node, v1beta1constants.OperatingSystemConfigUnitNameKubeletService)
	if err == nil {
		k.firstFailure = nil
	}
	return err
}

// ensureNodeInternalIP restores the internalIP of the node if this was initially set but lost in the process.
// This happens if Kubelet runs into a timeout when contacting the cloud provider API during start-up, see https://github.com/gardener/gardener/commit/1311de43a1745cbc8cf65d57c72e9ed0a2c5e586#diff-738db1352694482843441061260a6f02.
func (k *KubeletHealthChecker) ensureNodeInternalIP(ctx context.Context, node *corev1.Node) error {
	var (
		log        = logf.FromContext(ctx).WithName(k.Name())
		externalIP string
		internalIP string
	)

	for _, addr := range node.Status.Addresses {
		switch addr.Type {
		case corev1.NodeExternalIP:
			externalIP = addr.Address
		case corev1.NodeInternalIP:
			internalIP = addr.Address
		default:
			// ignore
		}
	}

	if externalIP == "" && internalIP == "" {
		if k.lastInternalIP.IsValid() {
			k.recorder.Eventf(node, corev1.EventTypeWarning, "kubelet", "Node status does neither have an internal nor an external IP, try to recover from last known internal IP:%s", k.lastInternalIP)
		} else {
			k.recorder.Event(node, corev1.EventTypeWarning, "kubelet", "Node status does neither have an internal nor an external IP")
		}
		addresses, err := k.getAddresses()
		if err != nil {
			return fmt.Errorf("unable to list all network interface IP addresses")
		}

		for _, addr := range addresses {
			parsed, err := netip.ParsePrefix(addr.String())
			if err != nil {
				return fmt.Errorf("unable to parse IP address %w", err)
			}
			parsedIP := parsed.Addr()
			if parsedIP.Compare(k.lastInternalIP) != 0 {
				continue
			}

			// One of the ip addresses on the node matches the previous set internalIP of the node which is now gone, set it again.
			node.Status.Addresses = []corev1.NodeAddress{
				{
					Type:    corev1.NodeInternalIP,
					Address: k.lastInternalIP.String(),
				},
			}

			err = k.client.Status().Update(ctx, node)
			if err != nil {
				if !apierrors.IsConflict(err) {
					log.Error(err, "Unable to update node status with internal IP")
					k.recorder.Eventf(node, corev1.EventTypeWarning, "kubelet", "Unable to update node status with internal IP: %s", err.Error())
					return k.dbus.Restart(ctx, k.recorder, node, v1beta1constants.OperatingSystemConfigUnitNameKubeletService)
				}
				return err
			}

			log.Info("Updated internal IP address of node status", "ip", k.lastInternalIP.String())
			k.recorder.Eventf(node, corev1.EventTypeNormal, "kubelet", "Updated the lost internal IP address of node status to the previous known: %s ", k.lastInternalIP.String())
		}
	} else if internalIP != "" {
		var err error
		k.lastInternalIP, err = netip.ParseAddr(internalIP)
		if err != nil {
			return fmt.Errorf("unable to parse internal IP address %w", err)
		}
	}

	return nil
}

// verifyNodeReady verifies the NodeReady condition of a node.
// If the condition changes 5 times within 10 minutes from NotReady->Ready the node will be rebooted.
func (k *KubeletHealthChecker) verifyNodeReady(log logr.Logger, node *corev1.Node) error {
	if isNodeReady(node) && !k.NodeReady {
		needsReboot := k.ToggleKubeletState()
		log.Info("Kubelet became Ready", "readinessChanges", len(k.KubeletReadinessToggles), "timespan", toggleTimeSpan)
		if needsReboot {
			log.Info("Kubelet toggled between NotReady and Ready too often. Rebooting the node now")
			k.recorder.Eventf(node, corev1.EventTypeWarning, "kubelet", "Kubelet toggled between NotReady and Ready at least %d times in a %s time window. Rebooting the node now", maxToggles, toggleTimeSpan)
			if err := k.dbus.Reboot(); err != nil {
				k.RevertToggleKubeletState()
				k.recorder.Event(node, corev1.EventTypeWarning, "kubelet", "Rebooting the node failed")
				return fmt.Errorf("rebooting the node failed %w", err)
			}
		}
	}
	k.NodeReady = isNodeReady(node)
	return nil
}

// ToggleKubeletState should be triggered if the state of the kubelet changed from NotReady -> Ready.
// It returns true if a reboot of the node should be triggered.
func (k *KubeletHealthChecker) ToggleKubeletState() bool {
	// Remove entries older toggleTimeSpan.
	for i := len(k.KubeletReadinessToggles); i > 0; i-- {
		if k.Clock.Since(k.KubeletReadinessToggles[i-1]).Abs() > toggleTimeSpan.Abs() {
			k.KubeletReadinessToggles = k.KubeletReadinessToggles[i:]
			break
		}
	}

	k.KubeletReadinessToggles = append(k.KubeletReadinessToggles, k.Clock.Now())

	return len(k.KubeletReadinessToggles) >= maxToggles
}

// RevertToggleKubeletState removes the last entry created by ToggleKubeletState().
func (k *KubeletHealthChecker) RevertToggleKubeletState() {
	if i := len(k.KubeletReadinessToggles); i > 0 {
		k.KubeletReadinessToggles = k.KubeletReadinessToggles[:i-1]
	}
}

// isNodeReady returns true if a node is ready; false otherwise.
func isNodeReady(node *corev1.Node) bool {
	for _, c := range node.Status.Conditions {
		if c.Type == corev1.NodeReady {
			return c.Status == corev1.ConditionTrue
		}
	}
	return false
}
