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
	"strconv"
	"strings"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	versionutils "github.com/gardener/gardener/pkg/utils/version"

	"github.com/Masterminds/semver"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/pointer"
)

// Now determines the current metav1.Time.
var Now = metav1.Now

// InitCondition initializes a new Condition with an Unknown status.
func InitCondition(conditionType gardencorev1beta1.ConditionType) gardencorev1beta1.Condition {
	now := Now()
	return gardencorev1beta1.Condition{
		Type:               conditionType,
		Status:             gardencorev1beta1.ConditionUnknown,
		Reason:             "ConditionInitialized",
		Message:            "The condition has been initialized but its semantic check has not been performed yet.",
		LastTransitionTime: now,
		LastUpdateTime:     now,
	}
}

// GetCondition returns the condition with the given <conditionType> out of the list of <conditions>.
// In case the required type could not be found, it returns nil.
func GetCondition(conditions []gardencorev1beta1.Condition, conditionType gardencorev1beta1.ConditionType) *gardencorev1beta1.Condition {
	for _, condition := range conditions {
		if condition.Type == conditionType {
			c := condition
			return &c
		}
	}
	return nil
}

// GetOrInitCondition tries to retrieve the condition with the given condition type from the given conditions.
// If the condition could not be found, it returns an initialized condition of the given type.
func GetOrInitCondition(conditions []gardencorev1beta1.Condition, conditionType gardencorev1beta1.ConditionType) gardencorev1beta1.Condition {
	if condition := GetCondition(conditions, conditionType); condition != nil {
		return *condition
	}
	return InitCondition(conditionType)
}

// UpdatedCondition updates the properties of one specific condition.
func UpdatedCondition(condition gardencorev1beta1.Condition, status gardencorev1beta1.ConditionStatus, reason, message string, codes ...gardencorev1beta1.ErrorCode) gardencorev1beta1.Condition {
	var (
		newCondition = gardencorev1beta1.Condition{
			Type:               condition.Type,
			Status:             status,
			Reason:             reason,
			Message:            message,
			LastTransitionTime: condition.LastTransitionTime,
			LastUpdateTime:     condition.LastUpdateTime,
			Codes:              codes,
		}
		now = Now()
	)

	if condition.Status != status {
		newCondition.LastTransitionTime = now
	}

	if condition.Reason != reason || condition.Message != message || !apiequality.Semantic.DeepEqual(condition.Codes, codes) {
		newCondition.LastUpdateTime = now
	}

	return newCondition
}

// UpdatedConditionUnknownError updates the condition to 'Unknown' status and the message of the given error.
func UpdatedConditionUnknownError(condition gardencorev1beta1.Condition, err error, codes ...gardencorev1beta1.ErrorCode) gardencorev1beta1.Condition {
	return UpdatedConditionUnknownErrorMessage(condition, err.Error(), codes...)
}

// UpdatedConditionUnknownErrorMessage updates the condition with 'Unknown' status and the given message.
func UpdatedConditionUnknownErrorMessage(condition gardencorev1beta1.Condition, message string, codes ...gardencorev1beta1.ErrorCode) gardencorev1beta1.Condition {
	return UpdatedCondition(condition, gardencorev1beta1.ConditionUnknown, gardencorev1beta1.ConditionCheckError, message, codes...)
}

// MergeConditions merges the given <oldConditions> with the <newConditions>. Existing conditions are superseded by
// the <newConditions> (depending on the condition type).
func MergeConditions(oldConditions []gardencorev1beta1.Condition, newConditions ...gardencorev1beta1.Condition) []gardencorev1beta1.Condition {
	var (
		out         = make([]gardencorev1beta1.Condition, 0, len(oldConditions))
		typeToIndex = make(map[gardencorev1beta1.ConditionType]int, len(oldConditions))
	)

	for i, condition := range oldConditions {
		out = append(out, condition)
		typeToIndex[condition.Type] = i
	}

	for _, condition := range newConditions {
		if index, ok := typeToIndex[condition.Type]; ok {
			out[index] = condition
			continue
		}
		out = append(out, condition)
	}

	return out
}

// ConditionsNeedUpdate returns true if the <existingConditions> must be updated based on <newConditions>.
func ConditionsNeedUpdate(existingConditions, newConditions []gardencorev1beta1.Condition) bool {
	return existingConditions == nil || !apiequality.Semantic.DeepEqual(newConditions, existingConditions)
}

// IsResourceSupported returns true if a given combination of kind/type is part of a controller resources list.
func IsResourceSupported(resources []gardencorev1beta1.ControllerResource, resourceKind, resourceType string) bool {
	for _, resource := range resources {
		if resource.Kind == resourceKind && strings.EqualFold(resource.Type, resourceType) {
			return true
		}
	}

	return false
}

// IsControllerInstallationSuccessful returns true if a ControllerInstallation has been marked as "successfully"
// installed.
func IsControllerInstallationSuccessful(controllerInstallation gardencorev1beta1.ControllerInstallation) bool {
	var (
		installed bool
		healthy   bool
	)

	for _, condition := range controllerInstallation.Status.Conditions {
		if condition.Type == gardencorev1beta1.ControllerInstallationInstalled && condition.Status == gardencorev1beta1.ConditionTrue {
			installed = true
		}
		if condition.Type == gardencorev1beta1.ControllerInstallationHealthy && condition.Status == gardencorev1beta1.ConditionTrue {
			healthy = true
		}
	}

	return installed && healthy
}

// IsControllerInstallationRequired returns true if a ControllerInstallation has been marked as "required".
func IsControllerInstallationRequired(controllerInstallation gardencorev1beta1.ControllerInstallation) bool {
	for _, condition := range controllerInstallation.Status.Conditions {
		if condition.Type == gardencorev1beta1.ControllerInstallationRequired && condition.Status == gardencorev1beta1.ConditionTrue {
			return true
		}
	}
	return false
}

