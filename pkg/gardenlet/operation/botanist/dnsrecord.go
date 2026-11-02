// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1helper "github.com/gardener/gardener/pkg/api/core/v1beta1/helper"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component"
	extensionsdnsrecord "github.com/gardener/gardener/pkg/component/extensions/dnsrecord"
	"github.com/gardener/gardener/pkg/controllerutils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

// DefaultExternalDNSRecord creates the default deployer for the external DNSRecord resource.
func (b *Botanist) DefaultExternalDNSRecord() extensionsdnsrecord.Interface {
	values := b.dnsRecordValues(v1beta1constants.DNSRecordExternalName, v1beta1constants.LabelDNSRecordExternal, b.annotateExternalDNSRecordOperation())
	return b.newDNSRecord(values, b.NeedsExternalDNS(), b.Shoot.ExternalDomain, apiServerDomain(b.Shoot.ExternalClusterDomain))
}

// DefaultPriorExternalDNSRecord creates the default deployer for the DNSRecord resource of the prior external domain.
func (b *Botanist) DefaultPriorExternalDNSRecord() extensionsdnsrecord.Interface {
	values := b.dnsRecordValues(v1beta1constants.DNSRecordPriorExternalName, v1beta1constants.LabelDNSRecordPriorExternal, b.annotateExternalDNSRecordOperation())
	return b.newDNSRecord(values, b.NeedsPriorExternalDNS(), b.Shoot.PriorExternalDomain, apiServerDomain(b.Shoot.PriorExternalClusterDomain))
}

// DefaultInternalDNSRecord creates the default deployer for the internal DNSRecord resource.
func (b *Botanist) DefaultInternalDNSRecord() extensionsdnsrecord.Interface {
	deleting := b.Shoot.GetInfo().DeletionTimestamp != nil
	values := b.dnsRecordValues(v1beta1constants.DNSRecordInternalName, v1beta1constants.LabelDNSRecordInternal,
		deleting || controllerutils.HasTask(b.Shoot.GetInfo().Annotations, v1beta1constants.ShootTaskDeployDNSRecordInternal) || b.Shoot.IsRestorePhase())
	values.ReconcileOnlyOnChangeOrError = deleting

	internalDomain := b.Shoot.SelfHostedShootInternalAPIServerDomain()
	if !b.NeedsInternalDNS() && internalDomain != nil && b.NeedsExternalDNS() {
		// Self-hosted shoots don't have an internal domain configured by the operator. Instead, the internal DNSRecord
		// is a sibling of the external DNSRecord (using the same provider and credentials), which points to the internal
		// IPs of the control plane nodes instead of the control plane exposure.
		return b.newDNSRecord(values, true, b.Shoot.ExternalDomain, *internalDomain)
	}

	return b.newDNSRecord(values, b.NeedsInternalDNS(), b.Garden.InternalDomain, apiServerDomain(b.Shoot.InternalClusterDomain))
}

func (b *Botanist) annotateExternalDNSRecordOperation() bool {
	return controllerutils.HasTask(b.Shoot.GetInfo().Annotations, v1beta1constants.ShootTaskDeployDNSRecordExternal) || b.Shoot.IsRestorePhase()
}

func apiServerDomain(clusterDomain *string) string {
	if clusterDomain == nil {
		return ""
	}
	return v1beta1helper.GetAPIServerDomain(*clusterDomain)
}

func (b *Botanist) dnsRecordValues(name, role string, annotateOperation bool) *extensionsdnsrecord.Values {
	return &extensionsdnsrecord.Values{
		Name:              b.Shoot.GetInfo().Name + "-" + name,
		SecretName:        DNSRecordSecretPrefix + "-" + b.Shoot.GetInfo().Name + "-" + name,
		Namespace:         b.Shoot.ControlPlaneNamespace,
		TTL:               b.dnsRecordTTLSeconds(),
		AnnotateOperation: annotateOperation,
		IPStack:           gardenerutils.GetIPStackForShoot(b.Shoot.GetInfo()),
		Labels: map[string]string{
			v1beta1constants.LabelRole:  role,
			v1beta1constants.GardenRole: v1beta1constants.GardenRoleControlPlane,
		},
	}
}

