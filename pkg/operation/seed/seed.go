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

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	v1alpha1constants "github.com/gardener/gardener/pkg/apis/core/v1alpha1/constants"
	"github.com/gardener/gardener/pkg/chartrenderer"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions/core/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	gardenletfeatures "github.com/gardener/gardener/pkg/gardenlet/features"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/chart"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	kutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/secrets"
	utilsecrets "github.com/gardener/gardener/pkg/utils/secrets"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/util/retry"
	componentbaseconfig "k8s.io/component-base/config"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	caSeed = "ca-seed"
)

var wantedCertificateAuthorities = map[string]*utilsecrets.CertificateSecretConfig{
	caSeed: {
		Name:       caSeed,
		CommonName: "kubernetes",
		CertType:   utilsecrets.CACert,
	},
}

// New takes a <k8sGardenClient>, the <k8sGardenCoreInformers> and a <seed> manifest, and creates a new Seed representation.
// It will add the CloudProfile and identify the cloud provider.
func New(k8sGardenClient kubernetes.Interface, k8sGardenCoreInformers gardencoreinformers.Interface, seed *gardencorev1alpha1.Seed) (*Seed, error) {
	seedObj := &Seed{Info: seed}

	if seed.Spec.SecretRef != nil {
		secret := &corev1.Secret{}
		if err := k8sGardenClient.Client().Get(context.TODO(), kutil.Key(seed.Spec.SecretRef.Namespace, seed.Spec.SecretRef.Name), secret); err != nil {
			return nil, err
		}
		seedObj.Secret = secret
	}

	return seedObj, nil
}

// NewFromName creates a new Seed object based on the name of a Seed manifest.
func NewFromName(k8sGardenClient kubernetes.Interface, k8sGardenCoreInformers gardencoreinformers.Interface, seedName string) (*Seed, error) {
	seed, err := k8sGardenCoreInformers.Seeds().Lister().Get(seedName)
	if err != nil {
		return nil, err
	}
	return New(k8sGardenClient, k8sGardenCoreInformers, seed)
}

// List returns a list of Seed clusters (along with the referenced secrets).
func List(k8sGardenClient kubernetes.Interface, k8sGardenCoreInformers gardencoreinformers.Interface) ([]*Seed, error) {
	var seedList []*Seed

	list, err := k8sGardenCoreInformers.Seeds().Lister().List(labels.Everything())
	if err != nil {
		return nil, err
	}

	for _, obj := range list {
		seed, err := New(k8sGardenClient, k8sGardenCoreInformers, obj)
		if err != nil {
			return nil, err
		}
		seedList = append(seedList, seed)
	}

	return seedList, nil
}

// GetSeedClient returns the Kubernetes client for the seed cluster. If `inCluster` is set to true then
// the in-cluster client is returned, otherwise the secret reference of the given `seedName` is read
// and a client with the stored kubeconfig is created.
func GetSeedClient(ctx context.Context, gardenClient client.Client, clientConnection componentbaseconfig.ClientConnectionConfiguration, inCluster bool, seedName string) (kubernetes.Interface, error) {
	if inCluster {
		return kubernetes.NewClientFromFile(
			"",
			clientConnection.Kubeconfig,
			kubernetes.WithClientConnectionOptions(clientConnection),
			kubernetes.WithClientOptions(
				client.Options{
					Scheme: kubernetes.SeedScheme,
				},
			),
		)
	}

	seed := &gardencorev1alpha1.Seed{}
	if err := gardenClient.Get(ctx, kutil.Key(seedName), seed); err != nil {
		return nil, err
	}

	if seed.Spec.SecretRef == nil {
		return nil, fmt.Errorf("seed has no secret reference pointing to a kubeconfig - cannot create client")
	}

	seedSecret, err := common.GetSecretFromSecretRef(ctx, gardenClient, seed.Spec.SecretRef)
	if err != nil {
		return nil, err
	}

	return kubernetes.NewClientFromSecretObject(
		seedSecret,
		kubernetes.WithClientConnectionOptions(clientConnection),
		kubernetes.WithClientOptions(client.Options{
			Scheme: kubernetes.SeedScheme,
		}),
	)
}

