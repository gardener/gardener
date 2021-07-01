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

package dns

import (
	"context"
	"time"

	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	dnsv1alpha1 "github.com/gardener/external-dns-management/pkg/apis/dns/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// OwnerValues contains the values used for DNSOwner creation
type OwnerValues struct {
	Name    string
	OwnerID string
	Active  *bool
}

// NewOwner creates a new instance of DeployWaiter for a specific DNSOwner.
func NewOwner(client client.Client, namespace string, values *OwnerValues) component.DeployWaiter {
	return &owner{
		client:    client,
		namespace: namespace,
		values:    values,

		dnsOwner: &dnsv1alpha1.DNSOwner{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace + "-" + values.Name,
			},
		},
	}
}

type owner struct {
	client    client.Client
	namespace string
	values    *OwnerValues

	dnsOwner *dnsv1alpha1.DNSOwner
}

func (o *owner) Deploy(ctx context.Context) error {
	active := o.values.Active
	if active == nil {
		active = pointer.Bool(true)
	}

	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, o.client, o.dnsOwner, func() error {
		o.dnsOwner.Spec = dnsv1alpha1.DNSOwnerSpec{
			OwnerId: o.values.OwnerID,
			Active:  active,
		}
		return nil
	})
	return err
}

func (o *owner) Destroy(ctx context.Context) error {
	return client.IgnoreNotFound(o.client.Delete(ctx, o.dnsOwner))
}

func (o *owner) Wait(_ context.Context) error { return nil }

func (o *owner) WaitCleanup(ctx context.Context) error {
	return kutil.WaitUntilResourceDeleted(ctx, o.client, o.dnsOwner, 5*time.Second)
}
