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

package seed

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorelisters "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/features"
	gardenletfeatures "github.com/gardener/gardener/pkg/gardenlet/features"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/operation/seed/istio"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/chart"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	versionutils "github.com/gardener/gardener/pkg/utils/version"
	"github.com/gardener/gardener/pkg/version"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// NewBuilder returns a new Builder.
func NewBuilder() *Builder {
	return &Builder{
		seedObjectFunc: func() (*gardencorev1beta1.Seed, error) { return nil, fmt.Errorf("seed object is required but not set") },
	}
}

// WithSeedObject sets the seedObjectFunc attribute at the Builder.
func (b *Builder) WithSeedObject(seedObject *gardencorev1beta1.Seed) *Builder {
	b.seedObjectFunc = func() (*gardencorev1beta1.Seed, error) { return seedObject, nil }
	return b
}

// WithSeedObjectFromLister sets the seedObjectFunc attribute at the Builder after fetching it from the given lister.
func (b *Builder) WithSeedObjectFromLister(seedLister gardencorelisters.SeedLister, seedName string) *Builder {
	b.seedObjectFunc = func() (*gardencorev1beta1.Seed, error) { return seedLister.Get(seedName) }
	return b
}

// WithSeedSecret sets the seedSecretFunc attribute at the Builder.
func (b *Builder) WithSeedSecret(seedSecret *corev1.Secret) *Builder {
	b.seedSecretFunc = func(*corev1.SecretReference) (*corev1.Secret, error) { return seedSecret, nil }
	return b
}

// WithSeedSecretFromClient sets the seedSecretFunc attribute at the Builder after reading it with the client.
func (b *Builder) WithSeedSecretFromClient(ctx context.Context, c client.Client) *Builder {
	b.seedSecretFunc = func(secretRef *corev1.SecretReference) (*corev1.Secret, error) {
		if secretRef == nil {
			return nil, fmt.Errorf("cannot fetch secret because spec.secretRef is nil")
		}

		secret := &corev1.Secret{}
		if err := c.Get(ctx, kutil.Key(secretRef.Namespace, secretRef.Name), secret); err != nil {
			return nil, err
		}
		return secret, nil
	}
	return b
}

// Build initializes a new Seed object.
func (b *Builder) Build() (*Seed, error) {
	seed := &Seed{}

	seedObject, err := b.seedObjectFunc()
	if err != nil {
		return nil, err
	}
	seed.Info = seedObject

	if b.seedSecretFunc != nil && seedObject.Spec.SecretRef != nil {
		seedSecret, err := b.seedSecretFunc(seedObject.Spec.SecretRef)
		if err != nil {
			return nil, err
		}
		seed.Secret = seedSecret
	}

	if seedObject.Spec.Settings != nil && seedObject.Spec.Settings.LoadBalancerServices != nil {
		seed.LoadBalancerServiceAnnotations = seedObject.Spec.Settings.LoadBalancerServices.Annotations
	}

	return seed, nil
}

const (
	caSeed = "ca-seed"
)

var wantedCertificateAuthorities = map[string]*secretsutils.CertificateSecretConfig{
	caSeed: {
		Name:       caSeed,
		CommonName: "kubernetes",
		CertType:   secretsutils.CACert,
	},
}

const (
	grafanaPrefix = "g-seed"
	grafanaTLS    = "grafana-tls"

	prometheusPrefix = "p-seed"
	prometheusTLS    = "aggregate-prometheus-tls"

	kibanaPrefix = "k-seed"
	kibanaTLS    = "kibana-tls"
)

