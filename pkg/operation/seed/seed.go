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
	"fmt"
	"path/filepath"

	"github.com/gardener/gardener/pkg/apis/garden"
	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/apis/garden/v1beta1/helper"
	"github.com/gardener/gardener/pkg/chartrenderer"
	gardeninformers "github.com/gardener/gardener/pkg/client/garden/informers/externalversions/garden/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	controllermanagerfeatures "github.com/gardener/gardener/pkg/controllermanager/features"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/operation/certmanagement"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	kutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	utilsecrets "github.com/gardener/gardener/pkg/utils/secrets"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/util/retry"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	caSeed = "ca-seed"
)

var wantedCertificateAuthorities = map[string]*utilsecrets.CertificateSecretConfig{
	caSeed: &utilsecrets.CertificateSecretConfig{
		Name:       caSeed,
		CommonName: "kubernetes",
		CertType:   utilsecrets.CACert,
	},
}

// New takes a <k8sGardenClient>, the <k8sGardenInformers> and a <seed> manifest, and creates a new Seed representation.
// It will add the CloudProfile and identify the cloud provider.
func New(k8sGardenClient kubernetes.Interface, k8sGardenInformers gardeninformers.Interface, seed *gardenv1beta1.Seed) (*Seed, error) {
	secret, err := k8sGardenClient.GetSecret(seed.Spec.SecretRef.Namespace, seed.Spec.SecretRef.Name)
	if err != nil {
		return nil, err
	}

	cloudProfile, err := k8sGardenInformers.CloudProfiles().Lister().Get(seed.Spec.Cloud.Profile)
	if err != nil {
		return nil, err
	}

	seedObj := &Seed{
		Info:         seed,
		Secret:       secret,
		CloudProfile: cloudProfile,
	}

	cloudProvider, err := helper.DetermineCloudProviderInProfile(cloudProfile.Spec)
	if err != nil {
		return nil, err
	}
	seedObj.CloudProvider = cloudProvider

	return seedObj, nil
}

// NewFromName creates a new Seed object based on the name of a Seed manifest.
func NewFromName(k8sGardenClient kubernetes.Interface, k8sGardenInformers gardeninformers.Interface, seedName string) (*Seed, error) {
	seed, err := k8sGardenInformers.Seeds().Lister().Get(seedName)
	if err != nil {
		return nil, err
	}
	return New(k8sGardenClient, k8sGardenInformers, seed)
}

// List returns a list of Seed clusters (along with the referenced secrets).
func List(k8sGardenClient kubernetes.Interface, k8sGardenInformers gardeninformers.Interface) ([]*Seed, error) {
	var seedList []*Seed

	list, err := k8sGardenInformers.Seeds().Lister().List(labels.Everything())
	if err != nil {
		return nil, err
	}

	for _, obj := range list {
		seed, err := New(k8sGardenClient, k8sGardenInformers, obj)
		if err != nil {
			return nil, err
		}
		seedList = append(seedList, seed)
	}

	return seedList, nil
}

// generateWantedSecrets returns a list of Secret configuration objects satisfying the secret config intface,
// each containing their specific configuration for the creation of certificates (server/client), RSA key pairs, basic
// authentication credentials, etc.
func generateWantedSecrets(seed *Seed, certificateAuthorities map[string]*utilsecrets.Certificate) ([]utilsecrets.ConfigInterface, error) {
	kibanaHost := seed.GetIngressFQDN("k", "", "garden")

	if len(certificateAuthorities) != len(wantedCertificateAuthorities) {
		return nil, fmt.Errorf("missing certificate authorities")
	}

	secretList := []utilsecrets.ConfigInterface{
		&utilsecrets.CertificateSecretConfig{
			Name: "kibana-tls",

			CommonName:   "kibana",
			Organization: []string{fmt.Sprintf("%s:logging:ingress", garden.GroupName)},
			DNSNames:     []string{kibanaHost},
			IPAddresses:  nil,

			CertType:  utilsecrets.ServerCert,
			SigningCA: certificateAuthorities[caSeed],
		},

		// Secret definition for monitoring
		&utilsecrets.BasicAuthSecretConfig{
			Name:   "seed-logging-ingress-credentials",
			Format: utilsecrets.BasicAuthFormatNormal,

			Username:       "admin",
			PasswordLength: 32,
		},
	}

	return secretList, nil
}

