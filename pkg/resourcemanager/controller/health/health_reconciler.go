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

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/resourcemanager/predicate"

	"github.com/go-logr/logr"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type reconciler struct {
	client       client.Client
	targetClient client.Client
	targetScheme *runtime.Scheme
	classFilter  *predicate.ClassFilter
	syncPeriod   time.Duration
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
			log.Info("Stopping health checks for ManagedResource, as it has been deleted")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("could not fetch ManagedResource: %w", err)
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
		resourcesObjectReferences = mr.Status.Resources
	)

	for _, ref := range resourcesObjectReferences {
		var (
			objectKey = client.ObjectKey{Namespace: ref.Namespace, Name: ref.Name}
			objectLog = log.WithValues("object", objectKey, "objectGVK", ref.GroupVersionKind())
		)

		obj, err := newObjectForReference(objectLog, r.targetScheme, ref)
		if err != nil {
			log.Error(err, "Failed to construct new object for reference. This should never happen, ignoring")
			return ctrl.Result{}, nil
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

		if checked, err := CheckHealth(healthCheckCtx, r.targetClient, obj); err != nil {
			var (
				reason  = ref.Kind + "Unhealthy"
				message = fmt.Sprintf("%s %q is unhealthy: %v", ref.Kind, objectKey.String(), err)
			)

			if !checked {
				// there was an error executing the health check (which is different than a failed health check)
				// handle it separately and log it prominently
				reason = "HealthCheckError"
				message = fmt.Sprintf("Error executing health check for %s %q: %v", ref.Kind, objectKey.String(), err)
				objectLog.Error(err, "Error executing health check for object")
			} else {
				objectLog.Info("Finished ManagedResource health checks", "status", "unhealthy", "reason", reason, "message", message)
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

func newObjectForReference(log logr.Logger, scheme *runtime.Scheme, ref resourcesv1alpha1.ObjectReference) (client.Object, error) {
	// sigs.k8s.io/controller-runtime/pkg/client.DelegatingReader does not use the cache for unstructured.Unstructured
	// objects, so we create a new object of the object's type to use the caching client
	typedObject, err := scheme.New(ref.GroupVersionKind())
	if err != nil {
		log.Info("Could not create new object of kind for health checks (probably not registered in the used scheme), falling back to unstructured request", "err", err.Error())

		// fallback to unstructured requests if the object's type is not registered in the scheme
		unstructuredObj := &unstructured.Unstructured{}
		unstructuredObj.SetAPIVersion(ref.APIVersion)
		unstructuredObj.SetKind(ref.Kind)
		return unstructuredObj, nil
	}

	if obj, ok := typedObject.(client.Object); ok {
		return obj, nil
	}

	return nil, fmt.Errorf("expected client.Object but got %T", typedObject)
}

func updateConditions(ctx context.Context, c client.Client, mr *resourcesv1alpha1.ManagedResource, condition gardencorev1beta1.Condition) error {
	mr.Status.Conditions = v1beta1helper.MergeConditions(mr.Status.Conditions, condition)
	return c.Status().Update(ctx, mr)
}
