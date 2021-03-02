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
	"path/filepath"
	"time"

	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/retry"

	dnsv1alpha1 "github.com/gardener/external-dns-management/pkg/apis/dns/v1alpha1"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// EntryValues contains the values used to create a DNSEntry
type EntryValues struct {
	Name    string   `json:"name,omitempty"`
	DNSName string   `json:"dnsName,omitempty"`
	Targets []string `json:"targets,omitempty"`
	OwnerID string   `json:"ownerID,omitempty"`
	TTL     int64    `json:"ttl,omitempty"`
}

// NewDNSEntry creates a new instance of DeployWaiter for a specific DNS entry.
// <waiter> is optional and it's defaulted to github.com/gardener/gardener/pkg/utils/retry.DefaultOps()
func NewDNSEntry(
	values *EntryValues,
	shootNamespace string,
	applier kubernetes.ChartApplier,
	chartsRootPath string,
	logger logrus.FieldLogger,
	client client.Client,
	waiter retry.Ops,

) component.DeployWaiter {
	if waiter == nil {
		waiter = retry.DefaultOps()
	}

	return &dnsEntry{
		ChartApplier:   applier,
		chartPath:      filepath.Join(chartsRootPath, "seed-dns", "entry"),
		client:         client,
		logger:         logger,
		shootNamespace: shootNamespace,
		values:         values,
		waiter:         waiter,
	}
}

type dnsEntry struct {
	values         *EntryValues
	shootNamespace string
	kubernetes.ChartApplier
	chartPath string
	logger    logrus.FieldLogger
	client    client.Client
	waiter    retry.Ops
}

func (d *dnsEntry) Deploy(ctx context.Context) error {
	return d.Apply(ctx, d.chartPath, d.shootNamespace, d.values.Name, kubernetes.Values(d.values))
}

func (d *dnsEntry) Destroy(ctx context.Context) error {
	return client.IgnoreNotFound(d.client.Delete(ctx, d.entry()))
}

func (d *dnsEntry) Wait(ctx context.Context) error {
	var (
		status  string
		message string

		retryCountUntilSevere int
		interval              = 5 * time.Second
		severeThreshold       = 15 * time.Second
		timeout               = 2 * time.Minute
	)

	if err := d.waiter.UntilTimeout(ctx, interval, timeout, func(ctx context.Context) (done bool, err error) {
		retryCountUntilSevere++

		entry := &dnsv1alpha1.DNSEntry{}
		if err := d.client.Get(
			ctx,
			client.ObjectKey{Name: d.values.Name, Namespace: d.shootNamespace},
			entry,
		); err != nil {
			if apierrors.IsNotFound(err) {
				return retry.MinorError(err)
			}
			return retry.SevereError(err)
		}

		if entry.Status.ObservedGeneration == entry.Generation && entry.Status.State == dnsv1alpha1.STATE_READY {
			return retry.Ok()
		}

		status = entry.Status.State
		if msg := entry.Status.Message; msg != nil {
			message = *msg
		}
		entryErr := fmt.Errorf("DNS record %q is not ready (status=%s, message=%s)", d.values.Name, status, message)

		d.logger.Infof("Waiting for %q DNS record to be ready... (status=%s, message=%s)", d.values.Name, status, message)
		if status == dnsv1alpha1.STATE_ERROR || status == dnsv1alpha1.STATE_INVALID {
			return retry.MinorOrSevereError(retryCountUntilSevere, int(severeThreshold.Nanoseconds()/interval.Nanoseconds()), entryErr)
		}

		return retry.MinorError(entryErr)
	}); err != nil {
		return gardencorev1beta1helper.DetermineError(err, fmt.Sprintf("Failed to create %q DNS record: %q (status=%s, message=%s)", d.values.Name, err.Error(), status, message))
	}

	return nil
}

func (d *dnsEntry) WaitCleanup(ctx context.Context) error {
	return kutil.WaitUntilResourceDeleted(ctx, d.client, d.entry(), 5*time.Second)
}

// entry returns an empty DNSEntry used for deletion.
func (d *dnsEntry) entry() *dnsv1alpha1.DNSEntry {
	return &dnsv1alpha1.DNSEntry{ObjectMeta: metav1.ObjectMeta{Name: d.values.Name, Namespace: d.shootNamespace}}
}