func (b *Botanist) newDNSRecord(values *extensionsdnsrecord.Values, needed bool, domain *gardenerutils.Domain, clusterAPIServerDomain string) extensionsdnsrecord.Interface {
	var credentialsDeployer extensionsdnsrecord.CredentialsDeployFunc

	if needed {
		values.Type = domain.Provider
		if domain.Zone != "" {
			values.Zone = &domain.Zone
		}
		credentialsDeployer = extensionsdnsrecord.CredentialsDeployerFromCredentials(domain.Credentials, b.Shoot.GetInfo())
		values.DNSName = clusterAPIServerDomain
	}

	return extensionsdnsrecord.New(
		b.Logger,
		b.SeedClientSet.Client(),
		values,
		extensionsdnsrecord.DefaultInterval,
		extensionsdnsrecord.DefaultSevereThreshold,
		extensionsdnsrecord.DefaultTimeout,
		credentialsDeployer,
	)
}

// DeployOrDestroyExternalDNSRecord deploys, restores, or destroys the external DNSRecord and waits for the operation to complete.
func (b *Botanist) DeployOrDestroyExternalDNSRecord(ctx context.Context) error {
	if b.NeedsExternalDNS() {
		return b.deployExternalDNSRecord(ctx)
	}
	return b.DestroyExternalDNSRecord(ctx)
}

// DeployOrDestroyPriorExternalDNSRecord deploys, restores, or destroys the prior external DNSRecord and waits for the
// operation to complete.
func (b *Botanist) DeployOrDestroyPriorExternalDNSRecord(ctx context.Context) error {
	if b.NeedsPriorExternalDNS() {
		return b.deployPriorExternalDNSRecord(ctx)
	}
	return b.DestroyPriorExternalDNSRecord(ctx)
}

// DeployOrDestroyInternalDNSRecord deploys, restores, or destroys the internal DNSRecord and waits for the operation to complete.
func (b *Botanist) DeployOrDestroyInternalDNSRecord(ctx context.Context) error {
	if b.NeedsInternalDNS() {
		return b.deployInternalDNSRecord(ctx)
	}
	return b.DestroyInternalDNSRecord(ctx)
}

// deployExternalDNSRecord deploys or restores the external DNSRecord and waits for the operation to complete.
func (b *Botanist) deployExternalDNSRecord(ctx context.Context) error {
	if b.NeedsPriorExternalDNS() {
		if err := b.handOverExternalDNSRecord(ctx); err != nil {
			return err
		}
	}

	if err := b.deployOrRestoreDNSRecord(ctx, b.Shoot.Components.Extensions.ExternalDNSRecord); err != nil {
		return err
	}
	return b.Shoot.Components.Extensions.ExternalDNSRecord.Wait(ctx)
}

func (b *Botanist) handOverExternalDNSRecord(ctx context.Context) error {
	dnsRecord := &extensionsv1alpha1.DNSRecord{}
	if err := b.SeedClientSet.Client().Get(ctx, client.ObjectKey{Namespace: b.Shoot.ControlPlaneNamespace, Name: b.Shoot.GetInfo().Name + "-" + v1beta1constants.DNSRecordExternalName}, dnsRecord); err != nil {
		return client.IgnoreNotFound(err)
	}

	if dnsRecord.Spec.Name == v1beta1helper.GetAPIServerDomain(*b.Shoot.ExternalClusterDomain) {
		return nil
	}

	// spec.name of a DNSRecord is immutable, so the record must be deleted and recreated for the new domain. A plain
	// deletion would make the DNS extension delete the DNS entry of the old domain, which the prior external DNSRecord
	// (deployed before) now also serves. The "migrate" operation makes the DNS extension keep the DNS entry and remove its
	// finalizers. Hence, the following deletion only removes the DNSRecord object, and the DNS entry stays in place.
	if err := b.MigrateExternalDNSRecord(ctx); err != nil {
		return err
	}
	return b.DestroyExternalDNSRecord(ctx)
}

