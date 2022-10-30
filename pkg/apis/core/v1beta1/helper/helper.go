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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/clock"
	"k8s.io/utils/pointer"
)

// Clock defines the clock for the helper functions
// Deprecated: Use ...WithClock(...) functions instead.
var Clock clock.Clock = clock.RealClock{}

// InitCondition initializes a new Condition with an Unknown status.
// Deprecated: Use InitConditionWithClock(...) instead.
func InitCondition(conditionType gardencorev1beta1.ConditionType) gardencorev1beta1.Condition {
	return InitConditionWithClock(Clock, conditionType)
}

// InitConditionWithClock initializes a new Condition with an Unknown status. It allows passing a custom clock for testing.
func InitConditionWithClock(clock clock.Clock, conditionType gardencorev1beta1.ConditionType) gardencorev1beta1.Condition {
	now := metav1.Time{Time: clock.Now()}
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
// Deprecated: Use GetOrInitConditionWithClock(...) instead.
func GetOrInitCondition(conditions []gardencorev1beta1.Condition, conditionType gardencorev1beta1.ConditionType) gardencorev1beta1.Condition {
	return GetOrInitConditionWithClock(Clock, conditions, conditionType)
}

// GetOrInitConditionWithClock tries to retrieve the condition with the given condition type from the given conditions.
// If the condition could not be found, it returns an initialized condition of the given type. It allows passing a custom clock for testing.
func GetOrInitConditionWithClock(clock clock.Clock, conditions []gardencorev1beta1.Condition, conditionType gardencorev1beta1.ConditionType) gardencorev1beta1.Condition {
	if condition := GetCondition(conditions, conditionType); condition != nil {
		return *condition
	}
	return InitConditionWithClock(clock, conditionType)
}

// UpdatedCondition updates the properties of one specific condition.
// Deprecated: Use UpdatedConditionWithClock(...) instead.
func UpdatedCondition(condition gardencorev1beta1.Condition, status gardencorev1beta1.ConditionStatus, reason, message string, codes ...gardencorev1beta1.ErrorCode) gardencorev1beta1.Condition {
	return UpdatedConditionWithClock(Clock, condition, status, reason, message, codes...)
}

// UpdatedConditionWithClock updates the properties of one specific condition. It allows passing a custom clock for testing.
func UpdatedConditionWithClock(clock clock.Clock, condition gardencorev1beta1.Condition, status gardencorev1beta1.ConditionStatus, reason, message string, codes ...gardencorev1beta1.ErrorCode) gardencorev1beta1.Condition {
	builder, err := NewConditionBuilder(condition.Type)
	utilruntime.Must(err)
	newCondition, _ := builder.
		WithOldCondition(condition).
		WithClock(clock).
		WithStatus(status).
		WithReason(reason).
		WithMessage(message).
		WithCodes(codes...).
		Build()

	return newCondition
}

// UpdatedConditionUnknownError updates the condition to 'Unknown' status and the message of the given error.
// Deprecated: Use UpdatedConditionUnknownErrorWithClock(...) instead.
func UpdatedConditionUnknownError(condition gardencorev1beta1.Condition, err error, codes ...gardencorev1beta1.ErrorCode) gardencorev1beta1.Condition {
	return UpdatedConditionUnknownErrorWithClock(Clock, condition, err, codes...)
}

// UpdatedConditionUnknownErrorWithClock updates the condition to 'Unknown' status and the message of the given error. It allows passing a custom clock for testing.
func UpdatedConditionUnknownErrorWithClock(clock clock.Clock, condition gardencorev1beta1.Condition, err error, codes ...gardencorev1beta1.ErrorCode) gardencorev1beta1.Condition {
	return UpdatedConditionUnknownErrorMessageWithClock(clock, condition, err.Error(), codes...)
}

// UpdatedConditionUnknownErrorMessage updates the condition with 'Unknown' status and the given message.
// Deprecated: Use UpdatedConditionUnknownErrorMessageWithClock(...) instead.
func UpdatedConditionUnknownErrorMessage(condition gardencorev1beta1.Condition, message string, codes ...gardencorev1beta1.ErrorCode) gardencorev1beta1.Condition {
	return UpdatedConditionUnknownErrorMessageWithClock(Clock, condition, message, codes...)
}

// UpdatedConditionUnknownErrorMessageWithClock updates the condition with 'Unknown' status and the given message. It allows passing a custom clock for testing.
func UpdatedConditionUnknownErrorMessageWithClock(clock clock.Clock, condition gardencorev1beta1.Condition, message string, codes ...gardencorev1beta1.ErrorCode) gardencorev1beta1.Condition {
	return UpdatedConditionWithClock(clock, condition, gardencorev1beta1.ConditionUnknown, gardencorev1beta1.ConditionCheckError, message, codes...)
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

// RemoveConditions removes the conditions with the given types from the given conditions slice.
func RemoveConditions(conditions []gardencorev1beta1.Condition, conditionTypes ...gardencorev1beta1.ConditionType) []gardencorev1beta1.Condition {
	conditionTypesMap := make(map[gardencorev1beta1.ConditionType]struct{}, len(conditionTypes))
	for _, conditionType := range conditionTypes {
		conditionTypesMap[conditionType] = struct{}{}
	}

	var newConditions []gardencorev1beta1.Condition
	for _, condition := range conditions {
		if _, ok := conditionTypesMap[condition.Type]; !ok {
			newConditions = append(newConditions, condition)
		}
	}

	return newConditions
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
		installed      bool
		healthy        bool
		notProgressing bool
	)

	for _, condition := range controllerInstallation.Status.Conditions {
		if condition.Type == gardencorev1beta1.ControllerInstallationInstalled && condition.Status == gardencorev1beta1.ConditionTrue {
			installed = true
		}
		if condition.Type == gardencorev1beta1.ControllerInstallationHealthy && condition.Status == gardencorev1beta1.ConditionTrue {
			healthy = true
		}
		if condition.Type == gardencorev1beta1.ControllerInstallationProgressing && condition.Status == gardencorev1beta1.ConditionFalse {
			notProgressing = true
		}
	}

	return installed && healthy && notProgressing
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
	case meta.Annotations[v1beta1constants.GardenerOperation] == v1beta1constants.GardenerOperationRestore:
		return gardencorev1beta1.LastOperationTypeRestore
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

// ManagedSeedAPIServer contains the configuration of a ManagedSeed API server.
type ManagedSeedAPIServer struct {
	Replicas   *int32
	Autoscaler *ManagedSeedAPIServerAutoscaler
}

// ManagedSeedAPIServerAutoscaler contains the configuration of a ManagedSeed API server autoscaler.
type ManagedSeedAPIServerAutoscaler struct {
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

func parseManagedSeedAPIServer(settings map[string]string) (*ManagedSeedAPIServer, error) {
	apiServerAutoscaler, err := parseManagedSeedAPIServerAutoscaler(settings)
	if err != nil {
		return nil, err
	}

	replicasString, ok := settings["apiServer.replicas"]
	if !ok && apiServerAutoscaler == nil {
		return nil, nil
	}

	var apiServer ManagedSeedAPIServer

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

func parseManagedSeedAPIServerAutoscaler(settings map[string]string) (*ManagedSeedAPIServerAutoscaler, error) {
	minReplicasString, ok1 := settings["apiServer.autoscaler.minReplicas"]
	maxReplicasString, ok2 := settings["apiServer.autoscaler.maxReplicas"]
	if !ok1 && !ok2 {
		return nil, nil
	}
	if !ok2 {
		return nil, fmt.Errorf("apiSrvMaxReplicas has to be specified for ManagedSeed API server autoscaler")
	}

	var apiServerAutoscaler ManagedSeedAPIServerAutoscaler

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

func validateManagedSeedAPIServer(apiServer *ManagedSeedAPIServer, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if apiServer.Replicas != nil && *apiServer.Replicas < 1 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("replicas"), *apiServer.Replicas, "must be greater than 0"))
	}
	if apiServer.Autoscaler != nil {
		allErrs = append(allErrs, validateManagedSeedAPIServerAutoscaler(apiServer.Autoscaler, fldPath.Child("autoscaler"))...)
	}

	return allErrs
}

func validateManagedSeedAPIServerAutoscaler(autoscaler *ManagedSeedAPIServerAutoscaler, fldPath *field.Path) field.ErrorList {
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

func setDefaults_ManagedSeedAPIServer(apiServer *ManagedSeedAPIServer) {
	if apiServer.Replicas == nil {
		three := int32(3)
		apiServer.Replicas = &three
	}
	if apiServer.Autoscaler == nil {
		apiServer.Autoscaler = &ManagedSeedAPIServerAutoscaler{
			MaxReplicas: 3,
		}
	}
	setDefaults_ManagedSeedAPIServerAutoscaler(apiServer.Autoscaler)
}

func minInt32(a int32, b int32) int32 {
	if a < b {
		return a
	}
	return b
}

func setDefaults_ManagedSeedAPIServerAutoscaler(autoscaler *ManagedSeedAPIServerAutoscaler) {
	if autoscaler.MinReplicas == nil {
		minReplicas := minInt32(3, autoscaler.MaxReplicas)
		autoscaler.MinReplicas = &minReplicas
	}
}

// ReadManagedSeedAPIServer reads the managed seed API server settings from the corresponding annotation.
func ReadManagedSeedAPIServer(shoot *gardencorev1beta1.Shoot) (*ManagedSeedAPIServer, error) {
	if shoot.Namespace != v1beta1constants.GardenNamespace || shoot.Annotations == nil {
		return nil, nil
	}

	val, ok := shoot.Annotations[v1beta1constants.AnnotationManagedSeedAPIServer]
	if !ok {
		return nil, nil
	}

	_, settings := getFlagsAndSettings(val)
	apiServer, err := parseManagedSeedAPIServer(settings)
	if err != nil {
		return nil, err
	}
	if apiServer == nil {
		return nil, nil
	}

	setDefaults_ManagedSeedAPIServer(apiServer)

	if errs := validateManagedSeedAPIServer(apiServer, nil); len(errs) > 0 {
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

// ShootSchedulingProfile returns the scheduling profile of the given Shoot.
func ShootSchedulingProfile(shoot *gardencorev1beta1.Shoot) *gardencorev1beta1.SchedulingProfile {
	if shoot.Spec.Kubernetes.KubeScheduler != nil {
		return shoot.Spec.Kubernetes.KubeScheduler.Profile
	}
	return nil
}

// SeedSettingVerticalPodAutoscalerEnabled returns true if the 'verticalPodAutoscaler' setting is enabled.
func SeedSettingVerticalPodAutoscalerEnabled(settings *gardencorev1beta1.SeedSettings) bool {
	return settings == nil || settings.VerticalPodAutoscaler == nil || settings.VerticalPodAutoscaler.Enabled
}

// SeedSettingOwnerChecksEnabled returns true if the 'ownerChecks' setting is enabled.
func SeedSettingOwnerChecksEnabled(settings *gardencorev1beta1.SeedSettings) bool {
	return settings == nil || settings.OwnerChecks == nil || settings.OwnerChecks.Enabled
}

// SeedSettingDependencyWatchdogEndpointEnabled returns true if the depedency-watchdog-endpoint is enabled.
func SeedSettingDependencyWatchdogEndpointEnabled(settings *gardencorev1beta1.SeedSettings) bool {
	return settings == nil || settings.DependencyWatchdog == nil || settings.DependencyWatchdog.Endpoint == nil || settings.DependencyWatchdog.Endpoint.Enabled
}

// SeedSettingDependencyWatchdogProbeEnabled returns true if the depedency-watchdog-probe is enabled.
func SeedSettingDependencyWatchdogProbeEnabled(settings *gardencorev1beta1.SeedSettings) bool {
	return settings == nil || settings.DependencyWatchdog == nil || settings.DependencyWatchdog.Probe == nil || settings.DependencyWatchdog.Probe.Enabled
}

// SeedUsesNginxIngressController returns true if the seed's specification requires an nginx ingress controller to be deployed.
func SeedUsesNginxIngressController(seed *gardencorev1beta1.Seed) bool {
	return seed.Spec.DNS.Provider != nil && seed.Spec.Ingress != nil && seed.Spec.Ingress.Controller.Kind == v1beta1constants.IngressKindNginx
}

// DetermineMachineImageForName finds the cloud specific machine images in the <cloudProfile> for the given <name> and
// region. In case it does not find the machine image with the <name>, it returns false. Otherwise, true and the
// cloud-specific machine image will be returned.
func DetermineMachineImageForName(cloudProfile *gardencorev1beta1.CloudProfile, name string) (bool, gardencorev1beta1.MachineImage) {
	for _, image := range cloudProfile.Spec.MachineImages {
		if strings.EqualFold(image.Name, name) {
			return true, image
		}
	}
	return false, gardencorev1beta1.MachineImage{}
}

// FindMachineImageVersion finds the machine image version in the <cloudProfile> for the given <name> and <version>.
// In case no machine image version can be found with the given <name> or <version>, false is being returned.
func FindMachineImageVersion(cloudProfile *gardencorev1beta1.CloudProfile, name, version string) (bool, gardencorev1beta1.MachineImageVersion) {
	for _, image := range cloudProfile.Spec.MachineImages {
		if image.Name == name {
			for _, imageVersion := range image.Versions {
				if imageVersion.Version == version {
					return true, imageVersion
				}
			}
		}
	}

	return false, gardencorev1beta1.MachineImageVersion{}
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

// FindMachineTypeByName tries to find the machine type details with the given name. If it cannot be found it returns nil.
func FindMachineTypeByName(machines []gardencorev1beta1.MachineType, name string) *gardencorev1beta1.MachineType {
	for _, m := range machines {
		if m.Name == name {
			return &m
		}
	}
	return nil
}

// SystemComponentsAllowed checks if the given worker allows system components to be scheduled onto it
func SystemComponentsAllowed(worker *gardencorev1beta1.Worker) bool {
	return worker.SystemComponents == nil || worker.SystemComponents.Allow
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

// VersionPredicate is a function that evaluates a condition on the given versions.
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

// KubeProxyEnabled returns true if the kube-proxy is enabled in the Shoot manifest.
func KubeProxyEnabled(config *gardencorev1beta1.KubeProxyConfig) bool {
	return config != nil && config.Enabled != nil && *config.Enabled
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

// GetShootAuditPolicyConfigMapName returns the Shoot's ConfigMap reference name for the audit policy.
func GetShootAuditPolicyConfigMapName(apiServerConfig *gardencorev1beta1.KubeAPIServerConfig) string {
	if ref := GetShootAuditPolicyConfigMapRef(apiServerConfig); ref != nil {
		return ref.Name
	}
	return ""
}

// GetShootAuditPolicyConfigMapRef returns the Shoot's ConfigMap reference for the audit policy.
func GetShootAuditPolicyConfigMapRef(apiServerConfig *gardencorev1beta1.KubeAPIServerConfig) *corev1.ObjectReference {
	if apiServerConfig != nil &&
		apiServerConfig.AuditConfig != nil &&
		apiServerConfig.AuditConfig.AuditPolicy != nil {
		return apiServerConfig.AuditConfig.AuditPolicy.ConfigMapRef
	}
	return nil
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

// CalculateSeedUsage returns a map representing the number of shoots per seed from the given list of shoots.
// It takes both spec.seedName and status.seedName into account.
func CalculateSeedUsage(shootList []gardencorev1beta1.Shoot) map[string]int {
	m := map[string]int{}

	for _, shoot := range shootList {
		var (
			specSeed   = pointer.StringDeref(shoot.Spec.SeedName, "")
			statusSeed = pointer.StringDeref(shoot.Status.SeedName, "")
		)

		if specSeed != "" {
			m[specSeed]++
		}
		if statusSeed != "" && specSeed != statusSeed {
			m[statusSeed]++
		}
	}

	return m
}

// CalculateEffectiveKubernetesVersion if a shoot has kubernetes version specified by worker group, return this,
// otherwise the shoot kubernetes version
func CalculateEffectiveKubernetesVersion(controlPlaneVersion *semver.Version, workerKubernetes *gardencorev1beta1.WorkerKubernetes) (*semver.Version, error) {
	if workerKubernetes != nil && workerKubernetes.Version != nil {
		return semver.NewVersion(*workerKubernetes.Version)
	}
	return controlPlaneVersion, nil
}

// GetSecretBindingTypes returns the SecretBinding provider types.
func GetSecretBindingTypes(secretBinding *gardencorev1beta1.SecretBinding) []string {
	return strings.Split(secretBinding.Provider.Type, ",")
}

// SecretBindingHasType checks if the given SecretBinding has the given provider type.
func SecretBindingHasType(secretBinding *gardencorev1beta1.SecretBinding, providerType string) bool {
	if secretBinding.Provider == nil {
		return false
	}

	types := GetSecretBindingTypes(secretBinding)
	if len(types) == 0 {
		return false
	}

	return sets.NewString(types...).Has(providerType)
}

// AddTypeToSecretBinding adds the given provider type to the SecretBinding.
func AddTypeToSecretBinding(secretBinding *gardencorev1beta1.SecretBinding, providerType string) {
	if secretBinding.Provider == nil {
		secretBinding.Provider = &gardencorev1beta1.SecretBindingProvider{
			Type: providerType,
		}
		return
	}

	types := GetSecretBindingTypes(secretBinding)
	if !sets.NewString(types...).Has(providerType) {
		types = append(types, providerType)
	}
	secretBinding.Provider.Type = strings.Join(types, ",")
}

// IsCoreDNSAutoscalingModeUsed indicates whether the specified autoscaling mode of CoreDNS is enabled or not.
func IsCoreDNSAutoscalingModeUsed(systemComponents *gardencorev1beta1.SystemComponents, autoscalingMode gardencorev1beta1.CoreDNSAutoscalingMode) bool {
	isDefaultMode := autoscalingMode == gardencorev1beta1.CoreDNSAutoscalingModeHorizontal
	if systemComponents == nil {
		return isDefaultMode
	}

	if systemComponents.CoreDNS == nil {
		return isDefaultMode
	}

	if systemComponents.CoreDNS.Autoscaling == nil {
		return isDefaultMode
	}

	return systemComponents.CoreDNS.Autoscaling.Mode == autoscalingMode
}

// IsCoreDNSRewritingEnabled indicates whether automatic query rewriting in CoreDNS is enabled or not.
func IsCoreDNSRewritingEnabled(featureGate bool, annotations map[string]string) bool {
	_, disabled := annotations[v1beta1constants.AnnotationCoreDNSRewritingDisabled]
	return featureGate && !disabled
}

// IsNodeLocalDNSEnabled indicates whether the node local DNS cache is enabled or not.
// It can be enabled via the annotation (legacy) or via the shoot specification.
func IsNodeLocalDNSEnabled(systemComponents *gardencorev1beta1.SystemComponents, annotations map[string]string) bool {
	fromSpec := false
	if systemComponents != nil && systemComponents.NodeLocalDNS != nil {
		fromSpec = systemComponents.NodeLocalDNS.Enabled
	}
	fromAnnotation := false
	if annotationValue, err := strconv.ParseBool(annotations[v1beta1constants.AnnotationNodeLocalDNS]); err == nil {
		fromAnnotation = annotationValue
	}
	return fromSpec || fromAnnotation
}

// IsTCPEnforcedForNodeLocalDNSToClusterDNS indicates whether TCP is enforced for connections from the node local DNS cache to the cluster DNS (Core DNS) or not.
// It can be disabled via the annotation (legacy) or via the shoot specification.
func IsTCPEnforcedForNodeLocalDNSToClusterDNS(systemComponents *gardencorev1beta1.SystemComponents, annotations map[string]string) bool {
	fromSpec := true
	if systemComponents != nil && systemComponents.NodeLocalDNS != nil && systemComponents.NodeLocalDNS.ForceTCPToClusterDNS != nil {
		fromSpec = *systemComponents.NodeLocalDNS.ForceTCPToClusterDNS
	}
	fromAnnotation := true
	if annotationValue, err := strconv.ParseBool(annotations[v1beta1constants.AnnotationNodeLocalDNSForceTcpToClusterDns]); err == nil {
		fromAnnotation = annotationValue
	}
	return fromSpec && fromAnnotation
}

// IsTCPEnforcedForNodeLocalDNSToUpstreamDNS indicates whether TCP is enforced for connections from the node local DNS cache to the upstream DNS (infrastructure DNS) or not.
// It can be disabled via the annotation (legacy) or via the shoot specification.
func IsTCPEnforcedForNodeLocalDNSToUpstreamDNS(systemComponents *gardencorev1beta1.SystemComponents, annotations map[string]string) bool {
	fromSpec := true
	if systemComponents != nil && systemComponents.NodeLocalDNS != nil && systemComponents.NodeLocalDNS.ForceTCPToUpstreamDNS != nil {
		fromSpec = *systemComponents.NodeLocalDNS.DeepCopy().ForceTCPToUpstreamDNS
	}
	fromAnnotation := true
	if annotationValue, err := strconv.ParseBool(annotations[v1beta1constants.AnnotationNodeLocalDNSForceTcpToUpstreamDns]); err == nil {
		fromAnnotation = annotationValue
	}
	return fromSpec && fromAnnotation
}

// GetShootCARotationPhase returns the specified shoot CA rotation phase or an empty string
func GetShootCARotationPhase(credentials *gardencorev1beta1.ShootCredentials) gardencorev1beta1.ShootCredentialsRotationPhase {
	if credentials != nil && credentials.Rotation != nil && credentials.Rotation.CertificateAuthorities != nil {
		return credentials.Rotation.CertificateAuthorities.Phase
	}
	return ""
}

// MutateShootCARotation mutates the .status.credentials.rotation.certificateAuthorities field based on the provided
// mutation function. If the field is nil then it is initialized.
func MutateShootCARotation(shoot *gardencorev1beta1.Shoot, f func(*gardencorev1beta1.ShootCARotation)) {
	if shoot.Status.Credentials == nil {
		shoot.Status.Credentials = &gardencorev1beta1.ShootCredentials{}
	}
	if shoot.Status.Credentials.Rotation == nil {
		shoot.Status.Credentials.Rotation = &gardencorev1beta1.ShootCredentialsRotation{}
	}
	if shoot.Status.Credentials.Rotation.CertificateAuthorities == nil {
		shoot.Status.Credentials.Rotation.CertificateAuthorities = &gardencorev1beta1.ShootCARotation{}
	}

	f(shoot.Status.Credentials.Rotation.CertificateAuthorities)
}

// MutateShootKubeconfigRotation mutates the .status.credentials.rotation.kubeconfig field based on the provided
// mutation function. If the field is nil then it is initialized.
func MutateShootKubeconfigRotation(shoot *gardencorev1beta1.Shoot, f func(*gardencorev1beta1.ShootKubeconfigRotation)) {
	if shoot.Status.Credentials == nil {
		shoot.Status.Credentials = &gardencorev1beta1.ShootCredentials{}
	}
	if shoot.Status.Credentials.Rotation == nil {
		shoot.Status.Credentials.Rotation = &gardencorev1beta1.ShootCredentialsRotation{}
	}
	if shoot.Status.Credentials.Rotation.Kubeconfig == nil {
		shoot.Status.Credentials.Rotation.Kubeconfig = &gardencorev1beta1.ShootKubeconfigRotation{}
	}

	f(shoot.Status.Credentials.Rotation.Kubeconfig)
}

// IsShootKubeconfigRotationInitiationTimeAfterLastCompletionTime returns true when the lastInitiationTime in the
// .status.credentials.rotation.kubeconfig field is newer than the lastCompletionTime. This is also true if the
// lastCompletionTime is unset.
func IsShootKubeconfigRotationInitiationTimeAfterLastCompletionTime(credentials *gardencorev1beta1.ShootCredentials) bool {
	if credentials == nil ||
		credentials.Rotation == nil ||
		credentials.Rotation.Kubeconfig == nil ||
		credentials.Rotation.Kubeconfig.LastInitiationTime == nil {
		return false
	}

	return credentials.Rotation.Kubeconfig.LastCompletionTime == nil ||
		credentials.Rotation.Kubeconfig.LastCompletionTime.Before(credentials.Rotation.Kubeconfig.LastInitiationTime)
}

// MutateShootSSHKeypairRotation mutates the .status.credentials.rotation.sshKeypair field based on the provided
// mutation function. If the field is nil then it is initialized.
func MutateShootSSHKeypairRotation(shoot *gardencorev1beta1.Shoot, f func(*gardencorev1beta1.ShootSSHKeypairRotation)) {
	if shoot.Status.Credentials == nil {
		shoot.Status.Credentials = &gardencorev1beta1.ShootCredentials{}
	}
	if shoot.Status.Credentials.Rotation == nil {
		shoot.Status.Credentials.Rotation = &gardencorev1beta1.ShootCredentialsRotation{}
	}
	if shoot.Status.Credentials.Rotation.SSHKeypair == nil {
		shoot.Status.Credentials.Rotation.SSHKeypair = &gardencorev1beta1.ShootSSHKeypairRotation{}
	}

	f(shoot.Status.Credentials.Rotation.SSHKeypair)
}

// IsShootSSHKeypairRotationInitiationTimeAfterLastCompletionTime returns true when the lastInitiationTime in the
// .status.credentials.rotation.sshKeypair field is newer than the lastCompletionTime. This is also true if the
// lastCompletionTime is unset.
func IsShootSSHKeypairRotationInitiationTimeAfterLastCompletionTime(credentials *gardencorev1beta1.ShootCredentials) bool {
	if credentials == nil ||
		credentials.Rotation == nil ||
		credentials.Rotation.SSHKeypair == nil ||
		credentials.Rotation.SSHKeypair.LastInitiationTime == nil {
		return false
	}

	return credentials.Rotation.SSHKeypair.LastCompletionTime == nil ||
		credentials.Rotation.SSHKeypair.LastCompletionTime.Before(credentials.Rotation.SSHKeypair.LastInitiationTime)
}

// MutateObservabilityRotation mutates the .status.credentials.rotation.observability field based on the provided
// mutation function. If the field is nil then it is initialized.
func MutateObservabilityRotation(shoot *gardencorev1beta1.Shoot, f func(*gardencorev1beta1.ShootObservabilityRotation)) {
	if shoot.Status.Credentials == nil {
		shoot.Status.Credentials = &gardencorev1beta1.ShootCredentials{}
	}
	if shoot.Status.Credentials.Rotation == nil {
		shoot.Status.Credentials.Rotation = &gardencorev1beta1.ShootCredentialsRotation{}
	}
	if shoot.Status.Credentials.Rotation.Observability == nil {
		shoot.Status.Credentials.Rotation.Observability = &gardencorev1beta1.ShootObservabilityRotation{}
	}

	f(shoot.Status.Credentials.Rotation.Observability)
}

// IsShootObservabilityRotationInitiationTimeAfterLastCompletionTime returns true when the lastInitiationTime in the
// .status.credentials.rotation.observability field is newer than the lastCompletionTime. This is also true if the
// lastCompletionTime is unset.
func IsShootObservabilityRotationInitiationTimeAfterLastCompletionTime(credentials *gardencorev1beta1.ShootCredentials) bool {
	if credentials == nil ||
		credentials.Rotation == nil ||
		credentials.Rotation.Observability == nil ||
		credentials.Rotation.Observability.LastInitiationTime == nil {
		return false
	}

	return credentials.Rotation.Observability.LastCompletionTime == nil ||
		credentials.Rotation.Observability.LastCompletionTime.Before(credentials.Rotation.Observability.LastInitiationTime)
}

// GetShootServiceAccountKeyRotationPhase returns the specified shoot service account key rotation phase or an empty
// string.
func GetShootServiceAccountKeyRotationPhase(credentials *gardencorev1beta1.ShootCredentials) gardencorev1beta1.ShootCredentialsRotationPhase {
	if credentials != nil && credentials.Rotation != nil && credentials.Rotation.ServiceAccountKey != nil {
		return credentials.Rotation.ServiceAccountKey.Phase
	}
	return ""
}

// MutateShootServiceAccountKeyRotation mutates the .status.credentials.rotation.serviceAccountKey field based on the
// provided mutation function. If the field is nil then it is initialized.
func MutateShootServiceAccountKeyRotation(shoot *gardencorev1beta1.Shoot, f func(*gardencorev1beta1.ShootServiceAccountKeyRotation)) {
	if shoot.Status.Credentials == nil {
		shoot.Status.Credentials = &gardencorev1beta1.ShootCredentials{}
	}
	if shoot.Status.Credentials.Rotation == nil {
		shoot.Status.Credentials.Rotation = &gardencorev1beta1.ShootCredentialsRotation{}
	}
	if shoot.Status.Credentials.Rotation.ServiceAccountKey == nil {
		shoot.Status.Credentials.Rotation.ServiceAccountKey = &gardencorev1beta1.ShootServiceAccountKeyRotation{}
	}

	f(shoot.Status.Credentials.Rotation.ServiceAccountKey)
}

// GetShootETCDEncryptionKeyRotationPhase returns the specified shoot ETCD encryption key rotation phase or an empty
// string.
func GetShootETCDEncryptionKeyRotationPhase(credentials *gardencorev1beta1.ShootCredentials) gardencorev1beta1.ShootCredentialsRotationPhase {
	if credentials != nil && credentials.Rotation != nil && credentials.Rotation.ETCDEncryptionKey != nil {
		return credentials.Rotation.ETCDEncryptionKey.Phase
	}
	return ""
}

// MutateShootETCDEncryptionKeyRotation mutates the .status.credentials.rotation.etcdEncryptionKey field based on the
// provided mutation function. If the field is nil then it is initialized.
func MutateShootETCDEncryptionKeyRotation(shoot *gardencorev1beta1.Shoot, f func(*gardencorev1beta1.ShootETCDEncryptionKeyRotation)) {
	if shoot.Status.Credentials == nil {
		shoot.Status.Credentials = &gardencorev1beta1.ShootCredentials{}
	}
	if shoot.Status.Credentials.Rotation == nil {
		shoot.Status.Credentials.Rotation = &gardencorev1beta1.ShootCredentialsRotation{}
	}
	if shoot.Status.Credentials.Rotation.ETCDEncryptionKey == nil {
		shoot.Status.Credentials.Rotation.ETCDEncryptionKey = &gardencorev1beta1.ShootETCDEncryptionKeyRotation{}
	}

	f(shoot.Status.Credentials.Rotation.ETCDEncryptionKey)
}

// IsPSPDisabled returns true if the PodSecurityPolicy plugin is explicitly disabled in the ShootSpec or the cluster version is >= 1.25.
func IsPSPDisabled(shoot *gardencorev1beta1.Shoot) bool {
	if versionutils.ConstraintK8sGreaterEqual125.Check(semver.MustParse(shoot.Spec.Kubernetes.Version)) {
		return true
	}

	if shoot.Spec.Kubernetes.KubeAPIServer != nil {
		for _, plugin := range shoot.Spec.Kubernetes.KubeAPIServer.AdmissionPlugins {
			if plugin.Name == "PodSecurityPolicy" && pointer.BoolDeref(plugin.Disabled, false) {
				return true
			}
		}
	}
	return false
}

// IsFailureToleranceTypeZone returns true if failureToleranceType is zone else returns false.
func IsFailureToleranceTypeZone(failureToleranceType *gardencorev1beta1.FailureToleranceType) bool {
	return failureToleranceType != nil && *failureToleranceType == gardencorev1beta1.FailureToleranceTypeZone
}

// IsFailureToleranceTypeNode returns true if failureToleranceType is node else returns false.
func IsFailureToleranceTypeNode(failureToleranceType *gardencorev1beta1.FailureToleranceType) bool {
	return failureToleranceType != nil && *failureToleranceType == gardencorev1beta1.FailureToleranceTypeNode
}

// IsHAControlPlaneConfigured returns true if HA configuration for the shoot control plane has been set either
// via an alpha-annotation or ControlPlane Spec.
func IsHAControlPlaneConfigured(shoot *gardencorev1beta1.Shoot) bool {
	return metav1.HasAnnotation(shoot.ObjectMeta, v1beta1constants.ShootAlphaControlPlaneHighAvailability) || shoot.Spec.ControlPlane != nil && shoot.Spec.ControlPlane.HighAvailability != nil
}

// IsMultiZonalShootControlPlane checks if the shoot should have a multi-zonal control plane.
func IsMultiZonalShootControlPlane(shoot *gardencorev1beta1.Shoot) bool {
	hasZonalAnnotation := shoot.ObjectMeta.Annotations[v1beta1constants.ShootAlphaControlPlaneHighAvailability] == v1beta1constants.ShootAlphaControlPlaneHighAvailabilityMultiZone
	hasZoneFailureToleranceTypeSetInSpec := shoot.Spec.ControlPlane != nil && shoot.Spec.ControlPlane.HighAvailability != nil && shoot.Spec.ControlPlane.HighAvailability.FailureTolerance.Type == gardencorev1beta1.FailureToleranceTypeZone
	return hasZonalAnnotation || hasZoneFailureToleranceTypeSetInSpec
}

// GetFailureToleranceType determines the FailureToleranceType by looking at both the alpha HA annotations and shoot spec ControlPlane.
func GetFailureToleranceType(shoot *gardencorev1beta1.Shoot) *gardencorev1beta1.FailureToleranceType {
	if haAnnot, ok := shoot.Annotations[v1beta1constants.ShootAlphaControlPlaneHighAvailability]; ok {
		var failureToleranceType gardencorev1beta1.FailureToleranceType
		if haAnnot == v1beta1constants.ShootAlphaControlPlaneHighAvailabilityMultiZone {
			failureToleranceType = gardencorev1beta1.FailureToleranceTypeZone
		} else {
			failureToleranceType = gardencorev1beta1.FailureToleranceTypeNode
		}
		return &failureToleranceType
	}
	if shoot.Spec.ControlPlane != nil && shoot.Spec.ControlPlane.HighAvailability != nil {
		return &shoot.Spec.ControlPlane.HighAvailability.FailureTolerance.Type
	}
	return nil
}

// SeedWantsManagedIngress returns true in case the seed cluster wants its ingress controller to be managed by Gardener.
func SeedWantsManagedIngress(seed *gardencorev1beta1.Seed) bool {
	return seed.Spec.DNS.Provider != nil && seed.Spec.Ingress != nil && seed.Spec.Ingress.Controller.Kind == v1beta1constants.IngressKindNginx
}
