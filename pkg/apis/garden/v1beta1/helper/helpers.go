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

	"strconv"

	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// Now determines the current metav1.Time.
var Now = metav1.Now

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
	if spec.Alicloud != nil {
		numClouds++
		cloud = gardenv1beta1.CloudProviderAlicloud
	}
	if spec.Local != nil {
		numClouds++
		cloud = gardenv1beta1.CloudProviderLocal
	}

	if numClouds != 1 {
		return "", errors.New("cloud profile must only contain exactly one field of alicloud/aws/azure/gcp/openstack/local")
	}
	return cloud, nil
}

// GetShootCloudProvider retrieves the cloud provider used for the given Shoot.
func GetShootCloudProvider(shoot *gardenv1beta1.Shoot) (gardenv1beta1.CloudProvider, error) {
	return DetermineCloudProviderInShoot(shoot.Spec.Cloud)
}

// IsShootHibernated checks if the given shoot is hibernated.
func IsShootHibernated(shoot *gardenv1beta1.Shoot) bool {
	return shoot.Spec.Hibernation != nil && shoot.Spec.Hibernation.Enabled
}

// ShootWantsClusterAutoscaler checks if the given Shoot needs a cluster autoscaler.
// This is determined by checking whether one of the Shoot workers has a different
// AutoScalerMax than AutoScalerMin.
func ShootWantsClusterAutoscaler(shoot *gardenv1beta1.Shoot) (bool, error) {
	cloudProvider, err := GetShootCloudProvider(shoot)
	if err != nil {
		return false, err
	}

	workers := GetShootCloudProviderWorkers(cloudProvider, shoot)
	for _, worker := range workers {
		if worker.AutoScalerMax > worker.AutoScalerMin {
			return true, nil
		}
	}
	return false, nil
}

// GetShootCloudProviderWorkers retrieves the cloud-specific workers of the given Shoot.
func GetShootCloudProviderWorkers(cloudProvider gardenv1beta1.CloudProvider, shoot *gardenv1beta1.Shoot) []gardenv1beta1.Worker {
	var (
		cloud   = shoot.Spec.Cloud
		workers []gardenv1beta1.Worker
	)

	switch cloudProvider {
	case gardenv1beta1.CloudProviderAWS:
		for _, worker := range cloud.AWS.Workers {
			workers = append(workers, worker.Worker)
		}
	case gardenv1beta1.CloudProviderAzure:
		for _, worker := range cloud.Azure.Workers {
			workers = append(workers, worker.Worker)
		}
	case gardenv1beta1.CloudProviderGCP:
		for _, worker := range cloud.GCP.Workers {
			workers = append(workers, worker.Worker)
		}
	case gardenv1beta1.CloudProviderAlicloud:
		for _, worker := range cloud.Alicloud.Workers {
			workers = append(workers, worker.Worker)
		}
	case gardenv1beta1.CloudProviderOpenStack:
		for _, worker := range cloud.OpenStack.Workers {
			workers = append(workers, worker.Worker)
		}
	case gardenv1beta1.CloudProviderLocal:
		workers = append(workers, gardenv1beta1.Worker{
			Name:          "local",
			AutoScalerMax: 1,
			AutoScalerMin: 1,
		})
	}

	return workers
}

// GetMachineTypesFromCloudProfile retrieves list of machine types from cloud profile
func GetMachineTypesFromCloudProfile(cloudProvider gardenv1beta1.CloudProvider, profile *gardenv1beta1.CloudProfile) []gardenv1beta1.MachineType {
	var (
		machineTypes []gardenv1beta1.MachineType
	)

	switch cloudProvider {
	case gardenv1beta1.CloudProviderAWS:
		return profile.Spec.AWS.Constraints.MachineTypes
	case gardenv1beta1.CloudProviderAzure:
		return profile.Spec.Azure.Constraints.MachineTypes
	case gardenv1beta1.CloudProviderGCP:
		return profile.Spec.GCP.Constraints.MachineTypes
	case gardenv1beta1.CloudProviderOpenStack:
		for _, openStackMachineType := range profile.Spec.OpenStack.Constraints.MachineTypes {
			machineTypes = append(machineTypes, openStackMachineType.MachineType)
		}
	case gardenv1beta1.CloudProviderLocal:
		machineTypes = append(machineTypes, gardenv1beta1.MachineType{
			Name: "local",
		})
	}

	return machineTypes
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
	if cloudObj.Alicloud != nil {
		numClouds++
		cloud = gardenv1beta1.CloudProviderAlicloud
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
		Status:             gardenv1beta1.ConditionUnknown,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: Now(),
	}
}