// generateWantedSecrets returns a list of Secret configuration objects satisfying the secret config intface,
// each containing their specific configuration for the creation of certificates (server/client), RSA key pairs, basic
// authentication credentials, etc.
func generateWantedSecrets(seed *Seed, certificateAuthorities map[string]*secretsutils.Certificate) ([]secretsutils.ConfigInterface, error) {
	if len(certificateAuthorities) != len(wantedCertificateAuthorities) {
		return nil, fmt.Errorf("missing certificate authorities")
	}

	endUserCrtValidity := common.EndUserCrtValidity

	secretList := []secretsutils.ConfigInterface{
		&secretsutils.CertificateSecretConfig{
			Name: "vpa-tls-certs",

			CommonName:   "vpa-webhook.garden.svc",
			Organization: nil,
			DNSNames:     []string{"vpa-webhook.garden.svc", "vpa-webhook"},
			IPAddresses:  nil,

			CertType:  secretsutils.ServerCert,
			SigningCA: certificateAuthorities[caSeed],
		},
		&secretsutils.CertificateSecretConfig{
			Name: common.GrafanaTLS,

			CommonName:   "grafana",
			Organization: []string{"garden.sapcloud.io:monitoring:ingress"},
			DNSNames:     []string{seed.GetIngressFQDN(grafanaPrefix)},
			IPAddresses:  nil,

			CertType:  secretsutils.ServerCert,
			SigningCA: certificateAuthorities[caSeed],
			Validity:  &endUserCrtValidity,
		},
		&secretsutils.CertificateSecretConfig{
			Name: prometheusTLS,

			CommonName:   "prometheus",
			Organization: []string{"garden.sapcloud.io:monitoring:ingress"},
			DNSNames:     []string{seed.GetIngressFQDN(prometheusPrefix)},
			IPAddresses:  nil,

			CertType:  secretsutils.ServerCert,
			SigningCA: certificateAuthorities[caSeed],
			Validity:  &endUserCrtValidity,
		},
	}

	// Logging feature gate
	if gardenletfeatures.FeatureGate.Enabled(features.Logging) {
		secretList = append(secretList,
			&secretsutils.CertificateSecretConfig{
				Name: kibanaTLS,

				CommonName:   "kibana",
				Organization: []string{"garden.sapcloud.io:logging:ingress"},
				DNSNames:     []string{seed.GetIngressFQDN(kibanaPrefix)},
				IPAddresses:  nil,

				CertType:  secretsutils.ServerCert,
				SigningCA: certificateAuthorities[caSeed],
				Validity:  &endUserCrtValidity,
			},

			// Secret definition for logging ingress
			&secretsutils.BasicAuthSecretConfig{
				Name:   "seed-logging-ingress-credentials",
				Format: secretsutils.BasicAuthFormatNormal,

				Username:       "admin",
				PasswordLength: 32,
			},
			&secretsutils.BasicAuthSecretConfig{
				Name:   "fluentd-es-sg-credentials",
				Format: secretsutils.BasicAuthFormatNormal,

				Username:                  "fluentd",
				PasswordLength:            32,
				BcryptPasswordHashRequest: true,
			},
		)
	}

	return secretList, nil
}

// deployCertificates deploys CA and TLS certificates inside the garden namespace
// It takes a map[string]*corev1.Secret object which contains secrets that have already been deployed inside that namespace to avoid duplication errors.
func deployCertificates(seed *Seed, k8sSeedClient kubernetes.Interface, existingSecretsMap map[string]*corev1.Secret) (map[string]*corev1.Secret, error) {
	_, certificateAuthorities, err := secretsutils.GenerateCertificateAuthorities(k8sSeedClient, existingSecretsMap, wantedCertificateAuthorities, v1beta1constants.GardenNamespace)
	if err != nil {
		return nil, err
	}

	wantedSecretsList, err := generateWantedSecrets(seed, certificateAuthorities)
	if err != nil {
		return nil, err
	}

	// Only necessary to renew certificates for Grafana, Kibana, Prometheus
	// TODO: (timuthy) remove in future version.
	var (
		renewedLabel = "cert.gardener.cloud/renewed-endpoint"
		browserCerts = sets.NewString(grafanaTLS, kibanaTLS, prometheusTLS)
	)
	for name, secret := range existingSecretsMap {
		_, ok := secret.Labels[renewedLabel]
		if browserCerts.Has(name) && !ok {
			if err := k8sSeedClient.Client().Delete(context.TODO(), secret); client.IgnoreNotFound(err) != nil {
				return nil, err
			}
			delete(existingSecretsMap, name)
		}
	}

	secrets, err := secretsutils.GenerateClusterSecrets(context.TODO(), k8sSeedClient, existingSecretsMap, wantedSecretsList, v1beta1constants.GardenNamespace)
	if err != nil {
		return nil, err
	}

	// Only necessary to renew certificates for Grafana, Kibana, Prometheus
	// TODO: (timuthy) remove in future version.
	for name, secret := range secrets {
		_, ok := secret.Labels[renewedLabel]
		if browserCerts.Has(name) && !ok {
			if secret.Labels == nil {
				secret.Labels = make(map[string]string)
			}
			secret.Labels[renewedLabel] = "true"

			if err := k8sSeedClient.Client().Update(context.TODO(), secret); err != nil {
				return nil, err
			}
		}
	}

	return secrets, nil
}

