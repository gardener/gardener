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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ProviderValues contains the values used to create a DNSProvider.
type ProviderValues struct {
	Name       string            `json:"name,omitempty"`
	Purpose    string            `json:"purpose,omitempty"`
	Provider   string            `json:"provider,omitempty"`
	Labels     map[string]string `json:"labels,omitempty"`
	SecretData map[string][]byte `json:"secretData,omitempty"`
	Domains    *IncludeExclude   `json:"domains,omitempty"`
	Zones      *IncludeExclude   `json:"zones,omitempty"`
}

// IncludeExclude contain slices of excluded and included domains/zones.
type IncludeExclude struct {
	Include []string `json:"include,omitempty"`
	Exclude []string `json:"exclude,omitempty"`
}

// NewDNSProvider creates a new instance of DeployWaiter for a specific DNS provider.
// <waiter> is optional and it's defaulted to github.com/gardener/gardener/pkg/utils/retry.DefaultOps()
func NewDNSProvider(
	values *ProviderValues,
	shootNamespace string,
	applier kubernetes.ChartApplier,
	chartsRootPath string,
	logger *logrus.Entry,
	client client.Client,
	waiter retry.Ops,
) component.DeployWaiter {
	if waiter == nil {
		waiter = retry.DefaultOps()
	}

	return &dnsProvider{
		values:         values,
		shootNamespace: shootNamespace,
		ChartApplier:   applier,
		chartPath:      filepath.Join(chartsRootPath, "seed-dns", "provider"),
		logger:         logger,
		client:         client,
		waiter:         waiter,
	}
}

type dnsProvider struct {
	values         *ProviderValues
	shootNamespace string
	kubernetes.ChartApplier
	chartPath string
	logger    *logrus.Entry
	client    client.Client
	waiter    retry.Ops
}

func (d *dnsProvider) Deploy(ctx context.Context) error {
	return d.Apply(ctx, d.chartPath, d.shootNamespace, d.values.Name, kubernetes.Values(d.values))
}

func (d *dnsProvider) Destroy(ctx context.Context) error {
	return client.IgnoreNotFound(d.client.Delete(ctx, d.provider()))
}

func (d *dnsProvider) Wait(ctx context.Context) error {
	var (
		status  string
		message string

		retryCountUntilSevere int
		interval              = 5 * time.Second
		severeThreshold       = 15 * time.Second
		timeout               = 2 * time.Minute
	)

	if err := d.waiter.UntilTimeout(ctx, interval, timeout, func(ctx context.Context) (done bool, err error) {
		provider := &dnsv1alpha1.DNSProvider{}
		if err := d.client.Get(
			ctx,
			client.ObjectKey{Name: d.values.Name, Namespace: d.shootNamespace},
			provider,
		); err != nil {
			return retry.SevereError(err)
		}

		if provider.Status.State == dnsv1alpha1.STATE_READY {
			return retry.Ok()
		}

		status = provider.Status.State
		if msg := provider.Status.Message; msg != nil {
			message = *msg
		}
		providerErr := fmt.Errorf("DNS provider %q is not ready (status=%s, message=%s)", d.values.Name, status, message)

		d.logger.Infof("Waiting for %q DNS provider to be ready... (status=%s, message=%s)", d.values.Name, status, message)
		if status == dnsv1alpha1.STATE_ERROR || status == dnsv1alpha1.STATE_INVALID {
			return retry.MinorOrSevereError(retryCountUntilSevere, int(severeThreshold.Nanoseconds()/interval.Nanoseconds()), providerErr)
		}

		return retry.MinorError(providerErr)
	}); err != nil {
		return gardencorev1beta1helper.DetermineError(err, fmt.Sprintf("Failed to create DNS provider for %q DNS record: %q (status=%s, message=%s)", d.values.Name, err.Error(), status, message))
	}

	return nil
}

func (d *dnsProvider) WaitCleanup(ctx context.Context) error {
	return kutil.WaitUntilResourceDeleted(ctx, d.client, d.provider(), 5*time.Second)
}

// provider returs a empty DNSProvider used for deletion.
func (d *dnsProvider) provider() *dnsv1alpha1.DNSProvider {
	return &dnsv1alpha1.DNSProvider{ObjectMeta: metav1.ObjectMeta{Name: d.values.Name, Namespace: d.shootNamespace}}
}
