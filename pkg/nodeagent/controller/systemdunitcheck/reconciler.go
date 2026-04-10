// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package systemdunitcheck

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	systemddbus "github.com/coreos/go-systemd/v22/dbus"
	"github.com/spf13/afero"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/clock"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	nodeagentconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/nodeagent/v1alpha1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/nodeagent"
	"github.com/gardener/gardener/pkg/nodeagent/dbus"
)

const (
	reasonAllUnitsHealthy = "AllUnitsHealthy"
	reasonUnhealthyUnits  = "UnhealthyUnits"
	reasonProgressing     = "ProgressingUnits"
)

// Reconciler checks the health of systemd units managed by gardener-node-agent and reports a condition on the Node.
type Reconciler struct {
	Client client.Client
	DBus   dbus.DBus
	Clock  clock.Clock
	FS     afero.Afero
	Config nodeagentconfigv1alpha1.SystemdUnitCheckControllerConfig
}

// Reconcile checks systemd unit health and updates the Node condition.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	node := &corev1.Node{}
	if err := r.Client.Get(ctx, request.NamespacedName, node); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	units, err := r.readManagedUnits()
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed reading managed units: %w", err)
	}
	if units == nil {
		log.V(1).Info("No last-applied OSC found, skipping systemd unit check")
		return reconcile.Result{RequeueAfter: r.Config.SyncPeriod.Duration}, nil
	}

	unhealthyMessages, progressingMessages, err := r.checkUnits(ctx, units)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed checking units: %w", err)
	}

	if err := r.updateNodeCondition(ctx, node, unhealthyMessages, progressingMessages); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{RequeueAfter: r.Config.SyncPeriod.Duration}, nil
}

// unitInfo holds the name and enabled state of a managed unit.
type unitInfo struct {
	name    string
	enabled bool
}

// readManagedUnits reads the last-applied OSC from disk and returns the managed units.
// Returns nil if the file does not exist yet.
func (r *Reconciler) readManagedUnits() ([]unitInfo, error) {
	data, err := r.FS.ReadFile(nodeagentconfigv1alpha1.LastAppliedOperatingSystemConfigFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("unable to read last-applied OSC: %w", err)
	}

	obj, _, err := nodeagent.OSCDecoder.Decode(data, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to decode last-applied OSC: %w", err)
	}

	osc, ok := obj.(*extensionsv1alpha1.OperatingSystemConfig)
	if !ok {
		return nil, fmt.Errorf("unexpected object type: %T", obj)
	}

	var units []unitInfo
	for _, unit := range osc.Spec.Units {
		units = append(units, unitInfo{name: unit.Name, enabled: ptr.Deref(unit.Enable, unitCarriesConfiguration(unit))})
	}

	// Extension units override spec units — apply on top.
	for _, unit := range osc.Status.ExtensionUnits {
		if idx := slices.IndexFunc(units, func(u unitInfo) bool { return u.name == unit.Name }); idx >= 0 {
			if unit.Enable != nil {
				units[idx].enabled = *unit.Enable
			}
		} else {
			units = append(units, unitInfo{name: unit.Name, enabled: ptr.Deref(unit.Enable, unitCarriesConfiguration(unit))})
		}
	}

	slices.SortFunc(units, func(a, b unitInfo) int { return strings.Compare(a.name, b.name) })

	return units, nil
}

// unitCarriesConfiguration returns true when a unit has content or drop-ins. Units with explicit configuration are
// considered actively managed — even without an explicit `Enable` field — and should be monitored. This covers
// extension units that add drop-ins to OS-provided services (e.g., adding dependencies to
// `systemd-user-sessions.service`): the unit is already enabled by the OS, but since the OSC customizes it, it is
// important enough to monitor.
func unitCarriesConfiguration(unit extensionsv1alpha1.Unit) bool {
	return unit.Content != nil || len(unit.DropIns) > 0
}

