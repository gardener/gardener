// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	"github.com/gardener/gardener/charts"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/operatingsystemconfig"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/operatingsystemconfig/downloader"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/operatingsystemconfig/executor"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/operatingsystemconfig/original/components"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils/flow"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	"github.com/gardener/gardener/pkg/utils/secrets"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DefaultOperatingSystemConfig creates the default deployer for the OperatingSystemConfig custom resource.
func (b *Botanist) DefaultOperatingSystemConfig() (operatingsystemconfig.Interface, error) {
	images, err := imagevector.FindImages(b.ImageVector, []string{charts.ImageNameHyperkube, charts.ImageNamePauseContainer, charts.ImageNamePromtail}, imagevector.RuntimeVersion(b.ShootVersion()), imagevector.TargetVersion(b.ShootVersion()))
	if err != nil {
		return nil, err
	}

	clusterDNSAddress := b.Shoot.Networks.CoreDNS.String()
	if b.Shoot.NodeLocalDNSEnabled && b.Shoot.IPVSEnabled() {
		// If IPVS is enabled then instruct the kubelet to create pods resolving DNS to the `nodelocaldns` network
		// interface link-local ip address. For more information checkout the usage documentation under
		// https://kubernetes.io/docs/tasks/administer-cluster/nodelocaldns/.
		clusterDNSAddress = common.NodeLocalIPVSAddress
	}

	return operatingsystemconfig.New(
		b.Logger,
		b.K8sSeedClient.Client(),
		&operatingsystemconfig.Values{
			Namespace:         b.Shoot.SeedNamespace,
			KubernetesVersion: b.Shoot.KubernetesVersion,
			Workers:           b.Shoot.GetInfo().Spec.Provider.Workers,
			OriginalValues: operatingsystemconfig.OriginalValues{
				ClusterDNSAddress:       clusterDNSAddress,
				ClusterDomain:           gardencorev1beta1.DefaultDomain,
				Images:                  images,
				KubeletConfigParameters: components.KubeletConfigParametersFromCoreV1beta1KubeletConfig(b.Shoot.GetInfo().Spec.Kubernetes.Kubelet),
				KubeletCLIFlags:         components.KubeletCLIFlagsFromCoreV1beta1KubeletConfig(b.Shoot.GetInfo().Spec.Kubernetes.Kubelet),
				MachineTypes:            b.Shoot.CloudProfile.Spec.MachineTypes,
			},
		},
		operatingsystemconfig.DefaultInterval,
		operatingsystemconfig.DefaultSevereThreshold,
		operatingsystemconfig.DefaultTimeout,
	), nil
}

// DeployOperatingSystemConfig deploys the OperatingSystemConfig custom resource and triggers the restore operation in
// case the Shoot is in the restore phase of the control plane migration.
func (b *Botanist) DeployOperatingSystemConfig(ctx context.Context) error {
	b.Shoot.Components.Extensions.OperatingSystemConfig.SetAPIServerURL(fmt.Sprintf("https://%s", b.Shoot.ComputeOutOfClusterAPIServerAddress(b.APIServerAddress, true)))
	b.Shoot.Components.Extensions.OperatingSystemConfig.SetCABundle(b.getOperatingSystemConfigCABundle())
	b.Shoot.Components.Extensions.OperatingSystemConfig.SetKubeletCACertificate(string(b.LoadSecret(v1beta1constants.SecretNameCAKubelet).Data[secrets.DataKeyCertificateCA]))

	publicKeys := []string{
		string(b.LoadSecret(v1beta1constants.SecretNameSSHKeyPair).Data[secrets.DataKeySSHAuthorizedKeys]),
	}

	if secret := b.LoadSecret(v1beta1constants.SecretNameOldSSHKeyPair); secret != nil {
		publicKeys = append(publicKeys, string(secret.Data[secrets.DataKeySSHAuthorizedKeys]))
	}

	b.Shoot.Components.Extensions.OperatingSystemConfig.SetSSHPublicKeys(publicKeys)

	if b.isShootNodeLoggingEnabled() {
		b.Shoot.Components.Extensions.OperatingSystemConfig.SetPromtailRBACAuthToken(b.PromtailRBACAuthToken)
		b.Shoot.Components.Extensions.OperatingSystemConfig.SetLokiIngressHostName(b.ComputeLokiHost())
	}

	if b.isRestorePhase() {
		return b.Shoot.Components.Extensions.OperatingSystemConfig.Restore(ctx, b.GetShootState())
	}

	return b.Shoot.Components.Extensions.OperatingSystemConfig.Deploy(ctx)
}

