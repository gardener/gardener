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

	"github.com/gardener/gardener/pkg/apis/garden"
	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/apis/garden/v1beta1/helper"
	"github.com/gardener/gardener/pkg/chartrenderer"
	gardeninformers "github.com/gardener/gardener/pkg/client/garden/informers/externalversions/garden/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	controllermanagerfeatures "github.com/gardener/gardener/pkg/controllermanager/features"
	"github.com/gardener/gardener/pkg/features"
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

// New takes a <k8sGardenClient>, the <k8sGardenInformers> and a <seed> manifest, and creates a new Seed representation.
// It will add the CloudProfile and identify the cloud provider.
func New(k8sGardenClient kubernetes.Interface, k8sGardenInformers gardeninformers.Interface, seed *gardenv1beta1.Seed) (*Seed, error) {
	secret := &corev1.Secret{}
	if err := k8sGardenClient.Client().Get(context.TODO(), kutil.Key(seed.Spec.SecretRef.Namespace, seed.Spec.SecretRef.Name), secret); err != nil {
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

// DetermineCloudProviderForSeed determines the cloud provider for the given seed.
func DetermineCloudProviderForSeed(ctx context.Context, c client.Client, seed *gardenv1beta1.Seed) (gardenv1beta1.CloudProvider, error) {
	cloudProfile := &gardenv1beta1.CloudProfile{}
	if err := c.Get(ctx, kutil.Key(seed.Spec.Cloud.Profile), cloudProfile); err != nil {
		return "", err
	}

	return helper.DetermineCloudProviderInProfile(cloudProfile.Spec)
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
	if len(certificateAuthorities) != len(wantedCertificateAuthorities) {
		return nil, fmt.Errorf("missing certificate authorities")
	}

	secretList := []utilsecrets.ConfigInterface{}

	// Logging feature gate
	if controllermanagerfeatures.FeatureGate.Enabled(features.Logging) {
		secretList = append(secretList,
			&utilsecrets.CertificateSecretConfig{
				Name: "kibana-tls",

				CommonName:   "kibana",
				Organization: []string{fmt.Sprintf("%s:logging:ingress", garden.GroupName)},
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

	// VPA feature gate
	if controllermanagerfeatures.FeatureGate.Enabled(features.VPA) {
		secretList = append(secretList,
			&utilsecrets.CertificateSecretConfig{
				Name: "vpa-tls-certs",

				CommonName:   "vpa-webhook.garden.svc",
				Organization: nil,
				DNSNames:     []string{"vpa-webhook.garden.svc", "vpa-webhook"},
				IPAddresses:  nil,

				CertType:  utilsecrets.ServerCert,
				SigningCA: certificateAuthorities[caSeed],
			},
		)
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
	if err = k8sSeedClient.Client().Create(context.TODO(), gardenNamespace); err != nil && !apierrors.IsAlreadyExists(err) {
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
			common.KibanaImageName,
			common.PauseContainerImageName,
			common.PrometheusImageName,
			common.VpaAdmissionControllerImageName,
			common.VpaExporterImageName,
			common.VpaRecommenderImageName,
			common.VpaUpdaterImageName,
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
		loggingEnabled        = controllermanagerfeatures.FeatureGate.Enabled(features.Logging)
		existingSecretsMap    = map[string]*corev1.Secret{}
	)

	if loggingEnabled {
		existingSecrets := &corev1.SecretList{}
		if err = k8sSeedClient.Client().List(context.TODO(), existingSecrets, client.InNamespace(common.GardenNamespace)); err != nil {
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
	} else {
		if err := common.DeleteLoggingStack(k8sSeedClient, common.GardenNamespace); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}

	// VPA feature gate
	var (
		vpaEnabled        = controllermanagerfeatures.FeatureGate.Enabled(features.VPA)
		vpaPodAnnotations map[string]interface{}
	)

	if vpaEnabled {
		existingSecrets := &corev1.SecretList{}
		if err = k8sSeedClient.Client().List(context.TODO(), existingSecrets, client.InNamespace(common.GardenNamespace)); err != nil {
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
	} else {
		if err := common.DeleteVpa(k8sSeedClient, common.GardenNamespace); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}

	// Cleanup legacy external admission controller (no longer needed).
	objects := []runtime.Object{
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "gardener-external-admission-controller", Namespace: common.GardenNamespace}},
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "gardener-external-admission-controller", Namespace: common.GardenNamespace}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "gardener-external-admission-controller-tls", Namespace: common.GardenNamespace}},
	}
	for _, object := range objects {
		if err = k8sSeedClient.Client().Delete(context.TODO(), object, kubernetes.DefaultDeleteOptionFuncs...); err != nil && !apierrors.IsNotFound(err) {
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
		vpaGK    = schema.GroupKind{Group: "autoscaling.k8s.io", Kind: "VerticalPodAutoscaler"}
		issuerGK = schema.GroupKind{Group: "certmanager.k8s.io", Kind: "ClusterIssuer"}
	)

	applierOptions.MergeFuncs[vpaGK] = retainStatusInformation
	applierOptions.MergeFuncs[issuerGK] = retainStatusInformation

	privateNetworks, err := common.ToExceptNetworks(
		common.AllPrivateNetworkBlocks(),
		seed.Info.Spec.Networks.Nodes,
		seed.Info.Spec.Networks.Pods,
		seed.Info.Spec.Networks.Services)
	if err != nil {
		return err
	}

	return chartApplier.ApplyChartWithOptions(context.TODO(), filepath.Join("charts", chartName), common.GardenNamespace, chartName, nil, map[string]interface{}{
		"cloudProvider": seed.CloudProvider,
		"global": map[string]interface{}{
			"images": chart.ImageMapToValues(images),
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
		},
		"alertmanager": alertManagerConfig,
		"vpa": map[string]interface{}{
			"enabled":        vpaEnabled,
			"podAnnotations": vpaPodAnnotations,
		},
		"global-network-policies": map[string]interface{}{
			// TODO (mvladev): Move the Provider specific metadata IP
			// somewhere else, so it's accessible here.
			// "metadataService": "169.254.169.254/32"
			"denyAll":         false,
			"privateNetworks": privateNetworks,
		},
	}, applierOptions)
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
	if err := k8sSeedClient.Client().Get(context.TODO(), kutil.Key(common.GardenNamespace, common.FluentdEsStatefulSetName), statefulSet); err != nil {
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

// GetPersistentVolumeProvider gets the Persistent Volume Provider of seed cluster. If it is not specified, return ""
func (s *Seed) GetPersistentVolumeProvider() string {
	if s.Info.Annotations == nil {
		return ""
	}

	return s.Info.Annotations[common.AnnotatePersistentVolumeProvider]
}
