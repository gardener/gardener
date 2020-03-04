// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package botanist

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/gardener/gardener/pkg/apis/core"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils/flow"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/retry"

	dnsv1alpha1 "github.com/gardener/external-dns-management/pkg/apis/dns/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var dnsChartPath = filepath.Join(common.ChartPath, "seed-dns")

const (
	// DNSInternalName is a constant for a DNS resources used for the internal domain name.
	DNSInternalName = "internal"
	// DNSExternalName is a constant for a DNS resources used for the external domain name.
	DNSExternalName = "external"
	// DNSProviderRolePrimary is a constant for a DNS providers used to manage shoot domains.
	DNSProviderRolePrimary = "primary-dns-provider"
	// DNSProviderRoleAdditional is a constant for additionally managed DNS providers.
	DNSProviderRoleAdditional = "managed-dns-provider"
)

// DeployInternalDomainDNSRecord deploys the DNS record for the internal cluster domain.
func (b *Botanist) DeployInternalDomainDNSRecord(ctx context.Context) error {
	if err := b.deployDNSProvider(ctx, DNSInternalName, DNSProviderRolePrimary, b.Garden.InternalDomain.Provider, b.Garden.InternalDomain.SecretData, []string{b.Shoot.InternalClusterDomain}, nil, b.Garden.InternalDomain.IncludeZones, b.Garden.InternalDomain.ExcludeZones); err != nil {
		return err
	}
	return b.deployDNSEntry(ctx, DNSInternalName, common.GetAPIServerDomain(b.Shoot.InternalClusterDomain), b.APIServerAddress)
}

// DestroyInternalDomainDNSRecord destroys the DNS record for the internal cluster domain.
func (b *Botanist) DestroyInternalDomainDNSRecord(ctx context.Context) error {
	return b.deleteDNSEntry(ctx, DNSInternalName)
}

// DeployExternalDomainDNSRecord deploys the DNS record for the external cluster domain.
func (b *Botanist) DeployExternalDomainDNSRecord(ctx context.Context) error {
	if b.Shoot.Info.Spec.DNS == nil || b.Shoot.Info.Spec.DNS.Domain == nil || b.Shoot.ExternalClusterDomain == nil || strings.HasSuffix(*b.Shoot.ExternalClusterDomain, ".nip.io") {
		return nil
	}

	if err := b.deployDNSProvider(ctx, DNSExternalName, DNSProviderRolePrimary, b.Shoot.ExternalDomain.Provider, b.Shoot.ExternalDomain.SecretData, sets.NewString(append(b.Shoot.ExternalDomain.IncludeDomains, *b.Shoot.ExternalClusterDomain)...).List(), b.Shoot.ExternalDomain.ExcludeDomains, b.Shoot.ExternalDomain.IncludeZones, b.Shoot.ExternalDomain.ExcludeZones); err != nil {
		return err
	}
	return b.deployDNSEntry(ctx, DNSExternalName, common.GetAPIServerDomain(*b.Shoot.ExternalClusterDomain), b.APIServerAddress)
}

// DestroyExternalDomainDNSRecord destroys the DNS record for the external cluster domain.
func (b *Botanist) DestroyExternalDomainDNSRecord(ctx context.Context) error {
	return b.deleteDNSEntry(ctx, DNSExternalName)
}