// BootstrapCluster bootstraps a Seed cluster and deploys various required manifests.
func BootstrapCluster(k8sSeedClient kubernetes.Interface, seed *Seed, secrets map[string]*corev1.Secret, imageVector imagevector.ImageVector, componentImageVectors imagevector.ComponentImageVectors) error {
	const chartName = "seed-bootstrap"

	gardenNamespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: v1beta1constants.GardenNamespace,
		},
	}
	if err := k8sSeedClient.Client().Create(context.TODO(), gardenNamespace); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	if _, err := kutil.TryUpdateNamespace(k8sSeedClient.Kubernetes(), retry.DefaultBackoff, gardenNamespace.ObjectMeta, func(ns *corev1.Namespace) (*corev1.Namespace, error) {
		kutil.SetMetaDataLabel(&ns.ObjectMeta, "role", v1beta1constants.GardenNamespace)
		return ns, nil
	}); err != nil {
		return err
	}
	if _, err := kutil.TryUpdateNamespace(k8sSeedClient.Kubernetes(), retry.DefaultBackoff, metav1.ObjectMeta{Name: metav1.NamespaceSystem}, func(ns *corev1.Namespace) (*corev1.Namespace, error) {
		kutil.SetMetaDataLabel(&ns.ObjectMeta, "role", metav1.NamespaceSystem)
		return ns, nil
	}); err != nil {
		return err
	}

	if monitoringSecrets := common.GetSecretKeysWithPrefix(common.GardenRoleGlobalMonitoring, secrets); len(monitoringSecrets) > 0 {
		for _, key := range monitoringSecrets {
			secret := secrets[key]
			secretObj := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("%s-%s", "seed", secret.Name),
					Namespace: "garden",
				},
			}

			if _, err := controllerutil.CreateOrUpdate(context.TODO(), k8sSeedClient.Client(), secretObj, func() error {
				secretObj.Type = corev1.SecretTypeOpaque
				secretObj.Data = secret.Data
				return nil
			}); err != nil {
				return err
			}
		}
	}

	images, err := imagevector.FindImages(imageVector,
		[]string{
			common.AlertManagerImageName,
			common.AlpineImageName,
			common.ConfigMapReloaderImageName,
			common.CuratorImageName,
			common.ElasticsearchImageName,
			common.ElasticsearchMetricsExporterImageName,
			common.FluentBitImageName,
			common.FluentdEsImageName,
			common.GardenerResourceManagerImageName,
			common.GrafanaImageName,
			common.KibanaImageName,
			common.PauseContainerImageName,
			common.PrometheusImageName,
			common.VpaAdmissionControllerImageName,
			common.VpaExporterImageName,
			common.VpaRecommenderImageName,
			common.VpaUpdaterImageName,
			common.HvpaControllerImageName,
			common.DependencyWatchdogImageName,
			common.KubeStateMetricsImageName,
			common.EtcdDruidImageName,
		},
		imagevector.RuntimeVersion(k8sSeedClient.Version()),
		imagevector.TargetVersion(k8sSeedClient.Version()),
	)
	if err != nil {
		return err
	}

	// Special handling for gardener-seed-admission-controller because it's a component whose version is controlled by
	// this project/repository

	gardenerSeedAdmissionControllerImage, err := imageVector.FindImage(common.GardenerSeedAdmissionControllerImageName)
	if err != nil {
		return err
	}
	var (
		repository = gardenerSeedAdmissionControllerImage.String()
		tag        = version.Get().GitVersion
	)
	if gardenerSeedAdmissionControllerImage.Tag != nil {
		repository = gardenerSeedAdmissionControllerImage.Repository
		tag = *gardenerSeedAdmissionControllerImage.Tag
	}
	images[common.GardenerSeedAdmissionControllerImageName] = &imagevector.Image{
		Repository: repository,
		Tag:        &tag,
	}

	// Logging feature gate
	var (
		basicAuth             string
		kibanaHost            string
		sgFluentdPassword     string
		sgFluentdPasswordHash string
		fluentdReplicaCount   int32
		loggingEnabled        = gardenletfeatures.FeatureGate.Enabled(features.Logging)
		existingSecretsMap    = map[string]*corev1.Secret{}
		filters               = strings.Builder{}
		parsers               = strings.Builder{}
	)

	if loggingEnabled {
		existingSecrets := &corev1.SecretList{}
		if err = k8sSeedClient.Client().List(context.TODO(), existingSecrets, client.InNamespace(v1beta1constants.GardenNamespace)); err != nil {
			return err
		}

		for _, secret := range existingSecrets.Items {
			secretObj := secret
			existingSecretsMap[secret.ObjectMeta.Name] = &secretObj
		}

		deployedSecretsMap, err := deployCertificates(seed, k8sSeedClient, existingSecretsMap)
		if err != nil {
			return err
		}

		if fluentdReplicaCount, err = GetFluentdReplicaCount(k8sSeedClient); err != nil {
			return err
		}

		credentials := deployedSecretsMap["seed-logging-ingress-credentials"]
		basicAuth = utils.CreateSHA1Secret(credentials.Data[secretsutils.DataKeyUserName], credentials.Data[secretsutils.DataKeyPassword])
		kibanaHost = seed.GetIngressFQDN(kibanaPrefix)

		sgFluentdCredentials := deployedSecretsMap["fluentd-es-sg-credentials"]
		sgFluentdPassword = string(sgFluentdCredentials.Data[secretsutils.DataKeyPassword])
		sgFluentdPasswordHash = string(sgFluentdCredentials.Data[secretsutils.DataKeyPasswordBcryptHash])

		// Read extension provider specific configuration
		existingConfigMaps := &corev1.ConfigMapList{}
		if err = k8sSeedClient.Client().List(context.TODO(), existingConfigMaps,
			client.InNamespace(v1beta1constants.GardenNamespace),
			client.MatchingLabels{v1beta1constants.LabelExtensionConfiguration: v1beta1constants.LabelLogging}); err != nil {
			return err
		}

		// Read all filters and parsers coming from the extension provider configurations
		for _, cm := range existingConfigMaps.Items {
			filters.WriteString(fmt.Sprintln(cm.Data[v1beta1constants.FluentBitConfigMapKubernetesFilter]))
			parsers.WriteString(fmt.Sprintln(cm.Data[v1beta1constants.FluentBitConfigMapParser]))
		}
	} else {
		if err := common.DeleteLoggingStack(context.TODO(), k8sSeedClient.Client(), v1beta1constants.GardenNamespace); client.IgnoreNotFound(err) != nil {
			return err
		}
	}

	// HVPA feature gate
	hvpaEnabled := gardenletfeatures.FeatureGate.Enabled(features.HVPA)
	if !hvpaEnabled {
		if err := common.DeleteHvpa(k8sSeedClient, v1beta1constants.GardenNamespace); client.IgnoreNotFound(err) != nil {
			return err
		}
	}

	vpaEnabled := seed.Info.Spec.Settings == nil || seed.Info.Spec.Settings.VerticalPodAutoscaler == nil || seed.Info.Spec.Settings.VerticalPodAutoscaler.Enabled
	if !vpaEnabled {
		if err := common.DeleteVpa(context.TODO(), k8sSeedClient.Client(), v1beta1constants.GardenNamespace, false); client.IgnoreNotFound(err) != nil {
			return err
		}
	}

	existingSecrets := &corev1.SecretList{}
	if err = k8sSeedClient.Client().List(context.TODO(), existingSecrets, client.InNamespace(v1beta1constants.GardenNamespace)); err != nil {
		return err
	}

	for _, secret := range existingSecrets.Items {
		secretObj := secret
		existingSecretsMap[secret.ObjectMeta.Name] = &secretObj
	}

	deployedSecretsMap, err := deployCertificates(seed, k8sSeedClient, existingSecretsMap)
	if err != nil {
		return err
	}
	jsonString, err := json.Marshal(deployedSecretsMap["vpa-tls-certs"].Data)
	if err != nil {
		return err
	}

	// AlertManager configuration
	alertManagerConfig := map[string]interface{}{
		"storage": seed.GetValidVolumeSize("1Gi"),
	}

	alertingSMTPKeys := common.GetSecretKeysWithPrefix(common.GardenRoleAlerting, secrets)

	if seedWantsAlertmanager(alertingSMTPKeys, secrets) {
		emailConfigs := make([]map[string]interface{}, 0, len(alertingSMTPKeys))
		for _, key := range alertingSMTPKeys {
			if string(secrets[key].Data["auth_type"]) == "smtp" {
				secret := secrets[key]
				emailConfigs = append(emailConfigs, map[string]interface{}{
					"to":            string(secret.Data["to"]),
					"from":          string(secret.Data["from"]),
					"smarthost":     string(secret.Data["smarthost"]),
					"auth_username": string(secret.Data["auth_username"]),
					"auth_identity": string(secret.Data["auth_identity"]),
					"auth_password": string(secret.Data["auth_password"]),
				})
				alertManagerConfig["enabled"] = true
				alertManagerConfig["emailConfigs"] = emailConfigs
				break
			}
		}
	} else {
		alertManagerConfig["enabled"] = false
		if err := common.DeleteAlertmanager(context.TODO(), k8sSeedClient.Client(), v1beta1constants.GardenNamespace); err != nil {
			return err
		}
	}

	if !seed.Info.Spec.Settings.ExcessCapacityReservation.Enabled {
		if err := common.DeleteReserveExcessCapacity(context.TODO(), k8sSeedClient.Client()); client.IgnoreNotFound(err) != nil {
			return err
		}
	}

	nodes := &corev1.NodeList{}
	if err = k8sSeedClient.Client().List(context.TODO(), nodes); err != nil {
		return err
	}
	nodeCount := len(nodes.Items)

	chartApplier := k8sSeedClient.ChartApplier()

	var (
		applierOptions          = kubernetes.CopyApplierOptions(kubernetes.DefaultMergeFuncs)
		retainStatusInformation = func(new, old *unstructured.Unstructured) {
			// Apply status from old Object to retain status information
			new.Object["status"] = old.Object["status"]
		}
		vpaGK                 = schema.GroupKind{Group: "autoscaling.k8s.io", Kind: "VerticalPodAutoscaler"}
		hvpaGK                = schema.GroupKind{Group: "autoscaling.k8s.io", Kind: "Hvpa"}
		druidGK               = schema.GroupKind{Group: "druid.gardener.cloud", Kind: "Etcd"}
		issuerGK              = schema.GroupKind{Group: "certmanager.k8s.io", Kind: "ClusterIssuer"}
		grafanaHost           = seed.GetIngressFQDN(grafanaPrefix)
		prometheusHost        = seed.GetIngressFQDN(prometheusPrefix)
		monitoringCredentials = existingSecretsMap["seed-monitoring-ingress-credentials"]
		monitoringBasicAuth   string
	)

	if monitoringCredentials != nil {
		monitoringBasicAuth = utils.CreateSHA1Secret(monitoringCredentials.Data[secretsutils.DataKeyUserName], monitoringCredentials.Data[secretsutils.DataKeyPassword])
	}
	applierOptions[vpaGK] = retainStatusInformation
	applierOptions[hvpaGK] = retainStatusInformation
	applierOptions[issuerGK] = retainStatusInformation
	applierOptions[druidGK] = retainStatusInformation

	networks := []string{
		seed.Info.Spec.Networks.Pods,
		seed.Info.Spec.Networks.Services,
	}
	if v := seed.Info.Spec.Networks.Nodes; v != nil {
		networks = append(networks, *v)
	}

	privateNetworks, err := common.ToExceptNetworks(common.AllPrivateNetworkBlocks(), networks...)
	if err != nil {
		return err
	}

	var (
		grafanaTLSOverride    = grafanaTLS
		prometheusTLSOverride = prometheusTLS
		kibanaTLSOverride     = kibanaTLS
	)

	wildcardCert, err := GetWildcardCertificate(context.TODO(), k8sSeedClient.Client())
	if err != nil {
		return err
	}

	if wildcardCert != nil {
		grafanaTLSOverride = wildcardCert.GetName()
		prometheusTLSOverride = wildcardCert.GetName()
		kibanaTLSOverride = wildcardCert.GetName()
	}

	imageVectorOverwrites := map[string]interface{}{}
	for name, data := range componentImageVectors {
		imageVectorOverwrites[name] = data
	}

	if gardenletfeatures.FeatureGate.Enabled(features.ManagedIstio) {
		istiodImage, err := imageVector.FindImage(common.IstioIstiodImageName)
		if err != nil {
			return err
		}

		igwImage, err := imageVector.FindImage(common.IstioProxyImageName)
		if err != nil {
			return err
		}

		chartApplier := k8sSeedClient.ChartApplier()
		istioCRDs := istio.NewIstioCRD(chartApplier, "charts", k8sSeedClient.Client())
		istiod := istio.NewIstiod(
			&istio.IstiodValues{
				TrustDomain: "cluster.local",
				Image:       istiodImage.String(),
			},
			common.IstioNamespace,
			chartApplier,
			"charts",
			k8sSeedClient.Client(),
		)

		igwConfig := &istio.IngressValues{
			TrustDomain:     "cluster.local",
			Image:           igwImage.String(),
			IstiodNamespace: common.IstioNamespace,
			Annotations:     seed.LoadBalancerServiceAnnotations,
		}

		if gardenletfeatures.FeatureGate.Enabled(features.APIServerSNI) {
			ports := []corev1.ServicePort{
				{Name: "proxy", Port: 8443, TargetPort: intstr.FromInt(8443)},
				{Name: "tcp", Port: 443, TargetPort: intstr.FromInt(443)},
			}

			if gardenletfeatures.FeatureGate.Enabled(features.KonnectivityTunnel) {
				ports = append(ports, corev1.ServicePort{Name: "tls-tunnel", Port: 8132, TargetPort: intstr.FromInt(8132)})
			}

			igwConfig.Ports = ports
		}

		igw := istio.NewIngressGateway(
			igwConfig,
			common.IstioIngressGatewayNamespace,
			chartApplier,
			"charts",
			k8sSeedClient.Client(),
		)

		if err := component.OpWaiter(istioCRDs, istiod, igw).Deploy(context.TODO()); err != nil {
			return err
		}
	}

	proxy := istio.NewProxyProtocolGateway(common.IstioIngressGatewayNamespace, chartApplier, "charts")

	if gardenletfeatures.FeatureGate.Enabled(features.APIServerSNI) {
		if err := proxy.Deploy(context.TODO()); err != nil {
			return err
		}
	} else {
		if err := proxy.Destroy(context.TODO()); err != nil {
			return err
		}
	}

	values := kubernetes.Values(map[string]interface{}{
		"cloudProvider": seed.Info.Spec.Provider.Type,
		"global": map[string]interface{}{
			"images":                chart.ImageMapToValues(images),
			"imageVectorOverwrites": imageVectorOverwrites,
		},
		"reserveExcessCapacity": seed.Info.Spec.Settings.ExcessCapacityReservation.Enabled,
		"replicas": map[string]interface{}{
			"reserve-excess-capacity": DesiredExcessCapacity(),
		},
		"prometheus": map[string]interface{}{
			"storage": seed.GetValidVolumeSize("10Gi"),
		},
		"aggregatePrometheus": map[string]interface{}{
			"storage":    seed.GetValidVolumeSize("20Gi"),
			"seed":       seed.Info.Name,
			"hostName":   prometheusHost,
			"secretName": prometheusTLSOverride,
		},
		"grafana": map[string]interface{}{
			"hostName":   grafanaHost,
			"secretName": grafanaTLSOverride,
		},
		"elastic-kibana-curator": map[string]interface{}{
			"enabled": loggingEnabled,
			"ingress": map[string]interface{}{
				"basicAuthSecret": basicAuth,
				"hosts": []map[string]interface{}{
					{
						"hostName":   kibanaHost,
						"secretName": kibanaTLSOverride,
					},
				},
			},
			"curator": map[string]interface{}{
				// Set curator threshold to 5Gi
				"diskSpaceThreshold": 5 * 1024 * 1024 * 1024,
			},
			"elasticsearch": map[string]interface{}{
				"objectCount": nodeCount,
				"persistence": map[string]interface{}{
					"size": seed.GetValidVolumeSize("100Gi"),
				},
			},
			"searchguard": map[string]interface{}{
				"users": map[string]interface{}{
					"fluentd": map[string]interface{}{
						"hash": sgFluentdPasswordHash,
					},
				},
			},
		},
		"fluentd-es": map[string]interface{}{
			"enabled": loggingEnabled,
			"fluentd": map[string]interface{}{
				"replicaCount": fluentdReplicaCount,
				"sgUsername":   "fluentd",
				"sgPassword":   sgFluentdPassword,
				"storage":      seed.GetValidVolumeSize("9Gi"),
			},
			"fluentbit": map[string]interface{}{
				"extensions": map[string]interface{}{
					"parsers": parsers.String(),
					"filters": filters.String(),
				},
			},
		},
		"alertmanager": alertManagerConfig,
		"vpa": map[string]interface{}{
			"enabled": vpaEnabled,
			"admissionController": map[string]interface{}{
				"podAnnotations": map[string]interface{}{
					"checksum/secret-vpa-tls-certs": utils.ComputeSHA256Hex(jsonString),
				},
			},
		},
		"hvpa": map[string]interface{}{
			"enabled": hvpaEnabled,
		},
		"global-network-policies": map[string]interface{}{
			"denyAll":         false,
			"privateNetworks": privateNetworks,
			"sniEnabled":      gardenletfeatures.FeatureGate.Enabled(features.APIServerSNI),
		},
		"gardenerResourceManager": map[string]interface{}{
			"resourceClass": v1beta1constants.SeedResourceManagerClass,
		},
		"ingress": map[string]interface{}{
			"basicAuthSecret": monitoringBasicAuth,
		},
	})

	return chartApplier.Apply(context.TODO(), filepath.Join("charts", chartName), v1beta1constants.GardenNamespace, chartName, values, applierOptions)
}

