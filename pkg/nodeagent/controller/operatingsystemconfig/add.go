// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package operatingsystemconfig

import (
	"bytes"
	"context"
	"time"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/go-logr/logr"
	"github.com/spf13/afero"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	predicateutils "github.com/gardener/gardener/pkg/controllerutils/predicate"
	nodeagentconfigv1alpha1 "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/nodeagent/dbus"
	"github.com/gardener/gardener/pkg/nodeagent/registry"
)

// ControllerName is the name of this controller.
const ControllerName = "operatingsystemconfig"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(ctx context.Context, mgr manager.Manager) error {
	if r.Client == nil {
		r.Client = mgr.GetClient()
	}
	if r.Recorder == nil {
		r.Recorder = mgr.GetEventRecorderFor(ControllerName)
	}
	if r.DBus == nil {
		r.DBus = dbus.New(mgr.GetLogger().WithValues("controller", ControllerName))
	}
	if r.FS.Fs == nil {
		r.FS = afero.Afero{Fs: afero.NewOsFs()}
	}
	if r.Extractor == nil {
		r.Extractor = registry.NewExtractor()
	}

	return builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		Watches(
			&corev1.Secret{},
			r.EnqueueWithJitterDelay(ctx, mgr.GetLogger().WithValues("controller", ControllerName).WithName("reconciliation-delayer")),
			builder.WithPredicates(r.SecretPredicate(), predicateutils.ForEventTypes(predicateutils.Create, predicateutils.Update)),
		).
		Watches(
			&corev1.Node{},
			handler.EnqueueRequestsFromMapFunc(NodeToSecretMapper()),
			builder.WithPredicates(NodeReadyForUpdate()),
		).
		WithOptions(controller.Options{MaxConcurrentReconciles: 1}).
		Complete(r)
}

// SecretPredicate returns the predicate for Secret events.
func (r *Reconciler) SecretPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(_ event.CreateEvent) bool { return true },
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldSecret, ok := e.ObjectOld.(*corev1.Secret)
			if !ok {
				return false
			}

			newSecret, ok := e.ObjectNew.(*corev1.Secret)
			if !ok {
				return false
			}

			return !bytes.Equal(oldSecret.Data[nodeagentconfigv1alpha1.DataKeyOperatingSystemConfig], newSecret.Data[nodeagentconfigv1alpha1.DataKeyOperatingSystemConfig])
		},
		DeleteFunc:  func(_ event.DeleteEvent) bool { return false },
		GenericFunc: func(_ event.GenericEvent) bool { return false },
	}
}

func reconcileRequest(obj client.Object) reconcile.Request {
	return reconcile.Request{NamespacedName: types.NamespacedName{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
	}}
}

// EnqueueWithJitterDelay returns handler.Funcs which enqueues the object with a random jitter duration for 'update'
// events. 'Create' events are enqueued immediately.
func (r *Reconciler) EnqueueWithJitterDelay(ctx context.Context, log logr.Logger) handler.EventHandler {
	delay := delayer{
		log:    log,
		client: r.Client,
	}

	return &handler.Funcs{
		CreateFunc: func(_ context.Context, evt event.CreateEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			if evt.Object == nil {
				return
			}
			q.Add(reconcileRequest(evt.Object))
		},

		UpdateFunc: func(_ context.Context, evt event.UpdateEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			oldSecret, ok := evt.ObjectOld.(*corev1.Secret)
			if !ok {
				return
			}
			newSecret, ok := evt.ObjectNew.(*corev1.Secret)
			if !ok {
				return
			}

			if !bytes.Equal(oldSecret.Data[nodeagentconfigv1alpha1.DataKeyOperatingSystemConfig], newSecret.Data[nodeagentconfigv1alpha1.DataKeyOperatingSystemConfig]) {
				duration := delay.fetch(ctx, r.NodeName)
				log.Info("Enqueued secret with operating system config with a jitter period", "duration", duration)
				q.AddAfter(reconcileRequest(evt.ObjectNew), duration)
			}
		},
	}
}

// NodeToSecretMapper returns a mapper that returns requests for a secret based on its node.
func NodeToSecretMapper() handler.MapFunc {
	return func(_ context.Context, obj client.Object) []reconcile.Request {
		node, ok := obj.(*corev1.Node)
		if !ok {
			return nil
		}

		secretName, ok := node.Labels[v1beta1constants.LabelWorkerPoolGardenerNodeAgentSecretName]
		if !ok {
			return nil
		}

		return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: secretName, Namespace: metav1.NamespaceSystem}}}
	}
}

// NodeReadyForUpdate returns a predicate that returns
// - true for Create event if the new node has the InPlaceUpdate condition with the reason ReadyForUpdate.
// - true for Update event if the new node has the InPlaceUpdate condition with the reason ReadyForUpdate and old node doesn't.
// - false for Delete and Generic events.
func NodeReadyForUpdate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			node, ok := e.Object.(*corev1.Node)
			if !ok {
				return false
			}

			return nodeHasInPlaceUpdateConditionWithReasonReadyForUpdate(node.Status.Conditions)
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			old, ok := e.ObjectOld.(*corev1.Node)
			if !ok {
				return false
			}
			new, ok := e.ObjectNew.(*corev1.Node)
			if !ok {
				return false
			}

			return !nodeHasInPlaceUpdateConditionWithReasonReadyForUpdate(old.Status.Conditions) && nodeHasInPlaceUpdateConditionWithReasonReadyForUpdate(new.Status.Conditions)
		},
		DeleteFunc: func(_ event.DeleteEvent) bool {
			return false
		},
		GenericFunc: func(_ event.GenericEvent) bool {
			return false
		},
	}
}

func nodeHasInPlaceUpdateConditionWithReasonReadyForUpdate(conditions []corev1.NodeCondition) bool {
	for _, condition := range conditions {
		if condition.Type == machinev1alpha1.NodeInPlaceUpdate && condition.Reason == machinev1alpha1.ReadyForUpdate {
			return true
		}
	}
	return false
}

type delayer struct {
	log    logr.Logger
	client client.Client

	delay time.Duration
}

func (d *delayer) fetch(ctx context.Context, nodeName string) time.Duration {
	if nodeName == "" {
		return 0
	}

	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: nodeName}}
	if err := d.client.Get(ctx, client.ObjectKeyFromObject(node), node); err != nil {
		d.log.Error(err, "Failed to read node for getting reconciliation delay, falling back to previously fetched delay", "nodeName", nodeName)
		return d.delay
	}

	v, ok := node.Annotations[v1beta1constants.AnnotationNodeAgentReconciliationDelay]
	if !ok {
		d.log.Info("Node has no reconciliation delay annotation, falling back to previously fetched delay", "nodeName", nodeName)
		return d.delay
	}

	delay, err := time.ParseDuration(v)
	if err != nil {
		d.log.Error(err, "Failed parsing reconciliation delay annotation to time.Duration, falling back to previously fetched delay", "nodeName", nodeName, "annotationValue", v)
		return d.delay
	}

	d.delay = delay
	return d.delay
}