func (b *Botanist) deployPriorExternalDNSRecord(ctx context.Context) error {
	if err := b.deployOrRestoreDNSRecord(ctx, b.Shoot.Components.Extensions.PriorExternalDNSRecord); err != nil {
		return err
	}
	return b.Shoot.Components.Extensions.PriorExternalDNSRecord.Wait(ctx)
}

// deployInternalDNSRecord deploys or restores the internal DNSRecord and waits for the operation to complete.
func (b *Botanist) deployInternalDNSRecord(ctx context.Context) error {
	if err := b.deployOrRestoreDNSRecord(ctx, b.Shoot.Components.Extensions.InternalDNSRecord); err != nil {
		return err
	}
	return b.Shoot.Components.Extensions.InternalDNSRecord.Wait(ctx)
}

// DestroyExternalDNSRecord destroys the external DNSRecord and waits for the operation to complete.
func (b *Botanist) DestroyExternalDNSRecord(ctx context.Context) error {
	if err := b.Shoot.Components.Extensions.ExternalDNSRecord.Destroy(ctx); err != nil {
		return err
	}
	return b.Shoot.Components.Extensions.ExternalDNSRecord.WaitCleanup(ctx)
}

// DestroyPriorExternalDNSRecord destroys the prior external DNSRecord and waits for the operation to complete.
func (b *Botanist) DestroyPriorExternalDNSRecord(ctx context.Context) error {
	if err := b.Shoot.Components.Extensions.PriorExternalDNSRecord.Destroy(ctx); err != nil {
		return err
	}
	return b.Shoot.Components.Extensions.PriorExternalDNSRecord.WaitCleanup(ctx)
}

// DestroyInternalDNSRecord destroys the internal DNSRecord and waits for the operation to complete.
func (b *Botanist) DestroyInternalDNSRecord(ctx context.Context) error {
	if err := b.Shoot.Components.Extensions.InternalDNSRecord.Destroy(ctx); err != nil {
		return err
	}
	return b.Shoot.Components.Extensions.InternalDNSRecord.WaitCleanup(ctx)
}

// MigrateExternalDNSRecord migrates the external DNSRecord and waits for the operation to complete.
func (b *Botanist) MigrateExternalDNSRecord(ctx context.Context) error {
	if err := b.Shoot.Components.Extensions.ExternalDNSRecord.Migrate(ctx); err != nil {
		return err
	}
	return b.Shoot.Components.Extensions.ExternalDNSRecord.WaitMigrate(ctx)
}

// MigratePriorExternalDNSRecord migrates the prior external DNSRecord and waits for the operation to complete.
func (b *Botanist) MigratePriorExternalDNSRecord(ctx context.Context) error {
	if err := b.Shoot.Components.Extensions.PriorExternalDNSRecord.Migrate(ctx); err != nil {
		return err
	}
	return b.Shoot.Components.Extensions.PriorExternalDNSRecord.WaitMigrate(ctx)
}

// MigrateInternalDNSRecord migrates the internal DNSRecord and waits for the operation to complete.
func (b *Botanist) MigrateInternalDNSRecord(ctx context.Context) error {
	if err := b.Shoot.Components.Extensions.InternalDNSRecord.Migrate(ctx); err != nil {
		return err
	}
	return b.Shoot.Components.Extensions.InternalDNSRecord.WaitMigrate(ctx)
}

func (b *Botanist) deployOrRestoreDNSRecord(ctx context.Context, dnsRecord component.DeployMigrateWaiter) error {
	if b.Shoot.IsRestorePhase() {
		return dnsRecord.Restore(ctx, b.Shoot.GetShootState())
	}
	return dnsRecord.Deploy(ctx)
}

func (b *Botanist) dnsRecordTTLSeconds() *int64 {
	if b.Config != nil && b.Config.Controllers != nil && b.Config.Controllers.Shoot != nil {
		return b.Config.Controllers.Shoot.DNSEntryTTLSeconds
	}
	return new(int64(120))
}