func (b *Botanist) getOperatingSystemConfigCABundle() *string {
	var caBundle string

	if cloudProfileCaBundle := b.Shoot.CloudProfile.Spec.CABundle; cloudProfileCaBundle != nil {
		caBundle = *cloudProfileCaBundle
	}

	if caCert, ok := b.LoadSecret(v1beta1constants.SecretNameCACluster).Data[secrets.DataKeyCertificateCA]; ok && len(caCert) != 0 {
		caBundle = fmt.Sprintf("%s\n%s", caBundle, caCert)
	}

	if caBundle == "" {
		return nil
	}
	return &caBundle
}

// CloudConfigExecutionManagedResourceName is a constant for the name of a ManagedResource in the seed cluster in the
// shoot namespace which contains the cloud config user data execution script.
const CloudConfigExecutionManagedResourceName = "shoot-cloud-config-execution"

// exposed for testing
var (
	// ExecutorScriptFn is a function for computing the cloud config user data executor script.
	ExecutorScriptFn = executor.Script
	// DownloaderGenerateRBACResourcesDataFn is a function for generating the RBAC resources data map for the cloud
	// config user data executor scripts downloader.
	DownloaderGenerateRBACResourcesDataFn = downloader.GenerateRBACResourcesData
)

// DeployManagedResourceForCloudConfigExecutor creates the cloud config managed resource that contains:
// 1. A secret containing the dedicated cloud config execution script for each worker group
// 2. A secret containing some shared RBAC policies for downloading the cloud config execution script
func (b *Botanist) DeployManagedResourceForCloudConfigExecutor(ctx context.Context) error {
	var (
		managedResource                  = managedresources.NewForShoot(b.K8sSeedClient.Client(), b.Shoot.SeedNamespace, CloudConfigExecutionManagedResourceName, false)
		managedResourceSecretsCount      = len(b.Shoot.GetInfo().Spec.Provider.Workers) + 1
		managedResourceSecretLabels      = map[string]string{SecretLabelKeyManagedResource: CloudConfigExecutionManagedResourceName}
		managedResourceSecretNamesWanted = sets.NewString()
		managedResourceSecretNameToData  = make(map[string]map[string][]byte, managedResourceSecretsCount)

		cloudConfigExecutorSecretNames        []string
		workerNameToOperatingSystemConfigMaps = b.Shoot.Components.Extensions.OperatingSystemConfig.WorkerNameToOperatingSystemConfigsMap()

		fns = make([]flow.TaskFn, 0, managedResourceSecretsCount)
	)

	// Generate cloud-config user-data executor scripts for all worker pools.
	for _, worker := range b.Shoot.GetInfo().Spec.Provider.Workers {
		oscData, ok := workerNameToOperatingSystemConfigMaps[worker.Name]
		if !ok {
			return fmt.Errorf("did not find osc data for worker pool %q", worker.Name)
		}

		kubernetesVersion, err := v1beta1helper.CalculateEffectiveKubernetesVersion(b.Shoot.KubernetesVersion, worker.Kubernetes)
		if err != nil {
			return err
		}

		hyperkubeImage, err := b.ImageVector.FindImage(charts.ImageNameHyperkube, imagevector.RuntimeVersion(kubernetesVersion.String()), imagevector.TargetVersion(kubernetesVersion.String()))
		if err != nil {
			return err
		}

		secretName, data, err := b.generateCloudConfigExecutorResourcesForWorker(worker, oscData.Original, hyperkubeImage)
		if err != nil {
			return err
		}

		cloudConfigExecutorSecretNames = append(cloudConfigExecutorSecretNames, secretName)
		managedResourceSecretNameToData[fmt.Sprintf("shoot-cloud-config-execution-%s", worker.Name)] = data
	}

	// Allow the cloud-config-downloader to download the generated cloud-config user-data scripts.
	downloaderRBACResourcesData, err := DownloaderGenerateRBACResourcesDataFn(cloudConfigExecutorSecretNames)
	if err != nil {
		return err
	}
	managedResourceSecretNameToData["shoot-cloud-config-rbac"] = downloaderRBACResourcesData

	// Create Secrets for the ManagedResource containing all the executor scripts as well as the RBAC resources.
	for secretName, data := range managedResourceSecretNameToData {
		var (
			managedResourceSecretName, managedResourceSecret = managedresources.NewSecret(b.K8sSeedClient.Client(), b.Shoot.SeedNamespace, secretName, nil, true)
			keyValues                                        = data
		)

		managedResource.WithSecretRef(managedResourceSecretName)
		managedResourceSecretNamesWanted.Insert(managedResourceSecretName)

		fns = append(fns, func(ctx context.Context) error {
			return managedResourceSecret.
				WithKeyValues(keyValues).
				WithLabels(managedResourceSecretLabels).
				Reconcile(ctx)
		})
	}

	if err := flow.Parallel(fns...)(ctx); err != nil {
		return err
	}

	if err := managedResource.Reconcile(ctx); err != nil {
		return err
	}

	// Cleanup no longer required Secrets for the ManagedResource (e.g., those for removed worker pools).
	secretList := &corev1.SecretList{}
	if err := b.K8sSeedClient.Client().List(ctx, secretList, client.InNamespace(b.Shoot.SeedNamespace), client.MatchingLabels(managedResourceSecretLabels)); err != nil {
		return err
	}

	return kutil.DeleteObjectsFromListConditionally(ctx, b.K8sSeedClient.Client(), secretList, func(obj runtime.Object) bool {
		acc, err := meta.Accessor(obj)
		if err != nil {
			return false
		}
		return !managedResourceSecretNamesWanted.Has(acc.GetName())
	})
}

