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
	"fmt"

	"github.com/gardener/gardener/pkg/apis/core"
	versionutils "github.com/gardener/gardener/pkg/utils/version"

	"github.com/Masterminds/semver"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
)

// GetConditionIndex returns the index of the condition with the given <conditionType> out of the list of <conditions>.
// In case the required type could not be found, it returns -1.
func GetConditionIndex(conditions []core.Condition, conditionType core.ConditionType) int {
	for index, condition := range conditions {
		if condition.Type == conditionType {
			return index
		}
	}
	return -1
}

// GetCondition returns the condition with the given <conditionType> out of the list of <conditions>.
// In case the required type could not be found, it returns nil.
func GetCondition(conditions []core.Condition, conditionType core.ConditionType) *core.Condition {
	if index := GetConditionIndex(conditions, conditionType); index != -1 {
		return &conditions[index]
	}
	return nil
}

// QuotaScope returns the scope of a quota scope reference.
func QuotaScope(scopeRef corev1.ObjectReference) (string, error) {
	if gvk := schema.FromAPIVersionAndKind(scopeRef.APIVersion, scopeRef.Kind); gvk.Group == "core.gardener.cloud" && gvk.Kind == "Project" {
		return "project", nil
	}
	if scopeRef.APIVersion == "v1" && scopeRef.Kind == "Secret" {
		return "secret", nil
	}
	return "", fmt.Errorf("unknown quota scope")
}

// DetermineLatestExpirableVersion determines the latest ExpirableVersion from a slice of ExpirableVersions
func DetermineLatestExpirableVersion(offeredVersions []core.ExpirableVersion) (core.ExpirableVersion, error) {
	var latestExpirableVersion core.ExpirableVersion

	for _, version := range offeredVersions {
		if len(latestExpirableVersion.Version) == 0 {
			latestExpirableVersion = version
			continue
		}
		isGreater, err := versionutils.CompareVersions(version.Version, ">", latestExpirableVersion.Version)
		if err != nil {
			return core.ExpirableVersion{}, fmt.Errorf("error while comparing versions: %s", err.Error())
		}
		if isGreater {
			latestExpirableVersion = version
		}
	}
	return latestExpirableVersion, nil
}

// DetermineLatestMachineImageVersions determines the latest versions (semVer) of the given machine images from a slice of machine images
func DetermineLatestMachineImageVersions(images []core.MachineImage) (map[string]core.ExpirableVersion, error) {
	resultMapVersions := make(map[string]core.ExpirableVersion)

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
func DetermineLatestMachineImageVersion(image core.MachineImage) (core.ExpirableVersion, error) {
	var (
		latestSemVerVersion       *semver.Version
		latestMachineImageVersion core.ExpirableVersion
	)

	for _, imageVersion := range image.Versions {
		v, err := semver.NewVersion(imageVersion.Version)
		if err != nil {
			return core.ExpirableVersion{}, fmt.Errorf("error while parsing machine image version '%s' of machine image '%s': version not valid: %s", imageVersion.Version, image.Name, err.Error())
		}
		if latestSemVerVersion == nil || v.GreaterThan(latestSemVerVersion) {
			latestSemVerVersion = v
			latestMachineImageVersion = imageVersion
		}
	}
	return latestMachineImageVersion, nil
}

// ShootWantsBasicAuthentication returns true if basic authentication is not configured or
// if it is set explicitly to 'true'.
func ShootWantsBasicAuthentication(kubeAPIServerConfig *core.KubeAPIServerConfig) bool {
	if kubeAPIServerConfig == nil {
		return true
	}
	if kubeAPIServerConfig.EnableBasicAuthentication == nil {
		return true
	}
	return *kubeAPIServerConfig.EnableBasicAuthentication
}

// TaintsHave returns true if the given key is part of the taints list.
func TaintsHave(taints []core.SeedTaint, key string) bool {
	for _, taint := range taints {
		if taint.Key == key {
			return true
		}
	}
	return false
}

// ShootUsesUnmanagedDNS returns true if the shoot's DNS section is marked as 'unmanaged'.
func ShootUsesUnmanagedDNS(shoot *core.Shoot) bool {
	if shoot.Spec.DNS == nil {
		return false
	}

	primary := FindPrimaryDNSProvider(shoot.Spec.DNS.Providers)
	if primary != nil {
		return *primary.Primary && primary.Type != nil && *primary.Type == core.DNSUnmanaged
	}

	return len(shoot.Spec.DNS.Providers) > 0 && shoot.Spec.DNS.Providers[0].Type != nil && *shoot.Spec.DNS.Providers[0].Type == core.DNSUnmanaged
}

// FindPrimaryDNSProvider finds the primary provider among the given `providers`.
// It returns the first provider if multiple candidates are found.
func FindPrimaryDNSProvider(providers []core.DNSProvider) *core.DNSProvider {
	for _, provider := range providers {
		if provider.Primary != nil && *provider.Primary {
			primaryProvider := provider
			return &primaryProvider
		}
	}
	return nil
}

// FindWorkerByName tries to find the worker with the given name. If it cannot be found it returns nil.
func FindWorkerByName(workers []core.Worker, name string) *core.Worker {
	for _, w := range workers {
		if w.Name == name {
			return &w
		}
	}
	return nil
}

// GetRemovedVersions finds versions that have been removed in the old compared to the new version slice.
// returns a map associating the version with its index in the in the old version slice.
func GetRemovedVersions(old, new []core.ExpirableVersion) map[string]int {
	return getVersionDiff(old, new)
}

// GetAddedVersions finds versions that have been added in the new compared to the new version slice.
// returns a map associating the version with its index in the in the old version slice.
func GetAddedVersions(old, new []core.ExpirableVersion) map[string]int {
	return getVersionDiff(new, old)
}

// getVersionDiff gets versions that are in v1 but not in v2.
// Returns versions mapped to their index in v1.
func getVersionDiff(v1, v2 []core.ExpirableVersion) map[string]int {
	v2Versions := sets.String{}
	for _, x := range v2 {
		v2Versions.Insert(x.Version)
	}
	diff := map[string]int{}
	for index, x := range v1 {
		if !v2Versions.Has(x.Version) {
			diff[x.Version] = index
		}
	}
	return diff
}

// FilterVersionsWithClassification filters versions for a classification
func FilterVersionsWithClassification(versions []core.ExpirableVersion, classification core.VersionClassification) []core.ExpirableVersion {
	var result []core.ExpirableVersion
	for _, version := range versions {
		if version.Classification == nil || *version.Classification != classification {
			continue
		}
		result = append(result, version)
	}
	return result
}

// FindVersionsWithSameMajorMinor filters the given versions slice for versions other the given one, having the same major and minor version as the given version
func FindVersionsWithSameMajorMinor(versions []core.ExpirableVersion, version semver.Version) ([]core.ExpirableVersion, error) {
	var result []core.ExpirableVersion
	for _, v := range versions {
		// semantic version already checked by validator
		semVer, err := semver.NewVersion(v.Version)
		if err != nil {
			return nil, err
		}
		if semVer.Equal(&version) || semVer.Minor() != version.Minor() || semVer.Major() != version.Major() {
			continue
		}
		result = append(result, v)
	}
	return result, nil
}
