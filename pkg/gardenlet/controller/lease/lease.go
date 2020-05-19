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
	"fmt"
	"time"

	coordinationv1 "k8s.io/api/coordination/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	coordclientset "k8s.io/client-go/kubernetes/typed/coordination/v1"
	"k8s.io/utils/pointer"
)

// Controller manages creating and renewing the lease for this Gardenlet.
type Controller interface {
	Sync(string, ...metav1.OwnerReference) error
}

type leaseController struct {
	leaseClient          coordclientset.LeaseInterface
	leaseDurationSeconds int32
	namespace            string
	nowFunc              func() time.Time
}

// NewLeaseController constructs and returns a controller.
func NewLeaseController(client clientset.Interface, nowFunc func() time.Time, leaseDurationSeconds int32, namespace string) Controller {
	var leaseClient coordclientset.LeaseInterface
	if client != nil {
		leaseClient = client.CoordinationV1().Leases(namespace)
	}

	return &leaseController{
		leaseClient:          leaseClient,
		leaseDurationSeconds: leaseDurationSeconds,
		namespace:            namespace,
		nowFunc:              nowFunc,
	}
}

// Sync updates the Lease
func (c *leaseController) Sync(holderIdentity string, ownerReferences ...metav1.OwnerReference) error {
	if c.leaseClient == nil {
		return fmt.Errorf("lease controller has nil lease client, will not claim or renew leases")
	}
	return c.tryUpdateOrCreateLease(holderIdentity, ownerReferences...)
}

// tryUpdateOrCreateLease updates or creates the lease if it does not exist.
func (c *leaseController) tryUpdateOrCreateLease(holderIdentity string, ownerReferences ...metav1.OwnerReference) error {
	lease, err := c.leaseClient.Get(holderIdentity, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			_, err2 := c.leaseClient.Create(c.newLease(nil, holderIdentity, ownerReferences...))
			return err2
		}
		return err
	}

	_, err = c.leaseClient.Update(c.newLease(lease, holderIdentity, ownerReferences...))
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
