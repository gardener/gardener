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

	gardencorev1alpha1helper "github.com/gardener/gardener/pkg/apis/core/v1alpha1/helper"
	"github.com/gardener/gardener/pkg/operation/common"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	dnsv1alpha1 "github.com/gardener/external-dns-management/pkg/apis/dns/v1alpha1"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

var dnsChartPath = filepath.Join(common.ChartPath, "seed-dns")

const (
	// DNSPurposeInternal is a constant for a DNS record used for the internal domain name.
	DNSPurposeInternal = "internal"
	// DNSPurposeExternal is a constant for a DNS record used for the external domain name.
	DNSPurposeExternal = "external"
)

// DeployInternalDomainDNSRecord deploys the DNS record for the internal cluster domain.
func (b *Botanist) DeployInternalDomainDNSRecord(ctx context.Context) error {
	if err := b.deployDNSProvider(ctx, DNSPurposeInternal, b.Garden.InternalDomain.Provider, b.Garden.InternalDomain.SecretData, b.Shoot.InternalClusterDomain); err != nil {
		return err
	}
	if err := b.deployDNSEntry(ctx, DNSPurposeInternal, b.Shoot.InternalClusterDomain, b.APIServerAddress); err != nil {
		return err
	}
	return b.deleteLegacyTerraformDNSResources(ctx, common.TerraformerPurposeInternalDNSDeprecated)
}

// DestroyInternalDomainDNSRecord destroys the DNS record for the internal cluster domain.
func (b *Botanist) DestroyInternalDomainDNSRecord(ctx context.Context) error {
	if err := b.deleteDNSEntry(ctx, DNSPurposeInternal); err != nil {
		return err
	}
	return b.deleteDNSProvider(ctx, DNSPurposeInternal)
}

// DeployExternalDomainDNSRecord deploys the DNS record for the external cluster domain.
func (b *Botanist) DeployExternalDomainDNSRecord(ctx context.Context) error {
	if b.Shoot.Info.Spec.DNS.Domain == nil || b.Shoot.ExternalClusterDomain == nil || strings.HasSuffix(*b.Shoot.ExternalClusterDomain, ".nip.io") {
		return nil
	}

	if err := b.deployDNSProvider(ctx, DNSPurposeExternal, b.Shoot.ExternalDomain.Provider, b.Shoot.ExternalDomain.SecretData, *b.Shoot.Info.Spec.DNS.Domain); err != nil {
		return err
	}
	if err := b.deployDNSEntry(ctx, DNSPurposeExternal, *b.Shoot.ExternalClusterDomain, b.Shoot.InternalClusterDomain); err != nil {
		return err
	}
	return b.deleteLegacyTerraformDNSResources(ctx, common.TerraformerPurposeExternalDNSDeprecated)
}

// DestroyExternalDomainDNSRecord destroys the DNS record for the external cluster domain.
func (b *Botanist) DestroyExternalDomainDNSRecord(ctx context.Context) error {
	if err := b.deleteDNSEntry(ctx, DNSPurposeExternal); err != nil {
		return err
	}
	return b.deleteDNSProvider(ctx, DNSPurposeExternal)
}

func (b *Botanist) deployDNSProvider(ctx context.Context, name, provider string, secretData map[string][]byte, includedDomains ...string) error {
	values := map[string]interface{}{
		"name":       name,
		"provider":   provider,
		"secretData": secretData,
		"domains": map[string]interface{}{
			"include": includedDomains,
		},
	}

	if err := b.ChartApplierSeed.ApplyChart(ctx, filepath.Join(dnsChartPath, "provider"), b.Shoot.SeedNamespace, name, nil, values); err != nil {
		return err
	}

	return b.waitUntilDNSProviderReady(ctx, name)
}

func (b *Botanist) waitUntilDNSProviderReady(ctx context.Context, name string) error {
	var (
		status  string
		message string
	)

	if err := wait.PollImmediate(5*time.Second, 2*time.Minute, func() (bool, error) {
		provider := &dnsv1alpha1.DNSProvider{}
		if err := b.K8sSeedClient.Client().Get(ctx, client.ObjectKey{Name: name, Namespace: b.Shoot.SeedNamespace}, provider); err != nil {
			return false, err
		}

		if provider.Status.State == dnsv1alpha1.STATE_READY {
			return true, nil
		}

		status = provider.Status.State
		if msg := provider.Status.Message; msg != nil {
			message = *msg
		}

		b.Logger.Infof("Waiting for %q DNS provider to be ready... (status=%s, message=%s)", name, status, message)
		return false, nil
	}); err != nil {
		return gardencorev1alpha1helper.DetermineError(fmt.Sprintf("Failed to create DNS provider for %q DNS record: %q (status=%s, message=%s)", name, err.Error(), status, message))
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

	if err := b.ChartApplierSeed.ApplyChart(ctx, filepath.Join(dnsChartPath, "entry"), b.Shoot.SeedNamespace, name, nil, values); err != nil {
		return err
	}

	return b.waitUntilDNSEntryReady(ctx, name)
}

func (b *Botanist) waitUntilDNSEntryReady(ctx context.Context, name string) error {
	var (
		status  string
		message string
	)

	if err := wait.PollImmediate(5*time.Second, 2*time.Minute, func() (bool, error) {
		entry := &dnsv1alpha1.DNSEntry{}
		if err := b.K8sSeedClient.Client().Get(ctx, client.ObjectKey{Name: name, Namespace: b.Shoot.SeedNamespace}, entry); err != nil {
			return false, err
		}

		if entry.Status.ObservedGeneration == entry.Generation && entry.Status.State == dnsv1alpha1.STATE_READY {
			return true, nil
		}

		status = entry.Status.State
		if msg := entry.Status.Message; msg != nil {
			message = *msg
		}

		b.Logger.Infof("Waiting for %q DNS record to be ready... (status=%s, message=%s)", name, status, message)
		return false, nil
	}); err != nil {
		return gardencorev1alpha1helper.DetermineError(fmt.Sprintf("Failed to create %q DNS record: %q (status=%s, message=%s)", name, err.Error(), status, message))
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

func (b *Botanist) deleteLegacyTerraformDNSResources(ctx context.Context, purpose string) error {
	tf, err := b.NewShootTerraformer(purpose)
	if err != nil {
		return err
	}

	return tf.CleanupConfiguration(ctx)
}
