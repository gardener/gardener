// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
)

// DeployInternalDNSResources deploys the appropriate internal DNS resources.
// * The DNSProvider, DNSEntry, and DNSOwner resources are deleted (if they exist)
// in the appropriate order to ensure that the DNS record is not deleted.
// Then, the DNSRecord resource is deployed (or restored).
func (b *Botanist) DeployInternalDNSResources(ctx context.Context) error {
	if err := b.MigrateInternalDNS(ctx); err != nil {
		return err
	}
	return b.DeployOrDestroyInternalDNSRecord(ctx)
}

// DeployExternalDNSResources deploys the appropriate external DNS resources.
// * The DNSProvider, DNSEntry, and DNSOwner resources are deleted (if they exist)
// in the appropriate order to ensure that the DNS record is not deleted.
// Then, the DNSRecord resource is deployed (or restored).
func (b *Botanist) DeployExternalDNSResources(ctx context.Context) error {
	if err := b.MigrateExternalDNS(ctx, true); err != nil {
		return err
	}
	return b.DeployOrDestroyExternalDNSRecord(ctx)
}

// DeployIngressDNSResources deploys the appropriate ingress DNS resources.
// * The DNSEntry and DNSOwner resources are deleted (if they exist)
// in the appropriate order to ensure that the DNS record is not deleted.
// Then, the DNSRecord resource is deployed (or restored).
func (b *Botanist) DeployIngressDNSResources(ctx context.Context) error {
	if err := b.MigrateIngressDNS(ctx); err != nil {
		return err
	}
	return b.DeployOrDestroyIngressDNSRecord(ctx)
}

// DeployOwnerDNSResources deploys or deletes the owner DNSRecord resource depending on whether
// the 'ownerChecks' setting is enabled.
// * If the ownerChecks is enabled, the DNSRecord resource is deployed (or restored).
// * Otherwise, it is deleted.
func (b *Botanist) DeployOwnerDNSResources(ctx context.Context) error {
	if gardencorev1beta1helper.SeedSettingOwnerChecksEnabled(b.Seed.GetInfo().Spec.Settings) {
		return b.DeployOrDestroyOwnerDNSRecord(ctx)
	} else {
		return b.DestroyOwnerDNSRecord(ctx)
	}
}

// DestroyInternalDNSResources deletes all internal DNS resources (DNSProvider, DNSEntry, DNSOwner, and DNSRecord)
// that currently exist, to ensure that the DNS record is deleted.
func (b *Botanist) DestroyInternalDNSResources(ctx context.Context) error {
	if err := b.DestroyInternalDNS(ctx); err != nil {
		return err
	}
	return b.DestroyInternalDNSRecord(ctx)
}

// DestroyExternalDNSResources deletes all external DNS resources (DNSProvider, DNSEntry, DNSOwner, and DNSRecord)
// that currently exist, to ensure that the DNS record is deleted.
func (b *Botanist) DestroyExternalDNSResources(ctx context.Context) error {
	if err := b.DestroyExternalDNS(ctx); err != nil {
		return err
	}
	return b.DestroyExternalDNSRecord(ctx)
}

// DestroyIngressDNSResources deletes all ingress DNS resources (DNSEntry, DNSOwner, and DNSRecord)
// that currently exist, to ensure that the DNS record is deleted.
func (b *Botanist) DestroyIngressDNSResources(ctx context.Context) error {
	if err := b.DestroyIngressDNS(ctx); err != nil {
		return err
	}
	return b.DestroyIngressDNSRecord(ctx)
}

// DestroyOwnerDNSResources deletes the owner DNSRecord resource if it exists.
func (b *Botanist) DestroyOwnerDNSResources(ctx context.Context) error {
	return b.DestroyOwnerDNSRecord(ctx)
}

// MigrateInternalDNSResources migrates or deletes all internal DNS resources (DNSProvider, DNSEntry, DNSOwner, and DNSRecord)
// that currently exist, in the appropriate order to ensure that the DNS record is not deleted.
func (b *Botanist) MigrateInternalDNSResources(ctx context.Context) error {
	if err := b.MigrateInternalDNS(ctx); err != nil {
		return err
	}
	return b.MigrateInternalDNSRecord(ctx)
}

// MigrateExternalDNSResources migrates or deletes all external DNS resources (DNSProvider, DNSEntry, DNSOwner, and DNSRecord)
// that currently exist, in the appropriate order to ensure that the DNS record is not deleted.
func (b *Botanist) MigrateExternalDNSResources(ctx context.Context) error {
	if err := b.MigrateExternalDNS(ctx, false); err != nil {
		return err
	}
	return b.MigrateExternalDNSRecord(ctx)
}

// MigrateIngressDNSResources migrates or deletes all ingress DNS resources (DNSEntry, DNSOwner, and DNSRecord)
// that currently exist, in the appropriate order to ensure that the DNS record is not deleted.
func (b *Botanist) MigrateIngressDNSResources(ctx context.Context) error {
	if err := b.MigrateIngressDNS(ctx); err != nil {
		return err
	}
	return b.MigrateIngressDNSRecord(ctx)
}

// MigrateOwnerDNSResources migrates or deletes the owner DNSRecord resource depending on whether
// the 'ownerChecks' setting is enabled.
// * If the ownerChecks is enabled, the DNSRecord resource is migrated.
// * Otherwise, it is deleted.
func (b *Botanist) MigrateOwnerDNSResources(ctx context.Context) error {
	if gardencorev1beta1helper.SeedSettingOwnerChecksEnabled(b.Seed.GetInfo().Spec.Settings) {
		return b.MigrateOwnerDNSRecord(ctx)
	} else {
		return b.DestroyOwnerDNSRecord(ctx)
	}
}
