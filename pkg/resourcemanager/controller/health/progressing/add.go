// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package progressing

import (
	"context"

	certv1alpha1 "github.com/gardener/cert-management/pkg/apis/cert/v1alpha1"
	"github.com/go-logr/logr"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/clock"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	predicateutils "github.com/gardener/gardener/pkg/controllerutils/predicate"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/health/utils"
	resourcemanagerpredicate "github.com/gardener/gardener/pkg/resourcemanager/predicate"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// ControllerName is the name of the controller.
const ControllerName = "health-progressing"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(ctx context.Context, mgr manager.Manager, sourceCluster, targetCluster cluster.Cluster, clusterID string) error {
	if r.SourceClient == nil {
		r.SourceClient = sourceCluster.GetClient()
	}
	if r.TargetClient == nil {
		r.TargetClient = targetCluster.GetClient()
	}
	if r.Clock == nil {
		r.Clock = clock.RealClock{}
	}

	c, err := builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		For(&resourcesv1alpha1.ManagedResource{}, builder.WithPredicates(
			predicate.Or(
				resourcemanagerpredicate.ClassChangedPredicate(),
				// start health checks immediately after MR has been reconciled
				resourcemanagerpredicate.ConditionStatusChanged(resourcesv1alpha1.ResourcesApplied, resourcemanagerpredicate.DefaultConditionChange),
				resourcemanagerpredicate.NoLongerIgnored(),
			),
			resourcemanagerpredicate.NotIgnored(),
			r.ClassFilter,
		)).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: ptr.Deref(r.Config.ConcurrentSyncs, 0),
		}).
		Build(r)
	if err != nil {
		return err
	}

	for resource, obj := range map[string]client.Object{
		"deployments":   &appsv1.Deployment{},
		"statefulsets":  &appsv1.StatefulSet{},
		"daemonsets":    &appsv1.DaemonSet{},
		"prometheuses":  &monitoringv1.Prometheus{},
		"alertmanagers": &monitoringv1.Alertmanager{},
		"certificates":  &certv1alpha1.Certificate{},
		"issuers":       &certv1alpha1.Issuer{},
	} {
		gvr := schema.GroupVersionResource{Group: appsv1.SchemeGroupVersion.Group, Version: appsv1.SchemeGroupVersion.Version, Resource: resource}

		if _, err := targetCluster.GetRESTMapper().KindFor(gvr); err != nil {
			if !meta.IsNoMatchError(err) {
				return err
			}
			c.GetLogger().Info("Resource is not available/enabled API of the target cluster, skip adding watches", "gvr", gvr)

			continue
		}

		if err := c.Watch(source.Kind[client.Object](
			targetCluster.GetCache(),
			obj,
			handler.EnqueueRequestsFromMapFunc(utils.MapToOriginManagedResource(c.GetLogger(), clusterID)),
			r.ProgressingStatusChanged(ctx),
		)); err != nil {
			return err
		}

		if resource == "deployments" {
			// Watch relevant objects for Progressing condition in order to immediately update the condition as soon as
			// there is a change on managed resources.
			pod := &metav1.PartialObjectMetadata{}
			pod.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Pod"))

			if err := c.Watch(source.Kind[client.Object](
				targetCluster.GetCache(),
				pod,
				handler.EnqueueRequestsFromMapFunc(r.MapPodToDeploymentToOriginManagedResource(c.GetLogger(), clusterID)),
				predicateutils.ForEventTypes(predicateutils.Create, predicateutils.Delete),
			)); err != nil {
				return err
			}
		}
	}

	return nil
}

// ProgressingStatusChanged returns a predicate that filters for events that indicate a change in the object's
// progressing status.
func (r *Reconciler) ProgressingStatusChanged(ctx context.Context) predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(_ event.CreateEvent) bool { return false },
		UpdateFunc: func(e event.UpdateEvent) bool {
			if e.ObjectOld.GetResourceVersion() == e.ObjectNew.GetResourceVersion() {
				// periodic cache resync, enqueue
				return true
			}

			oldProgressing, _, _ := r.checkProgressing(ctx, e.ObjectOld)
			newProgressing, _, _ := r.checkProgressing(ctx, e.ObjectNew)

			return oldProgressing != newProgressing
		},
		DeleteFunc:  func(_ event.DeleteEvent) bool { return false },
		GenericFunc: func(_ event.GenericEvent) bool { return false },
	}
}

// MapPodToDeploymentToOriginManagedResource is a handler.MapFunc for pods to their origin Deployment and origin
// ManagedResource.
func (r *Reconciler) MapPodToDeploymentToOriginManagedResource(log logr.Logger, clusterID string) handler.MapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		deployment, err := kubernetesutils.GetDeploymentForPod(ctx, r.TargetClient, obj.GetNamespace(), obj.GetOwnerReferences())
		if err != nil {
			log.Error(err, "Failed getting Deployment for Pod", "pod", client.ObjectKeyFromObject(obj))
			return nil
		}

		if deployment == nil {
			return nil
		}

		return utils.MapToOriginManagedResource(log, clusterID)(ctx, deployment)
	}
}