// DesiredExcessCapacity computes the required resources (CPU and memory) required to deploy new shoot control planes
// (on the seed) in terms of reserve-excess-capacity deployment replicas. Each deployment replica currently
// corresponds to resources of (request/limits) 2 cores of CPU and 6Gi of RAM.
// This roughly corresponds to a single, moderately large control-plane.
// The logic for computation of desired excess capacity corresponds to deploying 2 such shoot control planes.
// This excess capacity can be used for hosting new control planes or newly vertically scaled old control-planes.
func DesiredExcessCapacity() int {
	var (
		replicasToSupportSingleShoot = 1
		effectiveExcessCapacity      = 2
	)

	return effectiveExcessCapacity * replicasToSupportSingleShoot
}

// GetFluentdReplicaCount returns fluentd stateful set replica count if it exists, otherwise - the default (1).
// As fluentd HPA manages the number of replicas, we have to make sure to do not override HPA scaling.
func GetFluentdReplicaCount(k8sSeedClient kubernetes.Interface) (int32, error) {
	statefulSet := &appsv1.StatefulSet{}
	if err := k8sSeedClient.Client().Get(context.TODO(), kutil.Key(v1beta1constants.GardenNamespace, common.FluentdEsStatefulSetName), statefulSet); err != nil {
		if apierrors.IsNotFound(err) {
			// the stateful set is still not created, return the default replicas
			return 1, nil
		}

		return -1, err
	}

	replicas := statefulSet.Spec.Replicas
	if replicas == nil || *replicas == 0 {
		return 1, nil
	}

	return *replicas, nil
}

