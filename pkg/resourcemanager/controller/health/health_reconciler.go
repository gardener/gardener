// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package health

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	resourcemanagerpredicate "github.com/gardener/gardener/pkg/resourcemanager/predicate"
)

type reconciler struct {
	client       client.Client
	targetClient client.Client
	targetScheme *runtime.Scheme
	classFilter  *resourcemanagerpredicate.ClassFilter
	syncPeriod   time.Duration

	// EnsureWatchForGVK ensures that the controller is watching the given object to reconcile corresponding
	// ManagedResources on health status changes.
	EnsureWatchForGVK func(gvk schema.GroupVersionKind, obj client.Object) error
}

// InjectClient injects a client into the reconciler.
func (r *reconciler) InjectClient(c client.Client) error {
	r.client = c
	return nil
}

// Reconcile performs health checks.
func (r *reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// timeout for all calls (e.g. status updates), give status updates a bit of headroom if health checks
	// themselves run into timeouts, so that we will still update the status with that timeout error
	var cancel context.CancelFunc
	ctx, cancel = context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	mr := &resourcesv1alpha1.ManagedResource{}
	if err := r.client.Get(ctx, req.NamespacedName, mr); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	if isIgnored(mr) {
		log.Info("Skipping health checks since ManagedResource is ignored")
		return ctrl.Result{}, nil
	}

	// Check responsibility
	if _, responsible := r.classFilter.Active(mr); !responsible {
		log.Info("Stopping health checks as the responsibility changed")
		return ctrl.Result{}, nil
	}

	if !mr.DeletionTimestamp.IsZero() {
		log.Info("Stopping health checks for ManagedResource, as it is marked for deletion")
		return ctrl.Result{}, nil
	}

	// skip health checks until ManagedResource has been reconciled completely successfully to prevent writing
	// falsy health condition (resources may need a second try to apply, e.g. CRDs and CRs in the same MR)
	conditionResourcesApplied := v1beta1helper.GetCondition(mr.Status.Conditions, resourcesv1alpha1.ResourcesApplied)
	if conditionResourcesApplied == nil || conditionResourcesApplied.Status == gardencorev1beta1.ConditionProgressing || conditionResourcesApplied.Status == gardencorev1beta1.ConditionFalse {
		log.Info("Skipping health checks for ManagedResource, as it is has not been reconciled successfully yet")
		return ctrl.Result{RequeueAfter: r.syncPeriod}, nil
	}

	return r.executeHealthChecks(ctx, log, mr)
}

