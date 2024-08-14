// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package etcd

import (
	"context"
	"fmt"
	"time"

	druidv1alpha1 "github.com/gardener/etcd-druid/api/v1alpha1"
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
	etcd, ok := obj.(*druidv1alpha1.Etcd)
	if !ok {
		return fmt.Errorf("expected *duridv1alpha1.Etcd but got %T", obj)
	}

	if etcd.Status.LastError != nil {
		return retry.RetriableError(fmt.Errorf("error during reconciliation: %s", *etcd.Status.LastError))
	}

	if etcd.DeletionTimestamp != nil {
		return fmt.Errorf("unexpectedly has a deletion timestamp")
	}

	generation := etcd.Generation
	observedGeneration := etcd.Status.ObservedGeneration
	if observedGeneration == nil {
		return fmt.Errorf("observed generation not recorded")
	}
	if generation != *observedGeneration {
		return fmt.Errorf("observed generation outdated (%d/%d)", *observedGeneration, generation)
	}

	if op, ok := etcd.Annotations[v1beta1constants.GardenerOperation]; ok {
		return fmt.Errorf("gardener operation %q is not yet picked up by etcd-druid", op)
	}

	if !ptr.Deref(etcd.Status.Ready, false) {
		return fmt.Errorf("is not ready yet")
	}

	return nil
}
