// Copyright 2018 The Gardener Authors.
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

	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DetermineCloudProviderInProfile takes a CloudProfile specification and returns the cloud provider this profile is used for.
// If it is not able to determine it, an error will be returned.
func DetermineCloudProviderInProfile(spec gardenv1beta1.CloudProfileSpec) (gardenv1beta1.CloudProvider, error) {
	if spec.AWS != nil && spec.Azure == nil && spec.GCP == nil && spec.OpenStack == nil {
		return gardenv1beta1.CloudProviderAWS, nil
	}
	if spec.Azure != nil && spec.GCP == nil && spec.OpenStack == nil && spec.AWS == nil {
		return gardenv1beta1.CloudProviderAzure, nil
	}
	if spec.GCP != nil && spec.OpenStack == nil && spec.AWS == nil && spec.Azure == nil {
		return gardenv1beta1.CloudProviderGCP, nil
	}
	if spec.OpenStack != nil && spec.AWS == nil && spec.Azure == nil && spec.GCP == nil {
		return gardenv1beta1.CloudProviderOpenStack, nil
	}

	return "", errors.New("cloud profile must only contain exactly one field of aws/azure/gcp/openstack")
}

// DetermineCloudProviderInShoot takes a Shoot cloud object and returns the cloud provider this profile is used for.
// If it is not able to determine it, an error will be returned.
func DetermineCloudProviderInShoot(cloud gardenv1beta1.Cloud) (gardenv1beta1.CloudProvider, error) {
	if cloud.AWS != nil && cloud.Azure == nil && cloud.GCP == nil && cloud.OpenStack == nil {
		return gardenv1beta1.CloudProviderAWS, nil
	}
	if cloud.Azure != nil && cloud.GCP == nil && cloud.OpenStack == nil && cloud.AWS == nil {
		return gardenv1beta1.CloudProviderAzure, nil
	}
	if cloud.GCP != nil && cloud.OpenStack == nil && cloud.AWS == nil && cloud.Azure == nil {
		return gardenv1beta1.CloudProviderGCP, nil
	}
	if cloud.OpenStack != nil && cloud.AWS == nil && cloud.Azure == nil && cloud.GCP == nil {
		return gardenv1beta1.CloudProviderOpenStack, nil
	}

	return "", errors.New("cloud object must only contain exactly one field of aws/azure/gcp/openstack")
}

// InitCondition initializes a new ShootCondition with an Unknown status.
func InitCondition(conditionType gardenv1beta1.ShootConditionType, reason, message string) *gardenv1beta1.ShootCondition {
	if reason == "" {
		reason = "ConditionInitialized"
	}
	if message == "" {
		message = "The condition has been initialized but its semantic check has not been performed yet."
	}
	return &gardenv1beta1.ShootCondition{
		Type:               conditionType,
		Status:             corev1.ConditionUnknown,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.Now(),
	}
}

// ModifyCondition updates the properties of one specific condition.
func ModifyCondition(condition *gardenv1beta1.ShootCondition, status corev1.ConditionStatus, reason, message string) *gardenv1beta1.ShootCondition {
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
