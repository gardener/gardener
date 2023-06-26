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
	"encoding/json"
	"fmt"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1helper "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1/helper"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/extensions/dnsrecord"
	"github.com/gardener/gardener/pkg/utils"
)

func getManagedIngressDNSRecord(
	log logr.Logger,
	seedClient client.Client,
	gardenNamespaceName string,
	dnsConfig gardencorev1beta1.SeedDNS,
	secretData map[string][]byte,
	seedFQDN string,
	loadBalancerAddress string,
) component.DeployMigrateWaiter {
	values := &dnsrecord.Values{
		Name:                         "seed-ingress",
		SecretName:                   "seed-ingress",
		Namespace:                    gardenNamespaceName,
		SecretData:                   secretData,
		DNSName:                      seedFQDN,
		RecordType:                   extensionsv1alpha1helper.GetDNSRecordType(loadBalancerAddress),
		ReconcileOnlyOnChangeOrError: true,
	}

	if dnsConfig.Provider != nil {
		values.Type = dnsConfig.Provider.Type
	}

	if loadBalancerAddress != "" {
		values.Values = []string{loadBalancerAddress}
	}

	return dnsrecord.New(
		log,
		seedClient,
		values,
		dnsrecord.DefaultInterval,
		dnsrecord.DefaultSevereThreshold,
		dnsrecord.DefaultTimeout,
	)
}

func getConfig(seed *gardencorev1beta1.Seed) (map[string]string, error) {
	var (
		defaultConfig = map[string]interface{}{
			"server-name-hash-bucket-size": "256",
			"use-proxy-protocol":           "false",
			"worker-processes":             "2",
		}
		providerConfig = map[string]interface{}{}
	)
	if seed.Spec.Ingress != nil && seed.Spec.Ingress.Controller.ProviderConfig != nil {
		if err := json.Unmarshal(seed.Spec.Ingress.Controller.ProviderConfig.Raw, &providerConfig); err != nil {
			return nil, err
		}
	}

	return interfaceMapToStringMap(utils.MergeMaps(defaultConfig, providerConfig)), nil
}

func interfaceMapToStringMap(in map[string]interface{}) map[string]string {
	m := make(map[string]string, len(in))
	for k, v := range in {
		m[k] = fmt.Sprint(v)
	}
	return m
}
