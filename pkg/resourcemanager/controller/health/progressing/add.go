// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package progressing

import (
	"context"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/clock"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils/mapper"
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

	c, err := controller.New(
		ControllerName,
		mgr,
		controller.Options{
			Reconciler:              r,
			MaxConcurrentReconciles: pointer.IntDeref(r.Config.ConcurrentSyncs, 0),
		},
	)
	if err != nil {
		return err
	}

	b := builder.
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
			MaxConcurrentReconciles: pointer.IntDeref(r.Config.ConcurrentSyncs, 0),
		})

	for resource, obj := range map[string]client.Object{
		"deployments":  &appsv1.Deployment{},
		"statefulsets": &appsv1.StatefulSet{},
		"daemonsets":   &appsv1.DaemonSet{},
	} {
		gvr := schema.GroupVersionResource{Group: appsv1.SchemeGroupVersion.Group, Version: appsv1.SchemeGroupVersion.Version, Resource: resource}

		if _, err := targetCluster.GetRESTMapper().KindFor(gvr); err != nil {
			if !meta.IsNoMatchError(err) {
				return err
			}
			mgr.GetLogger().Info("Resource is not available/enabled API of the target cluster, skip adding watches", "gvr", gvr)
			continue
		}

		b = b.WatchesRawSource(
			source.Kind(targetCluster.GetCache(), obj),
			mapper.EnqueueRequestsFrom(ctx, mgr.GetCache(), utils.MapToOriginManagedResource(clusterID), mapper.UpdateWithNew, c.GetLogger()),
			builder.WithPredicates(r.ProgressingStatusChanged(ctx)),
		)

		if resource == "deployments" {
			// Watch relevant objects for Progressing condition in order to immediately update the condition as soon as
			// there is a change on managed resources.
			pod := &metav1.PartialObjectMetadata{}
			pod.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Pod"))

			b = b.WatchesRawSource(
				source.Kind(targetCluster.GetCache(), pod),
				mapper.EnqueueRequestsFrom(ctx, mgr.GetCache(), r.MapPodToDeploymentToOriginManagedResource(clusterID), mapper.UpdateWithNew, c.GetLogger()),
				builder.WithPredicates(predicateutils.ForEventTypes(predicateutils.Create, predicateutils.Delete)),
			)
		}
	}

	return b.Complete(r)
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

// MapPodToDeploymentToOriginManagedResource is a mapper.MapFunc for pods to their origin Deployment and origin
// ManagedResource.
func (r *Reconciler) MapPodToDeploymentToOriginManagedResource(clusterID string) mapper.MapFunc {
	return func(ctx context.Context, log logr.Logger, reader client.Reader, obj client.Object) []reconcile.Request {
		deployment, err := kubernetesutils.GetDeploymentForPod(ctx, reader, obj.GetNamespace(), obj.GetOwnerReferences())
		if err != nil {
			log.Error(err, "Failed getting Deployment for Pod", "pod", client.ObjectKeyFromObject(obj))
			return nil
		}

		if deployment == nil {
			return nil
		}

		return utils.MapToOriginManagedResource(clusterID)(ctx, log, reader, deployment)
	}
}