func (r *reconciler) executeHealthChecks(ctx context.Context, log logr.Logger, mr *resourcesv1alpha1.ManagedResource) (ctrl.Result, error) {
	log.Info("Starting ManagedResource health checks")
	// don't block workers if calls timeout for some reason
	healthCheckCtx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	var (
		conditionResourcesHealthy = v1beta1helper.GetOrInitCondition(mr.Status.Conditions, resourcesv1alpha1.ResourcesHealthy)
		oldCondition              = conditionResourcesHealthy.DeepCopy()
	)

	for _, ref := range mr.Status.Resources {
		var (
			objectGVK = ref.GroupVersionKind()
			objectKey = client.ObjectKey{Namespace: ref.Namespace, Name: ref.Name}
			objectLog = log.WithValues("object", objectKey, "objectGVK", objectGVK)
		)

		obj, err := newObjectForHealthCheck(objectLog, r.targetScheme, objectGVK)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to construct new object for reference: %w", err)
		}

		// ensure watch is started for object
		if err := r.EnsureWatchForGVK(objectGVK, obj); err != nil {
			return ctrl.Result{}, err
		}

		if err := r.targetClient.Get(healthCheckCtx, objectKey, obj); err != nil {
			if !apierrors.IsNotFound(err) && !meta.IsNoMatchError(err) {
				return ctrl.Result{}, err
			}

			var (
				reason  = ref.Kind + "Missing"
				message = fmt.Sprintf("Required %s %q in namespace %q is missing", ref.Kind, ref.Name, ref.Namespace)
			)
			if meta.IsNoMatchError(err) {
				message = fmt.Sprintf("%s: %v", message, err)
			}
			objectLog.Info("Finished ManagedResource health checks", "status", "unhealthy", "reason", reason, "message", message)

			conditionResourcesHealthy = v1beta1helper.UpdatedCondition(conditionResourcesHealthy, gardencorev1beta1.ConditionFalse, reason, message)
			if err := updateConditions(ctx, r.client, mr, conditionResourcesHealthy); err != nil {
				return ctrl.Result{}, fmt.Errorf("could not update the ManagedResource status: %w", err)
			}

			return ctrl.Result{RequeueAfter: r.syncPeriod}, nil

		}

		if checked, err := CheckHealth(obj); err != nil {
			var (
				reason  = ref.Kind + "Unhealthy"
				message = fmt.Sprintf("%s %q is unhealthy: %v", ref.Kind, objectKey.String(), err)
			)

			if checked {
				// consult object's events for more information if sensible
				additionalMessage, err := FetchAdditionalFailureMessage(ctx, r.targetClient, obj)
				if err != nil {
					objectLog.Error(err, "Failed to read events for more information about unhealthy object")
				} else if additionalMessage != "" {
					message += "\n\n" + additionalMessage
				}

				objectLog.Info("Finished ManagedResource health checks", "status", "unhealthy", "reason", reason, "message", message)
			} else {
				// there was an error executing the health check (which is different from a failed health check)
				// handle it separately and log it prominently
				reason = "HealthCheckError"
				message = fmt.Sprintf("Error executing health check for %s %q: %v", ref.Kind, objectKey.String(), err)
				objectLog.Error(err, "Error executing health check for object")
			}

			conditionResourcesHealthy = v1beta1helper.UpdatedCondition(conditionResourcesHealthy, gardencorev1beta1.ConditionFalse, reason, message)
			if err := updateConditions(ctx, r.client, mr, conditionResourcesHealthy); err != nil {
				return ctrl.Result{}, fmt.Errorf("could not update the ManagedResource status: %w", err)
			}

			return ctrl.Result{RequeueAfter: r.syncPeriod}, nil
		}
	}

	conditionResourcesHealthy = v1beta1helper.UpdatedCondition(conditionResourcesHealthy, gardencorev1beta1.ConditionTrue, "ResourcesHealthy", "All resources are healthy.")
	if !apiequality.Semantic.DeepEqual(oldCondition, conditionResourcesHealthy) {
		if err := updateConditions(ctx, r.client, mr, conditionResourcesHealthy); err != nil {
			return ctrl.Result{}, fmt.Errorf("could not update the ManagedResource status: %w", err)
		}
	}

	log.Info("Finished ManagedResource health checks", "status", "healthy")
	return ctrl.Result{RequeueAfter: r.syncPeriod}, nil
}

func newObjectForHealthCheck(log logr.Logger, scheme *runtime.Scheme, gvk schema.GroupVersionKind) (client.Object, error) {
	// Create a typed object if GVK is registered in scheme. This object will be fully watched in the target cluster.
	// If we don't know the GVK, we definitely don't have a dedicated health check for it.
	// I.e., we only care about whether the object is present or not.
	// Hence, we can use metadata-only requests/watches instead of watching the entire object, which saves bandwidth and
	// memory.
	// If the target cache is disabled, no watches will be started.
	typedObject, err := scheme.New(gvk)
	if err != nil {
		if !runtime.IsNotRegisteredError(err) {
			return nil, err
		}

		log.V(1).Info("Falling back to metadata-only object for health checks (not registered in the target scheme)", "groupVersionKind", gvk, "err", err.Error())
		obj := &metav1.PartialObjectMetadata{}
		obj.SetGroupVersionKind(gvk)
		return obj, nil
	}

	return typedObject.(client.Object), nil
}

func updateConditions(ctx context.Context, c client.Client, mr *resourcesv1alpha1.ManagedResource, condition gardencorev1beta1.Condition) error {
	mr.Status.Conditions = v1beta1helper.MergeConditions(mr.Status.Conditions, condition)
	return c.Status().Update(ctx, mr)
}
