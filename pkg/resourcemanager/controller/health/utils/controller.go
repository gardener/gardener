// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"context"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	resourcesv1alpha1helper "github.com/gardener/gardener/pkg/apis/resources/v1alpha1/helper"
)

// HealthStatusChanged returns a predicate that filters for events that indicate a change in the object's health status.
func HealthStatusChanged(log logr.Logger) predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return e.Object.GetAnnotations()[resourcesv1alpha1.SkipHealthCheck] != "true"
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return e.Object.GetAnnotations()[resourcesv1alpha1.SkipHealthCheck] != "true"
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			if e.ObjectOld.GetResourceVersion() == e.ObjectNew.GetResourceVersion() {
				// periodic cache resync, enqueue
				return true
			}

			var oldHealthy, newHealthy bool
			checked, oldErr := CheckHealth(e.ObjectOld)
			if !checked {
				if oldErr != nil {
					log.Error(oldErr, "Error determining health status of old object", "object", e.ObjectOld)
				}
				return false
			}
			oldHealthy = oldErr != nil

			checked, newErr := CheckHealth(e.ObjectNew)
			if !checked {
				if newErr != nil {
					log.Error(newErr, "Error determining health status of new object", "object", e.ObjectNew)
				}
				return false
			}
			newHealthy = newErr != nil

			return oldHealthy != newHealthy
		},
		GenericFunc: func(event.GenericEvent) bool {
			return false
		},
	}
}

// MapToOriginManagedResource is a handler.MapFunc for resources to their origin ManagedResource.
func MapToOriginManagedResource(log logr.Logger, clusterID string) handler.MapFunc {
	return func(_ context.Context, obj client.Object) []reconcile.Request {
		origin, ok := obj.GetAnnotations()[resourcesv1alpha1.OriginAnnotation]
		if !ok {
			return nil
		}

		originClusterID, key, err := resourcesv1alpha1helper.SplitOrigin(origin)
		if err != nil {
			log.Error(err, "Failed to parse origin of object", "object", obj, "origin", origin)
			return nil
		}

		if originClusterID != clusterID {
			// object isn't managed by this resource-manager instance
			return nil
		}

		return []reconcile.Request{{NamespacedName: key}}
	}
}