// DeployAdditionalDNSProviders deploys the additional DNS providers configured in the shoot resource.
func (b *Botanist) DeployAdditionalDNSProviders(ctx context.Context) error {
	if b.Shoot.Info.Spec.DNS == nil {
		return nil
	}

	var (
		fns               []flow.TaskFn
		deployedProviders = sets.NewString()
	)
	for i, provider := range b.Shoot.Info.Spec.DNS.Providers {
		p := provider
		if p.Primary != nil && *p.Primary {
			continue
		}
		fns = append(fns, func(ctx context.Context) error {
			var includeDomains, excludeDomains, includeZones, excludeZones []string
			if domains := p.Domains; domains != nil {
				includeDomains = domains.Include
				excludeDomains = domains.Exclude
			}
			if zones := p.Zones; zones != nil {
				includeZones = zones.Include
				excludeZones = zones.Exclude
			}
			providerType := p.Type
			if providerType == nil {
				return fmt.Errorf("dns provider[%d] doesn't speify a type", i)
			}
			if *providerType == core.DNSUnmanaged {
				b.Logger.Infof("Skipping deployment of DNS provider[%d] since it specifies type %q", i, core.DNSUnmanaged)
				return nil
			}
			secretName := p.SecretName
			if secretName == nil {
				return fmt.Errorf("dns provider[%d] doesn't specify a secretName", i)
			}
			secret := &corev1.Secret{}
			if err := b.K8sGardenClient.Client().Get(ctx, kutil.Key(b.Shoot.Info.Namespace, *secretName), secret); err != nil {
				return fmt.Errorf("could not get dns provider secret %q: %+v", *secretName, err)
			}
			providerName := GenerateDNSProviderName(*secretName, *providerType)
			if err := b.deployDNSProvider(ctx, providerName, DNSProviderRoleAdditional, *p.Type, secret.Data, includeDomains, excludeDomains, includeZones, excludeZones); err != nil {
				return err
			}
			deployedProviders.Insert(providerName)
			return nil
		})
	}

	if err := flow.Parallel(fns...)(ctx); err != nil {
		return err
	}

	// Clean-up old providers
	providerList := &dnsv1alpha1.DNSProviderList{}
	if err := b.K8sSeedClient.Client().List(ctx, providerList, client.InNamespace(b.Shoot.SeedNamespace), client.MatchingLabels{v1beta1constants.GardenRole: DNSProviderRoleAdditional}); err != nil {
		return err
	}
	fns = nil

	for _, provider := range providerList.Items {
		p := provider
		if !deployedProviders.Has(p.Name) {
			fns = append(fns, func(ctx context.Context) error {
				return b.deleteDNSProvider(ctx, p.Name)
			})
		}
	}
	if err := flow.Parallel(fns...)(ctx); err != nil {
		return err
	}

	fns = nil
	for _, provider := range deployedProviders.UnsortedList() {
		providerName := provider
		fns = append(fns, func(ctx context.Context) error {
			return b.waitUntilDNSProviderReady(ctx, providerName)
		})
	}

	return flow.Parallel(fns...)(ctx)
}

// DeleteDNSProviders deletes all DNS providers in the shoot namespace of the seed.
func (b *Botanist) DeleteDNSProviders(ctx context.Context) error {
	if err := b.K8sSeedClient.Client().DeleteAllOf(ctx, &dnsv1alpha1.DNSProvider{}, client.InNamespace(b.Shoot.SeedNamespace)); err != nil {
		return err
	}

	providers := &dnsv1alpha1.DNSProviderList{}
	return kutil.WaitUntilResourcesDeleted(ctx, b.K8sSeedClient.Client(), providers, 5*time.Second, client.InNamespace(b.Shoot.SeedNamespace))
}

func (b *Botanist) deployDNSProvider(ctx context.Context, name, role, provider string, secretData map[string][]byte, includeDomains, excludeDomains, includeZones, excludeZones []string) error {
	values := map[string]interface{}{
		"name":     name,
		"purpose":  name,
		"provider": provider,
		"providerLabels": map[string]interface{}{
			v1beta1constants.GardenRole: role,
		},
		"secretData": secretData,
		"domains": map[string]interface{}{
			"include": includeDomains,
			"exclude": excludeDomains,
		},
		"zones": map[string]interface{}{
			"include": includeZones,
			"exclude": excludeZones,
		},
	}

	if err := b.ChartApplierSeed.Apply(ctx, filepath.Join(dnsChartPath, "provider"), b.Shoot.SeedNamespace, name, kubernetes.Values(values)); err != nil {
		return err
	}

	return b.waitUntilDNSProviderReady(ctx, name)
}

