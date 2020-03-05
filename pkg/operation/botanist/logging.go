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

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/features"
	gardenletfeatures "github.com/gardener/gardener/pkg/gardenlet/features"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/secrets"

	corev1 "k8s.io/api/core/v1"
)

// DeploySeedLogging will install the Helm release "seed-bootstrap/charts/elastic-kibana-curator" in the Seed clusters.
func (b *Botanist) DeploySeedLogging(ctx context.Context) error {
	if b.Shoot.GetPurpose() == gardencorev1beta1.ShootPurposeTesting || !gardenletfeatures.FeatureGate.Enabled(features.Logging) {
		return common.DeleteLoggingStack(ctx, b.K8sSeedClient.Client(), b.Shoot.SeedNamespace)
	}

	var (
		kibanaCredentials                      = b.Secrets["kibana-logging-sg-credentials"]
		kibanaUserIngressCredentialsSecretName = "logging-ingress-credentials-users"
		sgKibanaUsername                       = kibanaCredentials.Data[secrets.DataKeyUserName]
		sgKibanaPassword                       = kibanaCredentials.Data[secrets.DataKeyPassword]
		sgKibanaPasswordHash                   = kibanaCredentials.Data[secrets.DataKeyPasswordBcryptHash]
		basicAuth                              = utils.EncodeBase64([]byte(fmt.Sprintf("%s:%s", sgKibanaUsername, sgKibanaPassword)))

		sgCuratorPassword     = b.Secrets["curator-sg-credentials"].Data[secrets.DataKeyPassword]
		sgCuratorPasswordHash = b.Secrets["curator-sg-credentials"].Data[secrets.DataKeyPasswordBcryptHash]

		sgUserPasswordHash  = b.Secrets[kibanaUserIngressCredentialsSecretName].Data[secrets.DataKeyPasswordBcryptHash]
		sgAdminPasswordHash = b.Secrets[common.KibanaAdminIngressCredentialsSecretName].Data[secrets.DataKeyPasswordBcryptHash]

		userIngressBasicAuth  = utils.CreateSHA1Secret(b.Secrets[kibanaUserIngressCredentialsSecretName].Data[secrets.DataKeyUserName], b.Secrets[kibanaUserIngressCredentialsSecretName].Data[secrets.DataKeyPassword])
		adminIngressBasicAuth = utils.CreateSHA1Secret(b.Secrets[common.KibanaAdminIngressCredentialsSecretName].Data[secrets.DataKeyUserName], b.Secrets[common.KibanaAdminIngressCredentialsSecretName].Data[secrets.DataKeyPassword])
		ingressBasicAuth      string

		sgFluentdPasswordHash string
	)

	userIngressBasicAuthDecoded, err := utils.DecodeBase64(userIngressBasicAuth)
	if err != nil {
		return err
	}

	adminIngressBasicAuthDecoded, err := utils.DecodeBase64(adminIngressBasicAuth)
	if err != nil {
		return err
	}

	ingressBasicAuthDecoded := fmt.Sprintf("%s\n%s", string(userIngressBasicAuthDecoded), string(adminIngressBasicAuthDecoded))
	ingressBasicAuth = utils.EncodeBase64([]byte(ingressBasicAuthDecoded))

	images, err := b.InjectSeedSeedImages(map[string]interface{}{},
		common.ElasticsearchImageName,
		common.ElasticsearchMetricsExporterImageName,
		common.ElasticsearchSearchguardImageName,
		common.CuratorImageName,
		common.KibanaImageName,
		common.SearchguardImageName,
		common.AlpineImageName,
	)
	if err != nil {
		return err
	}

	ct := b.Shoot.Info.CreationTimestamp.Time

	sgFluentdSecret := &corev1.Secret{}
	if err = b.K8sSeedClient.Client().Get(ctx, kutil.Key(v1beta1constants.GardenNamespace, "fluentd-es-sg-credentials"), sgFluentdSecret); err != nil {
		return err
	}

	sgFluentdPasswordHash = string(sgFluentdSecret.Data["bcryptPasswordHash"])

	kibanaTLSOverride := common.KibanaTLS
	if b.ControlPlaneWildcardCert != nil {
		kibanaTLSOverride = b.ControlPlaneWildcardCert.GetName()
	}

	hosts := []map[string]interface{}{
		// TODO: timuthy - remove in the future. Old Kibana host is retained for migration reasons.
		{
			"hostName":   b.ComputeKibanaHostDeprecated(),
			"secretName": common.KibanaTLS,
		},
		{
			"hostName":   b.ComputeKibanaHost(),
			"secretName": kibanaTLSOverride,
		},
	}

	elasticKibanaCurator := map[string]interface{}{
		"ingress": map[string]interface{}{
			"hosts":           hosts,
			"basicAuthSecret": ingressBasicAuth,
		},
		"elasticsearch": map[string]interface{}{
			"replicaCount": b.Shoot.GetReplicas(1),
			"readinessProbe": map[string]interface{}{
				"httpAuth": basicAuth,
			},
			"metricsExporter": map[string]interface{}{
				"username": string(sgKibanaUsername),
				"password": string(sgKibanaPassword),
			},
		},
		"kibana": map[string]interface{}{
			"replicaCount": b.Shoot.GetReplicas(1),
			"sgUsername":   "kibanaserver",
			"sgPassword":   string(sgKibanaPassword),
		},
		"curator": map[string]interface{}{
			"hourly": map[string]interface{}{
				"schedule": fmt.Sprintf("%d * * * *", ct.Minute()),
				"suspend":  b.Shoot.HibernationEnabled,
			},
			"daily": map[string]interface{}{
				"schedule": fmt.Sprintf("%d 0,6,12,18 * * *", ct.Minute()%54+5),
				"suspend":  b.Shoot.HibernationEnabled,
			},
			"sgUsername": "curator",
			"sgPassword": string(sgCuratorPassword),
		},
		"searchguard": map[string]interface{}{
			"enabled":      true,
			"replicaCount": b.Shoot.GetReplicas(1),
			"annotations": map[string]interface{}{
				"checksum/tls-secrets-server": b.CheckSums["elasticsearch-logging-server"],
				"checksum/sg-admin-client":    b.CheckSums["sg-admin-client"],
			},
			"users": map[string]interface{}{
				"fluentd": map[string]interface{}{
					"hash": string(sgFluentdPasswordHash),
				},
				"kibanaserver": map[string]interface{}{
					"hash": string(sgKibanaPasswordHash),
				},
				"curator": map[string]interface{}{
					"hash": string(sgCuratorPasswordHash),
				},
				"user": map[string]interface{}{
					"hash": string(sgUserPasswordHash),
				},
				"admin": map[string]interface{}{
					"hash": string(sgAdminPasswordHash),
				},
			},
		},
		"global": images,
	}

	return b.ChartApplierSeed.Apply(ctx, filepath.Join(common.ChartPath, "seed-bootstrap", "charts", "elastic-kibana-curator"), b.Shoot.SeedNamespace, fmt.Sprintf("%s-logging", b.Shoot.SeedNamespace), kubernetes.Values(elasticKibanaCurator))
}