// UpdatedCondition updates the properties of one specific condition.
func UpdatedCondition(condition *gardenv1beta1.Condition, status gardenv1beta1.ConditionStatus, reason, message string) *gardenv1beta1.Condition {
	newCondition := &gardenv1beta1.Condition{
		Type:               condition.Type,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: condition.LastTransitionTime,
		LastUpdateTime:     Now(),
	}

	if condition.Status != status {
		newCondition.LastTransitionTime = Now()
	}
	return newCondition
}

func UpdatedConditionUnknownError(condition *gardenv1beta1.Condition, err error) *gardenv1beta1.Condition {
	return UpdatedConditionUnknownErrorMessage(condition, err.Error())
}

func UpdatedConditionUnknownErrorMessage(condition *gardenv1beta1.Condition, message string) *gardenv1beta1.Condition {
	return UpdatedCondition(condition, gardenv1beta1.ConditionUnknown, gardenv1beta1.ConditionCheckError, message)
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
	case gardenv1beta1.CloudProviderAlicloud:
		for _, image := range cloudProfile.Spec.Alicloud.Constraints.MachineImages {
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
	case gardenv1beta1.CloudProviderAlicloud:
		for _, version := range cloudProfile.Spec.Alicloud.Constraints.Kubernetes.Versions {
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

type ShootedSeed struct {
	Protected *bool
	Visible   *bool
	APIServer *ShootedSeedAPIServer
}

type ShootedSeedAPIServer struct {
	Replicas   *int32
	Autoscaler *ShootedSeedAPIServerAutoscaler
}

type ShootedSeedAPIServerAutoscaler struct {
	MinReplicas *int32
	MaxReplicas int32
}

func parseInt32(s string) (int32, error) {
	i64, err := strconv.ParseInt(s, 10, 32)
	if err != nil {
		return 0, err
	}
	return int32(i64), nil
}

func parseShootedSeed(annotation string) (*ShootedSeed, error) {
	var (
		flags    = make(map[string]struct{})
		settings = make(map[string]string)

		trueVar  = true
		falseVar = false

		shootedSeed ShootedSeed
	)

	for _, fragment := range strings.Split(annotation, ",") {
		parts := strings.SplitN(fragment, "=", 2)
		if len(parts) == 1 {
			flags[fragment] = struct{}{}
			continue
		}

		settings[parts[0]] = parts[1]
	}

	if _, ok := flags["true"]; !ok {
		return nil, nil
	}

	apiServer, err := parseShootedSeedAPIServer(settings)
	if err != nil {
		return nil, err
	}

	shootedSeed.APIServer = apiServer

	if _, ok := flags["protected"]; ok {
		shootedSeed.Protected = &trueVar
	}
	if _, ok := flags["unprotected"]; ok {
		shootedSeed.Protected = &falseVar
	}
	if _, ok := flags["visible"]; ok {
		shootedSeed.Visible = &trueVar
	}
	if _, ok := flags["invisible"]; ok {
		shootedSeed.Visible = &falseVar
	}

	return &shootedSeed, nil
}

func parseShootedSeedAPIServer(settings map[string]string) (*ShootedSeedAPIServer, error) {
	apiServerAutoscaler, err := parseShootedSeedAPIServerAutoscaler(settings)
	if err != nil {
		return nil, err
	}

	replicasString, ok := settings["apiServer.replicas"]
	if !ok && apiServerAutoscaler == nil {
		return nil, nil
	}

	var apiServer ShootedSeedAPIServer

	apiServer.Autoscaler = apiServerAutoscaler

	if ok {
		replicas, err := parseInt32(replicasString)
		if err != nil {
			return nil, err
		}

		apiServer.Replicas = &replicas
	}

	return &apiServer, nil
}

func parseShootedSeedAPIServerAutoscaler(settings map[string]string) (*ShootedSeedAPIServerAutoscaler, error) {
	minReplicasString, ok1 := settings["apiServer.autoscaler.minReplicas"]
	maxReplicasString, ok2 := settings["apiServer.autoscaler.maxReplicas"]
	if !ok1 && !ok2 {
		return nil, nil
	}
	if !ok2 {
		return nil, fmt.Errorf("apiSrvMaxReplicas has to be specified for shooted seed API server autoscaler")
	}

	var apiServerAutoscaler ShootedSeedAPIServerAutoscaler

	if ok1 {
		minReplicas, err := parseInt32(minReplicasString)
		if err != nil {
			return nil, err
		}
		apiServerAutoscaler.MinReplicas = &minReplicas
	}

	maxReplicas, err := parseInt32(maxReplicasString)
	if err != nil {
		return nil, err
	}
	apiServerAutoscaler.MaxReplicas = maxReplicas

	return &apiServerAutoscaler, nil
}

func validateShootedSeed(shootedSeed *ShootedSeed, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if shootedSeed.APIServer != nil {
		allErrs = append(validateShootedSeedAPIServer(shootedSeed.APIServer, fldPath.Child("apiServer")))
	}

	return allErrs
}

func validateShootedSeedAPIServer(apiServer *ShootedSeedAPIServer, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if apiServer.Replicas != nil && *apiServer.Replicas < 1 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("replicas"), *apiServer.Replicas, "must be greater than 0"))
	}
	if apiServer.Autoscaler != nil {
		allErrs = append(allErrs, validateShootedSeedAPIServerAutoscaler(apiServer.Autoscaler, fldPath.Child("autoscaler"))...)
	}

	return allErrs
}

