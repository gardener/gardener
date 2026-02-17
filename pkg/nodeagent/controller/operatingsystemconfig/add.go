// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package operatingsystemconfig

import (
	"bytes"
	"context"
	"fmt"
	"time"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/go-logr/logr"
	"github.com/spf13/afero"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/clock"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	nodeagentconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/nodeagent/v1alpha1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/controllerutils"
	predicateutils "github.com/gardener/gardener/pkg/controllerutils/predicate"
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
	if r.Clock == nil {
		r.Clock = clock.RealClock{}
	}
	if r.Recorder == nil {
		r.Recorder = mgr.GetEventRecorder(ControllerName)
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

	log := mgr.GetLogger().WithValues("controller", ControllerName)
	controller := builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		Watches(
			&corev1.Secret{},
			r.EnqueueWithJitterDelay(ctx, log.WithName("reconciliation-delayer")),
			builder.WithPredicates(r.SecretPredicate(), predicateutils.ForEventTypes(predicateutils.Create, predicateutils.Update)),
		).
		Watches(
			&corev1.Node{},
			handler.EnqueueRequestsFromMapFunc(r.NodeToSecretMapper()),
			builder.WithPredicates(r.NodeReadyForInPlaceUpdate()),
		).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 1,
			ReconciliationTimeout:   controllerutils.DefaultReconciliationTimeout,
		})

	if r.NodeName != "" {
		// In case the OperatingSystemConfig reconciliation is serial, we require another Lease object for leader election
		// among the responsible gardener-node-agent instances. We don't use the Lease cache from the mgr because this is
		// already restricted in `cmd/gardener-node-agent/app.go` to the health check Lease object of the Node the
		// gardener-node-agent is responsible for.
		// It is not possible to add another field selector for the leader election Lease (multiple field selectors are
		// AND-ed). Completely lifting the field selector for Leases in the mgr would result in too many unrelated update
		// calls for all gardener-node-agent instances (and thus, increased network I/O).
		log.Info("Setting up cluster object for serial reconciliation leader election Lease cache")
		cluster, err := cluster.New(mgr.GetConfig(), func(opts *cluster.Options) {
			opts.Scheme = mgr.GetScheme()
			opts.Logger = log.WithName("serial-reconciliation-leader-election-cluster")
			opts.Cache.ByObject = map[client.Object]cache.ByObject{&coordinationv1.Lease{}: {
				Namespaces: map[string]cache.Config{metav1.NamespaceSystem: {}},
				Field:      fields.SelectorFromSet(fields.Set{metav1.ObjectNameField: r.Config.SecretName}),
			}}
		})
		if err != nil {
			return fmt.Errorf("could not instantiate cluster for serial reconciliation leader election Lease cache: %w", err)
		}
		if err := mgr.Add(cluster); err != nil {
			return fmt.Errorf("could not add cluster for serial reconciliation leader election Lease cache to manager: %w", err)
		}
		if r.LeaseClient == nil {
			r.LeaseClient = cluster.GetClient()
		}

		controller = controller.WatchesRawSource(source.Kind[client.Object](cluster.GetCache(),
			&coordinationv1.Lease{},
			handler.EnqueueRequestsFromMapFunc(r.LeaseToSecretMapper()),
			predicateutils.HasName(r.Config.SecretName), r.LeasePredicate(ctx, log.WithName("leader-election-lease-predicate")),
		))
	}

	return controller.Complete(r)
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

// LeasePredicate returns the predicate for Lease events. It only reacts on 'Update' events and returns true when both
// of the following conditions are met:
// - Lease was just released by another instance (i.e., it is free to be claimed)
// - Node is not up-to-date with the latest OperatingSystemConfig
func (r *Reconciler) LeasePredicate(ctx context.Context, log logr.Logger) predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldLease, ok := e.ObjectOld.(*coordinationv1.Lease)
			if !ok {
				return false
			}

			newLease, ok := e.ObjectNew.(*coordinationv1.Lease)
			if !ok {
				return false
			}

			leaseReleasedByOtherInstances := func() bool {
				return ptr.Deref(oldLease.Spec.HolderIdentity, "") != r.HostName &&
					newLease.Spec.HolderIdentity == nil
			}

			nodeIsUpToDate := func() bool {
				secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: newLease.GetName(), Namespace: newLease.GetNamespace()}}
				if err := r.Client.Get(ctx, client.ObjectKeyFromObject(secret), secret); err != nil {
					log.Error(err, "Failed to get secret containing OperatingSystemConfig, assuming node is outdated")
					return false
				}

				node, _, err := r.getNode(ctx)
				if err != nil {
					log.Error(err, "Failed to get node, assuming it is outdated")
					return false
				}
				if node == nil {
					return false
				}

				return secret.Annotations[nodeagentconfigv1alpha1.AnnotationKeyChecksumDownloadedOperatingSystemConfig] == node.Annotations[nodeagentconfigv1alpha1.AnnotationKeyChecksumAppliedOperatingSystemConfig]
			}

			return leaseReleasedByOtherInstances() && !nodeIsUpToDate()
		},
		CreateFunc:  func(_ event.CreateEvent) bool { return false },
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

// EnqueueWithJitterDelay returns handler.Funcs which enqueues the object with a random jitter duration for 'Update'
// events. 'Create' events are enqueued immediately. If the reconciliation is serial, also 'Update' events are enqueued
// immediately.
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

			if serialReconciliation(newSecret) {
				q.Add(reconcileRequest(evt.ObjectNew))
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

// LeaseToSecretMapper returns a mapper that returns requests for a secret based on a leader election Lease.
func (r *Reconciler) LeaseToSecretMapper() handler.MapFunc {
	return func(_ context.Context, obj client.Object) []reconcile.Request {
		return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: obj.GetName(), Namespace: obj.GetNamespace()}}}
	}
}

// NodeToSecretMapper returns a mapper that returns requests for a secret based on its node.
func (r *Reconciler) NodeToSecretMapper() handler.MapFunc {
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

// NodeReadyForInPlaceUpdate returns a predicate that returns
// - true for Create event if the new node has the InPlaceUpdate condition with the reason ReadyForUpdate.
// - true for Update event if the new node has the InPlaceUpdate condition with the reason ReadyForUpdate and old node doesn't.
// - false for Delete and Generic events.
func (r *Reconciler) NodeReadyForInPlaceUpdate() predicate.Predicate {
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
