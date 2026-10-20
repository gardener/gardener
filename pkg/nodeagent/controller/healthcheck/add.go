// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package healthcheck

import (
	"context"
	"fmt"
	"net"
	"slices"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/nodeagent/containerd"
	"github.com/gardener/gardener/pkg/nodeagent/dbus"
)

const (
	// ControllerName is the name of this controller.
	ControllerName = "health-check"

	defaultIntervalSeconds = 30
)

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(mgr manager.Manager, nodePredicate predicate.Predicate, isControlPlaneNode bool) error {
	if r.Client == nil {
		r.Client = mgr.GetClient()
	}

	if r.Recorder == nil {
		r.Recorder = mgr.GetEventRecorder(ControllerName)
	}

	if r.DBus == nil {
		r.DBus = dbus.New(mgr.GetLogger().WithValues("controller", ControllerName))
	}

	if len(r.HealthCheckers) == 0 {
		if err := r.setDefaultHealthChecks(isControlPlaneNode); err != nil {
			return err
		}
	}

	if r.HealthCheckIntervalSeconds == 0 {
		r.HealthCheckIntervalSeconds = defaultIntervalSeconds
	}

	return builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		Watches(
			&corev1.Pod{},
			handler.EnqueueRequestsFromMapFunc(r.PodToNodeMapper()),
			builder.WithPredicates(r.CriticalStaticPodsPredicate()),
		).
		For(&corev1.Node{}, builder.WithPredicates(nodePredicate)).
		WithOptions(controller.Options{MaxConcurrentReconciles: 1}).
		Complete(r)
}

func (r *Reconciler) setDefaultHealthChecks(isControlPlaneNode bool) error {
	clock := clock.RealClock{}

	client, err := containerd.NewClient()
	if err != nil {
		return fmt.Errorf("error creating containerd client: %w", err)
	}

	containerdHealthChecker := NewContainerdHealthChecker(r.Client, client, clock, r.DBus, r.Recorder)

	kubeletHealthChecker := NewKubeletHealthChecker(r.Client, clock, r.DBus, r.Recorder, net.InterfaceAddrs)

	r.HealthCheckers = []HealthChecker{containerdHealthChecker, kubeletHealthChecker}

	if isControlPlaneNode {
		r.HealthCheckers = append(r.HealthCheckers, NewCriticalStaticPodsChecker(r.Client, clock))
	}

	return nil
}

func (r *Reconciler) PodToNodeMapper() handler.MapFunc {
	return func(_ context.Context, obj client.Object) []reconcile.Request {
		pod, ok := obj.(*corev1.Pod)
		if !ok {
			return nil
		}

		return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: pod.Spec.NodeName}}}
	}
}

func (r *Reconciler) CriticalStaticPodsPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			pod, ok := e.Object.(*corev1.Pod)
			if !ok {
				return false
			}

			return pod.Labels[v1beta1constants.LabelIsCriticalStaticPod] == "true"
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldPod, ok := e.ObjectOld.(*corev1.Pod)
			if !ok {
				return false
			}
			newPod, ok := e.ObjectNew.(*corev1.Pod)
			if !ok {
				return false
			}

			if oldPod.Labels[v1beta1constants.LabelIsCriticalStaticPod] != "true" && newPod.Labels[v1beta1constants.LabelIsCriticalStaticPod] != "true" {
				return false
			}

			indexOldPod := slices.IndexFunc(oldPod.Status.Conditions, func(condition corev1.PodCondition) bool {
				return condition.Type == corev1.PodReady
			})

			indexNewPod := slices.IndexFunc(newPod.Status.Conditions, func(condition corev1.PodCondition) bool {
				return condition.Type == corev1.PodReady
			})

			if oldPod.Labels[v1beta1constants.LabelIsCriticalStaticPod] != newPod.Labels[v1beta1constants.LabelIsCriticalStaticPod] {
				return true
			}

			if oldPod.Status.Phase != newPod.Status.Phase {
				return true
			}

			if indexOldPod != -1 && indexNewPod != -1 && oldPod.Status.Conditions[indexOldPod].Status != newPod.Status.Conditions[indexNewPod].Status {
				return true
			}

			if (indexOldPod == -1 && indexNewPod != -1) || (indexOldPod != -1 && indexNewPod == -1) {
				return true
			}

			return false
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			pod, ok := e.Object.(*corev1.Pod)
			if !ok {
				return false
			}

			return pod.Labels[v1beta1constants.LabelIsCriticalStaticPod] == "true"
		},
		GenericFunc: func(_ event.GenericEvent) bool {
			return false
		},
	}
}
