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
	"sort"
	"strings"

	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DetermineCloudProviderInProfile takes a CloudProfile specification and returns the cloud provider this profile is used for.
// If it is not able to determine it, an error will be returned.
func DetermineCloudProviderInProfile(spec gardenv1beta1.CloudProfileSpec) (gardenv1beta1.CloudProvider, error) {
	var (
		cloud     gardenv1beta1.CloudProvider
		numClouds = 0
	)

	if spec.AWS != nil {
		numClouds++
		cloud = gardenv1beta1.CloudProviderAWS
	}
	if spec.Azure != nil {
		numClouds++
		cloud = gardenv1beta1.CloudProviderAzure
	}
	if spec.GCP != nil {
		numClouds++
		cloud = gardenv1beta1.CloudProviderGCP
	}
	if spec.OpenStack != nil {
		numClouds++
		cloud = gardenv1beta1.CloudProviderOpenStack
	}
	if spec.Local != nil {
		numClouds++
		cloud = gardenv1beta1.CloudProviderLocal
	}

	if numClouds != 1 {
		return "", errors.New("cloud profile must only contain exactly one field of aws/azure/gcp/openstack/local")
	}
	return cloud, nil
}

// DetermineCloudProviderInShoot takes a Shoot cloud object and returns the cloud provider this profile is used for.
// If it is not able to determine it, an error will be returned.
func DetermineCloudProviderInShoot(cloudObj gardenv1beta1.Cloud) (gardenv1beta1.CloudProvider, error) {
	var (
		cloud     gardenv1beta1.CloudProvider
		numClouds = 0
	)

	if cloudObj.AWS != nil {
		numClouds++
		cloud = gardenv1beta1.CloudProviderAWS
	}
	if cloudObj.Azure != nil {
		numClouds++
		cloud = gardenv1beta1.CloudProviderAzure
	}
	if cloudObj.GCP != nil {
		numClouds++
		cloud = gardenv1beta1.CloudProviderGCP
	}
	if cloudObj.OpenStack != nil {
		numClouds++
		cloud = gardenv1beta1.CloudProviderOpenStack
	}
	if cloudObj.Local != nil {
		numClouds++
		cloud = gardenv1beta1.CloudProviderLocal
	}

	if numClouds != 1 {
		return "", errors.New("cloud object must only contain exactly one field of aws/azure/gcp/openstack/local")
	}
	return cloud, nil
}

// InitCondition initializes a new Condition with an Unknown status.
func InitCondition(conditionType gardenv1beta1.ConditionType, reason, message string) *gardenv1beta1.Condition {
	if reason == "" {
		reason = "ConditionInitialized"
	}
	if message == "" {
		message = "The condition has been initialized but its semantic check has not been performed yet."
	}
	return &gardenv1beta1.Condition{
		Type:               conditionType,
		Status:             corev1.ConditionUnknown,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.Now(),
	}
}

// ModifyCondition updates the properties of one specific condition.
func ModifyCondition(condition *gardenv1beta1.Condition, status corev1.ConditionStatus, reason, message string) *gardenv1beta1.Condition {
	var update = false
	if status != (*condition).Status {
		update = true
		(*condition).Status = status
	}
	if reason != (*condition).Reason {
		update = true
		(*condition).Reason = reason
	}
	if message != (*condition).Message {
		update = true
		(*condition).Message = message
	}
	if update {
		(*condition).LastTransitionTime = metav1.Now()
	}
	return condition
}

// NewConditions initializes the provided conditions based on an existing list. If a condition type does not exist
// in the list yet, it will be set to default values.
func NewConditions(conditions []gardenv1beta1.Condition, conditionTypes ...gardenv1beta1.ConditionType) []*gardenv1beta1.Condition {
	newConditions := []*gardenv1beta1.Condition{}

	// We retrieve the current conditions in order to update them appropriately.
	for _, conditionType := range conditionTypes {
		if c := GetCondition(conditions, conditionType); c != nil {
			newConditions = append(newConditions, c)
			continue
		}
		newConditions = append(newConditions, InitCondition(conditionType, "", ""))
	}

	return newConditions
}

// GetCondition returns the condition with the given <conditionType> out of the list of <conditions>.
// In case the required type could not be found, it returns nil.
func GetCondition(conditions []gardenv1beta1.Condition, conditionType gardenv1beta1.ConditionType) *gardenv1beta1.Condition {
	for _, condition := range conditions {
		if condition.Type == conditionType {
			c := condition
			return &c
		}
	}
	return nil
}

// ConditionsNeedUpdate returns true if the <existingConditions> must be updated based on <newConditions>.
func ConditionsNeedUpdate(existingConditions, newConditions []gardenv1beta1.Condition) bool {
	return existingConditions == nil || !apiequality.Semantic.DeepEqual(newConditions, existingConditions)
}