// generateWantedSecrets returns a list of Secret configuration objects satisfying the secret config intface,
// each containing their specific configuration for the creation of certificates (server/client), RSA key pairs, basic
// authentication credentials, etc.
func generateWantedSecrets(seed *Seed, certificateAuthorities map[string]*utilsecrets.Certificate) ([]utilsecrets.ConfigInterface, error) {
	if len(certificateAuthorities) != len(wantedCertificateAuthorities) {
		return nil, fmt.Errorf("missing certificate authorities")
	}

	secretList := []utilsecrets.ConfigInterface{
		&utilsecrets.CertificateSecretConfig{
			Name: "vpa-tls-certs",

			CommonName:   "vpa-webhook.garden.svc",
			Organization: nil,
			DNSNames:     []string{"vpa-webhook.garden.svc", "vpa-webhook"},
			IPAddresses:  nil,

			CertType:  utilsecrets.ServerCert,
			SigningCA: certificateAuthorities[caSeed],
		},
		&utilsecrets.CertificateSecretConfig{
			Name: "grafana-tls",

			CommonName:   "grafana",
			Organization: []string{"garden.sapcloud.io:monitoring:ingress"},
			DNSNames:     []string{seed.GetIngressFQDN("g-seed", "", "garden")},
			IPAddresses:  nil,

			CertType:  utilsecrets.ServerCert,
			SigningCA: certificateAuthorities[caSeed],
		},
		&utilsecrets.CertificateSecretConfig{
			Name: "aggregate-prometheus-tls",

			CommonName:   "prometheus",
			Organization: []string{"garden.sapcloud.io:monitoring:ingress"},
			DNSNames:     []string{seed.GetIngressFQDN("p-seed", "", "garden")},
			IPAddresses:  nil,

			CertType:  utilsecrets.ServerCert,
			SigningCA: certificateAuthorities[caSeed],
		},
	}

	// Logging feature gate
	if gardenletfeatures.FeatureGate.Enabled(features.Logging) {
		secretList = append(secretList,
			&utilsecrets.CertificateSecretConfig{
				Name: "kibana-tls",

				CommonName:   "kibana",
				Organization: []string{"garden.sapcloud.io:logging:ingress"},
				DNSNames:     []string{seed.GetIngressFQDN("k", "", "garden")},
				IPAddresses:  nil,

				CertType:  utilsecrets.ServerCert,
				SigningCA: certificateAuthorities[caSeed],
			},

			// Secret definition for logging ingress
			&utilsecrets.BasicAuthSecretConfig{
				Name:   "seed-logging-ingress-credentials",
				Format: utilsecrets.BasicAuthFormatNormal,

				Username:       "admin",
				PasswordLength: 32,
			},
			&secrets.BasicAuthSecretConfig{
				Name:   "fluentd-es-sg-credentials",
				Format: secrets.BasicAuthFormatNormal,

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
	_, certificateAuthorities, err := utilsecrets.GenerateCertificateAuthorities(k8sSeedClient, existingSecretsMap, wantedCertificateAuthorities, v1alpha1constants.GardenNamespace)
	if err != nil {
		return nil, err
	}

	wantedSecretsList, err := generateWantedSecrets(seed, certificateAuthorities)
	if err != nil {
		return nil, err
	}

	return utilsecrets.GenerateClusterSecrets(context.TODO(), k8sSeedClient, existingSecretsMap, wantedSecretsList, v1alpha1constants.GardenNamespace)
}

// BootstrapCluster bootstraps a Seed cluster and deploys various required manifests.
func BootstrapCluster(k8sGardenClient kubernetes.Interface, seed *Seed, config *config.GardenletConfiguration, secrets map[string]*corev1.Secret, imageVector imagevector.ImageVector, numberOfAssociatedShoots int) error {
	const chartName = "seed-bootstrap"

	k8sSeedClient, err := GetSeedClient(context.TODO(), k8sGardenClient.Client(), config.SeedClientConnection.ClientConnectionConfiguration, config.SeedSelector == nil, seed.Info.Name)
	if err != nil {
		return err
	}

	gardenNamespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: v1alpha1constants.GardenNamespace,
		},
	}
	if err = k8sSeedClient.Client().Create(context.TODO(), gardenNamespace); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	if _, err := kutils.TryUpdateNamespace(k8sSeedClient.Kubernetes(), retry.DefaultBackoff, gardenNamespace.ObjectMeta, func(ns *corev1.Namespace) (*corev1.Namespace, error) {
		kutils.SetMetaDataLabel(&ns.ObjectMeta, "role", v1alpha1constants.GardenNamespace)
		return ns, nil
	}); err != nil {
		return err
	}
	if _, err := kutils.TryUpdateNamespace(k8sSeedClient.Kubernetes(), retry.DefaultBackoff, metav1.ObjectMeta{Name: metav1.NamespaceSystem}, func(ns *corev1.Namespace) (*corev1.Namespace, error) {
		kutils.SetMetaDataLabel(&ns.ObjectMeta, "role", metav1.NamespaceSystem)
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

			if err := kutils.CreateOrUpdate(context.TODO(), k8sSeedClient.Client(), secretObj, func() error {
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
		},
		imagevector.RuntimeVersion(k8sSeedClient.Version()),
		imagevector.TargetVersion(k8sSeedClient.Version()),
	)
	if err != nil {
		return err
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
		if err = k8sSeedClient.Client().List(context.TODO(), existingSecrets, client.InNamespace(v1alpha1constants.GardenNamespace)); err != nil {
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
		basicAuth = utils.CreateSHA1Secret(credentials.Data[utilsecrets.DataKeyUserName], credentials.Data[utilsecrets.DataKeyPassword])
		kibanaHost = seed.GetIngressFQDN("k", "", "garden")

		sgFluentdCredentials := deployedSecretsMap["fluentd-es-sg-credentials"]
		sgFluentdPassword = string(sgFluentdCredentials.Data[utilsecrets.DataKeyPassword])
		sgFluentdPasswordHash = string(sgFluentdCredentials.Data[utilsecrets.DataKeyPasswordBcryptHash])

		// Read extension provider specific configuration
		existingConfigMaps := &corev1.ConfigMapList{}
		if err = k8sSeedClient.Client().List(context.TODO(), existingConfigMaps,
			client.InNamespace(v1alpha1constants.GardenNamespace),
			client.MatchingLabels{v1alpha1constants.LabelExtensionConfiguration: v1alpha1constants.LabelLogging}); err != nil {
			return err
		}

		// Read all filters and parsers coming from the extension provider configurations
		for _, cm := range existingConfigMaps.Items {
			filters.WriteString(fmt.Sprintln(cm.Data[v1alpha1constants.FluentBitConfigMapKubernetesFilter]))
			parsers.WriteString(fmt.Sprintln(cm.Data[v1alpha1constants.FluentBitConfigMapParser]))
		}
	} else {
		if err := common.DeleteLoggingStack(context.TODO(), k8sSeedClient.Client(), v1alpha1constants.GardenNamespace); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}

	// HVPA feature gate
	var hvpaEnabled = gardenletfeatures.FeatureGate.Enabled(features.HVPA)

	if !hvpaEnabled {
		if err := common.DeleteHvpa(k8sSeedClient, v1alpha1constants.GardenNamespace); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}

	var vpaPodAnnotations map[string]interface{}

	existingSecrets := &corev1.SecretList{}
	if err = k8sSeedClient.Client().List(context.TODO(), existingSecrets, client.InNamespace(v1alpha1constants.GardenNamespace)); err != nil {
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

	vpaPodAnnotations = map[string]interface{}{
		"checksum/secret-vpa-tls-certs": utils.ComputeSHA256Hex(jsonString),
	}

	// Cleanup legacy external admission controller (no longer needed).
	objects := []runtime.Object{
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "gardener-external-admission-controller", Namespace: v1alpha1constants.GardenNamespace}},
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "gardener-external-admission-controller", Namespace: v1alpha1constants.GardenNamespace}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "gardener-external-admission-controller-tls", Namespace: v1alpha1constants.GardenNamespace}},
	}
	for _, object := range objects {
		if err = k8sSeedClient.Client().Delete(context.TODO(), object, kubernetes.DefaultDeleteOptions...); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
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
		if err := common.DeleteAlertmanager(context.TODO(), k8sSeedClient.Client(), v1alpha1constants.GardenNamespace); err != nil {
			return err
		}
	}

	nodes := &corev1.NodeList{}
	if err = k8sSeedClient.Client().List(context.TODO(), nodes); err != nil {
		return err
	}
	nodeCount := len(nodes.Items)

	chartRenderer, err := chartrenderer.NewForConfig(k8sSeedClient.RESTConfig())
	if err != nil {
		return err
	}
	applier, err := kubernetes.NewApplierForConfig(k8sSeedClient.RESTConfig())
	if err != nil {
		return err
	}
	chartApplier := kubernetes.NewChartApplier(chartRenderer, applier)

	var (
		applierOptions          = kubernetes.CopyApplierOptions(kubernetes.DefaultApplierOptions)
		retainStatusInformation = func(new, old *unstructured.Unstructured) {
			// Apply status from old Object to retain status information
			new.Object["status"] = old.Object["status"]
		}
		vpaGK                 = schema.GroupKind{Group: "autoscaling.k8s.io", Kind: "VerticalPodAutoscaler"}
		hvpaGK                = schema.GroupKind{Group: "autoscaling.k8s.io", Kind: "Hvpa"}
		issuerGK              = schema.GroupKind{Group: "certmanager.k8s.io", Kind: "ClusterIssuer"}
		grafanaHost           = seed.GetIngressFQDN("g-seed", "", "garden")
		prometheusHost        = seed.GetIngressFQDN("p-seed", "", "garden")
		monitoringCredentials = existingSecretsMap["seed-monitoring-ingress-credentials"]
		monitoringBasicAuth   string
	)

	if monitoringCredentials != nil {
		monitoringBasicAuth = utils.CreateSHA1Secret(monitoringCredentials.Data[utilsecrets.DataKeyUserName], monitoringCredentials.Data[utilsecrets.DataKeyPassword])
	}
	applierOptions.MergeFuncs[vpaGK] = retainStatusInformation
	applierOptions.MergeFuncs[hvpaGK] = retainStatusInformation
	applierOptions.MergeFuncs[issuerGK] = retainStatusInformation

	privateNetworks, err := common.ToExceptNetworks(
		common.AllPrivateNetworkBlocks(),
		seed.Info.Spec.Networks.Nodes,
		seed.Info.Spec.Networks.Pods,
		seed.Info.Spec.Networks.Services)
	if err != nil {
		return err
	}

	err = chartApplier.ApplyChartWithOptions(context.TODO(), filepath.Join("charts", chartName), v1alpha1constants.GardenNamespace, chartName, nil, map[string]interface{}{
		"cloudProvider": seed.Info.Spec.Provider.Type,
		"global": map[string]interface{}{
			"images": chart.ImageMapToValues(images),
		},
		"reserveExcessCapacity": seed.reserveExcessCapacity,
		"replicas": map[string]interface{}{
			"reserve-excess-capacity": DesiredExcessCapacity(numberOfAssociatedShoots),
		},
		"prometheus": map[string]interface{}{
			"storage": seed.GetValidVolumeSize("10Gi"),
		},
		"aggregatePrometheus": map[string]interface{}{
			"storage": seed.GetValidVolumeSize("20Gi"),
			"seed":    seed.Info.Name,
			"host":    prometheusHost,
		},
		"grafana": map[string]interface{}{
			"host": grafanaHost,
		},
		"elastic-kibana-curator": map[string]interface{}{
			"enabled": loggingEnabled,
			"ingress": map[string]interface{}{
				"basicAuthSecret": basicAuth,
				"host":            kibanaHost,
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
			"podAnnotations": vpaPodAnnotations,
		},
		"hvpa": map[string]interface{}{
			"enabled": hvpaEnabled,
		},
		"global-network-policies": map[string]interface{}{
			// TODO (mvladev): Move the Provider specific metadata IP
			// somewhere else, so it's accessible here.
			// "metadataService": "169.254.169.254/32"
			"denyAll":         false,
			"privateNetworks": privateNetworks,
		},
		"gardenerResourceManager": map[string]interface{}{
			"resourceClass": v1alpha1constants.SeedResourceManagerClass,
		},
		"ingress": map[string]interface{}{
			"basicAuthSecret": monitoringBasicAuth,
		},
	}, applierOptions)

	if err != nil {
		return err
	}

	// Delete the shoot specific dependency-watchdog deployments in the
	// invidual shoot control-planes in favour of the central deployment
	// in the seed-bootstrap.
	// TODO: This code is to be removed in the next release.
	return deleteControlPlaneDependencyWatchdogs(k8sSeedClient.Client())
}

// DesiredExcessCapacity computes the required resources (CPU and memory) required to deploy new shoot control planes
// (on the seed) in terms of reserve-excess-capacity deployment replicas. Each deployment replica currently
// corresponds to resources of (request/limits) 500m of CPU and 1200Mi of Memory.
// ReplicasRequiredToSupportSingleShoot is 4 which is 2000m of CPU and 4800Mi of RAM.
// The logic for computation of desired excess capacity corresponds to either deploying 2 new shoot control planes
// or 3% of existing shoot control planes of current number of shoots deployed in seed (3 if current shoots are 100),
// whichever of the two is larger
func DesiredExcessCapacity(numberOfAssociatedShoots int) int {
	var (
		replicasToSupportSingleShoot          = 4
		effectiveExcessCapacity               = 2
		excessCapacityBasedOnAssociatedShoots = int(float64(numberOfAssociatedShoots) * 0.03)
	)

	if excessCapacityBasedOnAssociatedShoots > effectiveExcessCapacity {
		effectiveExcessCapacity = excessCapacityBasedOnAssociatedShoots
	}

	return effectiveExcessCapacity * replicasToSupportSingleShoot
}

// GetFluentdReplicaCount returns fluentd stateful set replica count if it exists, otherwise - the default (1).
// As fluentd HPA manages the number of replicas, we have to make sure to do not override HPA scaling.
func GetFluentdReplicaCount(k8sSeedClient kubernetes.Interface) (int32, error) {
	statefulSet := &appsv1.StatefulSet{}
	if err := k8sSeedClient.Client().Get(context.TODO(), kutil.Key(v1alpha1constants.GardenNamespace, common.FluentdEsStatefulSetName), statefulSet); err != nil {
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

// GetIngressFQDN returns the fully qualified domain name of ingress sub-resource for the Seed cluster. The
// end result is '<subDomain>.<shootName>.<projectName>.<seed-ingress-domain>'.
func (s *Seed) GetIngressFQDN(subDomain, shootName, projectName string) string {
	if shootName == "" {
		return fmt.Sprintf("%s.%s.%s", subDomain, projectName, s.Info.Spec.DNS.IngressDomain)
	}
	return fmt.Sprintf("%s.%s.%s.%s", subDomain, shootName, projectName, s.Info.Spec.DNS.IngressDomain)
}

// CheckMinimumK8SVersion checks whether the Kubernetes version of the Seed cluster fulfills the minimal requirements.
func (s *Seed) CheckMinimumK8SVersion(ctx context.Context, k8sGardenClient client.Client, clientConnection componentbaseconfig.ClientConnectionConfiguration, inCluster bool) (string, error) {
	// We require CRD status subresources for the extension controllers that we install into the seeds.
	minSeedVersion := "1.11"

	k8sSeedClient, err := GetSeedClient(ctx, k8sGardenClient, clientConnection, inCluster, s.Info.Name)
	if err != nil {
		return "<unknown>", err
	}

	version := k8sSeedClient.Version()

	seedVersionOK, err := utils.CompareVersions(version, ">=", minSeedVersion)
	if err != nil {
		return "<unknown>", err
	}
	if !seedVersionOK {
		return "<unknown>", fmt.Errorf("the Kubernetes version of the Seed cluster must be at least %s", minSeedVersion)
	}
	return version, nil
}

// MustReserveExcessCapacity configures whether we have to reserve excess capacity in the Seed cluster.
func (s *Seed) MustReserveExcessCapacity(must bool) {
	s.reserveExcessCapacity = must
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

type _continue string

func (c _continue) ApplyToList(opts *client.ListOptions) {
	if opts.Raw == nil {
		opts.Raw = &metav1.ListOptions{}
	}
	opts.Raw.Continue = string(c)
}

// deleteControlPlaneDependencyWatchdogs deletes the shoot specific dependency-watchdog
// deployments in the invidual shoot control-planes in favour of the central deployment
// in the seed-bootstrap.
// TODO: This code is to be removed in the next release.
func deleteControlPlaneDependencyWatchdogs(crClient client.Client) error {
	var continueToken string

	for {
		list := &corev1.NamespaceList{}
		if err := crClient.List(context.TODO(), list, _continue(continueToken)); err != nil {
			return nil
		}

		for i := range list.Items {
			ns := &list.Items[i]
			if ns.DeletionTimestamp != nil {
				continue // Already deleted
			}

			if err := deleteDependencyWatchdogFromNS(crClient, ns.Name); err != nil {
				return err
			}
		}

		if list.Continue == "" {
			break
		}
		continueToken = list.Continue
	}

	return nil
}

func deleteDependencyWatchdogFromNS(crClient client.Client, ns string) error {
	for _, obj := range []struct {
		apiGroup string
		version  string
		kind     string
		name     string
	}{
		{"autoscaling.k8s.io", "v1beta2", "VerticalPodAutoscaler", v1alpha1constants.VPANameDependencyWatchdog},
		{"autoscaling.k8s.io", "v1beta2", "VerticalPodAutoscalerCheckpoint", v1alpha1constants.VPANameDependencyWatchdog},
		{"apps", "v1", "Deployment", v1alpha1constants.DeploymentNameDependencyWatchdog},
		{"", "v1", "ConfigMap", v1alpha1constants.ConfigMapNameDependencyWatchdog},
		{"rbac.authorization.k8s.io", "v1", "RoleBinding", v1alpha1constants.RoleBindingNameDependencyWatchdog},
		{"", "v1", "ServiceAccount", v1alpha1constants.ServiceAccountNameDependencyWatchdog},
		{"", "v1", "Secret", common.DeprecatedKubecfgInternalProbeSecretName},
	} {
		u := &unstructured.Unstructured{}
		u.SetName(obj.name)
		u.SetNamespace(ns)
		u.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   obj.apiGroup,
			Version: obj.version,
			Kind:    obj.kind,
		})
		if err := crClient.Delete(context.TODO(), u); client.IgnoreNotFound(err) != nil {
			return err
		}
	}

	return nil
}
