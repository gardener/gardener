// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package etcd

import (
	"context"
	"fmt"
	"time"

	druidcorev1alpha1 "github.com/gardener/etcd-druid/api/core/v1alpha1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/extensions"
	"github.com/gardener/gardener/pkg/utils/retry"
)

var (
	// DefaultInterval is the default interval for retry operations.
	DefaultInterval = 5 * time.Second
	// DefaultSevereThreshold is the default threshold until an error reported by another component is treated as 'severe'.
	DefaultSevereThreshold = 3 * time.Minute
	// DefaultTimeout is the default timeout and defines how long Gardener should wait
	// for a successful reconciliation of an Etcd resource.
	DefaultTimeout = 5 * time.Minute
)

func (e *etcd) Wait(ctx context.Context) error {
	return extensions.WaitUntilObjectReadyWithHealthFunction(
		ctx,
		e.client,
		e.log,
		CheckEtcdObject,
		e.etcd,
		"Etcd",
		DefaultInterval,
		DefaultSevereThreshold,
		DefaultTimeout,
		nil,
	)
}

func (e *etcd) WaitCleanup(_ context.Context) error { return nil }

// CheckEtcdObject checks if the given Etcd object was reconciled successfully.
func CheckEtcdObject(obj client.Object) error {
	e, ok := obj.(*druidcorev1alpha1.Etcd)
	if !ok {
		return fmt.Errorf("expected *duridv1alpha1.Etcd but got %T", obj)
	}

	if len(e.Status.LastErrors) != 0 {
		return retry.RetriableError(fmt.Errorf("errors during reconciliation: %+v", e.Status.LastErrors))
	}

	if e.DeletionTimestamp != nil {
		return fmt.Errorf("unexpectedly has a deletion timestamp")
	}

	generation := e.Generation
	observedGeneration := e.Status.ObservedGeneration
	if observedGeneration == nil {
		return fmt.Errorf("observed generation not recorded")
	}
	if generation != *observedGeneration {
		return fmt.Errorf("observed generation outdated (%d/%d)", *observedGeneration, generation)
	}

	if op, ok := e.Annotations[v1beta1constants.GardenerOperation]; ok {
		return fmt.Errorf("gardener operation %q is not yet picked up by etcd-druid", op)
	}

	// If etcd replicas are set to 0, then we can skip readiness and updation checks,
	// because druid does not perform condition checks on hibernated etcd clusters,
	// and readiness no longer makes sense for hibernated etcd clusters.
	if e.Spec.Replicas == 0 {
		return nil
	}

	// condition `AllMembersUpdated` denotes whether an etcd cluster rollout has been completed,
	// so the Waiter can wait for operations such etcd CA rotation to be completed.
	conditionAllMembersUpdatedExists := false
	for _, cond := range e.Status.Conditions {
		if cond.Type == druidcorev1alpha1.ConditionTypeAllMembersUpdated {
			conditionAllMembersUpdatedExists = true
			if cond.Status != druidcorev1alpha1.ConditionTrue {
				return fmt.Errorf("condition %s is %s: %s", cond.Type, cond.Status, cond.Message)
			}
			break
		}
	}
	if !conditionAllMembersUpdatedExists {
		return fmt.Errorf("condition %s is not present", druidcorev1alpha1.ConditionTypeAllMembersUpdated)
	}

	if !ptr.Deref(e.Status.Ready, false) {
		return fmt.Errorf("is not ready yet")
	}

	return nil
}
