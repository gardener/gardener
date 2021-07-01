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

	dnsv1alpha1 "github.com/gardener/external-dns-management/pkg/apis/dns/v1alpha1"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/extensions"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// EntryValues contains the values used to create a DNSEntry
type EntryValues struct {
	Name    string
	DNSName string
	Targets []string
	OwnerID string
	TTL     int64
}

// NewEntry creates a new instance of DeployWaiter for a specific DNSEntry.
func NewEntry(
	logger logrus.FieldLogger,
	client client.Client,
	namespace string,
	values *EntryValues,
) component.DeployWaiter {
	return &entry{
		logger:    logger,
		client:    client,
		namespace: namespace,
		values:    values,

		dnsEntry: &dnsv1alpha1.DNSEntry{
			ObjectMeta: metav1.ObjectMeta{
				Name:      values.Name,
				Namespace: namespace,
			},
		},
	}
}

type entry struct {
	logger    logrus.FieldLogger
	client    client.Client
	namespace string
	values    *EntryValues

	dnsEntry *dnsv1alpha1.DNSEntry
}

func (e *entry) Deploy(ctx context.Context) error {
	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, e.client, e.dnsEntry, func() error {
		metav1.SetMetaDataAnnotation(&e.dnsEntry.ObjectMeta, v1beta1constants.GardenerTimestamp, TimeNow().UTC().String())

		e.dnsEntry.Spec = dnsv1alpha1.DNSEntrySpec{
			DNSName: e.values.DNSName,
			TTL:     pointer.Int64(120),
			Targets: e.values.Targets,
		}

		if e.values.TTL > 0 {
			e.dnsEntry.Spec.TTL = &e.values.TTL
		}

		if len(e.values.OwnerID) > 0 {
			e.dnsEntry.Spec.OwnerId = &e.values.OwnerID
		}

		return nil
	})
	return err
}

func (e *entry) Destroy(ctx context.Context) error {
	return client.IgnoreNotFound(e.client.Delete(ctx, e.dnsEntry))
}

func (e *entry) Wait(ctx context.Context) error {
	return extensions.WaitUntilObjectReadyWithHealthFunction(
		ctx,
		e.client,
		e.logger,
		CheckDNSObject,
		e.dnsEntry,
		dnsv1alpha1.DNSEntryKind,
		5*time.Second,
		15*time.Second,
		2*time.Minute,
		nil,
	)
}

func (e *entry) WaitCleanup(ctx context.Context) error {
	return kutil.WaitUntilResourceDeleted(ctx, e.client, e.dnsEntry, 5*time.Second)
}
