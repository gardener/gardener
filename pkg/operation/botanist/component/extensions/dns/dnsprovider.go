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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// ProviderValues contains the values used to create a DNSProvider.
type ProviderValues struct {
	Name       string
	Purpose    string
	Provider   string
	Labels     map[string]string
	SecretData map[string][]byte
	Domains    *IncludeExclude
	Zones      *IncludeExclude
}

// IncludeExclude contain slices of excluded and included domains/zones.
type IncludeExclude struct {
	Include []string
	Exclude []string
}

// NewProvider creates a new instance of DeployWaiter for a specific DNS emptyProvider.
// <waiter> is optional and it's defaulted to github.com/gardener/gardener/pkg/utils/retry.DefaultOps()
func NewProvider(
	logger logrus.FieldLogger,
	client client.Client,
	namespace string,
	values *ProviderValues,
	waiter retry.Ops,
) component.DeployWaiter {
	if waiter == nil {
		waiter = retry.DefaultOps()
	}

	return &provider{
		logger:    logger,
		client:    client,
		namespace: namespace,
		values:    values,
		waiter:    waiter,
	}
}

type provider struct {
	logger    logrus.FieldLogger
	client    client.Client
	namespace string
	values    *ProviderValues
	waiter    retry.Ops
}

func (p *provider) Deploy(ctx context.Context) error {
	var (
		secret      = p.emptySecret()
		dnsProvider = p.emptyProvider()
	)

	if _, err := controllerutil.CreateOrUpdate(ctx, p.client, secret, func() error {
		secret.Labels = p.values.Labels
		secret.Type = corev1.SecretTypeOpaque
		secret.Data = p.values.SecretData
		return nil
	}); err != nil {
		return err
	}

	_, err := controllerutil.CreateOrUpdate(ctx, p.client, dnsProvider, func() error {
		dnsProvider.Labels = p.values.Labels

		dnsProvider.Spec = dnsv1alpha1.DNSProviderSpec{
			Type:      p.values.Provider,
			SecretRef: &corev1.SecretReference{Name: secret.Name},
		}

		if p.values.Domains != nil {
			dnsProvider.Spec.Domains = &dnsv1alpha1.DNSSelection{
				Include: p.values.Domains.Include,
				Exclude: p.values.Domains.Exclude,
			}
		}

		if p.values.Zones != nil {
			dnsProvider.Spec.Zones = &dnsv1alpha1.DNSSelection{
				Include: p.values.Zones.Include,
				Exclude: p.values.Zones.Exclude,
			}
		}

		return nil
	})
	return err
}

func (p *provider) Destroy(ctx context.Context) error {
	return client.IgnoreNotFound(p.client.Delete(ctx, p.emptyProvider()))
}

func (p *provider) Wait(ctx context.Context) error {
	var (
		status  string
		message string

		retryCountUntilSevere int
		interval              = 5 * time.Second
		severeThreshold       = 15 * time.Second
		timeout               = 2 * time.Minute
	)

	if err := p.waiter.UntilTimeout(ctx, interval, timeout, func(ctx context.Context) (done bool, err error) {
		obj := &dnsv1alpha1.DNSProvider{}
		if err := p.client.Get(
			ctx,
			client.ObjectKey{Name: p.values.Name, Namespace: p.namespace},
			obj,
		); err != nil {
			return retry.SevereError(err)
		}

		if obj.Status.State == dnsv1alpha1.STATE_READY {
			return retry.Ok()
		}

		status = obj.Status.State
		if msg := obj.Status.Message; msg != nil {
			message = *msg
		}
		providerErr := fmt.Errorf("DNS emptyProvider %q is not ready (status=%s, message=%s)", p.values.Name, status, message)

		p.logger.Infof("Waiting for %q DNS emptyProvider to be ready... (status=%s, message=%s)", p.values.Name, status, message)
		if status == dnsv1alpha1.STATE_ERROR || status == dnsv1alpha1.STATE_INVALID {
			return retry.MinorOrSevereError(retryCountUntilSevere, int(severeThreshold.Nanoseconds()/interval.Nanoseconds()), providerErr)
		}

		return retry.MinorError(providerErr)
	}); err != nil {
		return gardencorev1beta1helper.DetermineError(err, fmt.Sprintf("Failed to create DNS emptyProvider for %q DNS record: %q (status=%s, message=%s)", p.values.Name, err.Error(), status, message))
	}

	return nil
}

func (p *provider) WaitCleanup(ctx context.Context) error {
	return kutil.WaitUntilResourceDeleted(ctx, p.client, p.emptyProvider(), 5*time.Second)
}

func (p *provider) emptySecret() *corev1.Secret {
	return &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "extensions-dns-" + p.values.Name, Namespace: p.namespace}}
}

func (p *provider) emptyProvider() *dnsv1alpha1.DNSProvider {
	return &dnsv1alpha1.DNSProvider{ObjectMeta: metav1.ObjectMeta{Name: p.values.Name, Namespace: p.namespace}}
}