// ComputeOperationType checks the <lastOperation> and determines whether it is Create, Delete, Reconcile, Migrate or Restore operation
func ComputeOperationType(meta metav1.ObjectMeta, lastOperation *gardencorev1beta1.LastOperation) gardencorev1beta1.LastOperationType {
	switch {
	case meta.Annotations[v1beta1constants.GardenerOperation] == v1beta1constants.GardenerOperationMigrate:
		return gardencorev1beta1.LastOperationTypeMigrate
	case meta.DeletionTimestamp != nil:
		return gardencorev1beta1.LastOperationTypeDelete
	case lastOperation == nil:
		return gardencorev1beta1.LastOperationTypeCreate
	case lastOperation.Type == gardencorev1beta1.LastOperationTypeCreate && lastOperation.State != gardencorev1beta1.LastOperationStateSucceeded:
		return gardencorev1beta1.LastOperationTypeCreate
	case lastOperation.Type == gardencorev1beta1.LastOperationTypeMigrate && lastOperation.State != gardencorev1beta1.LastOperationStateSucceeded:
		return gardencorev1beta1.LastOperationTypeMigrate
	case lastOperation.Type == gardencorev1beta1.LastOperationTypeRestore && lastOperation.State != gardencorev1beta1.LastOperationStateSucceeded:
		return gardencorev1beta1.LastOperationTypeRestore
	}
	return gardencorev1beta1.LastOperationTypeReconcile
}

// HasOperationAnnotation returns true if the operation annotation is present and its value is "reconcile", "restore, or "migrate".
func HasOperationAnnotation(meta metav1.ObjectMeta) bool {
	return meta.Annotations[v1beta1constants.GardenerOperation] == v1beta1constants.GardenerOperationReconcile ||
		meta.Annotations[v1beta1constants.GardenerOperation] == v1beta1constants.GardenerOperationRestore ||
		meta.Annotations[v1beta1constants.GardenerOperation] == v1beta1constants.GardenerOperationMigrate
}

// TaintsHave returns true if the given key is part of the taints list.
func TaintsHave(taints []gardencorev1beta1.SeedTaint, key string) bool {
	for _, taint := range taints {
		if taint.Key == key {
			return true
		}
	}
	return false
}

// TaintsAreTolerated returns true when all the given taints are tolerated by the given tolerations.
func TaintsAreTolerated(taints []gardencorev1beta1.SeedTaint, tolerations []gardencorev1beta1.Toleration) bool {
	if len(taints) == 0 {
		return true
	}
	if len(taints) > len(tolerations) {
		return false
	}

	tolerationKeyValues := make(map[string]string, len(tolerations))
	for _, toleration := range tolerations {
		v := ""
		if toleration.Value != nil {
			v = *toleration.Value
		}
		tolerationKeyValues[toleration.Key] = v
	}

	for _, taint := range taints {
		tolerationValue, ok := tolerationKeyValues[taint.Key]
		if !ok {
			return false
		}
		if taint.Value != nil && *taint.Value != tolerationValue {
			return false
		}
	}

	return true
}

type ShootedSeed struct {
	DisableDNS                      *bool
	DisableCapacityReservation      *bool
	Protected                       *bool
	Visible                         *bool
	LoadBalancerServicesAnnotations map[string]string
	MinimumVolumeSize               *string
	APIServer                       *ShootedSeedAPIServer
	BlockCIDRs                      []string
	ShootDefaults                   *gardencorev1beta1.ShootNetworks
	Backup                          *gardencorev1beta1.SeedBackup
	SeedProviderConfig              *runtime.RawExtension
	IngressController               *gardencorev1beta1.IngressController
	NoGardenlet                     bool
	UseServiceAccountBootstrapping  bool
	WithSecretRef                   bool
	FeatureGates                    map[string]bool
	Resources                       *ShootedSeedResources
}

type ShootedSeedAPIServer struct {
	Replicas   *int32
	Autoscaler *ShootedSeedAPIServerAutoscaler
}

type ShootedSeedAPIServerAutoscaler struct {
	MinReplicas *int32
	MaxReplicas int32
}

type ShootedSeedResources struct {
	Capacity corev1.ResourceList
	Reserved corev1.ResourceList
}

func parseInt32(s string) (int32, error) {
	i64, err := strconv.ParseInt(s, 10, 32)
	if err != nil {
		return 0, err
	}
	return int32(i64), nil
}

func parseShootedSeed(annotation string) (*ShootedSeed, error) {
	flags, settings := getFlagsAndSettings(annotation)

	if _, ok := flags["true"]; !ok {
		return nil, nil
	}

	shootedSeed := ShootedSeed{
		LoadBalancerServicesAnnotations: parseShootedSeedLoadBalancerServicesAnnotations(settings),
		FeatureGates:                    parseShootedSeedFeatureGates(settings),
	}

	apiServer, err := parseShootedSeedAPIServer(settings)
	if err != nil {
		return nil, err
	}
	shootedSeed.APIServer = apiServer

	blockCIDRs, err := parseShootedSeedBlockCIDRs(settings)
	if err != nil {
		return nil, err
	}
	shootedSeed.BlockCIDRs = blockCIDRs

	shootDefaults, err := parseShootedSeedShootDefaults(settings)
	if err != nil {
		return nil, err
	}
	shootedSeed.ShootDefaults = shootDefaults

	backup, err := parseShootedSeedBackup(settings)
	if err != nil {
		return nil, err
	}
	shootedSeed.Backup = backup

	seedProviderConfig, err := parseProviderConfig("providerConfig.", settings)
	if err != nil {
		return nil, err
	}
	shootedSeed.SeedProviderConfig = seedProviderConfig

	resources, err := parseShootedSeedResources(settings)
	if err != nil {
		return nil, err
	}
	shootedSeed.Resources = resources

	ingressController, err := parseIngressController(settings)
	if err != nil {
		return nil, err
	}
	shootedSeed.IngressController = ingressController

	if size, ok := settings["minimumVolumeSize"]; ok {
		shootedSeed.MinimumVolumeSize = &size
	}
	if _, ok := flags["disable-dns"]; ok {
		shootedSeed.DisableDNS = pointer.Bool(true)
	}
	if _, ok := flags["disable-capacity-reservation"]; ok {
		shootedSeed.DisableCapacityReservation = pointer.Bool(true)
	}
	if _, ok := flags["no-gardenlet"]; ok {
		shootedSeed.NoGardenlet = true
	}
	if _, ok := flags["use-serviceaccount-bootstrapping"]; ok {
		shootedSeed.UseServiceAccountBootstrapping = true
	}
	if _, ok := flags["with-secret-ref"]; ok {
		shootedSeed.WithSecretRef = true
	}
	if _, ok := flags["protected"]; ok {
		shootedSeed.Protected = pointer.Bool(true)
	}
	if _, ok := flags["unprotected"]; ok {
		shootedSeed.Protected = pointer.Bool(false)
	}
	if _, ok := flags["visible"]; ok {
		shootedSeed.Visible = pointer.Bool(true)
	}
	if _, ok := flags["invisible"]; ok {
		shootedSeed.Visible = pointer.Bool(false)
	}

	return &shootedSeed, nil
}

