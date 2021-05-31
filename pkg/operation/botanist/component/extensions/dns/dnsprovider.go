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

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	"github.com/gardener/gardener/pkg/utils/retry"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	dnsv1alpha1 "github.com/gardener/external-dns-management/pkg/apis/dns/v1alpha1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ProviderValues contains the values used to create a DNSProvider.
type ProviderValues struct {
	Name        string
	Purpose     string
	Provider    string
	Labels      map[string]string
	Annotations map[string]string
	SecretData  map[string][]byte
	Domains     *IncludeExclude
	Zones       *IncludeExclude
}

// IncludeExclude contain slices of excluded and included domains/zones.
type IncludeExclude struct {
	Include []string
	Exclude []string
}

// NewProvider creates a new instance of DeployWaiter for a specific DNSProvider.
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

		dnsProvider: &dnsv1alpha1.DNSProvider{
			ObjectMeta: metav1.ObjectMeta{
				Name:      values.Name,
				Namespace: namespace,
			},
		},
	}
}

type provider struct {
	logger    logrus.FieldLogger
	client    client.Client
	namespace string
	values    *ProviderValues
	waiter    retry.Ops

	dnsProvider *dnsv1alpha1.DNSProvider
}

func (p *provider) Deploy(ctx context.Context) error {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "extensions-dns-" + p.values.Name,
			Namespace: p.namespace,
		},
	}

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, p.client, secret, func() error {
		secret.Labels = p.values.Labels
		secret.Type = corev1.SecretTypeOpaque
		secret.Data = p.values.SecretData
		return nil
	}); err != nil {
		return err
	}

	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, p.client, p.dnsProvider, func() error {
		p.dnsProvider.Labels = p.values.Labels
		p.dnsProvider.Annotations = p.values.Annotations
		metav1.SetMetaDataAnnotation(&p.dnsProvider.ObjectMeta, v1beta1constants.GardenerTimestamp, TimeNow().UTC().String())

		p.dnsProvider.Spec = dnsv1alpha1.DNSProviderSpec{
			Type:      p.values.Provider,
			SecretRef: &corev1.SecretReference{Name: secret.Name},
		}

		if p.values.Domains != nil {
			p.dnsProvider.Spec.Domains = &dnsv1alpha1.DNSSelection{
				Include: p.values.Domains.Include,
				Exclude: p.values.Domains.Exclude,
			}
		}

		if p.values.Zones != nil {
			p.dnsProvider.Spec.Zones = &dnsv1alpha1.DNSSelection{
				Include: p.values.Zones.Include,
				Exclude: p.values.Zones.Exclude,
			}
		}

		return nil
	})
	return err
}

func (p *provider) Destroy(ctx context.Context) error {
	return client.IgnoreNotFound(p.client.Delete(ctx, p.dnsProvider))
}

func (p *provider) Wait(ctx context.Context) error {
	var (
		status  string
		message string

		retryCountUntilSevere int
		interval              = 5 * time.Second
		severeThreshold       = 15 * time.Second
		timeout               = 2 * time.Minute

		annotationHealthFunc health.Func
	)

	// wait until we see the timestamp annotation, that we set earlier in Deploy, to prevent falsely returning from Wait
	// in case of stale cache reads
	if expectedTimestamp, ok := p.dnsProvider.Annotations[v1beta1constants.GardenerTimestamp]; ok {
		annotationHealthFunc = health.ObjectHasAnnotationWithValue(v1beta1constants.GardenerTimestamp, expectedTimestamp)
	}

	// TODO: switch to extensions.WaitUntilObjectReadyWithHealthFunction?
	if err := p.waiter.UntilTimeout(ctx, interval, timeout, func(ctx context.Context) (bool, error) {
		if err := p.client.Get(ctx, client.ObjectKeyFromObject(p.dnsProvider), p.dnsProvider); err != nil {
			if apierrors.IsNotFound(err) {
				return retry.MinorError(err)
			}
			return retry.SevereError(err)
		}

		if annotationHealthFunc != nil {
			if err := annotationHealthFunc(p.dnsProvider); err != nil {
				return retry.MinorError(err)
			}
		}

		// TODO: why are we not checking generation == observedGeneration here?

		if p.dnsProvider.Status.State == dnsv1alpha1.STATE_READY {
			return retry.Ok()
		}

		status = p.dnsProvider.Status.State
		if msg := p.dnsProvider.Status.Message; msg != nil {
			message = *msg
		}
		providerErr := fmt.Errorf("DNS provider %q is not ready (status=%s, message=%s)", p.values.Name, status, message)

		p.logger.Infof("Waiting for %q DNS provider to be ready... (status=%s, message=%s)", p.values.Name, status, message)
		if status == dnsv1alpha1.STATE_ERROR || status == dnsv1alpha1.STATE_INVALID {
			return retry.MinorOrSevereError(retryCountUntilSevere, int(severeThreshold.Nanoseconds()/interval.Nanoseconds()), providerErr)
		}

		return retry.MinorError(providerErr)
	}); err != nil {
		return fmt.Errorf("failed to reconcile DNS provider %q: %s (status=%s, message=%s)", p.values.Name, err.Error(), status, message)
	}

	return nil
}

func (p *provider) WaitCleanup(ctx context.Context) error {
	return kutil.WaitUntilResourceDeleted(ctx, p.client, p.dnsProvider, 5*time.Second)
}