// checkUnits queries systemd for unit status and returns messages for unhealthy and progressing units.
func (r *Reconciler) checkUnits(ctx context.Context, units []unitInfo) (unhealthyMessages, progressingMessages []string, err error) {
	unitNames := make([]string, 0, len(units))
	for _, unit := range units {
		unitNames = append(unitNames, unit.name)
	}

	statuses, err := r.DBus.ListByNames(ctx, unitNames)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to list systemd unit statuses: %w", err)
	}

	unitNameToStatus := make(map[string]systemddbus.UnitStatus, len(statuses))
	for _, status := range statuses {
		unitNameToStatus[status.Name] = status
	}

	for _, unit := range units {
		status, found := unitNameToStatus[unit.name]
		if !found {
			if unit.enabled {
				unhealthyMessages = append(unhealthyMessages, fmt.Sprintf("%s: not loaded", unit.name))
			}
			continue
		}

		switch status.ActiveState {
		case "active":
			if !unit.enabled {
				unhealthyMessages = append(unhealthyMessages, fmt.Sprintf("%s: active but should be disabled", unit.name))
			}

		case "failed":
			unhealthyMessages = append(unhealthyMessages, fmt.Sprintf("%s: failed", unit.name))

		case "activating", "deactivating":
			// Services configured with Restart=always/on-success cycle through "activating (auto-restart)"
			// between runs. This is normal operation, not a health issue.
			if status.SubState == "auto-restart" {
				break
			}

			stateChangeTime, err := r.DBus.GetUnitStateChangeTimestamp(ctx, unit.name)
			if err != nil {
				return nil, nil, fmt.Errorf("unable to get state change timestamp for unit %s: %w", unit.name, err)
			}

			stuckDuration := r.Clock.Since(stateChangeTime)
			if stuckDuration >= r.Config.StuckThreshold.Duration {
				unhealthyMessages = append(unhealthyMessages, fmt.Sprintf("%s: stuck in %s for %s", unit.name, status.ActiveState, stuckDuration.Truncate(time.Second)))
			} else {
				progressingMessages = append(progressingMessages, fmt.Sprintf("%s: %s for %s", unit.name, status.ActiveState, stuckDuration.Truncate(time.Second)))
			}

		case "inactive":
			if !unit.enabled {
				break
			}

			// An enabled unit that is "inactive (dead)" is not necessarily unhealthy. Two common cases:
			// 1. Oneshot services (Type=oneshot) run once and exit — "inactive (dead)" is their normal steady state.
			// 2. Services triggered by timer/path units complete and go back to "inactive (dead)" between activations.
			if status.SubState == "dead" {
				serviceType, err := r.DBus.GetServiceType(ctx, unit.name)
				if err != nil {
					return nil, nil, fmt.Errorf("unable to get service type for unit %s: %w", unit.name, err)
				}
				if serviceType == "oneshot" {
					break
				}

				triggers, err := r.DBus.GetTriggeredBy(ctx, unit.name)
				if err != nil {
					return nil, nil, fmt.Errorf("unable to get TriggeredBy for unit %s: %w", unit.name, err)
				}
				if len(triggers) > 0 {
					break
				}
			}

			unhealthyMessages = append(unhealthyMessages, fmt.Sprintf("%s: inactive but should be enabled", unit.name))
		}
	}

	return unhealthyMessages, progressingMessages, nil
}

// updateNodeCondition patches the Node's SystemdUnitsReady condition.
func (r *Reconciler) updateNodeCondition(ctx context.Context, node *corev1.Node, unhealthyMessages, progressingMessages []string) error {
	var (
		patch = client.MergeFrom(node.DeepCopy())
		now   = metav1.NewTime(r.Clock.Now())

		newCondition corev1.NodeCondition
	)
	switch {
	case len(unhealthyMessages) > 0:
		newCondition = corev1.NodeCondition{
			Type:    nodeagentconfigv1alpha1.ConditionTypeSystemdUnitsReady,
			Status:  corev1.ConditionFalse,
			Reason:  reasonUnhealthyUnits,
			Message: strings.Join(unhealthyMessages, "; "),
		}
	case len(progressingMessages) > 0:
		newCondition = corev1.NodeCondition{
			Type:    nodeagentconfigv1alpha1.ConditionTypeSystemdUnitsReady,
			Status:  corev1.ConditionTrue,
			Reason:  reasonProgressing,
			Message: strings.Join(progressingMessages, "; "),
		}
	default:
		newCondition = corev1.NodeCondition{
			Type:    nodeagentconfigv1alpha1.ConditionTypeSystemdUnitsReady,
			Status:  corev1.ConditionTrue,
			Reason:  reasonAllUnitsHealthy,
			Message: "All systemd units from the operating system config are running as expected.",
		}
	}

	existingIdx := slices.IndexFunc(node.Status.Conditions, func(c corev1.NodeCondition) bool {
		return c.Type == nodeagentconfigv1alpha1.ConditionTypeSystemdUnitsReady
	})

	if existingIdx >= 0 && node.Status.Conditions[existingIdx].Status == newCondition.Status {
		newCondition.LastTransitionTime = node.Status.Conditions[existingIdx].LastTransitionTime
	} else {
		newCondition.LastTransitionTime = now
	}
	newCondition.LastHeartbeatTime = now

	if existingIdx >= 0 {
		node.Status.Conditions[existingIdx] = newCondition
	} else {
		node.Status.Conditions = append(node.Status.Conditions, newCondition)
	}

	if err := r.Client.Status().Patch(ctx, node, patch); err != nil {
		return fmt.Errorf("failed patching node status with systemd unit check condition: %w", err)
	}

	return nil
}