// GetIngressFQDNDeprecated returns the fully qualified domain name of ingress sub-resource for the Seed cluster. The
// end result is '<subDomain>.<shootName>.<projectName>.<seed-ingress-domain>'.
// Only necessary to renew certificates for Alertmanager, Grafana, Kibana, Prometheus
// TODO: (timuthy) remove in future version.
func (s *Seed) GetIngressFQDNDeprecated(subDomain, shootName, projectName string) string {
	if shootName == "" {
		return fmt.Sprintf("%s.%s.%s", subDomain, projectName, s.Info.Spec.DNS.IngressDomain)
	}
	return fmt.Sprintf("%s.%s.%s.%s", subDomain, shootName, projectName, s.Info.Spec.DNS.IngressDomain)
}

// GetIngressFQDN returns the fully qualified domain name of ingress sub-resource for the Seed cluster. The
// end result is '<subDomain>.<shootName>.<projectName>.<seed-ingress-domain>'.
func (s *Seed) GetIngressFQDN(subDomain string) string {
	return fmt.Sprintf("%s.%s", subDomain, s.Info.Spec.DNS.IngressDomain)
}

// CheckMinimumK8SVersion checks whether the Kubernetes version of the Seed cluster fulfills the minimal requirements.
func (s *Seed) CheckMinimumK8SVersion(seedClient kubernetes.Interface) (string, error) {
	// We require CRD status subresources for the extension controllers that we install into the seeds.
	minSeedVersion := "1.11"

	version := seedClient.Version()

	seedVersionOK, err := versionutils.CompareVersions(version, ">=", minSeedVersion)
	if err != nil {
		return "<unknown>", err
	}
	if !seedVersionOK {
		return "<unknown>", fmt.Errorf("the Kubernetes version of the Seed cluster must be at least %s", minSeedVersion)
	}
	return version, nil
}

