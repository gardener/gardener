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
	"fmt"
	"time"

	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/retry"

	dnsv1alpha1 "github.com/gardener/external-dns-management/pkg/apis/dns/v1alpha1"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// EntryValues contains the values used to create a DNSEntry
type EntryValues struct {
	Name    string
	DNSName string
	Targets []string
	OwnerID string
	TTL     int64
}

// NewEntry creates a new instance of DeployWaiter for a specific DNS emptyEntry.
// <waiter> is optional and it's defaulted to github.com/gardener/gardener/pkg/utils/retry.DefaultOps()
func NewEntry(
	logger logrus.FieldLogger,
	client client.Client,
	namespace string,
	values *EntryValues,
	waiter retry.Ops,
) component.DeployWaiter {
	if waiter == nil {
		waiter = retry.DefaultOps()
	}

	return &entry{
		logger:    logger,
		client:    client,
		namespace: namespace,
		values:    values,
		waiter:    waiter,
	}
}

type entry struct {
	logger    logrus.FieldLogger
	client    client.Client
	namespace string
	values    *EntryValues
	waiter    retry.Ops
}

func (e *entry) Deploy(ctx context.Context) error {
	obj := e.emptyEntry()

	_, err := controllerutil.CreateOrUpdate(ctx, e.client, obj, func() error {
		obj.Spec = dnsv1alpha1.DNSEntrySpec{
			DNSName: e.values.DNSName,
			TTL:     pointer.Int64Ptr(120),
			Targets: e.values.Targets,
		}

		if e.values.TTL > 0 {
			obj.Spec.TTL = &e.values.TTL
		}

		if len(e.values.OwnerID) > 0 {
			obj.Spec.OwnerId = &e.values.OwnerID
		}

		return nil
	})
	return err
}

func (e *entry) Destroy(ctx context.Context) error {
	return client.IgnoreNotFound(e.client.Delete(ctx, e.emptyEntry()))
}

func (e *entry) Wait(ctx context.Context) error {
	var (
		status  string
		message string

		retryCountUntilSevere int
		interval              = 5 * time.Second
		severeThreshold       = 15 * time.Second
		timeout               = 2 * time.Minute
	)

	if err := e.waiter.UntilTimeout(ctx, interval, timeout, func(ctx context.Context) (done bool, err error) {
		retryCountUntilSevere++

		obj := &dnsv1alpha1.DNSEntry{}
		if err2 := e.client.Get(ctx, client.ObjectKey{Name: e.values.Name, Namespace: e.namespace}, obj); err2 != nil {
			if apierrors.IsNotFound(err2) {
				return retry.MinorError(err2)
			}
			return retry.SevereError(err2)
		}

		if obj.Status.ObservedGeneration == obj.Generation && obj.Status.State == dnsv1alpha1.STATE_READY {
			return retry.Ok()
		}

		status = obj.Status.State
		if msg := obj.Status.Message; msg != nil {
			message = *msg
		}
		entryErr := fmt.Errorf("DNS record %q is not ready (status=%s, message=%s)", e.values.Name, status, message)

		e.logger.Infof("Waiting for %q DNS record to be ready... (status=%s, message=%s)", e.values.Name, status, message)
		if status == dnsv1alpha1.STATE_ERROR || status == dnsv1alpha1.STATE_INVALID {
			return retry.MinorOrSevereError(retryCountUntilSevere, int(severeThreshold.Nanoseconds()/interval.Nanoseconds()), entryErr)
		}

		return retry.MinorError(entryErr)
	}); err != nil {
		return gardencorev1beta1helper.DetermineError(err, fmt.Sprintf("Failed to create %q DNS record: %q (status=%s, message=%s)", e.values.Name, err.Error(), status, message))
	}

	return nil
}

func (e *entry) WaitCleanup(ctx context.Context) error {
	return kutil.WaitUntilResourceDeleted(ctx, e.client, e.emptyEntry(), 5*time.Second)
}

func (e *entry) emptyEntry() *dnsv1alpha1.DNSEntry {
	return &dnsv1alpha1.DNSEntry{ObjectMeta: metav1.ObjectMeta{Name: e.values.Name, Namespace: e.namespace}}
}