// DetermineMachineImage finds the cloud specific machine image in the <cloudProfile> for the given <name> and
// region. In case it does not find a machine image with the <name>, it returns false. Otherwise, true and the
// cloud-specific machine image object will be returned.
func DetermineMachineImage(cloudProfile gardenv1beta1.CloudProfile, name gardenv1beta1.MachineImageName, region string) (bool, interface{}, error) {
	cloudProvider, err := DetermineCloudProviderInProfile(cloudProfile.Spec)
	if err != nil {
		return false, nil, err
	}

	switch cloudProvider {
	case gardenv1beta1.CloudProviderAWS:
		for _, image := range cloudProfile.Spec.AWS.Constraints.MachineImages {
			if image.Name == name {
				for _, regionMapping := range image.Regions {
					if regionMapping.Name == region {
						return true, &gardenv1beta1.AWSMachineImage{
							Name: name,
							AMI:  regionMapping.AMI,
						}, nil
					}
				}
			}
		}
	case gardenv1beta1.CloudProviderAzure:
		for _, image := range cloudProfile.Spec.Azure.Constraints.MachineImages {
			if image.Name == name {
				ptr := image
				return true, &ptr, nil
			}
		}
	case gardenv1beta1.CloudProviderGCP:
		for _, image := range cloudProfile.Spec.GCP.Constraints.MachineImages {
			if image.Name == name {
				ptr := image
				return true, &ptr, nil
			}
		}
	case gardenv1beta1.CloudProviderOpenStack:
		for _, image := range cloudProfile.Spec.OpenStack.Constraints.MachineImages {
			if image.Name == name {
				ptr := image
				return true, &ptr, nil
			}
		}
	default:
		return false, nil, fmt.Errorf("unknown cloud provider %s", cloudProvider)
	}

	return false, nil, nil
}

// DetermineLatestKubernetesVersion finds the latest Kubernetes patch version in the <cloudProfile> compared
// to the given <currentVersion>. In case it does not find a newer patch version, it returns false. Otherwise,
// true and the found version will be returned.
func DetermineLatestKubernetesVersion(cloudProfile gardenv1beta1.CloudProfile, currentVersion string) (bool, string, error) {
	cloudProvider, err := DetermineCloudProviderInProfile(cloudProfile.Spec)
	if err != nil {
		return false, "", err
	}

	var (
		versions      = []string{}
		newerVersions = []string{}
	)

	switch cloudProvider {
	case gardenv1beta1.CloudProviderAWS:
		for _, version := range cloudProfile.Spec.AWS.Constraints.Kubernetes.Versions {
			versions = append(versions, version)
		}
	case gardenv1beta1.CloudProviderAzure:
		for _, version := range cloudProfile.Spec.Azure.Constraints.Kubernetes.Versions {
			versions = append(versions, version)
		}
	case gardenv1beta1.CloudProviderGCP:
		for _, version := range cloudProfile.Spec.GCP.Constraints.Kubernetes.Versions {
			versions = append(versions, version)
		}
	case gardenv1beta1.CloudProviderOpenStack:
		for _, version := range cloudProfile.Spec.OpenStack.Constraints.Kubernetes.Versions {
			versions = append(versions, version)
		}
	default:
		return false, "", fmt.Errorf("unknown cloud provider %s", cloudProvider)
	}

	for _, version := range versions {
		ok, err := utils.CompareVersions(version, "~", currentVersion)
		if err != nil {
			return false, "", err
		}
		if version != currentVersion && ok {
			newerVersions = append(newerVersions, version)
		}
	}

	if len(newerVersions) > 0 {
		sort.Strings(newerVersions)
		return true, newerVersions[len(newerVersions)-1], nil
	}

	return false, "", nil
}

// IsUsedAsSeed determines whether the Shoot has been marked to be registered automatically as a Seed cluster.
// The first return value indicates whether it has been marked at all.
// The second return value indicates whether the Shoot should be registered as "protected" Seed.
// The third return value indicates whether the Shoot should be registered as "visible" Seed.
func IsUsedAsSeed(shoot *gardenv1beta1.Shoot) (bool, *bool, *bool) {
	if shoot.Namespace != common.GardenNamespace {
		return false, nil, nil
	}

	val, ok := shoot.Annotations[common.ShootUseAsSeed]
	if !ok {
		return false, nil, nil
	}

	var (
		trueVar  = true
		falseVar = false

		usages = map[string]bool{}

		useAsSeed bool
		protected *bool
		visible   *bool
	)

	for _, u := range strings.Split(val, ",") {
		usages[u] = true
	}

	if _, ok := usages["true"]; ok {
		useAsSeed = true
	}
	if _, ok := usages["protected"]; ok {
		protected = &trueVar
	}
	if _, ok := usages["unprotected"]; ok {
		protected = &falseVar
	}
	if _, ok := usages["visible"]; ok {
		visible = &trueVar
	}
	if _, ok := usages["invisible"]; ok {
		visible = &falseVar
	}

	return useAsSeed, protected, visible
}
