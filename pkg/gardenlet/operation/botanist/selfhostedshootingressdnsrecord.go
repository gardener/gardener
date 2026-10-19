// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"fmt"
	"time"

	extensionsv1alpha1helper "github.com/gardener/gardener/pkg/api/extensions/v1alpha1/helper"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/component"
	extensionsdnsrecord "github.com/gardener/gardener/pkg/component/extensions/dnsrecord"
	"github.com/gardener/gardener/pkg/component/networking/istio"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// DefaultSelfHostedShootIngressDNSRecord creates the deployer for the ingress DNSRecord of a self-hosted shoot cluster.
// It is a wildcard record ('*.ingress.<Shoot.Spec.DNS.Domain>') resolving to the LoadBalancer address of the Istio
// ingress gateway that this (kube-system) gardenlet deploys inside the self-hosted shoot (see DefaultIstio).
func (b *Botanist) DefaultSelfHostedShootIngressDNSRecord() extensionsdnsrecord.Interface {
	if !b.Shoot.IsSelfHosted() || !b.NeedsExternalDNS() {
		return nil
	}

	values := &extensionsdnsrecord.Values{
		Name:                         b.Shoot.GetInfo().Name + "-" + v1beta1constants.LabelDNSRecordIngress,
		SecretName:                   DNSRecordSecretPrefix + "-" + b.Shoot.GetInfo().Name + "-" + v1beta1constants.LabelDNSRecordIngress,
		Namespace:                    b.Shoot.ControlPlaneNamespace,
		TTL:                          b.dnsRecordTTLSeconds(),
		DNSName:                      b.Shoot.GetIngressFQDN("*"),
		Type:                         b.Shoot.ExternalDomain.Provider,
		ReconcileOnlyOnChangeOrError: true,
		IPStack:                      gardenerutils.GetIPStackForShoot(b.Shoot.GetInfo()),
		Labels: map[string]string{
			v1beta1constants.LabelRole:  v1beta1constants.LabelDNSRecordIngress,
			v1beta1constants.GardenRole: v1beta1constants.GardenRoleControlPlane,
		},
	}

	if b.Shoot.ExternalDomain.Zone != "" {
		values.Zone = &b.Shoot.ExternalDomain.Zone
	}

	return extensionsdnsrecord.New(
		b.Logger,
		b.SeedClientSet.Client(),
		values,
		extensionsdnsrecord.DefaultInterval,
		extensionsdnsrecord.DefaultSevereThreshold,
		extensionsdnsrecord.DefaultTimeout,
		extensionsdnsrecord.CredentialsDeployerFromCredentials(b.Shoot.ExternalDomain.Credentials, b.Shoot.GetInfo()),
	)
}

// DeploySelfHostedShootIngressDNSRecord resolves the LoadBalancer address of the self-hosted shoot's Istio ingress
// gateway and deploys the ingress DNSRecord pointing to it. It is a no-op if no deployer was instantiated (i.e. for
// non-self-hosted shoots or shoots without managed external DNS).
func (b *Botanist) DeploySelfHostedShootIngressDNSRecord(ctx context.Context) error {
	dnsRecord := b.Shoot.Components.Extensions.SelfHostedShootIngressDNSRecord
	if dnsRecord == nil {
		return nil
	}

	loadBalancerAddress, err := kubernetesutils.WaitUntilLoadBalancerIsReady(
		ctx,
		b.Logger,
		b.SeedClientSet.Client(),
		istio.SelfHostedShootNamePrefix+v1beta1constants.DefaultSNIIngressNamespace,
		v1beta1constants.DefaultSNIIngressServiceName,
		time.Minute,
	)
	if err != nil {
		return fmt.Errorf("failed waiting for self-hosted shoot Istio ingress gateway LoadBalancer to be ready: %w", err)
	}

	dnsRecord.SetRecordType(extensionsv1alpha1helper.GetDNSRecordType(loadBalancerAddress))
	dnsRecord.SetValues([]string{loadBalancerAddress})

	return component.OpWait(dnsRecord).Deploy(ctx)
}