func (b *Botanist) generateCloudConfigExecutorResourcesForWorker(
	worker gardencorev1beta1.Worker,
	oscDataOriginal operatingsystemconfig.Data,
	hyperkubeImage *imagevector.Image,
) (
	string,
	map[string][]byte,
	error,
) {
	kubernetesVersion, err := v1beta1helper.CalculateEffectiveKubernetesVersion(b.Shoot.KubernetesVersion, worker.Kubernetes)
	if err != nil {
		return "", nil, err
	}
	var (
		registry   = managedresources.NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer)
		secretName = operatingsystemconfig.Key(worker.Name, kubernetesVersion)
	)

	var kubeletDataVolume *gardencorev1beta1.DataVolume
	if worker.KubeletDataVolumeName != nil && worker.DataVolumes != nil {
		kubeletDataVolName := worker.KubeletDataVolumeName
		for _, dataVolume := range worker.DataVolumes {
			if dataVolume.Name == *kubeletDataVolName {
				kubeletDataVolume = &dataVolume
				break
			}
		}
	}

	executorScript, err := ExecutorScriptFn([]byte(oscDataOriginal.Content), hyperkubeImage, kubernetesVersion.String(), kubeletDataVolume, *oscDataOriginal.Command, oscDataOriginal.Units)
	if err != nil {
		return "", nil, err
	}

	resources, err := registry.AddAllAndSerialize(executor.Secret(secretName, metav1.NamespaceSystem, worker.Name, executorScript))
	if err != nil {
		return "", nil, err
	}

	return secretName, resources, nil
}
