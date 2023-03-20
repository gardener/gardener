// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	"k8s.io/utils/pointer"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1helper "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1/helper"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	extensionsdnsrecord "github.com/gardener/gardener/pkg/operation/botanist/component/extensions/dnsrecord"
	"github.com/gardener/gardener/pkg/operation/botanist/component/nginxingressshoot"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/images"
	"github.com/gardener/gardener/pkg/utils/imagevector"
)

// DefaultNginxIngress returns a deployer for the nginxingress.
func (b *Botanist) DefaultNginxIngress() (component.DeployWaiter, error) {
	imageController, err := b.ImageVector.FindImage(images.ImageNameNginxIngressController, imagevector.RuntimeVersion(b.ShootVersion()), imagevector.TargetVersion(b.ShootVersion()))
	if err != nil {
		return nil, err
	}
	imageDefaultBackend, err := b.ImageVector.FindImage(images.ImageNameIngressDefaultBackend, imagevector.RuntimeVersion(b.ShootVersion()), imagevector.TargetVersion(b.ShootVersion()))
	if err != nil {
		return nil, err
	}

	values := nginxingressshoot.Values{
		NginxControllerImage: imageController.String(),
		DefaultBackendImage:  imageDefaultBackend.String(),
		KubernetesVersion:    b.Shoot.KubernetesVersion,
		VPAEnabled:           b.Shoot.WantsVerticalPodAutoscaler,
		PSPDisabled:          b.Shoot.PSPDisabled,
	}

	if b.APIServerSNIEnabled() {
		values.KubeAPIServerHost = pointer.String(b.outOfClusterAPIServerFQDN())
	}

	if nginxIngressSpec := b.Shoot.GetInfo().Spec.Addons.NginxIngress; nginxIngressSpec != nil {
		values.ConfigData = getConfig(nginxIngressSpec.Config)

		if nginxIngressSpec.LoadBalancerSourceRanges != nil {
			values.LoadBalancerSourceRanges = nginxIngressSpec.LoadBalancerSourceRanges
		}
		if nginxIngressSpec.ExternalTrafficPolicy != nil {
			values.ExternalTrafficPolicy = *nginxIngressSpec.ExternalTrafficPolicy
		}
	}

	return nginxingressshoot.New(b.SeedClientSet.Client(), b.Shoot.SeedNamespace, values), nil
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
		Name:              b.Shoot.GetInfo().Name + "-" + common.ShootDNSIngressName,
		SecretName:        DNSRecordSecretPrefix + "-" + b.Shoot.GetInfo().Name + "-" + v1beta1constants.DNSRecordExternalName,
		Namespace:         b.Shoot.SeedNamespace,
		TTL:               b.Config.Controllers.Shoot.DNSEntryTTLSeconds,
		AnnotateOperation: controllerutils.HasTask(b.Shoot.GetInfo().Annotations, v1beta1constants.ShootTaskDeployDNSRecordIngress) || b.isRestorePhase(),
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
