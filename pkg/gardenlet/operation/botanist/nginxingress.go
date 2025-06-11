// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1helper "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1/helper"
	"github.com/gardener/gardener/pkg/component"
	extensionsdnsrecord "github.com/gardener/gardener/pkg/component/extensions/dnsrecord"
	sharedcomponent "github.com/gardener/gardener/pkg/component/shared"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

// DefaultNginxIngress returns a deployer for the nginxingress.
func (b *Botanist) DefaultNginxIngress() (component.DeployWaiter, error) {
	var (
		configData               map[string]string
		loadBalancerSourceRanges []string
		externalTrafficPolicy    corev1.ServiceExternalTrafficPolicy
	)

	if nginxIngressSpec := b.Shoot.GetInfo().Spec.Addons.NginxIngress; nginxIngressSpec != nil {
		configData = getConfig(nginxIngressSpec.Config)

		if nginxIngressSpec.LoadBalancerSourceRanges != nil {
			loadBalancerSourceRanges = nginxIngressSpec.LoadBalancerSourceRanges
		}
		if nginxIngressSpec.ExternalTrafficPolicy != nil {
			externalTrafficPolicy = *nginxIngressSpec.ExternalTrafficPolicy
		}
	}

	return sharedcomponent.NewNginxIngress(
		b.SeedClientSet.Client(),
		b.Shoot.ControlPlaneNamespace,
		metav1.NamespaceSystem,
		b.Shoot.KubernetesVersion,
		configData,
		nil,
		loadBalancerSourceRanges,
		v1beta1constants.PriorityClassNameShootSystem600,
		b.Shoot.WantsVerticalPodAutoscaler,
		component.ClusterTypeShoot,
		externalTrafficPolicy,
		v1beta1constants.ShootNginxIngressClass,
		nil,
		nil,
		false,
	)
}

// DeployNginxIngressAddon deploys the NginxIngress Addon component.
func (b *Botanist) DeployNginxIngressAddon(ctx context.Context) error {
	if !v1beta1helper.NginxIngressEnabled(b.Shoot.GetInfo().Spec.Addons) {
		return b.Shoot.Components.Addons.NginxIngress.Destroy(ctx)
	}

	return b.Shoot.Components.Addons.NginxIngress.Deploy(ctx)
}

func getConfig(config map[string]string) map[string]string {
	var (
		defaultConfig = map[string]string{
			"server-name-hash-bucket-size": "256",
			"use-proxy-protocol":           "false",
			"worker-processes":             "2",
		}
	)
	if config != nil {
		return utils.MergeStringMaps(defaultConfig, config)
	}
	return defaultConfig
}

// NeedsIngressDNS returns true if the Shoot cluster needs ingress DNS.
func (b *Botanist) NeedsIngressDNS() bool {
	return b.NeedsExternalDNS() && v1beta1helper.NginxIngressEnabled(b.Shoot.GetInfo().Spec.Addons)
}

// DefaultIngressDNSRecord creates the default deployer for the ingress DNSRecord resource.
func (b *Botanist) DefaultIngressDNSRecord() extensionsdnsrecord.Interface {
	values := &extensionsdnsrecord.Values{
		Name:              b.Shoot.GetInfo().Name + "-ingress",
		SecretName:        DNSRecordSecretPrefix + "-" + b.Shoot.GetInfo().Name + "-" + v1beta1constants.DNSRecordExternalName,
		Namespace:         b.Shoot.ControlPlaneNamespace,
		TTL:               b.dnsRecordTTLSeconds(),
		AnnotateOperation: controllerutils.HasTask(b.Shoot.GetInfo().Annotations, v1beta1constants.ShootTaskDeployDNSRecordIngress) || b.IsRestorePhase(),
		IPStack:           gardenerutils.GetIPStackForShoot(b.Shoot.GetInfo()),
	}

	// Set component values even if the nginx-ingress addons is not enabled.
	if b.NeedsExternalDNS() {
		values.Type = b.Shoot.ExternalDomain.Provider
		if b.Shoot.ExternalDomain.Zone != "" {
			values.Zone = &b.Shoot.ExternalDomain.Zone
		}
		values.SecretData = b.Shoot.ExternalDomain.SecretData
		values.DNSName = b.Shoot.GetIngressFQDN("*")
	}

	return extensionsdnsrecord.New(
		b.Logger,
		b.SeedClientSet.Client(),
		values,
		extensionsdnsrecord.DefaultInterval,
		extensionsdnsrecord.DefaultSevereThreshold,
		extensionsdnsrecord.DefaultTimeout,
	)
}

// DeployOrDestroyIngressDNSRecord deploys, restores, or destroys the ingress DNSRecord and waits for the operation to complete.
func (b *Botanist) DeployOrDestroyIngressDNSRecord(ctx context.Context) error {
	if b.NeedsIngressDNS() {
		return b.deployIngressDNSRecord(ctx)
	}
	return b.DestroyIngressDNSRecord(ctx)
}

// deployIngressDNSRecord deploys or restores the ingress DNSRecord and waits for the operation to complete.
func (b *Botanist) deployIngressDNSRecord(ctx context.Context) error {
	if err := b.deployOrRestoreDNSRecord(ctx, b.Shoot.Components.Extensions.IngressDNSRecord); err != nil {
		return err
	}
	return b.Shoot.Components.Extensions.IngressDNSRecord.Wait(ctx)
}

// DestroyIngressDNSRecord destroys the ingress DNSRecord and waits for the operation to complete.
func (b *Botanist) DestroyIngressDNSRecord(ctx context.Context) error {
	if err := b.Shoot.Components.Extensions.IngressDNSRecord.Destroy(ctx); err != nil {
		return err
	}
	return b.Shoot.Components.Extensions.IngressDNSRecord.WaitCleanup(ctx)
}

// MigrateIngressDNSRecord migrates the ingress DNSRecord and waits for the operation to complete.
func (b *Botanist) MigrateIngressDNSRecord(ctx context.Context) error {
	if err := b.Shoot.Components.Extensions.IngressDNSRecord.Migrate(ctx); err != nil {
		return err
	}
	return b.Shoot.Components.Extensions.IngressDNSRecord.WaitMigrate(ctx)
}

// SetNginxIngressAddress sets the IP address of the API server's LoadBalancer.
func (b *Botanist) SetNginxIngressAddress(address string) {
	if b.NeedsIngressDNS() {
		b.Shoot.Components.Extensions.IngressDNSRecord.SetRecordType(extensionsv1alpha1helper.GetDNSRecordType(address))
		b.Shoot.Components.Extensions.IngressDNSRecord.SetValues([]string{address})
	}
}
