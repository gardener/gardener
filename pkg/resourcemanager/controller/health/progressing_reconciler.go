// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/resourcemanager/predicate"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
)

const progressingReconcilerName = "progressing"

type progressingReconciler struct {
	client       client.Client
	targetClient client.Client
	targetScheme *runtime.Scheme
	classFilter  *predicate.ClassFilter
	syncPeriod   time.Duration
}

func (r *progressingReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	// timeout for all calls (e.g. status updates), give status updates a bit of headroom if checks
	// themselves run into timeouts, so that we will still update the status with that timeout error
	var cancel context.CancelFunc
	ctx, cancel = context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	mr := &resourcesv1alpha1.ManagedResource{}
	if err := r.client.Get(ctx, req.NamespacedName, mr); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Stopping checks for ManagedResource, as it has been deleted")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("could not fetch ManagedResource: %w", err)
	}

	if isIgnored(mr) {
		log.Info("Skipping checks since ManagedResource is ignored")
		return ctrl.Result{}, nil
	}

	// Check responsibility
	if _, responsible := r.classFilter.Active(mr); !responsible {
		log.Info("Stopping checks as the responsibility changed")
		return ctrl.Result{}, nil
	}

	if !mr.DeletionTimestamp.IsZero() {
		log.Info("Stopping checks for ManagedResource as it is marked for deletion")
		return ctrl.Result{}, nil
	}

	// skip checks until ManagedResource has been reconciled completely successfully to prevent updating status while
	// resource controller is still applying the resources (this might lead to wrongful results inconsistent with the
	// actual set of applied resources and causes a myriad of conflicts)
	conditionResourcesApplied := v1beta1helper.GetCondition(mr.Status.Conditions, resourcesv1alpha1.ResourcesApplied)
	if conditionResourcesApplied == nil || conditionResourcesApplied.Status == gardencorev1beta1.ConditionProgressing || conditionResourcesApplied.Status == gardencorev1beta1.ConditionFalse {
		log.Info("Skipping checks for ManagedResource as the resources were not applied yet")
		return ctrl.Result{RequeueAfter: r.syncPeriod}, nil
	}

	return r.reconcile(ctx, log, mr)
}

func (r *progressingReconciler) reconcile(ctx context.Context, log logr.Logger, mr *resourcesv1alpha1.ManagedResource) (ctrl.Result, error) {
	log.V(1).Info("Starting ManagedResource progressing checks")
	// don't block workers if calls timeout for some reason
	checkCtx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	conditionResourcesProgressing := v1beta1helper.GetOrInitCondition(mr.Status.Conditions, resourcesv1alpha1.ResourcesProgressing)

	for _, ref := range mr.Status.Resources {
		// only Deployment, StatefulSet and DaemonSet are considered for Progressing condition
		if ref.GroupVersionKind().Group != appsv1.GroupName {
			continue
		}

		var obj client.Object
		switch ref.Kind {
		case "Deployment":
			obj = &appsv1.Deployment{}
		case "StatefulSet":
			obj = &appsv1.StatefulSet{}
		case "DaemonSet":
			obj = &appsv1.DaemonSet{}
		default:
			continue
		}

		var (
			objectKey = client.ObjectKey{Namespace: ref.Namespace, Name: ref.Name}
			objectLog = log.WithValues("object", objectKey, "objectGVK", ref.GroupVersionKind())
		)

		if err := r.targetClient.Get(checkCtx, objectKey, obj); err != nil {
			if apierrors.IsNotFound(err) {
				// missing objects already handled by health controller, skip
				continue
			}
			return ctrl.Result{}, err
		}

		if progressing, description := CheckProgressing(obj); progressing {
			var (
				reason  = ref.Kind + "Progressing"
				message = fmt.Sprintf("%s %q is progressing: %s", ref.Kind, objectKey.String(), description)
			)

			objectLog.Info("ManagedResource rollout is progressing, detected progressing object", "status", "progressing", "reason", reason, "message", message)

			conditionResourcesProgressing = v1beta1helper.UpdatedCondition(conditionResourcesProgressing, gardencorev1beta1.ConditionTrue, reason, message)
			if err := updateConditions(ctx, r.client, mr, conditionResourcesProgressing); err != nil {
				return ctrl.Result{}, fmt.Errorf("could not update the ManagedResource status: %w", err)
			}

			return ctrl.Result{RequeueAfter: r.syncPeriod}, nil
		}
	}

	b, err := v1beta1helper.NewConditionBuilder(resourcesv1alpha1.ResourcesProgressing)
	if err != nil {
		return ctrl.Result{}, err
	}

	var needsUpdate bool
	conditionResourcesProgressing, needsUpdate = b.WithOldCondition(conditionResourcesProgressing).
		WithStatus(gardencorev1beta1.ConditionFalse).WithReason("ResourcesRolledOut").
		WithMessage("All resources have been fully rolled out.").
		Build()

	if needsUpdate {
		log.Info("ManagedResource has been fully rolled out", "status", "rolled out")
		if err := updateConditions(ctx, r.client, mr, conditionResourcesProgressing); err != nil {
			return ctrl.Result{}, fmt.Errorf("could not update the ManagedResource status: %w", err)
		}
	}

	return ctrl.Result{RequeueAfter: r.syncPeriod}, nil
}

// CheckProgressing checks whether the given object is progressing. It returns a bool indicating whether the object is
// progressing, a reason for it if so and an error if the check failed.
func CheckProgressing(obj client.Object) (bool, string) {
	switch o := obj.(type) {
	case *appsv1.Deployment:
		return health.IsDeploymentProgressing(o)
	case *appsv1.StatefulSet:
		return health.IsStatefulSetProgressing(o)
	case *appsv1.DaemonSet:
		return health.IsDaemonSetProgressing(o)
	}

	return false, ""
}
