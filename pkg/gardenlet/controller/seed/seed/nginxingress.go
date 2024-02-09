// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	seedpkg "github.com/gardener/gardener/pkg/operation/seed"
	"github.com/gardener/gardener/pkg/utils"
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
		defaultConfig = map[string]interface{}{
			"server-name-hash-bucket-size": "256",
			"use-proxy-protocol":           "false",
			"worker-processes":             "2",
			"allow-snippet-annotations":    "true",
		}
		providerConfig = map[string]interface{}{}
	)
	if seed.Spec.Ingress != nil && seed.Spec.Ingress.Controller.ProviderConfig != nil {
		if err := json.Unmarshal(seed.Spec.Ingress.Controller.ProviderConfig.Raw, &providerConfig); err != nil {
			return nil, err
		}
	}

	return utils.InterfaceMapToStringMap(utils.MergeMaps(defaultConfig, providerConfig)), nil
}