// deployCertificates deploys CA and TLS certificates inside the garden namespace
// It takes a map[string]*corev1.Secret object which contains secrets that have already been deployed inside that namespace to avoid duplication errors.
func deployCertificates(seed *Seed, k8sSeedClient kubernetes.Interface, existingSecretsMap map[string]*corev1.Secret) (map[string]*corev1.Secret, error) {
	_, certificateAuthorities, err := utilsecrets.GenerateCertificateAuthorities(k8sSeedClient, existingSecretsMap, wantedCertificateAuthorities, common.GardenNamespace)
	if err != nil {
		return nil, err
	}

	wantedSecretsList, err := generateWantedSecrets(seed, certificateAuthorities)
	if err != nil {
		return nil, err
	}

	return utilsecrets.GenerateClusterSecrets(k8sSeedClient, existingSecretsMap, wantedSecretsList, common.GardenNamespace)
}

// BootstrapCluster bootstraps a Seed cluster and deploys various required manifests.
func BootstrapCluster(seed *Seed, secrets map[string]*corev1.Secret, imageVector imagevector.ImageVector, numberOfAssociatedShoots int) error {
	const chartName = "seed-bootstrap"
	var existingSecretsMap = map[string]*corev1.Secret{}

	k8sSeedClient, err := kubernetes.NewClientFromSecretObject(seed.Secret, client.Options{
		Scheme: kubernetes.SeedScheme,
	})
	if err != nil {
		return err
	}

	gardenNamespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: common.GardenNamespace,
		},
	}
	if _, err := k8sSeedClient.CreateNamespace(gardenNamespace, false); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	if _, err := kutils.TryUpdateNamespace(k8sSeedClient.Kubernetes(), retry.DefaultBackoff, gardenNamespace.ObjectMeta, func(ns *corev1.Namespace) (*corev1.Namespace, error) {
		kutils.SetMetaDataLabel(&ns.ObjectMeta, "role", common.GardenNamespace)
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

	images, err := imageVector.FindImages([]string{
		common.AlertManagerImageName,
		common.AlpineImageName,
		common.CertManagerImageName,
		common.ConfigMapReloaderImageName,
		common.CuratorImageName,
		common.ElasticsearchImageName,
		common.FluentBitImageName,
		common.FluentdEsImageName,
		common.GardenerExternalAdmissionControllerImageName,
		common.KibanaImageName,
		common.PauseContainerImageName,
		common.PrometheusImageName,
	}, k8sSeedClient.Version(), k8sSeedClient.Version())
	if err != nil {
		return err
	}

	// Logging feature gate

	var (
		basicAuth      string
		kibanaHost     string
		loggingEnabled = controllermanagerfeatures.FeatureGate.Enabled(features.Logging)
	)

	if loggingEnabled {
		existingSecrets, err := k8sSeedClient.ListSecrets(common.GardenNamespace, metav1.ListOptions{})
		if err != nil {
			return err
		}

		for _, secret := range existingSecrets.Items {
			secretObj := secret
			existingSecretsMap[secret.ObjectMeta.Name] = &secretObj
		}

		// currently the generated certificates are only used by the kibana so they are all disabled/enabled when the logging feature is disabled/enabled
		deployedSecretsMap, err := deployCertificates(seed, k8sSeedClient, existingSecretsMap)
		if err != nil {
			return err
		}

		credentials := deployedSecretsMap["seed-logging-ingress-credentials"]
		basicAuth = utils.CreateSHA1Secret(credentials.Data[utilsecrets.DataKeyUserName], credentials.Data[utilsecrets.DataKeyPassword])

		kibanaHost = seed.GetIngressFQDN("k", "", "garden")
	} else {
		if err := common.DeleteLoggingStack(k8sSeedClient, common.GardenNamespace); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}

	// Certificate Management feature gate
	var (
		clusterIssuer      map[string]interface{}
		certManagerEnabled = controllermanagerfeatures.FeatureGate.Enabled(features.CertificateManagement)
	)

	if certManagerEnabled {
		certificateManagement, ok := secrets[common.GardenRoleCertificateManagement]
		if !ok {
			return fmt.Errorf("certificate management is enabled but no secret could be found with role: %s", common.GardenRoleCertificateManagement)
		}

		clusterIssuer, err = createClusterIssuer(k8sSeedClient, certificateManagement)
		if err != nil {
			return fmt.Errorf("cannot create Cluster Issuer for certificate management: %v", err)
		}
	} else {
		if err := k8sSeedClient.DeleteDeployment(common.GardenNamespace, common.CertManagerResourceName); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}

	// AlertManager configuration
	alertManagerConfig := map[string]interface{}{
		"storage": seed.GetValidVolumeSize("1Gi"),
	}

	if alertingSMTPKeys := common.GetSecretKeysWithPrefix(common.GardenRoleAlertingSMTP, secrets); len(alertingSMTPKeys) > 0 {
		emailConfigs := make([]map[string]interface{}, 0, len(alertingSMTPKeys))
		for _, key := range alertingSMTPKeys {
			secret := secrets[key]
			emailConfigs = append(emailConfigs, map[string]interface{}{
				"to":            string(secret.Data["to"]),
				"from":          string(secret.Data["from"]),
				"smarthost":     string(secret.Data["smarthost"]),
				"auth_username": string(secret.Data["auth_username"]),
				"auth_identity": string(secret.Data["auth_identity"]),
				"auth_password": string(secret.Data["auth_password"]),
			})
		}
		alertManagerConfig["emailConfigs"] = emailConfigs
	}

	nodes, err := k8sSeedClient.ListNodes(metav1.ListOptions{})
	if err != nil {
		return err
	}
	nodeCount := len(nodes.Items)

	chartRenderer, err := chartrenderer.New(k8sSeedClient.Kubernetes())
	if err != nil {
		return err
	}

	var (
		applierOptions     = kubernetes.DefaultApplierOptions
		clusterIssuerMerge = func(new, old *unstructured.Unstructured) {
			// Apply status from old ClusterIssuer to retain the issuer's readiness state.
			new.Object["status"] = old.Object["status"]
		}
	)
	applierOptions.MergeFuncs["ClusterIssuer"] = clusterIssuerMerge

	return common.ApplyChartWithOptions(k8sSeedClient, chartRenderer, filepath.Join("charts", chartName), chartName, common.GardenNamespace, nil, map[string]interface{}{
		"cloudProvider": seed.CloudProvider,
		"global": map[string]interface{}{
			"images": images,
		},
		"reserveExcessCapacity": seed.reserveExcessCapacity,
		"replicas": map[string]interface{}{
			"reserve-excess-capacity": DesiredExcessCapacity(numberOfAssociatedShoots),
		},
		"prometheus": map[string]interface{}{
			"objectCount": nodeCount,
			"storage":     seed.GetValidVolumeSize("10Gi"),
		},
		"elastic-kibana-curator": map[string]interface{}{
			"enabled": loggingEnabled,
			"ingress": map[string]interface{}{
				"basicAuthSecret": basicAuth,
				"host":            kibanaHost,
			},
			"curator": map[string]interface{}{
				"objectCount":       nodeCount,
				"baseDiskThreshold": 2 * 1073741824,
			},
			"elasticsearch": map[string]interface{}{
				"objectCount":               nodeCount,
				"elasticsearchVolumeSizeGB": 100,
			},
		},
		"fluentd-es": map[string]interface{}{
			"enabled": loggingEnabled,
			"fluentd": map[string]interface{}{
				"storage": seed.GetValidVolumeSize("9Gi"),
			},
		},
		"cert-manager": map[string]interface{}{
			"enabled":       certManagerEnabled,
			"clusterissuer": clusterIssuer,
		},
		"alertmanager": alertManagerConfig,
	}, applierOptions)
}

func createClusterIssuer(k8sSeedclient kubernetes.Interface, certificateManagement *corev1.Secret) (map[string]interface{}, error) {
	certManagementConfig, err := certmanagement.RetrieveCertificateManagementConfig(certificateManagement)
	if err != nil {
		return nil, err
	}

	var (
		clusterIssuerName = certManagementConfig.ClusterIssuerName
		acmeConfig        = certManagementConfig.ACME
		route53Config     = certManagementConfig.Providers.Route53
		clouddnsConfig    = certManagementConfig.Providers.CloudDNS
	)

	var dnsProviders []certmanagement.DNSProviderConfig
	for _, route53provider := range route53Config {
		it := route53provider
		dnsProviders = append(dnsProviders, &it)
	}
	for _, cloudDNSProvider := range clouddnsConfig {
		it := cloudDNSProvider
		dnsProviders = append(dnsProviders, &it)
	}

	var (
		letsEncryptSecretName = "lets-encrypt"
		providers             = createDNSProviderValues(dnsProviders)
		acmePrivateKey        = acmeConfig.ACMEPrivateKey()
	)

	return map[string]interface{}{
		"name": string(clusterIssuerName),
		"acme": map[string]interface{}{
			"email":  acmeConfig.Email,
			"server": acmeConfig.Server,
			"letsEncrypt": map[string]interface{}{
				"name": letsEncryptSecretName,
				"key":  acmePrivateKey,
			},
			"dns01": map[string]interface{}{
				"providers": providers,
			},
		},
	}, nil
}

func createDNSProviderValues(configs []certmanagement.DNSProviderConfig) []interface{} {
	var providers []interface{}
	for _, config := range configs {
		name := config.ProviderName()
		switch config.DNSProvider() {
		case certmanagement.Route53:
			route53config, ok := config.(*certmanagement.Route53Config)
			if !ok {
				logger.Logger.Errorf("Failed to cast to Route53Config object for DNSProviderConfig  %+v", config)
				return nil
			}

			providers = append(providers, map[string]interface{}{
				"name":        name,
				"type":        certmanagement.Route53,
				"region":      route53config.Region,
				"accessKeyID": route53config.AccessKeyID,
				"accessKey":   route53config.AccessKey(),
			})
		case certmanagement.CloudDNS:
			cloudDNSConfig, ok := config.(*certmanagement.CloudDNSConfig)
			if !ok {
				logger.Logger.Errorf("Failed to cast to CloudDNSConfig object for DNSProviderConfig  %+v", config)
				return nil
			}

			providers = append(providers, map[string]interface{}{
				"name":      name,
				"type":      certmanagement.CloudDNS,
				"project":   cloudDNSConfig.Project,
				"accessKey": cloudDNSConfig.AccessKey(),
			})
		default:
		}
	}
	return providers
}

// DesiredExcessCapacity computes the required resources (CPU and memory) required to deploy new shoot control planes
// (on the seed) in terms of reserve-excess-capacity deployment replicas. Each deployment replica currently
// corresponds to resources of (request/limits) 500m of CPU and 1200Mi of Memory.
// ReplicasRequiredToSupportSingleShoot is 4 which is 2000m of CPU and 4800Mi of RAM.
// The logic for computation of desired excess capacity corresponds to either deploying 3 new shoot control planes
// or 5% of existing shoot control planes of current number of shoots deployed in seed (5 if current shoots are 100),
// whichever of the two is larger
func DesiredExcessCapacity(numberOfAssociatedShoots int) int {
	var (
		replicasToSupportSingleShoot          = 4
		effectiveExcessCapacity               = 3
		excessCapacityBasedOnAssociatedShoots = int(float64(numberOfAssociatedShoots) * 0.05)
	)

	if excessCapacityBasedOnAssociatedShoots > effectiveExcessCapacity {
		effectiveExcessCapacity = excessCapacityBasedOnAssociatedShoots
	}

	return effectiveExcessCapacity * replicasToSupportSingleShoot
}

// GetIngressFQDN returns the fully qualified domain name of ingress sub-resource for the Seed cluster. The
// end result is '<subDomain>.<shootName>.<projectName>.<seed-ingress-domain>'.
func (s *Seed) GetIngressFQDN(subDomain, shootName, projectName string) string {
	if shootName == "" {
		return fmt.Sprintf("%s.%s.%s", subDomain, projectName, s.Info.Spec.IngressDomain)
	}
	return fmt.Sprintf("%s.%s.%s.%s", subDomain, shootName, projectName, s.Info.Spec.IngressDomain)
}

// CheckMinimumK8SVersion checks whether the Kubernetes version of the Seed cluster fulfills the minimal requirements.
func (s *Seed) CheckMinimumK8SVersion() error {
	// We require CRD status subresources for the extension controllers that we install into the seeds.
	// CRD status subresources are alpha in 1.10 and can be enabled with the `CustomResourceSubresources` feature gate.
	// They are enabled by default in 1.11. We allow 1.10 but users must make sure that the feature gate is enabled in
	// this case.
	minSeedVersion := "1.10"

	k8sSeedClient, err := kubernetes.NewClientFromSecretObject(s.Secret, client.Options{
		Scheme: kubernetes.SeedScheme,
	})
	if err != nil {
		return err
	}

	seedVersionOK, err := utils.CompareVersions(k8sSeedClient.Version(), ">=", minSeedVersion)
	if err != nil {
		return err
	}
	if !seedVersionOK {
		return fmt.Errorf("the Kubernetes version of the Seed cluster must be at least %s", minSeedVersion)
	}
	return nil
}

// MustReserveExcessCapacity configures whether we have to reserve excess capacity in the Seed cluster.
func (s *Seed) MustReserveExcessCapacity(must bool) {
	s.reserveExcessCapacity = must
}

// GetValidVolumeSize is to get a valid volume size.
// If the given size is smaller than the minimum volume size permitted by cloud provider on which seed cluster is running, it will return the minimum size.
func (s *Seed) GetValidVolumeSize(size string) string {
	if s.Info.Annotations == nil {
		return size
	}

	smv, ok := s.Info.Annotations[common.AnnotatePersistentVolumeMinimumSize]
	if ok {
		if qmv, err := resource.ParseQuantity(smv); err == nil {
			qs, err := resource.ParseQuantity(size)
			if err == nil && qs.Cmp(qmv) < 0 {
				return smv
			}
		}
	}

	return size
}