func (b *Botanist) waitUntilDNSProviderReady(ctx context.Context, name string) error {
	var (
		status  string
		message string
	)

	if err := retry.UntilTimeout(ctx, 5*time.Second, 2*time.Minute, func(ctx context.Context) (done bool, err error) {
		provider := &dnsv1alpha1.DNSProvider{}
		if err := b.K8sSeedClient.Client().Get(ctx, client.ObjectKey{Name: name, Namespace: b.Shoot.SeedNamespace}, provider); err != nil {
			return retry.SevereError(err)
		}

		if provider.Status.State == dnsv1alpha1.STATE_READY {
			return retry.Ok()
		}

		status = provider.Status.State
		if msg := provider.Status.Message; msg != nil {
			message = *msg
		}

		b.Logger.Infof("Waiting for %q DNS provider to be ready... (status=%s, message=%s)", name, status, message)
		return retry.MinorError(fmt.Errorf("DNS provider %q is not ready (status=%s, message=%s)", name, status, message))
	}); err != nil {
		return gardencorev1beta1helper.DetermineError(fmt.Sprintf("Failed to create DNS provider for %q DNS record: %q (status=%s, message=%s)", name, err.Error(), status, message))
	}

	return nil
}

func (b *Botanist) deleteDNSProvider(ctx context.Context, name string) error {
	if err := b.K8sSeedClient.Client().Delete(ctx, &dnsv1alpha1.DNSProvider{ObjectMeta: metav1.ObjectMeta{Namespace: b.Shoot.SeedNamespace, Name: name}}); err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	return kutil.WaitUntilResourceDeleted(ctx, b.K8sSeedClient.Client(), &dnsv1alpha1.DNSProvider{ObjectMeta: metav1.ObjectMeta{Namespace: b.Shoot.SeedNamespace, Name: name}}, 5*time.Second)
}

func (b *Botanist) deployDNSEntry(ctx context.Context, name, dnsName, target string) error {
	values := map[string]interface{}{
		"name":    name,
		"dnsName": dnsName,
		"targets": []string{target},
	}

	if err := b.ChartApplierSeed.Apply(ctx, filepath.Join(dnsChartPath, "entry"), b.Shoot.SeedNamespace, name, kubernetes.Values(values)); err != nil {
		return err
	}

	return b.waitUntilDNSEntryReady(ctx, name)
}

func (b *Botanist) waitUntilDNSEntryReady(ctx context.Context, name string) error {
	var (
		status  string
		message string
	)

	if err := retry.UntilTimeout(ctx, 5*time.Second, 2*time.Minute, func(ctx context.Context) (done bool, err error) {
		entry := &dnsv1alpha1.DNSEntry{}
		if err := b.K8sSeedClient.Client().Get(ctx, client.ObjectKey{Name: name, Namespace: b.Shoot.SeedNamespace}, entry); err != nil {
			return retry.SevereError(err)
		}

		if entry.Status.ObservedGeneration == entry.Generation && entry.Status.State == dnsv1alpha1.STATE_READY {
			return retry.Ok()
		}

		status = entry.Status.State
		if msg := entry.Status.Message; msg != nil {
			message = *msg
		}

		b.Logger.Infof("Waiting for %q DNS record to be ready... (status=%s, message=%s)", name, status, message)
		return retry.MinorError(fmt.Errorf("DNS record %q is not ready (status=%s, message=%s)", name, status, message))
	}); err != nil {
		return gardencorev1beta1helper.DetermineError(fmt.Sprintf("Failed to create %q DNS record: %q (status=%s, message=%s)", name, err.Error(), status, message))
	}

	return nil
}

func (b *Botanist) deleteDNSEntry(ctx context.Context, name string) error {
	if err := b.K8sSeedClient.Client().Delete(ctx, &dnsv1alpha1.DNSEntry{ObjectMeta: metav1.ObjectMeta{Namespace: b.Shoot.SeedNamespace, Name: name}}); err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	return kutil.WaitUntilResourceDeleted(ctx, b.K8sSeedClient.Client(), &dnsv1alpha1.DNSEntry{ObjectMeta: metav1.ObjectMeta{Namespace: b.Shoot.SeedNamespace, Name: name}}, 5*time.Second)
}

// GenerateDNSProviderName creates a name for the dns provider out of the passed `secretName` and `providerType`.
func GenerateDNSProviderName(secretName, providerType string) string {
	switch {
	case secretName != "" && providerType != "":
		return fmt.Sprintf("%s-%s", providerType, secretName)
	case secretName != "":
		return secretName
	case providerType != "":
		return providerType
	default:
		return ""
	}
}
