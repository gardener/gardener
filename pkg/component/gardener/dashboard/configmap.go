// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package dashboard

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/component/gardener/dashboard/config"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

const (
	configMapNamePrefix       = "gardener-dashboard-config"
	configMapAssetsNamePrefix = "gardener-dashboard-assets"
	dataKeyConfig             = "config.yaml"
	dataKeyFrontendConfig     = "frontend-config.yaml"
	dataKeyLoginConfig        = "login-config.json"
)

func (g *gardenerDashboard) configMap(ctx context.Context) (*corev1.ConfigMap, error) {
	var frontendConfig map[string]any
	if g.values.FrontendConfigMapName != nil {
		frontendConfigMap := &corev1.ConfigMap{}
		if err := g.client.Get(ctx, client.ObjectKey{Name: *g.values.FrontendConfigMapName, Namespace: g.namespace}, frontendConfigMap); err != nil {
			return nil, err
		}

		frontendConfig = make(map[string]any)
		if err := yaml.Unmarshal([]byte(frontendConfigMap.Data[dataKeyFrontendConfig]), &frontendConfig); err != nil {
			return nil, err
		}
	}

	var (
		cfg = &config.Config{
			Port:               portServer,
			LogFormat:          "text",
			LogLevel:           g.values.LogLevel,
			APIServerURL:       "https://" + g.values.APIServerURL,
			APIServerCAData:    g.values.APIServerCABundle,
			MaxRequestBodySize: "500kb",
			ReadinessProbe:     config.ReadinessProbe{PeriodSeconds: readinessProbePeriodSeconds},
			UnreachableSeeds:   config.UnreachableSeeds{MatchLabels: map[string]string{v1beta1constants.LabelSeedNetwork: v1beta1constants.LabelSeedNetworkPrivate}},
		}
		loginCfg = &config.LoginConfig{}
	)

	if frontendConfig != nil {
		cfg.Frontend = frontendConfig

		if v, ok := frontendConfig["landingPageUrl"]; ok {
			loginCfg.LandingPageURL = v.(string)
		}
		if v, ok := frontendConfig["branding"]; ok {
			loginCfg.Branding = v.(map[string]any)
		}
		if v, ok := frontendConfig["themes"]; ok {
			loginCfg.Themes = v.(map[string]any)
		}
	}

	if g.values.EnableTokenLogin {
		loginCfg.LoginTypes = append(loginCfg.LoginTypes, "token")
	}

	if g.values.Terminal != nil {
		cfg.ContentSecurityPolicy = &config.ContentSecurityPolicy{ConnectSources: []string{"'self'"}}
		for _, host := range g.values.Terminal.AllowedHosts {
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

		if cfg.Frontend == nil {
			cfg.Frontend = make(map[string]any)
		}
		if cfg.Frontend["features"] == nil {
			cfg.Frontend["features"] = make(map[string]any)
		}
		cfg.Frontend["features"].(map[string]any)["terminalEnabled"] = true
	}

	if g.values.OIDC != nil {
		redirectURIs := make([]string, 0, len(g.ingressHosts()))
		for _, host := range g.ingressHosts() {
			redirectURIs = append(redirectURIs, "https://"+host+"/auth/callback")
		}

		cfg.OIDC = &config.OIDC{
			Issuer:             g.values.OIDC.IssuerURL,
			SessionLifetime:    int64(ptr.Deref(g.values.OIDC.DashboardOIDC.SessionLifetime, metav1.Duration{Duration: 12 * time.Hour}).Seconds()),
			RedirectURIs:       redirectURIs,
			Scope:              strings.Join(append([]string{"openid", "email"}, g.values.OIDC.AdditionalScopes...), " "),
			RejectUnauthorized: true,
			Public: config.OIDCPublic{
				ClientID: g.values.OIDC.ClientIDPublic,
				UsePKCE:  true,
			},
		}

		if g.values.OIDC.CertificateAuthoritySecretRef != nil {
			caSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      g.values.OIDC.CertificateAuthoritySecretRef.Name,
					Namespace: g.namespace,
				},
			}

			if err := g.client.Get(ctx, client.ObjectKeyFromObject(caSecret), caSecret); err != nil {
				return nil, fmt.Errorf("failed reading referenced ca secret: %w", err)
			}

			caData, ok := caSecret.Data["ca.crt"]
			if !ok {
				return nil, fmt.Errorf("failed reading ca secret: missing ca.crt key")
			}

			cfg.OIDC.CA = string(caData)
		}

		loginCfg.LoginTypes = append([]string{"oidc"}, loginCfg.LoginTypes...)
	}

	if g.values.GitHub != nil {
		secret := &corev1.Secret{}
		if err := g.client.Get(ctx, client.ObjectKey{Name: g.values.GitHub.SecretRef.Name, Namespace: g.namespace}, secret); err != nil {
			return nil, fmt.Errorf("failed reading referenced GitHub secret %q: %w", g.values.GitHub.SecretRef.Name, err)
		}

		var pollIntervalSeconds *int64
		if _, ok := secret.Data["webhookSecret"]; !ok {
			pollIntervalSeconds = ptr.To(int64(ptr.Deref(g.values.GitHub.PollInterval, metav1.Duration{Duration: 15 * time.Minute}).Seconds()))
		} else if g.values.GitHub.PollInterval != nil {
			pollIntervalSeconds = ptr.To(int64(g.values.GitHub.PollInterval.Seconds()))
		}

		cfg.GitHub = &config.GitHub{
			APIURL:              g.values.GitHub.APIURL,
			Org:                 g.values.GitHub.Organisation,
			Repository:          g.values.GitHub.Repository,
			PollIntervalSeconds: pollIntervalSeconds,
			SyncThrottleSeconds: 20,
			SyncConcurrency:     10,
		}
	}

	rawConfig := &bytes.Buffer{}
	yamlEncoder := yaml.NewEncoder(rawConfig)
	yamlEncoder.SetIndent(2)
	if err := yamlEncoder.Encode(&cfg); err != nil {
		return nil, fmt.Errorf("failed marshalling config: %w", err)
	}

	rawLoginConfig, err := json.Marshal(loginCfg)
	if err != nil {
		return nil, fmt.Errorf("failed marshalling login config: %w", err)
	}

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapNamePrefix,
			Namespace: g.namespace,
			Labels:    GetLabels(),
		},
		Data: map[string]string{
			dataKeyConfig:      rawConfig.String(),
			dataKeyLoginConfig: string(rawLoginConfig),
		},
	}

	utilruntime.Must(kubernetesutils.MakeUnique(configMap))
	return configMap, nil
}
