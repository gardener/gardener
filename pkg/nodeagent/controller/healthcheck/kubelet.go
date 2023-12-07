// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package healthcheck

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/gardener/gardener/pkg/nodeagent/dbus"
)

const (
	// kubeletServiceName is the systemd service name of the kubelet.
	kubeletServiceName = "kubelet.service"
	// defaultKubeletHealthEndpoint is the health endpoint of the kubelet.
	defaultKubeletHealthEndpoint = "http://127.0.0.1:10248/healthz"
	// maxToggles defines how often the kubelet can change the readiness during toggleTimeSpan until the node will be rebooted.
	maxToggles = 5
	// toggleTimeSpan is a floating time window where the kubelet readiness toggles are considered harmful.
	toggleTimeSpan = 10 * time.Minute
)

// KubeletHealthChecker configures the kubelet healthcheck.
type KubeletHealthChecker struct {
	// Clock exported for testing.
	Clock clock.Clock
	// KubeletReadinessToggles contains an entry for every toggle between kubelet Ready->NotReady or NotReady->Ready state.
	KubeletReadinessToggles []time.Time
	// NodeReady indicates if the node is ready. Exported for testing.
	NodeReady bool

	client client.Client
	// firstFailure stores the time of the first failed kubelet health check
	firstFailure *time.Time
	dbus         dbus.DBus
	recorder     record.EventRecorder
	// lastInternalIP stores the node internalIP
	lastInternalIP netip.Addr
	// getAddresses is a func which returns a slice of ip addresses on this node
	getAddresses          func() ([]net.Addr, error)
	kubeletHealthEndpoint string
}

// NewKubeletHealthChecker create a instance of a kubelet health check.
func NewKubeletHealthChecker(client client.Client, clock clock.Clock, dbus dbus.DBus, recorder record.EventRecorder, getAddresses func() ([]net.Addr, error)) *KubeletHealthChecker {
	return &KubeletHealthChecker{
		client:                  client,
		dbus:                    dbus,
		Clock:                   clock,
		recorder:                recorder,
		getAddresses:            getAddresses,
		KubeletReadinessToggles: []time.Time{},
		kubeletHealthEndpoint:   defaultKubeletHealthEndpoint,
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
	log := logf.FromContext(ctx).WithName("kubelet")

	// This mimics the old behavior of the kubelet-health-monitor which checks for
	// to many NotReady->Ready toggles in a certain time and reboots the node in such a case.
	if isNodeReady(node) && !k.NodeReady {
		needsReboot := k.ToggleKubeletState()
		log.Info("Kubelet became Ready", "readinessChanges", len(k.KubeletReadinessToggles), "timespan", toggleTimeSpan)
		if needsReboot {
			log.Info("Kubelet toggled between NotReady and Ready too often. Rebooting the node now")
			k.recorder.Eventf(node, corev1.EventTypeWarning, "kubelet", "Kubelet toggled between NotReady and Ready at least %d times in a %s time window. Rebooting the node now", maxToggles, toggleTimeSpan)
			err := k.dbus.Reboot()
			if err != nil {
				k.RevertToggleKubeletState()
				k.recorder.Event(node, corev1.EventTypeWarning, "kubelet", "Rebooting the node failed")
				return fmt.Errorf("rebooting the node failed %w", err)
			}
		}
	}
	k.NodeReady = isNodeReady(node)

	err := k.ensureNodeInternalIP(ctx, node)
	if err != nil {
		return err
	}

	httpClient := &http.Client{
		Timeout: 10 * time.Second,
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, k.kubeletHealthEndpoint, nil)
	if err != nil {
		log.Error(err, "Creating request to kubelet health endpoint failed")
		return err
	}
	response, err := httpClient.Do(request)
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
		k.recorder.Event(node, corev1.EventTypeWarning, "kubelet", "Kubelet is unhealthy")
	}

	if time.Since(*k.firstFailure).Abs() < maxFailureDuration {
		return nil
	}

	log.Error(err, "Kubelet is not healthy, restarting")
	err = k.dbus.Restart(ctx, k.recorder, node, kubeletServiceName)
	if err == nil {
		k.firstFailure = nil
	}
	return err
}

// ensureNodeInternalIP mimics the old weird logic which restores the internalIP of the node
// if this was lost for some reason.
func (k *KubeletHealthChecker) ensureNodeInternalIP(ctx context.Context, node *corev1.Node) error {
	log := logf.FromContext(ctx).WithName("kubelet")
	var (
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
					log.Error(err, "Unable to update node with internal IP")
					k.recorder.Eventf(node, corev1.EventTypeWarning, "kubelet", "Unable to update node with internal IP: %s", err.Error())
					return k.dbus.Restart(ctx, k.recorder, node, kubeletServiceName)
				}
				return err
			}
			log.Info("Updated internal IP address of node", "ip", k.lastInternalIP.String())
			k.recorder.Eventf(node, corev1.EventTypeNormal, "kubelet", "Updated the lost internal IP address of node to the previous known: %s ", k.lastInternalIP.String())
		}
	} else {
		var err error
		k.lastInternalIP, err = netip.ParseAddr(internalIP)
		if err != nil {
			return fmt.Errorf("unable to parse internal IP address %w", err)
		}
	}

	return nil
}

// ToggleKubeletState should be triggered if the state of the kubelet changed from Ready -> NotReady.
// It returns true if a reboot of the node should be triggered.
func (k *KubeletHealthChecker) ToggleKubeletState() bool {
	// Remove entries older toggleTimeSpan.
	for i := len(k.KubeletReadinessToggles); i > 0; i-- {
		if k.Clock.Since(k.KubeletReadinessToggles[i-1]).Abs() > toggleTimeSpan.Abs() {
			k.KubeletReadinessToggles = k.KubeletReadinessToggles[:i-1]
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
