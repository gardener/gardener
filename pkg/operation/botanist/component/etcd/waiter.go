// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package etcd

import (
	"context"
	"fmt"
	"time"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/extensions"
	"github.com/gardener/gardener/pkg/utils/retry"

	druidv1alpha1 "github.com/gardener/etcd-druid/api/v1alpha1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
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
		return retry.RetriableError(fmt.Errorf(*etcd.Status.LastError))
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

	if etcd.Status.Replicas != etcd.Status.UpdatedReplicas {
		return fmt.Errorf("update is being rolled out, only %d/%d replicas are up-to-date", etcd.Status.UpdatedReplicas, etcd.Status.Replicas)
	}

	if op, ok := etcd.Annotations[v1beta1constants.GardenerOperation]; ok {
		return fmt.Errorf("gardener operation %q is not yet picked up by etcd-druid", op)
	}

	if !pointer.BoolDeref(etcd.Status.Ready, false) {
		return fmt.Errorf("is not ready yet")
	}

	return nil
}
