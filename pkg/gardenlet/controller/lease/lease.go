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

package lease

import (
	"context"
	"fmt"
	"time"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	coordinationv1 "k8s.io/api/coordination/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	coordclientset "k8s.io/client-go/kubernetes/typed/coordination/v1"
	"k8s.io/utils/pointer"

	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
)

// Controller manages creating and renewing the lease for this Gardenlet.
type Controller interface {
	Sync(string, ...metav1.OwnerReference) error
}

type leaseController struct {
	clientMap            clientmap.ClientMap
	leaseDurationSeconds int32
	namespace            string
	nowFunc              func() time.Time
}

// NewLeaseController constructs and returns a controller.
func NewLeaseController(nowFunc func() time.Time, clientMap clientmap.ClientMap, leaseDurationSeconds int32, namespace string) Controller {
	return &leaseController{
		clientMap:            clientMap,
		leaseDurationSeconds: leaseDurationSeconds,
		namespace:            namespace,
		nowFunc:              nowFunc,
	}
}

// Sync updates the Lease
func (c *leaseController) Sync(holderIdentity string, ownerReferences ...metav1.OwnerReference) error {
	ctx := context.TODO()
	gardenClient, err := c.clientMap.GetClient(ctx, keys.ForGarden())
	if err != nil {
		return fmt.Errorf("failed to get garden client: %w", err)
	}
	leaseClient := gardenClient.Kubernetes().CoordinationV1().Leases(c.namespace)

	return c.tryUpdateOrCreateLease(ctx, leaseClient, holderIdentity, ownerReferences...)
}

// tryUpdateOrCreateLease updates or creates the lease if it does not exist.
func (c *leaseController) tryUpdateOrCreateLease(ctx context.Context, leaseClient coordclientset.LeaseInterface, holderIdentity string, ownerReferences ...metav1.OwnerReference) error {
	lease, err := leaseClient.Get(ctx, holderIdentity, kubernetes.DefaultGetOptions())
	if err != nil {
		if apierrors.IsNotFound(err) {
			_, err2 := leaseClient.Create(ctx, c.newLease(nil, holderIdentity, ownerReferences...), kubernetes.DefaultCreateOptions())
			return err2
		}
		return err
	}

	_, err = leaseClient.Update(ctx, c.newLease(lease, holderIdentity, ownerReferences...), kubernetes.DefaultUpdateOptions())
	return err
}

// newLease constructs a new lease if base is nil, or returns a copy of base
// with desired state asserted on the copy.
func (c *leaseController) newLease(base *coordinationv1.Lease, holderIdentity string, ownerReferences ...metav1.OwnerReference) *coordinationv1.Lease {
	lease := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      holderIdentity,
			Namespace: c.namespace,
		},
		Spec: coordinationv1.LeaseSpec{
			HolderIdentity:       pointer.StringPtr(holderIdentity),
			LeaseDurationSeconds: pointer.Int32Ptr(c.leaseDurationSeconds),
		},
	}

	if base != nil {
		lease = base.DeepCopy()
	}

	lease.Spec.RenewTime = &metav1.MicroTime{Time: c.nowFunc()}
	if ownerReferences != nil {
		if len(lease.OwnerReferences) == 0 {
			lease.OwnerReferences = ownerReferences
		}
	}

	return lease
}
