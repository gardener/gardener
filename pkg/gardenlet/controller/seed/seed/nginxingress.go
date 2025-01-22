// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package seed

import (
	"context"
	"encoding/json"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1helper "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1/helper"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/extensions/dnsrecord"
	seedpkg "github.com/gardener/gardener/pkg/gardenlet/operation/seed"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

func (r *Reconciler) newIngressDNSRecord(ctx context.Context, log logr.Logger, seed *seedpkg.Seed, loadBalancerAddress string) (component.DeployMigrateWaiter, error) {
	secretData, err := getDNSProviderSecretData(ctx, r.GardenClient, seed.GetInfo())
	if err != nil {
		return nil, err
	}

	values := &dnsrecord.Values{
		Name:                         "seed-ingress",
		SecretName:                   "seed-ingress",
		Namespace:                    r.GardenNamespace,
		SecretData:                   secretData,
		DNSName:                      seed.GetIngressFQDN("*"),
		RecordType:                   extensionsv1alpha1helper.GetDNSRecordType(loadBalancerAddress),
		ReconcileOnlyOnChangeOrError: true,
		IPStack:                      gardenerutils.GetIPStackForSeed(seed.GetInfo()),
	}

	if provider := seed.GetInfo().Spec.DNS.Provider; provider != nil {
		values.Type = provider.Type
	}

	if loadBalancerAddress != "" {
		values.Values = []string{loadBalancerAddress}
	}

	return dnsrecord.New(
		log,
		r.SeedClientSet.Client(),
		values,
		dnsrecord.DefaultInterval,
		dnsrecord.DefaultSevereThreshold,
		dnsrecord.DefaultTimeout,
	), nil
}

func getDNSProviderSecretData(ctx context.Context, gardenClient client.Client, seed *gardencorev1beta1.Seed) (map[string][]byte, error) {
	if dnsConfig := seed.Spec.DNS; dnsConfig.Provider != nil {
		secret, err := kubernetesutils.GetSecretByReference(ctx, gardenClient, &dnsConfig.Provider.SecretRef)
		if err != nil {
			return nil, err
		}
		return secret.Data, nil
	}
	return nil, nil
}

func getConfig(seed *gardencorev1beta1.Seed) (map[string]string, error) {
	var (
		defaultConfig = map[string]any{
			"server-name-hash-bucket-size": "256",
			"use-proxy-protocol":           "false",
			"worker-processes":             "2",
			"allow-snippet-annotations":    "true",
			// This is needed to override the default which is "High" starting from nginx-ingress-controller v1.12.0
			// and we use the nginx.ingress.kubernetes.io/server-snippet annotation in our plutono and alertmanager ingresses.
			// This is acceptable for the seed as we control the ingress resources solely and no malicious configuration can be injected by users.
			// See https://github.com/gardener/gardener/pull/11087 for more details.
			"annotations-risk-level": "Critical",
		}
		providerConfig = map[string]any{}
	)
	if seed.Spec.Ingress != nil && seed.Spec.Ingress.Controller.ProviderConfig != nil {
		if err := json.Unmarshal(seed.Spec.Ingress.Controller.ProviderConfig.Raw, &providerConfig); err != nil {
			return nil, err
		}
	}

	return utils.InterfaceMapToStringMap(utils.MergeMaps(defaultConfig, providerConfig)), nil
}
