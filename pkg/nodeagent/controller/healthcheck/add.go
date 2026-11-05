// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package healthcheck

import (
	"context"
	"fmt"
	"net"

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
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
)

// ControllerName is the name of this controller.
const ControllerName = "health-check"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(mgr manager.Manager, nodePredicate predicate.Predicate, addCriticalStaticPodsChecker bool) error {
	if r.Client == nil {
		r.Client = mgr.GetClient()
	}
	if r.Clock == nil {
		r.Clock = clock.RealClock{}
	}
	if r.Recorder == nil {
		r.Recorder = mgr.GetEventRecorder(ControllerName)
	}
	if r.DBus == nil {
		r.DBus = dbus.New(mgr.GetLogger().WithValues("controller", ControllerName))
	}
	if r.HealthCheckIntervalSeconds == 0 {
		r.HealthCheckIntervalSeconds = 30
	}
	if len(r.HealthCheckers) == 0 {
		if err := r.setDefaultHealthChecks(addCriticalStaticPodsChecker); err != nil {
			return err
		}
	}

	controller := builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		For(&corev1.Node{}, builder.WithPredicates(nodePredicate)).
		WithOptions(controller.Options{MaxConcurrentReconciles: 1})

	if addCriticalStaticPodsChecker {
		controller = controller.Watches(
			&corev1.Pod{},
			handler.EnqueueRequestsFromMapFunc(r.PodToNodeMapper()),
			builder.WithPredicates(r.IsCriticalStaticPodPredicate(), r.CriticalPodHealthStatusChangedPredicate()),
		)
	}

	return controller.Complete(r)
}

func (r *Reconciler) setDefaultHealthChecks(addCriticalStaticPodsChecker bool) error {
	containerdClient, err := containerd.NewClient()
	if err != nil {
		return fmt.Errorf("error creating containerd client: %w", err)
	}

	r.HealthCheckers = []HealthChecker{
		NewContainerdHealthChecker(r.Client, containerdClient, r.Clock, r.DBus, r.Recorder),
		NewKubeletHealthChecker(r.Client, r.Clock, r.DBus, r.Recorder, net.InterfaceAddrs),
	}

	if addCriticalStaticPodsChecker {
		r.HealthCheckers = append(r.HealthCheckers, NewCriticalStaticPodsChecker(r.Client, r.Clock))
	}

	return nil
}

// PodToNodeMapper maps a pod to the node it references in .spec.nodeName.
func (r *Reconciler) PodToNodeMapper() handler.MapFunc {
	return func(_ context.Context, obj client.Object) []reconcile.Request {
		pod, ok := obj.(*corev1.Pod)
		if !ok {
			return nil
		}

		return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: pod.Spec.NodeName}}}
	}
}

// IsCriticalStaticPodPredicate returns true when the pod is labeled with
// shoot.gardener.cloud/is-critical-static-pod=true.
func (r *Reconciler) IsCriticalStaticPodPredicate() predicate.Predicate {
	return predicate.NewPredicateFuncs(func(object client.Object) bool {
		return object.GetLabels()[v1beta1constants.LabelIsCriticalStaticPod] == "true"
	})
}

// CriticalPodHealthStatusChangedPredicate returns true when the .status.phase or the status of the `Ready` condition
// of a critical static pod has changed.
func (r *Reconciler) CriticalPodHealthStatusChangedPredicate() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldPod, ok := e.ObjectOld.(*corev1.Pod)
			if !ok {
				return false
			}
			newPod, ok := e.ObjectNew.(*corev1.Pod)
			if !ok {
				return false
			}

			return oldPod.Status.Phase != newPod.Status.Phase ||
				readyConditionStatus(oldPod) != readyConditionStatus(newPod)
		},
	}
}

// readyConditionStatus returns the status of the pod's `Ready` condition, or an empty status if the condition is absent.
func readyConditionStatus(pod *corev1.Pod) corev1.ConditionStatus {
	if condition := health.GetPodReadyCondition(pod.Status); condition != nil {
		return condition.Status
	}
	return ""
}