func getFlagsAndSettings(annotation string) (map[string]struct{}, map[string]string) {
	var (
		flags    = make(map[string]struct{})
		settings = make(map[string]string)
	)

	for _, fragment := range strings.Split(annotation, ",") {
		parts := strings.SplitN(fragment, "=", 2)
		if len(parts) == 1 {
			flags[fragment] = struct{}{}
			continue
		}
		settings[parts[0]] = parts[1]
	}

	return flags, settings
}

func parseShootedSeedBlockCIDRs(settings map[string]string) ([]string, error) {
	cidrs, ok := settings["blockCIDRs"]
	if !ok {
		return nil, nil
	}

	return strings.Split(cidrs, ";"), nil
}

func parseShootedSeedShootDefaults(settings map[string]string) (*gardencorev1beta1.ShootNetworks, error) {
	var (
		podCIDR, ok1     = settings["shootDefaults.pods"]
		serviceCIDR, ok2 = settings["shootDefaults.services"]
	)

	if !ok1 && !ok2 {
		return nil, nil
	}

	shootNetworks := &gardencorev1beta1.ShootNetworks{}

	if ok1 {
		shootNetworks.Pods = &podCIDR
	}

	if ok2 {
		shootNetworks.Services = &serviceCIDR
	}

	return shootNetworks, nil
}

func parseIngressController(settings map[string]string) (*gardencorev1beta1.IngressController, error) {
	ingressController := &gardencorev1beta1.IngressController{}

	kind, ok := settings["ingress.controller.kind"]
	if !ok {
		return nil, nil
	}
	ingressController.Kind = kind

	parsedProviderConfig, err := parseProviderConfig("ingress.controller.providerConfig.", settings)
	if err != nil {
		return nil, fmt.Errorf("parsing Ingress providerConfig failed: %s", err.Error())
	}
	ingressController.ProviderConfig = parsedProviderConfig

	return ingressController, nil
}

func parseShootedSeedBackup(settings map[string]string) (*gardencorev1beta1.SeedBackup, error) {
	var (
		provider, ok1           = settings["backup.provider"]
		region, ok2             = settings["backup.region"]
		secretRefName, ok3      = settings["backup.secretRef.name"]
		secretRefNamespace, ok4 = settings["backup.secretRef.namespace"]
	)

	if ok1 && provider == "none" {
		return nil, nil
	}

	backup := &gardencorev1beta1.SeedBackup{}

	if ok1 {
		backup.Provider = provider
	}
	if ok2 {
		backup.Region = &region
	}
	if ok3 {
		backup.SecretRef.Name = secretRefName
	}
	if ok4 {
		backup.SecretRef.Namespace = secretRefNamespace
	}

	return backup, nil
}

func parseShootedSeedFeatureGates(settings map[string]string) map[string]bool {
	featureGates := make(map[string]bool)

	for k, v := range settings {
		if strings.HasPrefix(k, "featureGates.") {
			val, _ := strconv.ParseBool(v)
			featureGates[strings.Split(k, ".")[1]] = val
		}
	}

	if len(featureGates) == 0 {
		return nil
	}

	return featureGates
}

func parseProviderConfig(prefix string, settings map[string]string) (*runtime.RawExtension, error) {
	// reconstruct providerConfig structure
	providerConfig := map[string]interface{}{}

	var err error
	for k, v := range settings {
		if strings.HasPrefix(k, prefix) {
			var value interface{}
			if strings.HasPrefix(v, `"`) && strings.HasSuffix(v, `"`) {
				value, err = strconv.Unquote(v)
				if err != nil {
					return nil, err
				}
			} else if b, err := strconv.ParseBool(v); err == nil {
				value = b
			} else if f, err := strconv.ParseFloat(v, 64); err == nil {
				value = f
			} else {
				value = v
			}

			path := strings.TrimPrefix(k, prefix)
			if err := unstructured.SetNestedField(providerConfig, value, strings.Split(path, ".")...); err != nil {
				return nil, err
			}
		}
	}

	if len(providerConfig) == 0 {
		return nil, nil
	}

	jsonStr, err := json.Marshal(providerConfig)
	if err != nil {
		return nil, err
	}

	return &runtime.RawExtension{
		Raw: jsonStr,
	}, nil
}