// GetValidVolumeSize is to get a valid volume size.
// If the given size is smaller than the minimum volume size permitted by cloud provider on which seed cluster is running, it will return the minimum size.
func (s *Seed) GetValidVolumeSize(size string) string {
	if s.Info.Spec.Volume == nil || s.Info.Spec.Volume.MinimumSize == nil {
		return size
	}

	qs, err := resource.ParseQuantity(size)
	if err == nil && qs.Cmp(*s.Info.Spec.Volume.MinimumSize) < 0 {
		return s.Info.Spec.Volume.MinimumSize.String()
	}

	return size
}

func seedWantsAlertmanager(keys []string, secrets map[string]*corev1.Secret) bool {
	for _, key := range keys {
		if string(secrets[key].Data["auth_type"]) == "smtp" {
			return true
		}
	}
	return false
}

// GetWildcardCertificate gets the wildcard certificate for the seed's ingress domain.
// Nil is returned if no wildcard certificate is configured.
func GetWildcardCertificate(ctx context.Context, c client.Client) (*corev1.Secret, error) {
	wildcardCerts := &corev1.SecretList{}
	if err := c.List(
		ctx,
		wildcardCerts,
		client.InNamespace(v1beta1constants.GardenNamespace),
		client.MatchingLabels{v1beta1constants.GardenRole: common.ControlPlaneWildcardCert},
	); err != nil {
		return nil, err
	}

	if len(wildcardCerts.Items) > 1 {
		return nil, fmt.Errorf("misconfigured seed cluster: not possible to provide more than one secret with annotation %s", common.ControlPlaneWildcardCert)
	}

	if len(wildcardCerts.Items) == 1 {
		return &wildcardCerts.Items[0], nil
	}
	return nil, nil
}
