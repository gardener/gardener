// Copyright 2024 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package dashboard

import (
	"encoding/json"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/ptr"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/component/gardener/dashboard/config"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

const (
	configMapNamePrefix = "gardener-dashboard-config"
	dataKeyConfig       = "config.yaml"
	dataKeyLoginConfig  = "login-config.json"
)

func (g *gardenerDashboard) configMap() (*corev1.ConfigMap, error) {
	var (
		cfg = &config.Config{
			Port:               portServer,
			LogFormat:          "text",
			LogLevel:           g.values.LogLevel,
			APIServerURL:       g.values.APIServerURL,
			MaxRequestBodySize: "500kb",
			// TODO: Remove this field once https://github.com/gardener/dashboard/issues/1788 is fixed
			ExperimentalUseWatchCacheForListShoots: "yes",
			ReadinessProbe:                         config.ReadinessProbe{PeriodSeconds: readinessProbePeriodSeconds},
			UnreachableSeeds:                       config.UnreachableSeeds{MatchLabels: map[string]string{v1beta1constants.LabelSeedNetwork: v1beta1constants.LabelSeedNetworkPrivate}},
		}
		loginCfg = &config.LoginConfig{}
	)

	if g.values.EnableTokenLogin {
		loginCfg.LoginTypes = append(loginCfg.LoginTypes, "token")
	}

	if g.values.Terminal != nil {
		cfg.ContentSecurityPolicy = &config.ContentSecurityPolicy{ConnectSources: []string{"self"}}
		for _, host := range g.values.Terminal.AllowedHostSourceList {
			cfg.ContentSecurityPolicy.ConnectSources = append(cfg.ContentSecurityPolicy.ConnectSources,
				"wss://"+host,
				"https://"+host,
			)
		}

		cfg.Terminal = &config.Terminal{
			Container: config.TerminalContainer{Image: g.values.Terminal.Container.Image},
			ContainerImageDescriptions: []config.TerminalContainerImageDescription{{
				Image:       `/.*/`,
				Description: ptr.Deref(g.values.Terminal.Container.Description, ""),
			}},
			GardenTerminalHost: config.TerminalGardenHost{SeedRef: g.values.Terminal.GardenTerminalSeedHost},
			Garden: config.TerminalGarden{OperatorCredentials: config.TerminalOperatorCredentials{ServiceAccountRef: corev1.SecretReference{
				Name:      serviceAccountNameTerminal,
				Namespace: metav1.NamespaceSystem,
			}}},
		}
	}

	if g.values.OIDC != nil {
		redirectURIs := make([]string, 0, len(g.values.OIDC.IngressDomains))
		for _, domain := range g.values.OIDC.IngressDomains {
			redirectURIs = append(redirectURIs, "https://dashboard."+domain+"/auth/callback")
		}

		cfg.OIDC = &config.OIDC{
			Issuer:             g.values.OIDC.IssuerURL,
			SessionLifetime:    int64(ptr.Deref(g.values.OIDC.DashboardOIDC.SessionLifetime, metav1.Duration{Duration: 12 * time.Hour}).Duration.Seconds()),
			RedirectURIs:       redirectURIs,
			Scope:              strings.Join(append([]string{"openid", "email"}, g.values.OIDC.DashboardOIDC.AdditionalScopes...), " "),
			RejectUnauthorized: true,
			Public: config.OIDCPublic{
				ClientID: g.values.OIDC.ClientIDPublic,
				UsePKCE:  true,
			},
		}

		loginCfg.LoginTypes = append([]string{"oidc"}, loginCfg.LoginTypes...)
	}

	rawConfig, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, err
	}

	rawLoginConfig, err := json.Marshal(loginCfg)
	if err != nil {
		return nil, err
	}

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapNamePrefix,
			Namespace: g.namespace,
			Labels:    GetLabels(),
		},
		Data: map[string]string{
			dataKeyConfig:      string(rawConfig),
			dataKeyLoginConfig: string(rawLoginConfig),
		},
	}

	utilruntime.Must(kubernetesutils.MakeUnique(configMap))
	return configMap, nil
}