func parseShootedSeedLoadBalancerServicesAnnotations(settings map[string]string) map[string]string {
	const optionPrefix = "loadBalancerServices.annotations."

	annotations := make(map[string]string)
	for k, v := range settings {
		if strings.HasPrefix(k, optionPrefix) {
			annotationKey := strings.TrimPrefix(k, optionPrefix)
			annotations[annotationKey] = v
		}
	}

	if len(annotations) == 0 {
		return nil
	}

	return annotations
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

func parseShootedSeedResources(settings map[string]string) (*ShootedSeedResources, error) {
	var capacity, reserved corev1.ResourceList

	for k, v := range settings {
		var resourceName corev1.ResourceName
		var quantity resource.Quantity
		var err error
		if strings.HasPrefix(k, "resources.capacity.") || strings.HasPrefix(k, "resources.reserved.") {
			resourceName = corev1.ResourceName(strings.Split(k, ".")[2])
			quantity, err = resource.ParseQuantity(v)
			if err != nil {
				return nil, err
			}
			if strings.HasPrefix(k, "resources.capacity.") {
				if capacity == nil {
					capacity = make(corev1.ResourceList)
				}
				capacity[resourceName] = quantity
			} else {
				if reserved == nil {
					reserved = make(corev1.ResourceList)
				}
				reserved[resourceName] = quantity
			}
		}
	}

	if len(capacity) == 0 && len(reserved) == 0 {
		return nil, nil
	}
	return &ShootedSeedResources{
		Capacity: capacity,
		Reserved: reserved,
	}, nil
}

func validateShootedSeed(shootedSeed *ShootedSeed, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if shootedSeed.APIServer != nil {
		allErrs = append(allErrs, validateShootedSeedAPIServer(shootedSeed.APIServer, fldPath.Child("apiServer"))...)
	}
	if shootedSeed.Resources != nil {
		allErrs = append(allErrs, validateShootedSeedResources(shootedSeed.Resources, fldPath.Child("resources"))...)
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

func validateShootedSeedResources(resources *ShootedSeedResources, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	for resourceName, quantity := range resources.Capacity {
		if reservedQuantity, ok := resources.Reserved[resourceName]; ok && reservedQuantity.Value() > quantity.Value() {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("reserved", string(resourceName)), resources.Reserved[resourceName], "must be lower or equal to capacity"))
		}
	}
	for resourceName := range resources.Reserved {
		if _, ok := resources.Capacity[resourceName]; !ok {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("reserved", string(resourceName)), resources.Reserved[resourceName], "reserved without capacity"))
		}
	}

	return allErrs
}

