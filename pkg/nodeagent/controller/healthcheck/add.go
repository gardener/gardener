// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package healthcheck

import (
	"fmt"
	"net"
	"os"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/defaults"
	"github.com/containerd/containerd/namespaces"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/gardener/gardener/pkg/nodeagent/dbus"
)

const (
	// ControllerName is the name of this controller.
	ControllerName = "health-check"

	defaultIntervalSeconds = 30
)

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(mgr manager.Manager, nodePredicate predicate.Predicate) error {
	if r.Client == nil {
		r.Client = mgr.GetClient()
	}

	if r.Recorder == nil {
		r.Recorder = mgr.GetEventRecorderFor(ControllerName)
	}

	if r.DBus == nil {
		r.DBus = dbus.New(mgr.GetLogger().WithValues("controller", ControllerName))
	}

	if len(r.HealthCheckers) == 0 {
		if err := r.setDefaultHealthChecks(); err != nil {
			return err
		}
	}

	if r.HealthCheckIntervalSeconds == 0 {
		r.HealthCheckIntervalSeconds = defaultIntervalSeconds
	}

	return builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		For(&corev1.Node{}, builder.WithPredicates(nodePredicate)).
		WithOptions(controller.Options{MaxConcurrentReconciles: 1}).
		Complete(r)
}

func (r *Reconciler) setDefaultHealthChecks() error {
	clock := clock.RealClock{}

	address := os.Getenv("CONTAINERD_ADDRESS")
	if address == "" {
		address = defaults.DefaultAddress
	}

	namespace := os.Getenv(namespaces.NamespaceEnvVar)
	if namespace == "" {
		namespace = namespaces.Default
	}

	client, err := containerd.New(address, containerd.WithDefaultNamespace(namespace))
	if err != nil {
		return fmt.Errorf("error creating containerd client: %w", err)
	}

	containerdHealthChecker := NewContainerdHealthChecker(r.Client, client, clock, r.DBus, r.Recorder)

	kubeletHealthChecker := NewKubeletHealthChecker(r.Client, clock, r.DBus, r.Recorder, net.InterfaceAddrs)
	r.HealthCheckers = []HealthChecker{containerdHealthChecker, kubeletHealthChecker}
	return nil
}