func validateShootedSeedAPIServerAutoscaler(autoscaler *ShootedSeedAPIServerAutoscaler, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if autoscaler.MinReplicas != nil && *autoscaler.MinReplicas < 1 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("minReplicas"), *autoscaler.MinReplicas, "must be greater than 0"))
	}
	if autoscaler.MaxReplicas < 1 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("maxReplicas"), autoscaler.MaxReplicas, "must be greater than 0"))
	}
	if autoscaler.MinReplicas != nil && autoscaler.MaxReplicas < *autoscaler.MinReplicas {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("maxReplicas"), autoscaler.MaxReplicas, "must be greater than or equal to `minReplicas`"))
	}

	return allErrs
}

func setDefaults_ShootedSeed(shootedSeed *ShootedSeed) {
	if shootedSeed.APIServer == nil {
		shootedSeed.APIServer = &ShootedSeedAPIServer{}
	}
	setDefaults_ShootedSeedAPIServer(shootedSeed.APIServer)
}

func setDefaults_ShootedSeedAPIServer(apiServer *ShootedSeedAPIServer) {
	if apiServer.Replicas == nil {
		three := int32(3)
		apiServer.Replicas = &three
	}
	if apiServer.Autoscaler == nil {
		apiServer.Autoscaler = &ShootedSeedAPIServerAutoscaler{
			MaxReplicas: 3,
		}
	}
	setDefaults_ShootedSeedAPIServerAutoscaler(apiServer.Autoscaler)
}

func minInt32(a int32, b int32) int32 {
	if a < b {
		return a
	}
	return b
}

func setDefaults_ShootedSeedAPIServerAutoscaler(autoscaler *ShootedSeedAPIServerAutoscaler) {
	if autoscaler.MinReplicas == nil {
		minReplicas := minInt32(3, autoscaler.MaxReplicas)
		autoscaler.MinReplicas = &minReplicas
	}
}

// ReadShootedSeed determines whether the Shoot has been marked to be registered automatically as a Seed cluster.
func ReadShootedSeed(shoot *gardenv1beta1.Shoot) (*ShootedSeed, error) {
	if shoot.Namespace != common.GardenNamespace || shoot.Annotations == nil {
		return nil, nil
	}

	val, ok := shoot.Annotations[common.ShootUseAsSeed]
	if !ok {
		return nil, nil
	}

	shootedSeed, err := parseShootedSeed(val)
	if err != nil {
		return nil, err
	}

	if shootedSeed == nil {
		return nil, nil
	}

	setDefaults_ShootedSeed(shootedSeed)

	if errs := validateShootedSeed(shootedSeed, nil); len(errs) > 0 {
		return nil, errs.ToAggregate()
	}

	return shootedSeed, nil
}

// Coder is an error that may produce an ErrorCode visible to the outside.
type Coder interface {
	error
	Code() gardenv1beta1.ErrorCode
}

// ExtractErrorCodes extracts all error codes from the given error by using utils.Errors
func ExtractErrorCodes(err error) []gardenv1beta1.ErrorCode {
	var codes []gardenv1beta1.ErrorCode
	for _, err := range utils.Errors(err) {
		if coder, ok := err.(Coder); ok {
			codes = append(codes, coder.Code())
		}
	}
	return codes
}

func FormatLastErrDescription(err error) string {
	errString := err.Error()
	if len(errString) > 0 {
		errString = strings.ToUpper(string(errString[0])) + errString[1:]
	}
	return errString
}