func setDefaults_ShootedSeed(shootedSeed *ShootedSeed) {
	if shootedSeed.APIServer == nil {
		shootedSeed.APIServer = &ShootedSeedAPIServer{}
	}
	setDefaults_ShootedSeedAPIServer(shootedSeed.APIServer)
	if shootedSeed.Resources == nil {
		shootedSeed.Resources = &ShootedSeedResources{}
	}
	setDefaults_ShootedSeedResources(shootedSeed.Resources)
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

func setDefaults_ShootedSeedResources(resources *ShootedSeedResources) {
	if _, ok := resources.Capacity[gardencorev1beta1.ResourceShoots]; !ok {
		if resources.Capacity == nil {
			resources.Capacity = make(corev1.ResourceList)
		}
		resources.Capacity[gardencorev1beta1.ResourceShoots] = resource.MustParse("250")
	}
}

// ReadShootedSeed determines whether the Shoot has been marked to be registered automatically as a Seed cluster.
func ReadShootedSeed(shoot *gardencorev1beta1.Shoot) (*ShootedSeed, error) {
	if shoot.Namespace != v1beta1constants.GardenNamespace || shoot.Annotations == nil {
		return nil, nil
	}

	val, ok := shoot.Annotations[v1beta1constants.AnnotationShootUseAsSeed]
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

// ReadManagedSeedAPIServer reads the managed seed API server settings from the corresponding annotation.
func ReadManagedSeedAPIServer(shoot *gardencorev1beta1.Shoot) (*ShootedSeedAPIServer, error) {
	if shoot.Namespace != v1beta1constants.GardenNamespace || shoot.Annotations == nil {
		return nil, nil
	}

	val, ok := shoot.Annotations[v1beta1constants.AnnotationManagedSeedAPIServer]
	if !ok {
		return nil, nil
	}

	_, settings := getFlagsAndSettings(val)
	apiServer, err := parseShootedSeedAPIServer(settings)
	if err != nil {
		return nil, err
	}
	if apiServer == nil {
		return nil, nil
	}

	setDefaults_ShootedSeedAPIServer(apiServer)

	if errs := validateShootedSeedAPIServer(apiServer, nil); len(errs) > 0 {
		return nil, errs.ToAggregate()
	}

	return apiServer, nil
}

// HibernationIsEnabled checks if the given shoot's desired state is hibernated.
func HibernationIsEnabled(shoot *gardencorev1beta1.Shoot) bool {
	return shoot.Spec.Hibernation != nil && shoot.Spec.Hibernation.Enabled != nil && *shoot.Spec.Hibernation.Enabled
}

// ShootWantsClusterAutoscaler checks if the given Shoot needs a cluster autoscaler.
// This is determined by checking whether one of the Shoot workers has a different
// Maximum than Minimum.
func ShootWantsClusterAutoscaler(shoot *gardencorev1beta1.Shoot) (bool, error) {
	for _, worker := range shoot.Spec.Provider.Workers {
		if worker.Maximum > worker.Minimum {
			return true, nil
		}
	}
	return false, nil
}

// ShootWantsVerticalPodAutoscaler checks if the given Shoot needs a VPA.
func ShootWantsVerticalPodAutoscaler(shoot *gardencorev1beta1.Shoot) bool {
	return shoot.Spec.Kubernetes.VerticalPodAutoscaler != nil && shoot.Spec.Kubernetes.VerticalPodAutoscaler.Enabled
}

// ShootIgnoresAlerts checks if the alerts for the annotated shoot cluster should be ignored.
func ShootIgnoresAlerts(shoot *gardencorev1beta1.Shoot) bool {
	ignore := false
	if value, ok := shoot.Annotations[v1beta1constants.AnnotationShootIgnoreAlerts]; ok {
		ignore, _ = strconv.ParseBool(value)
	}
	return ignore
}

// ShootWantsAlertManager checks if the given shoot specification requires an alert manager.
func ShootWantsAlertManager(shoot *gardencorev1beta1.Shoot) bool {
	return !ShootIgnoresAlerts(shoot) && shoot.Spec.Monitoring != nil && shoot.Spec.Monitoring.Alerting != nil && len(shoot.Spec.Monitoring.Alerting.EmailReceivers) > 0
}

// ShootWantsBasicAuthentication returns true if basic authentication is not configured or
// if it is set explicitly to 'true'.
func ShootWantsBasicAuthentication(shoot *gardencorev1beta1.Shoot) bool {
	kubeAPIServerConfig := shoot.Spec.Kubernetes.KubeAPIServer
	if kubeAPIServerConfig == nil {
		return true
	}
	if kubeAPIServerConfig.EnableBasicAuthentication == nil {
		return true
	}
	return *kubeAPIServerConfig.EnableBasicAuthentication
}

// ShootUsesUnmanagedDNS returns true if the shoot's DNS section is marked as 'unmanaged'.
func ShootUsesUnmanagedDNS(shoot *gardencorev1beta1.Shoot) bool {
	return shoot.Spec.DNS != nil && len(shoot.Spec.DNS.Providers) > 0 && shoot.Spec.DNS.Providers[0].Type != nil && *shoot.Spec.DNS.Providers[0].Type == "unmanaged"
}

// SeedSettingVerticalPodAutoscalerEnabled returns true if the 'verticalPodAutoscaler' setting is enabled.
func SeedSettingVerticalPodAutoscalerEnabled(settings *gardencorev1beta1.SeedSettings) bool {
	return settings == nil || settings.VerticalPodAutoscaler == nil || settings.VerticalPodAutoscaler.Enabled
}

// DetermineMachineImageForName finds the cloud specific machine images in the <cloudProfile> for the given <name> and
// region. In case it does not find the machine image with the <name>, it returns false. Otherwise, true and the
// cloud-specific machine image will be returned.
func DetermineMachineImageForName(cloudProfile *gardencorev1beta1.CloudProfile, name string) (bool, gardencorev1beta1.MachineImage, error) {
	for _, image := range cloudProfile.Spec.MachineImages {
		if strings.EqualFold(image.Name, name) {
			return true, image, nil
		}
	}
	return false, gardencorev1beta1.MachineImage{}, nil
}

// ShootMachineImageVersionExists checks if the shoot machine image (name, version) exists in the machine image constraint and returns true if yes and the index in the versions slice
func ShootMachineImageVersionExists(constraint gardencorev1beta1.MachineImage, image gardencorev1beta1.ShootMachineImage) (bool, int) {
	if constraint.Name != image.Name {
		return false, 0
	}

	for index, v := range constraint.Versions {
		if image.Version != nil && v.Version == *image.Version {
			return true, index
		}
	}

	return false, 0
}

func toExpirableVersions(versions []gardencorev1beta1.MachineImageVersion) []gardencorev1beta1.ExpirableVersion {
	expVersions := []gardencorev1beta1.ExpirableVersion{}
	for _, version := range versions {
		expVersions = append(expVersions, version.ExpirableVersion)
	}
	return expVersions
}

// GetLatestQualifyingShootMachineImage determines the latest qualifying version in a machine image and returns that as a ShootMachineImage.
// A version qualifies if its classification is not preview and the version is not expired.
// Older but non-deprecated version is preferred over newer but deprecated one.
func GetLatestQualifyingShootMachineImage(image gardencorev1beta1.MachineImage, predicates ...VersionPredicate) (bool, *gardencorev1beta1.ShootMachineImage, error) {
	predicates = append(predicates, FilterExpiredVersion())

	// Try to find non-deprecated version first
	qualifyingVersionFound, latestNonDeprecatedImageVersion, err := GetLatestQualifyingVersion(toExpirableVersions(image.Versions), append(predicates, FilterDeprecatedVersion())...)
	if err != nil {
		return false, nil, err
	}
	if qualifyingVersionFound {
		return true, &gardencorev1beta1.ShootMachineImage{Name: image.Name, Version: &latestNonDeprecatedImageVersion.Version}, nil
	}

	// It looks like there is no non-deprecated version, now look also into the deprecated versions
	qualifyingVersionFound, latestImageVersion, err := GetLatestQualifyingVersion(toExpirableVersions(image.Versions), predicates...)
	if err != nil {
		return false, nil, err
	}
	if !qualifyingVersionFound {
		return false, nil, nil
	}
	return true, &gardencorev1beta1.ShootMachineImage{Name: image.Name, Version: &latestImageVersion.Version}, nil
}

// SystemComponentsAllowed checks if the given worker allows system components to be scheduled onto it
func SystemComponentsAllowed(worker *gardencorev1beta1.Worker) bool {
	return worker.SystemComponents == nil || worker.SystemComponents.Allow
}

// UpdateMachineImages updates the machine images in place.
func UpdateMachineImages(workers []gardencorev1beta1.Worker, machineImages []*gardencorev1beta1.ShootMachineImage) {
	for _, machineImage := range machineImages {
		for idx, worker := range workers {
			if worker.Machine.Image != nil && machineImage.Name == worker.Machine.Image.Name {
				workers[idx].Machine.Image = machineImage
			}
		}
	}
}

// KubernetesVersionExistsInCloudProfile checks if the given Kubernetes version exists in the CloudProfile
func KubernetesVersionExistsInCloudProfile(cloudProfile *gardencorev1beta1.CloudProfile, currentVersion string) (bool, gardencorev1beta1.ExpirableVersion, error) {
	for _, version := range cloudProfile.Spec.Kubernetes.Versions {
		ok, err := versionutils.CompareVersions(version.Version, "=", currentVersion)
		if err != nil {
			return false, gardencorev1beta1.ExpirableVersion{}, err
		}
		if ok {
			return true, version, nil
		}
	}
	return false, gardencorev1beta1.ExpirableVersion{}, nil
}

// SetMachineImageVersionsToMachineImage sets imageVersions to the matching imageName in the machineImages.
func SetMachineImageVersionsToMachineImage(machineImages []gardencorev1beta1.MachineImage, imageName string, imageVersions []gardencorev1beta1.MachineImageVersion) ([]gardencorev1beta1.MachineImage, error) {
	for index, image := range machineImages {
		if strings.EqualFold(image.Name, imageName) {
			machineImages[index].Versions = imageVersions
			return machineImages, nil
		}
	}
	return nil, fmt.Errorf("machine image with name '%s' could not be found", imageName)
}

// GetDefaultMachineImageFromCloudProfile gets the first MachineImage from the CloudProfile
func GetDefaultMachineImageFromCloudProfile(profile gardencorev1beta1.CloudProfile) *gardencorev1beta1.MachineImage {
	if len(profile.Spec.MachineImages) == 0 {
		return nil
	}
	return &profile.Spec.MachineImages[0]
}

// WrapWithLastError is wrapper function for gardencorev1beta1.LastError
func WrapWithLastError(err error, lastError *gardencorev1beta1.LastError) error {
	if err == nil || lastError == nil {
		return err
	}
	return fmt.Errorf("last error: %w: %s", err, lastError.Description)
}

// IsAPIServerExposureManaged returns true, if the Object is managed by Gardener for API server exposure.
// This indicates to extensions that they should not mutate the object.
// Gardener marks the kube-apiserver Service and Deployment as managed by it when it uses SNI to expose them.
func IsAPIServerExposureManaged(obj metav1.Object) bool {
	if obj == nil {
		return false
	}

	if v, found := obj.GetLabels()[v1beta1constants.LabelAPIServerExposure]; found &&
		v == v1beta1constants.LabelAPIServerExposureGardenerManaged {
		return true
	}

	return false
}

// FindPrimaryDNSProvider finds the primary provider among the given `providers`.
// It returns the first provider in case no primary provider is available or the first one if multiple candidates are found.
func FindPrimaryDNSProvider(providers []gardencorev1beta1.DNSProvider) *gardencorev1beta1.DNSProvider {
	for _, provider := range providers {
		if provider.Primary != nil && *provider.Primary {
			primaryProvider := provider
			return &primaryProvider
		}
	}
	return nil
}

type VersionPredicate func(expirableVersion gardencorev1beta1.ExpirableVersion, version *semver.Version) (bool, error)

// GetKubernetesVersionForPatchUpdate finds the latest Kubernetes patch version for its minor version in the <cloudProfile> compared
// to the given <currentVersion>. Preview and expired versions do not qualify for the kubernetes patch update. In case it does not find a newer patch version, it returns false. Otherwise,
// true and the found version will be returned.
func GetKubernetesVersionForPatchUpdate(cloudProfile *gardencorev1beta1.CloudProfile, currentVersion string) (bool, string, error) {
	currentSemVerVersion, err := semver.NewVersion(currentVersion)
	if err != nil {
		return false, "", err
	}

	qualifyingVersionFound, latestVersion, err := GetLatestQualifyingVersion(cloudProfile.Spec.Kubernetes.Versions, FilterDifferentMajorMinorVersion(*currentSemVerVersion), FilterSameVersion(*currentSemVerVersion), FilterExpiredVersion())
	if err != nil {
		return false, "", err
	}
	// latest version cannot be found. Do not return an error, but allow for minor upgrade if Shoot's machine image version is expired.
	if !qualifyingVersionFound {
		return false, "", nil
	}

	return true, latestVersion.Version, nil
}

// GetKubernetesVersionForMinorUpdate finds a Kubernetes version in the <cloudProfile> that qualifies for a Kubernetes minor level update given a <currentVersion>.
// A qualifying version is a non-preview version having the minor version increased by exactly one version.
// In case the consecutive minor version has only expired versions, picks the latest expired version (will do another minor update during the next maintenance time)
// If a version can be found, returns true and the qualifying patch version of the next minor version.
// In case it does not find a version, it returns false.
func GetKubernetesVersionForMinorUpdate(cloudProfile *gardencorev1beta1.CloudProfile, currentVersion string) (bool, string, error) {
	currentSemVerVersion, err := semver.NewVersion(currentVersion)
	if err != nil {
		return false, "", err
	}

	qualifyingVersionFound, latestVersion, err := GetLatestQualifyingVersion(cloudProfile.Spec.Kubernetes.Versions, FilterNonConsecutiveMinorVersion(*currentSemVerVersion), FilterSameVersion(*currentSemVerVersion), FilterExpiredVersion())
	if err != nil {
		return false, "", err
	}
	if !qualifyingVersionFound {
		// in case there are only expired versions in the consecutive minor version, pick the latest expired version
		qualifyingVersionFound, latestVersion, err = GetLatestQualifyingVersion(cloudProfile.Spec.Kubernetes.Versions, FilterNonConsecutiveMinorVersion(*currentSemVerVersion), FilterSameVersion(*currentSemVerVersion))
		if err != nil {
			return false, "", err
		}
		if !qualifyingVersionFound {
			return false, "", nil
		}
	}

	return true, latestVersion.Version, nil
}

// GetLatestQualifyingVersion returns the latest expirable version from a set of expirable versions
// A version qualifies if its classification is not preview and the optional predicate does not filter out the version.
// If the predicate returns true, the version is not considered for the latest qualifying version.
func GetLatestQualifyingVersion(versions []gardencorev1beta1.ExpirableVersion, predicate ...VersionPredicate) (qualifyingVersionFound bool, latest *gardencorev1beta1.ExpirableVersion, err error) {
	latestSemanticVersion := &semver.Version{}
	var latestVersion *gardencorev1beta1.ExpirableVersion
OUTER:
	for _, v := range versions {
		if v.Classification != nil && *v.Classification == gardencorev1beta1.ClassificationPreview {
			continue
		}

		semver, err := semver.NewVersion(v.Version)
		if err != nil {
			return false, nil, fmt.Errorf("error while parsing version '%s': %s", v.Version, err.Error())
		}

		for _, p := range predicate {
			if p == nil {
				continue
			}

			shouldFilter, err := p(v, semver)
			if err != nil {
				return false, nil, fmt.Errorf("error while evaluation predicate: '%s'", err.Error())
			}
			if shouldFilter {
				continue OUTER
			}
		}

		if semver.GreaterThan(latestSemanticVersion) {
			latestSemanticVersion = semver
			// avoid DeepCopy
			latest := v
			latestVersion = &latest
		}
	}
	// unable to find qualified versions
	if latestVersion == nil {
		return false, nil, nil
	}
	return true, latestVersion, nil
}

// FilterDifferentMajorMinorVersion returns a VersionPredicate(closure) that evaluates whether a given version v has a different same major.minor version compared to the currentSemVerVersion
// returns true if v has a different major.minor version
func FilterDifferentMajorMinorVersion(currentSemVerVersion semver.Version) VersionPredicate {
	return func(_ gardencorev1beta1.ExpirableVersion, v *semver.Version) (bool, error) {
		isWithinRange, err := versionutils.CompareVersions(v.String(), "~", currentSemVerVersion.String())
		if err != nil {
			return true, err
		}
		return !isWithinRange, nil
	}
}

// FilterNonConsecutiveMinorVersion returns a VersionPredicate(closure) that evaluates whether a given version v has a consecutive minor version compared to the currentSemVerVersion
// returns true if v does not have a consecutive minor version
func FilterNonConsecutiveMinorVersion(currentSemVerVersion semver.Version) VersionPredicate {
	return func(_ gardencorev1beta1.ExpirableVersion, v *semver.Version) (bool, error) {
		isWithinRange, err := versionutils.CompareVersions(v.String(), "^", currentSemVerVersion.String())
		if err != nil {
			return true, err
		}

		if !isWithinRange {
			return true, nil
		}

		hasIncorrectMinor := currentSemVerVersion.Minor()+1 != v.Minor()
		return hasIncorrectMinor, nil
	}
}

// FilterSameVersion returns a VersionPredicate(closure) that evaluates whether a given version v is equal to the currentSemVerVersion
// returns true if it is equal
func FilterSameVersion(currentSemVerVersion semver.Version) VersionPredicate {
	return func(_ gardencorev1beta1.ExpirableVersion, v *semver.Version) (bool, error) {
		return v.Equal(&currentSemVerVersion), nil
	}
}

// FilterLowerVersion returns a VersionPredicate(closure) that evaluates whether a given version v is lower than the currentSemVerVersion
// returns true if it is lower
func FilterLowerVersion(currentSemVerVersion semver.Version) VersionPredicate {
	return func(_ gardencorev1beta1.ExpirableVersion, v *semver.Version) (bool, error) {
		return v.LessThan(&currentSemVerVersion), nil
	}
}

// FilterExpiredVersion returns a closure that evaluates whether a given expirable version is expired
// returns true if it is expired
func FilterExpiredVersion() func(expirableVersion gardencorev1beta1.ExpirableVersion, version *semver.Version) (bool, error) {
	return func(expirableVersion gardencorev1beta1.ExpirableVersion, _ *semver.Version) (bool, error) {
		return expirableVersion.ExpirationDate != nil && (time.Now().UTC().After(expirableVersion.ExpirationDate.UTC()) || time.Now().UTC().Equal(expirableVersion.ExpirationDate.UTC())), nil
	}
}

// FilterDeprecatedVersion returns a closure that evaluates whether a given expirable version is deprecated
// returns true if it is deprecated
func FilterDeprecatedVersion() func(expirableVersion gardencorev1beta1.ExpirableVersion, version *semver.Version) (bool, error) {
	return func(expirableVersion gardencorev1beta1.ExpirableVersion, _ *semver.Version) (bool, error) {
		return expirableVersion.Classification != nil && *expirableVersion.Classification == gardencorev1beta1.ClassificationDeprecated, nil
	}
}

// GetResourceByName returns the first NamedResourceReference with the given name in the given slice, or nil if not found.
func GetResourceByName(resources []gardencorev1beta1.NamedResourceReference, name string) *gardencorev1beta1.NamedResourceReference {
	for _, resource := range resources {
		if resource.Name == name {
			return &resource
		}
	}
	return nil
}

// UpsertLastError adds a 'last error' to the given list of existing 'last errors' if it does not exist yet. Otherwise,
// it updates it.
func UpsertLastError(lastErrors []gardencorev1beta1.LastError, lastError gardencorev1beta1.LastError) []gardencorev1beta1.LastError {
	var (
		out   []gardencorev1beta1.LastError
		found bool
	)

	for _, lastErr := range lastErrors {
		if lastErr.TaskID != nil && lastError.TaskID != nil && *lastErr.TaskID == *lastError.TaskID {
			out = append(out, lastError)
			found = true
		} else {
			out = append(out, lastErr)
		}
	}

	if !found {
		out = append(out, lastError)
	}

	return out
}

// DeleteLastErrorByTaskID removes the 'last error' with the given task ID from the given 'last error' list.
func DeleteLastErrorByTaskID(lastErrors []gardencorev1beta1.LastError, taskID string) []gardencorev1beta1.LastError {
	var out []gardencorev1beta1.LastError
	for _, lastErr := range lastErrors {
		if lastErr.TaskID == nil || taskID != *lastErr.TaskID {
			out = append(out, lastErr)
		}
	}
	return out
}

// ShootItems provides helper functions with ShootLists
type ShootItems gardencorev1beta1.ShootList

// Union returns a set of Shoots that presents either in s or shootList
func (s *ShootItems) Union(shootItems *ShootItems) []gardencorev1beta1.Shoot {
	unionedShoots := make(map[string]gardencorev1beta1.Shoot)
	for _, s := range s.Items {
		unionedShoots[objectKey(s.Namespace, s.Name)] = s
	}

	for _, s := range shootItems.Items {
		unionedShoots[objectKey(s.Namespace, s.Name)] = s
	}

	shoots := make([]gardencorev1beta1.Shoot, 0, len(unionedShoots))
	for _, v := range unionedShoots {
		shoots = append(shoots, v)
	}

	return shoots
}

func objectKey(namesapce, name string) string {
	return fmt.Sprintf("%s/%s", namesapce, name)
}

// GetPurpose returns the purpose of the shoot or 'evaluation' if it's nil.
func GetPurpose(s *gardencorev1beta1.Shoot) gardencorev1beta1.ShootPurpose {
	if v := s.Spec.Purpose; v != nil {
		return *v
	}
	return gardencorev1beta1.ShootPurposeEvaluation
}

// KubernetesDashboardEnabled returns true if the kubernetes-dashboard addon is enabled in the Shoot manifest.
func KubernetesDashboardEnabled(addons *gardencorev1beta1.Addons) bool {
	return addons != nil && addons.KubernetesDashboard != nil && addons.KubernetesDashboard.Enabled
}

// NginxIngressEnabled returns true if the nginx-ingress addon is enabled in the Shoot manifest.
func NginxIngressEnabled(addons *gardencorev1beta1.Addons) bool {
	return addons != nil && addons.NginxIngress != nil && addons.NginxIngress.Enabled
}

// BackupBucketIsErroneous returns `true` if the given BackupBucket has a last error.
// It also returns the error description if available.
func BackupBucketIsErroneous(bb *gardencorev1beta1.BackupBucket) (bool, string) {
	if bb == nil {
		return false, ""
	}
	lastErr := bb.Status.LastError
	if lastErr == nil {
		return false, ""
	}
	return true, lastErr.Description
}

// SeedBackupSecretRefEqual returns true when the secret reference of the backup configuration is the same.
func SeedBackupSecretRefEqual(oldBackup, newBackup *gardencorev1beta1.SeedBackup) bool {
	var (
		oldSecretRef corev1.SecretReference
		newSecretRef corev1.SecretReference
	)

	if oldBackup != nil {
		oldSecretRef = oldBackup.SecretRef
	}

	if newBackup != nil {
		newSecretRef = newBackup.SecretRef
	}

	return apiequality.Semantic.DeepEqual(oldSecretRef, newSecretRef)
}

// ShootAuditPolicyConfigMapRefEqual returns true if the name of the ConfigMap reference for the audit policy
// configuration is the same.
func ShootAuditPolicyConfigMapRefEqual(oldAPIServerConfig, newAPIServerConfig *gardencorev1beta1.KubeAPIServerConfig) bool {
	var (
		oldConfigMapRefName string
		newConfigMapRefName string
	)

	if oldAPIServerConfig != nil &&
		oldAPIServerConfig.AuditConfig != nil &&
		oldAPIServerConfig.AuditConfig.AuditPolicy != nil &&
		oldAPIServerConfig.AuditConfig.AuditPolicy.ConfigMapRef != nil {
		oldConfigMapRefName = oldAPIServerConfig.AuditConfig.AuditPolicy.ConfigMapRef.Name
	}

	if newAPIServerConfig != nil &&
		newAPIServerConfig.AuditConfig != nil &&
		newAPIServerConfig.AuditConfig.AuditPolicy != nil &&
		newAPIServerConfig.AuditConfig.AuditPolicy.ConfigMapRef != nil {
		newConfigMapRefName = newAPIServerConfig.AuditConfig.AuditPolicy.ConfigMapRef.Name
	}

	return oldConfigMapRefName == newConfigMapRefName
}

// ShootDNSProviderSecretNamesEqual returns true when all the secretNames in the `.spec.dns.providers[]` list are the
// same.
func ShootDNSProviderSecretNamesEqual(oldDNS, newDNS *gardencorev1beta1.DNS) bool {
	var (
		oldNames = sets.NewString()
		newNames = sets.NewString()
	)

	if oldDNS != nil {
		for _, provider := range oldDNS.Providers {
			if provider.SecretName != nil {
				oldNames.Insert(*provider.SecretName)
			}
		}
	}

	if newDNS != nil {
		for _, provider := range newDNS.Providers {
			if provider.SecretName != nil {
				newNames.Insert(*provider.SecretName)
			}
		}
	}

	return oldNames.Equal(newNames)
}

// ShootSecretResourceReferencesEqual returns true when at least one of the Secret resource references inside a Shoot
// has been changed.
func ShootSecretResourceReferencesEqual(oldResources, newResources []gardencorev1beta1.NamedResourceReference) bool {
	var (
		oldNames = sets.NewString()
		newNames = sets.NewString()
	)

	for _, resource := range oldResources {
		if resource.ResourceRef.APIVersion == "v1" && resource.ResourceRef.Kind == "Secret" {
			oldNames.Insert(resource.ResourceRef.Name)
		}
	}

	for _, resource := range newResources {
		if resource.ResourceRef.APIVersion == "v1" && resource.ResourceRef.Kind == "Secret" {
			newNames.Insert(resource.ResourceRef.Name)
		}
	}

	return oldNames.Equal(newNames)
}

// ShootWantsAnonymousAuthentication returns true if anonymous authentication is set explicitly to 'true' and false otherwise.
func ShootWantsAnonymousAuthentication(kubeAPIServerConfig *gardencorev1beta1.KubeAPIServerConfig) bool {
	if kubeAPIServerConfig == nil {
		return false
	}
	if kubeAPIServerConfig.EnableAnonymousAuthentication == nil {
		return false
	}
	return *kubeAPIServerConfig.EnableAnonymousAuthentication
}
