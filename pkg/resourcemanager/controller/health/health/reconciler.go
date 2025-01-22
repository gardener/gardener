// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package health

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
	resourcemanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/resourcemanager/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/health/utils"
	resourcemanagerpredicate "github.com/gardener/gardener/pkg/resourcemanager/predicate"
)

// Reconciler performs health checks for resources managed as part of ManagedResources.
type Reconciler struct {
	SourceClient client.Client
	TargetClient client.Client
	TargetScheme *runtime.Scheme
	Config       resourcemanagerconfigv1alpha1.HealthControllerConfig
	Clock        clock.Clock
	ClassFilter  *resourcemanagerpredicate.ClassFilter

	// ensureWatchForGVK ensures that the controller is watching the given object to reconcile corresponding
	// ManagedResources on health status changes.
	ensureWatchForGVK func(gvk schema.GroupVersionKind, obj client.Object) error
}

// Reconcile performs the health checks.
func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	// timeout for all calls (e.g. status updates), give status updates a bit of headroom if health checks
	// themselves run into timeouts, so that we will still update the status with that timeout error
	var cancel context.CancelFunc
	ctx, cancel = controllerutils.GetMainReconciliationContext(ctx, r.Config.SyncPeriod.Duration)
	defer cancel()

	mr := &resourcesv1alpha1.ManagedResource{}
	if err := r.SourceClient.Get(ctx, req.NamespacedName, mr); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	if utils.IsIgnored(mr) {
		log.Info("Skipping health checks since ManagedResource is ignored")
		return reconcile.Result{}, nil
	}

	// Check responsibility
	if responsible := r.ClassFilter.Responsible(mr); !responsible {
		log.Info("Stopping health checks as the responsibility changed")
		return reconcile.Result{}, nil
	}

	if !mr.DeletionTimestamp.IsZero() {
		log.Info("Stopping health checks for ManagedResource, as it is marked for deletion")
		return reconcile.Result{}, nil
	}

	// skip health checks until ManagedResource has been reconciled completely successfully to prevent writing
	// falsy health condition (resources may need a second try to apply, e.g. CRDs and CRs in the same MR)
	conditionResourcesApplied := v1beta1helper.GetCondition(mr.Status.Conditions, resourcesv1alpha1.ResourcesApplied)
	if conditionResourcesApplied == nil || conditionResourcesApplied.Status == gardencorev1beta1.ConditionProgressing || conditionResourcesApplied.Status == gardencorev1beta1.ConditionFalse {
		log.Info("Skipping health checks for ManagedResource, as it is has not been reconciled successfully yet")
		return reconcile.Result{RequeueAfter: r.Config.SyncPeriod.Duration}, nil
	}

	return r.executeHealthChecks(ctx, log, mr)
}

func (r *Reconciler) executeHealthChecks(ctx context.Context, log logr.Logger, mr *resourcesv1alpha1.ManagedResource) (reconcile.Result, error) {
	log.Info("Starting ManagedResource health checks")
	// don't block workers if calls timeout for some reason
	healthCheckCtx, cancel := controllerutils.GetChildReconciliationContext(ctx, r.Config.SyncPeriod.Duration)
	defer cancel()

	var (
		conditionResourcesHealthy = v1beta1helper.GetOrInitConditionWithClock(r.Clock, mr.Status.Conditions, resourcesv1alpha1.ResourcesHealthy)
		oldCondition              = conditionResourcesHealthy.DeepCopy()
	)

	for _, ref := range mr.Status.Resources {
		var (
			objectGVK = ref.GroupVersionKind()
			objectKey = client.ObjectKey{Namespace: ref.Namespace, Name: ref.Name}
			objectLog = log.WithValues("object", objectKey, "objectGVK", objectGVK)
		)

		obj, err := newObjectForHealthCheck(objectLog, r.TargetScheme, objectGVK)
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to construct new object for reference: %w", err)
		}

		// ensure watch is started for object
		if err := r.ensureWatchForGVK(objectGVK, obj); err != nil {
			return reconcile.Result{}, err
		}

		if err := r.TargetClient.Get(healthCheckCtx, objectKey, obj); err != nil {
			if !apierrors.IsNotFound(err) && !meta.IsNoMatchError(err) {
				return reconcile.Result{}, err
			}

			var (
				reason  = ref.Kind + "Missing"
				message = fmt.Sprintf("Required %s %q in namespace %q is missing", ref.Kind, ref.Name, ref.Namespace)
			)
			if meta.IsNoMatchError(err) {
				message = fmt.Sprintf("%s: %v", message, err)
			}
			objectLog.Info("Finished ManagedResource health checks", "status", "unhealthy", "reason", reason, "message", message)

			conditionResourcesHealthy = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditionResourcesHealthy, gardencorev1beta1.ConditionFalse, reason, message)
			mr.Status.Conditions = v1beta1helper.MergeConditions(mr.Status.Conditions, conditionResourcesHealthy)
			if err := r.SourceClient.Status().Update(ctx, mr); err != nil {
				return reconcile.Result{}, fmt.Errorf("could not update the ManagedResource status: %w", err)
			}

			return reconcile.Result{RequeueAfter: r.Config.SyncPeriod.Duration}, nil
		}

		if checked, err := utils.CheckHealth(obj); err != nil {
			var (
				reason  = ref.Kind + "Unhealthy"
				message = fmt.Sprintf("%s %q is unhealthy: %v", ref.Kind, objectKey.String(), err)
			)

			if checked {
				// consult object's events for more information if sensible
				additionalMessage, err := utils.FetchAdditionalFailureMessage(ctx, r.TargetClient, obj)
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

			conditionResourcesHealthy = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditionResourcesHealthy, gardencorev1beta1.ConditionFalse, reason, message)
			mr.Status.Conditions = v1beta1helper.MergeConditions(mr.Status.Conditions, conditionResourcesHealthy)
			if err := r.SourceClient.Status().Update(ctx, mr); err != nil {
				return reconcile.Result{}, fmt.Errorf("could not update the ManagedResource status: %w", err)
			}

			return reconcile.Result{RequeueAfter: r.Config.SyncPeriod.Duration}, nil
		}
	}

	conditionResourcesHealthy = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditionResourcesHealthy, gardencorev1beta1.ConditionTrue, "ResourcesHealthy", "All resources are healthy.")
	if !apiequality.Semantic.DeepEqual(oldCondition, conditionResourcesHealthy) {
		mr.Status.Conditions = v1beta1helper.MergeConditions(mr.Status.Conditions, conditionResourcesHealthy)
		if err := r.SourceClient.Status().Update(ctx, mr); err != nil {
			return reconcile.Result{}, fmt.Errorf("could not update the ManagedResource status: %w", err)
		}
	}

	log.Info("Finished ManagedResource health checks", "status", "healthy")
	return reconcile.Result{RequeueAfter: r.Config.SyncPeriod.Duration}, nil
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
