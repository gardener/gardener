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

package hybridbotanist

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/chartrenderer"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/operation/shoot"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/secrets"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	bootstraptokenapi "k8s.io/cluster-bootstrap/token/api"
	bootstraptokenutil "k8s.io/cluster-bootstrap/token/util"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var operatingSystemConfigChartPath = filepath.Join(common.ChartPath, "seed-operatingsystemconfig")

// first hard, second soft
func getEvictionMemoryAvailable(machineTypes []gardenv1beta1.MachineType, machineType string) (string, string) {
	memoryThreshold, _ := resource.ParseQuantity("8Gi")

	for _, machtype := range machineTypes {
		if machtype.Name == machineType {
			if machtype.Memory.Cmp(memoryThreshold) > 0 {
				return "1Gi", "1.5Gi"
			}
			return "5%", "10%"
		}
	}
	return "100Mi", "200Mi"
}

// ComputeShootOperatingSystemConfig generates the shoot operating system configuration. Both, the downloader
// and original configuration will be generated and stored in the shoot specific cloud config map for later usage.
func (b *HybridBotanist) ComputeShootOperatingSystemConfig() error {
	var (
		machineTypes     = b.Shoot.GetMachineTypesFromCloudProfile()
		machineImageName = b.Shoot.GetMachineImageName()
	)

	downloaderConfig := b.generateDownloaderConfig(machineImageName)
	originalConfig, err := b.generateOriginalConfig()
	if err != nil {
		return err
	}

	type oscOutput struct {
		workerName  string
		cloudConfig *shoot.CloudConfig
		err         error
	}

	var (
		results   = make(chan *oscOutput)
		wg        sync.WaitGroup
		errorList = []error{}
	)

	for _, worker := range b.Shoot.GetWorkers() {
		wg.Add(1)

		go func(worker gardenv1beta1.Worker) {
			defer wg.Done()
			cloudConfig, err := b.computeOperatingSystemConfigsForWorker(machineTypes, machineImageName, utils.MergeMaps(downloaderConfig, nil), utils.MergeMaps(originalConfig, nil), worker)
			results <- &oscOutput{worker.Name, cloudConfig, err}
		}(worker)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	for out := range results {
		if out.err != nil {
			errorList = append(errorList, out.err)
			continue
		}
		b.Shoot.CloudConfigMap[out.workerName] = *out.cloudConfig
	}

	if len(errorList) > 0 {
		return fmt.Errorf("errors occurred during operating system config generation: %+v", errorList)
	}
	return nil
}

func (b *HybridBotanist) generateDownloaderConfig(machineImageName gardenv1beta1.MachineImageName) map[string]interface{} {
	return map[string]interface{}{
		"type":    machineImageName,
		"purpose": extensionsv1alpha1.OperatingSystemConfigPurposeProvision,
		"server":  fmt.Sprintf("https://%s", b.Shoot.ComputeAPIServerURL(false, true)),
	}
}

func (b *HybridBotanist) generateOriginalConfig() (map[string]interface{}, error) {
	var (
		userDataConfig = b.ShootCloudBotanist.GenerateCloudConfigUserDataConfig()
		serviceNetwork = b.Shoot.GetServiceNetwork()

		cloudProvider = map[string]interface{}{
			"name": b.ShootCloudBotanist.GetCloudProviderName(),
		}

		originalConfig = map[string]interface{}{
			"cloudProvider": cloudProvider,
			"kubernetes": map[string]interface{}{
				"clusterDNS": common.ComputeClusterIP(serviceNetwork, 10),
				// TODO: resolve conformance test issue before changing:
				// https://github.com/kubernetes/kubernetes/blob/master/test/e2e/network/dns.go#L44
				"domain": gardenv1beta1.DefaultDomain,
				"kubelet": map[string]interface{}{
					"caCert":             string(b.Secrets["ca-kubelet"].Data[secrets.DataKeyCertificateCA]),
					"parameters":         userDataConfig.KubeletParameters,
					"hostnameOverride":   userDataConfig.HostnameOverride,
					"enableCSI":          userDataConfig.EnableCSI,
					"providerIDProvided": userDataConfig.ProviderIDProvided,
				},
				"version": b.Shoot.Info.Spec.Kubernetes.Version,
			},
		}
	)

	if userDataConfig.ProvisionCloudProviderConfig {
		cloudProviderConfig, err := b.ShootCloudBotanist.GenerateCloudProviderConfig()
		if err != nil {
			return nil, err
		}
		cloudProvider["config"] = cloudProviderConfig
	}

	kubeletConfig := b.Shoot.Info.Spec.Kubernetes.Kubelet
	if kubeletConfig != nil {
		originalConfig["kubernetes"].(map[string]interface{})["kubelet"].(map[string]interface{})["featureGates"] = kubeletConfig.FeatureGates
	}

	if caBundle := b.Shoot.CloudProfile.Spec.CABundle; caBundle != nil {
		originalConfig["caBundle"] = *caBundle
	}

	return b.ImageVector.InjectImages(originalConfig, b.ShootVersion(), b.ShootVersion(), common.HyperkubeImageName, common.PauseContainerImageName)
}

func (b *HybridBotanist) computeOperatingSystemConfigsForWorker(machineTypes []gardenv1beta1.MachineType, machineImageName gardenv1beta1.MachineImageName, downloaderConfig, originalConfig map[string]interface{}, worker gardenv1beta1.Worker) (*shoot.CloudConfig, error) {
	var (
		evictionHardMemoryAvailable, evictionSoftMemoryAvailable = getEvictionMemoryAvailable(machineTypes, worker.MachineType)
		secretName                                               = b.Shoot.ComputeCloudConfigSecretName(worker.Name)
	)

	downloaderConfig["secretName"] = secretName
	originalConfig["osc"] = map[string]interface{}{
		"type":                 machineImageName,
		"purpose":              extensionsv1alpha1.OperatingSystemConfigPurposeReconcile,
		"reloadConfigFilePath": common.CloudConfigFilePath,
		"secretName":           secretName,
	}
	originalConfig["worker"] = map[string]interface{}{
		"name":                        worker.Name,
		"evictionHardMemoryAvailable": evictionHardMemoryAvailable,
		"evictionSoftMemoryAvailable": evictionSoftMemoryAvailable,
	}

	downloader, err := b.applyAndWaitForShootOperatingSystemConfig(filepath.Join(operatingSystemConfigChartPath, "downloader"), fmt.Sprintf("%s-downloader", secretName), downloaderConfig)
	if err != nil {
		return nil, err
	}
	original, err := b.applyAndWaitForShootOperatingSystemConfig(filepath.Join(operatingSystemConfigChartPath, "original"), fmt.Sprintf("%s-original", secretName), originalConfig)
	if err != nil {
		return nil, err
	}

	return &shoot.CloudConfig{Downloader: *downloader, Original: *original}, nil
}

func (b *HybridBotanist) applyAndWaitForShootOperatingSystemConfig(chartPath, name string, values map[string]interface{}) (*shoot.CloudConfigData, error) {
	var result *shoot.CloudConfigData

	if err := b.ApplyChartSeed(chartPath, b.Shoot.SeedNamespace, name, values, nil); err != nil {
		return nil, err
	}

	return result, wait.PollImmediate(time.Second, 30*time.Second, func() (bool, error) {
		var osc extensionsv1alpha1.OperatingSystemConfig
		if err := b.K8sSeedClient.Client().Get(context.TODO(), client.ObjectKey{Name: name, Namespace: b.Shoot.SeedNamespace}, &osc); err != nil {
			return false, err
		}

		if osc.Status.ObservedGeneration == osc.Generation && osc.Status.LastOperation.State == extensionsv1alpha1.LastOperationStateSucceeded && osc.Status.CloudConfig != nil {
			var secret corev1.Secret
			if err := b.K8sSeedClient.Client().Get(context.TODO(), client.ObjectKey{Name: osc.Status.CloudConfig.SecretRef.Name, Namespace: osc.Status.CloudConfig.SecretRef.Namespace}, &secret); err != nil {
				return false, err
			}
			result = &shoot.CloudConfigData{
				Content: string(secret.Data[extensionsv1alpha1.OperatingSystemConfigSecretDataKey]),
				Command: osc.Status.Command,
				Units:   osc.Status.Units,
			}
			return true, nil
		}
		return false, nil
	})
}

// generateCloudConfigExecutionChart renders the kube-addon-manager configuration for the cloud config user data.
// It will be stored as a Secret and mounted into the Pod. The configuration contains
// specially labelled Kubernetes manifests which will be created and periodically reconciled.
func (b *HybridBotanist) generateCloudConfigExecutionChart() (*chartrenderer.RenderedChart, error) {
	bootstrapTokenSecret, err := b.computeBootstrapToken()
	if err != nil {
		return nil, err
	}

	var (
		shootWorkers = b.Shoot.GetWorkers()
		workers      = make([]map[string]interface{}, 0, len(shootWorkers))
	)

	for _, worker := range shootWorkers {
		cloudConfigData := b.Shoot.CloudConfigMap[worker.Name]

		w := map[string]interface{}{
			"name":        worker.Name,
			"secretName":  b.Shoot.ComputeCloudConfigSecretName(worker.Name),
			"cloudConfig": cloudConfigData.Original.Content,
			"units":       cloudConfigData.Original.Units,
		}

		if cmd := cloudConfigData.Original.Command; cmd != nil {
			w["command"] = *cmd
		}

		workers = append(workers, w)
	}

	config := map[string]interface{}{
		"bootstrapToken": bootstraptokenutil.TokenFromIDAndSecret(string(bootstrapTokenSecret.Data[bootstraptokenapi.BootstrapTokenIDKey]), string(bootstrapTokenSecret.Data[bootstraptokenapi.BootstrapTokenSecretKey])),
		"configFilePath": common.CloudConfigFilePath,
		"workers":        workers,
	}

	config, err = b.ImageVector.InjectImages(config, b.ShootVersion(), b.ShootVersion(), common.HyperkubeImageName)
	if err != nil {
		return nil, err
	}

	return b.ChartApplierShoot.Render(filepath.Join(common.ChartPath, "shoot-cloud-config"), "shoot-cloud-config-execution", metav1.NamespaceSystem, config)
}

func (b *HybridBotanist) computeBootstrapToken() (secret *corev1.Secret, err error) {
	var (
		tokenID    = utils.ComputeSHA256Hex([]byte(time.Now().Format("2006-01-02-15")))[:6]
		secretName = bootstraptokenutil.BootstrapTokenSecretName(tokenID)
	)

	secret, err = b.K8sShootClient.GetSecret(metav1.NamespaceSystem, secretName)
	if apierrors.IsNotFound(err) {
		bootstrapTokenSecretKey, err := utils.GenerateRandomStringFromCharset(16, "0123456789abcdefghijklmnopqrstuvwxyz")
		if err != nil {
			return nil, err
		}
		data := map[string][]byte{
			bootstraptokenapi.BootstrapTokenDescriptionKey:      []byte("A bootstrap token generated by Gardener."),
			bootstraptokenapi.BootstrapTokenIDKey:               []byte(tokenID),
			bootstraptokenapi.BootstrapTokenSecretKey:           []byte(bootstrapTokenSecretKey),
			bootstraptokenapi.BootstrapTokenExpirationKey:       []byte(metav1.Now().Add(90 * time.Minute).Format(time.RFC3339)),
			bootstraptokenapi.BootstrapTokenUsageAuthentication: []byte("true"),
			bootstraptokenapi.BootstrapTokenUsageSigningKey:     []byte("true"),
		}
		return b.K8sShootClient.CreateSecret(metav1.NamespaceSystem, secretName, bootstraptokenapi.SecretTypeBootstrapToken, data, true)
	}
	return secret, err
}
