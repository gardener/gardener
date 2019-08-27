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

package helper

import (
	"errors"
	"fmt"

	"github.com/gardener/gardener/pkg/apis/garden"
	"github.com/gardener/gardener/pkg/utils"

	"github.com/Masterminds/semver"
	corev1 "k8s.io/api/core/v1"
)

// DetermineCloudProviderInProfile takes a CloudProfile specification and returns the cloud provider this profile is used for.
// If it is not able to determine it, an error will be returned.
func DetermineCloudProviderInProfile(spec garden.CloudProfileSpec) (garden.CloudProvider, error) {
	var (
		cloud     garden.CloudProvider
		numClouds = 0
	)

	if spec.AWS != nil {
		numClouds++
		cloud = garden.CloudProviderAWS
	}
	if spec.Azure != nil {
		numClouds++
		cloud = garden.CloudProviderAzure
	}
	if spec.GCP != nil {
		numClouds++
		cloud = garden.CloudProviderGCP
	}
	if spec.OpenStack != nil {
		numClouds++
		cloud = garden.CloudProviderOpenStack
	}
	if spec.Alicloud != nil {
		numClouds++
		cloud = garden.CloudProviderAlicloud
	}
	if spec.Packet != nil {
		numClouds++
		cloud = garden.CloudProviderPacket
	}

	if numClouds != 1 {
		return "", errors.New("cloud profile must only contain exactly one field of alicloud/aws/azure/gcp/openstack/packet")
	}
	return cloud, nil
}

// DetermineCloudProviderInShoot takes a Shoot cloud object and returns the cloud provider this profile is used for.
// If it is not able to determine it, an error will be returned.
func DetermineCloudProviderInShoot(cloudObj garden.Cloud) (garden.CloudProvider, error) {
	var (
		cloud     garden.CloudProvider
		numClouds = 0
	)

	if cloudObj.AWS != nil {
		numClouds++
		cloud = garden.CloudProviderAWS
	}
	if cloudObj.Azure != nil {
		numClouds++
		cloud = garden.CloudProviderAzure
	}
	if cloudObj.GCP != nil {
		numClouds++
		cloud = garden.CloudProviderGCP
	}
	if cloudObj.OpenStack != nil {
		numClouds++
		cloud = garden.CloudProviderOpenStack
	}
	if cloudObj.Alicloud != nil {
		numClouds++
		cloud = garden.CloudProviderAlicloud
	}
	if cloudObj.Packet != nil {
		numClouds++
		cloud = garden.CloudProviderPacket
	}

	if numClouds != 1 {
		return "", errors.New("cloud object must only contain exactly one field of aws/azure/gcp/openstack/packet")
	}
	return cloud, nil
}

// DetermineLatestMachineImageVersions determines the latest versions (semVer) of the given machine images from a slice of machine images
func DetermineLatestMachineImageVersions(images []garden.MachineImage) (map[string]garden.MachineImageVersion, error) {
	resultMapVersions := make(map[string]garden.MachineImageVersion)

	for _, image := range images {
		latestMachineImageVersion, err := DetermineLatestMachineImageVersion(image)
		if err != nil {
			return nil, err
		}
		resultMapVersions[image.Name] = latestMachineImageVersion
	}
	return resultMapVersions, nil
}

// DetermineLatestMachineImageVersion determines the latest MachineImageVersion from a MachineImage
func DetermineLatestMachineImageVersion(image garden.MachineImage) (garden.MachineImageVersion, error) {
	var (
		latestSemVerVersion       *semver.Version
		latestMachineImageVersion garden.MachineImageVersion
	)

	for _, imageVersion := range image.Versions {
		v, err := semver.NewVersion(imageVersion.Version)
		if err != nil {
			return garden.MachineImageVersion{}, fmt.Errorf("error while parsing machine image version '%s' of machine image '%s': version not valid: %s", imageVersion.Version, image.Name, err.Error())
		}
		if latestSemVerVersion == nil || v.GreaterThan(latestSemVerVersion) {
			latestSemVerVersion = v
			latestMachineImageVersion = imageVersion
		}
	}
	return latestMachineImageVersion, nil
}

// DetermineLatestKubernetesVersion determines the latest KubernetesVersion from a slice of KubernetesVersions
func DetermineLatestKubernetesVersion(offeredVersions []garden.KubernetesVersion) (garden.KubernetesVersion, error) {
	var latestKubernetesVersion garden.KubernetesVersion

	for _, version := range offeredVersions {
		if len(latestKubernetesVersion.Version) == 0 {
			latestKubernetesVersion = version
			continue
		}
		isGreater, err := utils.CompareVersions(version.Version, ">", latestKubernetesVersion.Version)
		if err != nil {
			return garden.KubernetesVersion{}, fmt.Errorf("error while comparing Kubernetes versions: %s", err.Error())
		}
		if isGreater {
			latestKubernetesVersion = version
		}
	}
	return latestKubernetesVersion, nil
}

// ShootWantsBasicAuthentication returns true if basic authentication is not configured or
// if it is set explicitly to 'true'.
func ShootWantsBasicAuthentication(kubeAPIServerConfig *garden.KubeAPIServerConfig) bool {
	if kubeAPIServerConfig == nil {
		return true
	}
	if kubeAPIServerConfig.EnableBasicAuthentication == nil {
		return true
	}
	return *kubeAPIServerConfig.EnableBasicAuthentication
}

// GetConditionIndex returns the index of the condition with the given <conditionType> out of the list of <conditions>.
// In case the required type could not be found, it returns -1.
func GetConditionIndex(conditions []garden.Condition, conditionType garden.ConditionType) int {
	for index, condition := range conditions {
		if condition.Type == conditionType {
			return index
		}
	}
	return -1
}

// GetCondition returns the condition with the given <conditionType> out of the list of <conditions>.
// In case the required type could not be found, it returns nil.
func GetCondition(conditions []garden.Condition, conditionType garden.ConditionType) *garden.Condition {
	if index := GetConditionIndex(conditions, conditionType); index != -1 {
		return &conditions[index]
	}
	return nil
}

// TaintsHave returns true if the given key is part of the taints list.
func TaintsHave(taints []garden.SeedTaint, key string) bool {
	for _, taint := range taints {
		if taint.Key == key {
			return true
		}
	}
	return false
}

// QuotaScope returns the scope of a quota scope reference.
func QuotaScope(scopeRef corev1.ObjectReference) (string, error) {
	if scopeRef.APIVersion == "core.gardener.cloud/v1alpha1" && scopeRef.Kind == "Project" {
		return "project", nil
	}
	if scopeRef.APIVersion == "v1" && scopeRef.Kind == "Secret" {
		return "secret", nil
	}
	return "", fmt.Errorf("unknown quota scope")
}
