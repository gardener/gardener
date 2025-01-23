// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package etcd

import (
	"context"
	"fmt"
	"time"

	druidv1alpha1 "github.com/gardener/etcd-druid/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/extensions"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
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
	if err := extensions.WaitUntilObjectReadyWithHealthFunction(
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
	); err != nil {
		return err
	}

	// This is a band-aid for https://github.com/gardener/etcd-druid/issues/985
	// and can be removed as soon as the issue is fixed in Etcd-Druid.
	return e.checkStatefulSetIsNotProgressing(ctx)
}

func (e *etcd) checkStatefulSetIsNotProgressing(ctx context.Context) error {
	etcd := druidv1alpha1.Etcd{}
	if err := e.client.Get(ctx, client.ObjectKeyFromObject(e.etcd), &etcd); err != nil {
		return err
	}

	if etcd.Status.Etcd == nil || len(etcd.Status.Etcd.Name) == 0 {
		e.log.Info("Skip checking etcd StatefulSet as name is not given in etcd resource")
		return nil
	}

	etcdSts := &appsv1.StatefulSet{}
	if err := e.apiReader.Get(ctx, client.ObjectKey{Namespace: e.namespace, Name: etcd.Status.Etcd.Name}, etcdSts); err != nil {
		return err
	}

	if processing, message := health.IsStatefulSetProgressing(etcdSts); processing {
		return fmt.Errorf("etcd stateful set %q is progressing: %s", etcdSts.Name, message)
	}
	return nil
}

func (e *etcd) WaitCleanup(_ context.Context) error { return nil }

// CheckEtcdObject checks if the given Etcd object was reconciled successfully.
func CheckEtcdObject(obj client.Object) error {
	e, ok := obj.(*druidv1alpha1.Etcd)
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

	if !ptr.Deref(e.Status.Ready, false) {
		return fmt.Errorf("is not ready yet")
	}

	return nil
}
