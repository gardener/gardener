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
	"sync"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/chartrenderer"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/operation/shoot"
	"github.com/gardener/gardener/pkg/utils"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	"github.com/gardener/gardener/pkg/utils/retry"
	"github.com/gardener/gardener/pkg/utils/secrets"

	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var operatingSystemConfigChartPath = filepath.Join(common.ChartPath, "seed-operatingsystemconfig")

// first hard, second soft
func getEvictionMemoryAvailable(machineTypes []gardencorev1beta1.MachineType, machineType string) (string, string) {
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
func (b *Botanist) ComputeShootOperatingSystemConfig(ctx context.Context) error {
	originalConfig, err := b.generateOriginalConfig()
	if err != nil {
		return err
	}

	type oscOutput struct {
		workerName string
		oscs       *shoot.OperatingSystemConfigs
		err        error
	}

	var (
		results      = make(chan *oscOutput)
		wg           sync.WaitGroup
		errorList    = []error{}
		usedOscNames = make(map[string]string)
	)

	for _, worker := range b.Shoot.Info.Spec.Provider.Workers {
		if worker.Machine.Image == nil {
			return fmt.Errorf("worker %q doesn't have a machine image, cannot continue", worker.Name)
		}

		wg.Add(1)
		go func(worker gardencorev1beta1.Worker) {
			defer wg.Done()

			downloaderConfig := b.generateDownloaderConfig(worker.Machine.Image.Name)
			oscs, err := b.deployOperatingSystemConfigsForWorker(ctx, b.Shoot.CloudProfile.Spec.MachineTypes, worker.Machine.Image, utils.MergeMaps(downloaderConfig, nil), utils.MergeMaps(originalConfig, nil), worker)
			results <- &oscOutput{worker.Name, oscs, err}
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

		b.Shoot.OperatingSystemConfigsMap[out.workerName] = *out.oscs
		usedOscNames[out.oscs.Downloader.Name] = out.oscs.Downloader.Name
		usedOscNames[out.oscs.Original.Name] = out.oscs.Original.Name
	}

	if len(errorList) > 0 {
		return fmt.Errorf("errors occurred during operating system config generation: %+v", errorList)
	}

	// Delete all old operating system configs (i.e. those which were previously computed but now are unused).
	return b.CleanupOperatingSystemConfigs(ctx, usedOscNames)
}

func (b *Botanist) generateDownloaderConfig(machineImageName string) map[string]interface{} {
	return map[string]interface{}{
		"type":    machineImageName,
		"purpose": extensionsv1alpha1.OperatingSystemConfigPurposeProvision,
		"server":  fmt.Sprintf("https://%s", b.Shoot.ComputeOutOfClusterAPIServerAddress(b.APIServerAddress, true)),
	}
}

func (b *Botanist) generateOriginalConfig() (map[string]interface{}, error) {
	var (
		originalConfig = map[string]interface{}{
			"kubernetes": map[string]interface{}{
				"clusterDNS": b.Shoot.Networks.CoreDNS.String(),
				"domain":     gardencorev1beta1.DefaultDomain,
				"version":    b.Shoot.Info.Spec.Kubernetes.Version,
			},
		}
	)
	caBundle := ""
	if cloudProfileCaBundle := b.Shoot.CloudProfile.Spec.CABundle; cloudProfileCaBundle != nil {
		caBundle = *cloudProfileCaBundle
	}
	if caCert, ok := b.Secrets[v1beta1constants.SecretNameCACluster].Data[secrets.DataKeyCertificateCA]; ok && len(caCert) != 0 {
		caBundle = fmt.Sprintf("%s\n%s", caBundle, caCert)
	}
	originalConfig["caBundle"] = caBundle

	return b.InjectShootShootImages(originalConfig, common.PauseContainerImageName, common.HyperkubeImageName)
}

func (b *Botanist) deployOperatingSystemConfigsForWorker(ctx context.Context, machineTypes []gardencorev1beta1.MachineType, machineImage *gardencorev1beta1.ShootMachineImage, downloaderConfig, originalConfig map[string]interface{}, worker gardencorev1beta1.Worker) (*shoot.OperatingSystemConfigs, error) {
	secretName := b.Shoot.ComputeCloudConfigSecretName(worker.Name)

	downloaderConfig["secretName"] = secretName

	var customization = map[string]interface{}{}
	if machineImage.ProviderConfig != nil {
		err := yaml.Unmarshal(machineImage.ProviderConfig.Raw, &customization)
		if err != nil {
			return nil, err
		}
	}

	sshKey := b.Secrets[v1beta1constants.SecretNameSSHKeyPair].Data[secrets.DataKeySSHAuthorizedKeys]

	originalConfig["osc"] = map[string]interface{}{
		"type":                 machineImage.Name,
		"purpose":              extensionsv1alpha1.OperatingSystemConfigPurposeReconcile,
		"reloadConfigFilePath": common.CloudConfigFilePath,
		"secretName":           secretName,
		"customization":        customization,
		"sshKey":               string(sshKey),
	}

	if data := worker.CABundle; data != nil {
		if existingCABundle, ok := originalConfig["caBundle"]; ok {
			originalConfig["caBundle"] = fmt.Sprintf("%s\n%s", existingCABundle, *data)
		} else {
			originalConfig["caBundle"] = *data
		}
	}

	var (
		evictionHard            = map[string]string{}
		evictionSoft            = map[string]string{}
		evictionSoftGracePeriod = map[string]string{}
		evictionMinimumReclaim  = map[string]string{}
	)

	// use the spec.Kubernetes.Kubelet as default for worker
	kubeletConfig := b.Shoot.Info.Spec.Kubernetes.Kubelet
	if worker.Kubernetes != nil && worker.Kubernetes.Kubelet != nil {
		kubeletConfig = worker.Kubernetes.Kubelet
	}

	// ensure sane defaults for evictionHard.memoryAvailable and evictionSoft.memoryAvailable
	evictionHard["memoryAvailable"], evictionSoft["memoryAvailable"] = getEvictionMemoryAvailable(machineTypes, worker.Machine.Type)

	if kubeletConfig != nil {
		if kubeletConfig.EvictionHard != nil {
			eviction := kubeletConfig.EvictionHard
			if memoryAvailable := eviction.MemoryAvailable; memoryAvailable != nil {
				evictionHard["memoryAvailable"] = *memoryAvailable
			}
			if imageFSAvailable := eviction.ImageFSAvailable; imageFSAvailable != nil {
				evictionHard["imageFSAvailable"] = *imageFSAvailable
			}
			if imageFSInodesFree := eviction.ImageFSInodesFree; imageFSInodesFree != nil {
				evictionHard["imageFSInodesFree"] = *imageFSInodesFree
			}
			if nodeFSAvailable := eviction.NodeFSAvailable; nodeFSAvailable != nil {
				evictionHard["nodeFSAvailable"] = *nodeFSAvailable
			}
			if nodeFSInodesFree := eviction.NodeFSInodesFree; nodeFSInodesFree != nil {
				evictionHard["nodeFSInodesFree"] = *nodeFSInodesFree
			}
		}

		if kubeletConfig.EvictionSoft != nil {
			eviction := kubeletConfig.EvictionSoft
			if memoryAvailable := eviction.MemoryAvailable; memoryAvailable != nil {
				evictionSoft["memoryAvailable"] = *memoryAvailable
			}
			if imageFSAvailable := eviction.ImageFSAvailable; imageFSAvailable != nil {
				evictionSoft["imageFSAvailable"] = *imageFSAvailable
			}
			if imageFSInodesFree := eviction.ImageFSInodesFree; imageFSInodesFree != nil {
				evictionSoft["imageFSInodesFree"] = *imageFSInodesFree
			}
			if nodeFSAvailable := eviction.NodeFSAvailable; nodeFSAvailable != nil {
				evictionSoft["nodeFSAvailable"] = *nodeFSAvailable
			}
			if nodeFSInodesFree := eviction.NodeFSInodesFree; nodeFSInodesFree != nil {
				evictionSoft["nodeFSInodesFree"] = *nodeFSInodesFree
			}
		}

		if kubeletConfig.EvictionSoftGracePeriod != nil {
			eviction := kubeletConfig.EvictionSoftGracePeriod
			if memoryAvailable := eviction.MemoryAvailable; memoryAvailable != nil {
				evictionSoftGracePeriod["memoryAvailable"] = memoryAvailable.Duration.String()
			}
			if imageFSAvailable := eviction.ImageFSAvailable; imageFSAvailable != nil {
				evictionSoftGracePeriod["imageFSAvailable"] = imageFSAvailable.Duration.String()
			}
			if imageFSInodesFree := eviction.ImageFSInodesFree; imageFSInodesFree != nil {
				evictionSoftGracePeriod["imageFSInodesFree"] = imageFSInodesFree.Duration.String()
			}
			if nodeFSAvailable := eviction.NodeFSAvailable; nodeFSAvailable != nil {
				evictionSoftGracePeriod["nodeFSAvailable"] = nodeFSAvailable.Duration.String()
			}
			if nodeFSInodesFree := eviction.NodeFSInodesFree; nodeFSInodesFree != nil {
				evictionSoftGracePeriod["nodeFSInodesFree"] = nodeFSInodesFree.Duration.String()
			}
		}

		if kubeletConfig.EvictionMinimumReclaim != nil {
			eviction := kubeletConfig.EvictionMinimumReclaim
			if memoryAvailable := eviction.MemoryAvailable; memoryAvailable != nil {
				evictionMinimumReclaim["memoryAvailable"] = memoryAvailable.String()
			}
			if imageFSAvailable := eviction.ImageFSAvailable; imageFSAvailable != nil {
				evictionMinimumReclaim["imageFSAvailable"] = imageFSAvailable.String()
			}
			if imageFSInodesFree := eviction.ImageFSInodesFree; imageFSInodesFree != nil {
				evictionMinimumReclaim["imageFSInodesFree"] = imageFSInodesFree.String()
			}
			if nodeFSAvailable := eviction.NodeFSAvailable; nodeFSAvailable != nil {
				evictionMinimumReclaim["nodeFSAvailable"] = nodeFSAvailable.String()
			}
			if nodeFSInodesFree := eviction.NodeFSInodesFree; nodeFSInodesFree != nil {
				evictionMinimumReclaim["nodeFSInodesFree"] = nodeFSInodesFree.String()
			}
		}
	}

	var kubelet = map[string]interface{}{
		"caCert":                  string(b.Secrets[v1beta1constants.SecretNameCAKubelet].Data[secrets.DataKeyCertificateCA]),
		"evictionHard":            evictionHard,
		"evictionSoft":            evictionSoft,
		"evictionSoftGracePeriod": evictionSoftGracePeriod,
		"evictionMinimumReclaim":  evictionMinimumReclaim,
	}

	if kubeletConfig := kubeletConfig; kubeletConfig != nil {
		if featureGates := kubeletConfig.FeatureGates; featureGates != nil {
			kubelet["featureGates"] = featureGates
		}
		if podPIDsLimit := kubeletConfig.PodPIDsLimit; podPIDsLimit != nil {
			kubelet["podPIDsLimit"] = *podPIDsLimit
		}
		if imagePullProgressDeadline := kubeletConfig.ImagePullProgressDeadline; imagePullProgressDeadline != nil {
			kubelet["imagePullProgressDeadline"] = *imagePullProgressDeadline
		}
		if cpuCFSQuota := kubeletConfig.CPUCFSQuota; cpuCFSQuota != nil {
			kubelet["cpuCFSQuota"] = *cpuCFSQuota
		}
		if cpuManagerPolicy := kubeletConfig.CPUManagerPolicy; cpuManagerPolicy != nil {
			kubelet["cpuManagerPolicy"] = *cpuManagerPolicy
		}
		if maxPods := kubeletConfig.MaxPods; maxPods != nil {
			kubelet["maxPods"] = *maxPods
		}
		if evictionPressureTransitionPeriod := kubeletConfig.EvictionPressureTransitionPeriod; evictionPressureTransitionPeriod != nil {
			kubelet["evictionPressureTransitionPeriod"] = *evictionPressureTransitionPeriod
		}
		if evictionMaxPodGracePeriod := kubeletConfig.EvictionMaxPodGracePeriod; evictionMaxPodGracePeriod != nil {
			kubelet["evictionMaxPodGracePeriod"] = *evictionMaxPodGracePeriod
		}
	}

	originalConfig["worker"] = map[string]interface{}{
		"name":              worker.Name,
		"kubelet":           kubelet,
		"kubeletDataVolume": worker.KubeletDataVolumeName,
	}

	var (
		downloaderName = fmt.Sprintf("%s-downloader", secretName)
		originalName   = fmt.Sprintf("%s-original", secretName)
	)
	downloaderData, err := b.applyAndWaitForShootOperatingSystemConfig(ctx, filepath.Join(operatingSystemConfigChartPath, "downloader"), downloaderName, downloaderConfig)
	if err != nil {
		return nil, err
	}
	originalData, err := b.applyAndWaitForShootOperatingSystemConfig(ctx, filepath.Join(operatingSystemConfigChartPath, "original"), originalName, originalConfig)
	if err != nil {
		return nil, err
	}

	return &shoot.OperatingSystemConfigs{
		Downloader: shoot.OperatingSystemConfig{
			Name: downloaderName,
			Data: *downloaderData,
		},
		Original: shoot.OperatingSystemConfig{
			Name: originalName,
			Data: *originalData,
		},
	}, nil
}

func (b *Botanist) applyAndWaitForShootOperatingSystemConfig(ctx context.Context, chartPath, name string, values map[string]interface{}) (*shoot.OperatingSystemConfigData, error) {
	if err := b.ChartApplierSeed.Apply(ctx, chartPath, b.Shoot.SeedNamespace, name, kubernetes.Values(values)); err != nil {
		return nil, err
	}

	var oscStatus extensionsv1alpha1.OperatingSystemConfigStatus
	if err := retry.UntilTimeout(context.TODO(), time.Second, 30*time.Second, func(ctx context.Context) (done bool, err error) {
		osc := &extensionsv1alpha1.OperatingSystemConfig{}
		if err := b.K8sSeedClient.Client().Get(context.TODO(), client.ObjectKey{Name: name, Namespace: b.Shoot.SeedNamespace}, osc); err != nil {
			return retry.SevereError(err)
		}

		if err := health.CheckExtensionObject(osc); err != nil {
			b.Logger.WithError(err).Error("Operating system config did not become ready yet")
			return retry.MinorError(err)
		}
		oscStatus = osc.Status
		return retry.Ok()
	}); err != nil {
		return nil, err
	}

	secret := &corev1.Secret{}
	if err := b.K8sSeedClient.Client().Get(context.TODO(), client.ObjectKey{Name: oscStatus.CloudConfig.SecretRef.Name, Namespace: oscStatus.CloudConfig.SecretRef.Namespace}, secret); err != nil {
		return nil, err
	}

	return &shoot.OperatingSystemConfigData{
		Content: string(secret.Data[extensionsv1alpha1.OperatingSystemConfigSecretDataKey]),
		Command: oscStatus.Command,
		Units:   oscStatus.Units,
	}, nil
}

// generateCloudConfigExecutionChart renders the gardener-resource-manager configuration for the cloud config user data.
// After that it creates a ManagedResource CRD that references the rendered manifests and creates it.
func (b *Botanist) generateCloudConfigExecutionChart() (*chartrenderer.RenderedChart, error) {
	bootstrapTokenSecret, err := kutil.ComputeBootstrapToken(context.TODO(), b.K8sShootClient.Client(), utils.ComputeSHA256Hex([]byte(time.Now().Format("2006-01-02")))[:6], "A bootstrap token generated by Gardener.", 48*time.Hour)
	if err != nil {
		return nil, err
	}
	workers := make([]map[string]interface{}, 0, len(b.Shoot.Info.Spec.Provider.Workers))
	for _, worker := range b.Shoot.Info.Spec.Provider.Workers {

		oscData := b.Shoot.OperatingSystemConfigsMap[worker.Name]

		w := map[string]interface{}{
			"name":        worker.Name,
			"secretName":  b.Shoot.ComputeCloudConfigSecretName(worker.Name),
			"cloudConfig": oscData.Original.Data.Content,
			"units":       oscData.Original.Data.Units,
		}

		if cmd := oscData.Original.Data.Command; cmd != nil {
			w["command"] = *cmd
		}

		if worker.KubeletDataVolumeName != nil && worker.DataVolumes != nil {
			kubeletDataVolName := worker.KubeletDataVolumeName
			for _, dataVolume := range worker.DataVolumes {
				volName := dataVolume.Name
				if *volName == *kubeletDataVolName {
					size, err := resource.ParseQuantity(dataVolume.Size)
					if err != nil {
						return nil, err
					}
					sizeInBytes, ok := size.AsInt64()
					if !ok {
						sizeInBytes, ok = size.AsDec().Unscaled()
						if !ok {
							return nil, fmt.Errorf("failed to parse volume size %s", dataVolume.Size)
						}
					}
					w["kubeletDataVolume"] = map[string]interface{}{
						"name": volName,
						"type": dataVolume.Type,
						"size": fmt.Sprintf("%d", sizeInBytes),
					}
				}
			}
		}

		workers = append(workers, w)
	}

	config := map[string]interface{}{
		"bootstrapToken":    kutil.BootstrapTokenFrom(bootstrapTokenSecret.Data),
		"configFilePath":    common.CloudConfigFilePath,
		"kubernetesVersion": b.Shoot.Info.Spec.Kubernetes.Version,
		"workers":           workers,
	}

	config, err = b.InjectShootShootImages(config, common.HyperkubeImageName)
	if err != nil {
		return nil, err
	}

	return b.ChartApplierShoot.Render(filepath.Join(common.ChartPath, "shoot-cloud-config"), "shoot-cloud-config-execution", metav1.NamespaceSystem, config)
}

// CleanupOperatingSystemConfigs deletes all unused operating system configs in the shoot seed namespace
// (i.e., those which are not part of the provided map <usedOscNames>.
func (b *Botanist) CleanupOperatingSystemConfigs(ctx context.Context, usedOscNames map[string]string) error {
	var (
		k8sSeedClient = b.K8sSeedClient.Client()
		list          = &extensionsv1alpha1.OperatingSystemConfigList{}
	)
	if err := k8sSeedClient.List(ctx, list, client.InNamespace(b.Shoot.SeedNamespace)); err != nil {
		return err
	}

	for _, osc := range list.Items {
		if _, ok := usedOscNames[osc.Name]; !ok {
			if err := k8sSeedClient.Delete(ctx, &osc); err != nil && !apierrors.IsNotFound(err) {
				return err
			}
		}
	}

	return nil
}
