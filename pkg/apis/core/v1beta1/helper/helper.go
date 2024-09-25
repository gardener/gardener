// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	versionutils "github.com/gardener/gardener/pkg/utils/version"
)

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
func HasOperationAnnotation(annotations map[string]string) bool {
	return annotations[v1beta1constants.GardenerOperation] == v1beta1constants.GardenerOperationReconcile ||
		annotations[v1beta1constants.GardenerOperation] == v1beta1constants.GardenerOperationRestore ||
		annotations[v1beta1constants.GardenerOperation] == v1beta1constants.GardenerOperationMigrate
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
		return nil, errors.New("apiSrvMaxReplicas has to be specified for ManagedSeed API server autoscaler")
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

// ShootUsesUnmanagedDNS returns true if the shoot's DNS section is marked as 'unmanaged'.
func ShootUsesUnmanagedDNS(shoot *gardencorev1beta1.Shoot) bool {
	return shoot.Spec.DNS != nil && len(shoot.Spec.DNS.Providers) > 0 && shoot.Spec.DNS.Providers[0].Type != nil && *shoot.Spec.DNS.Providers[0].Type == "unmanaged"
}

// ShootNeedsForceDeletion determines whether a Shoot should be force deleted or not.
func ShootNeedsForceDeletion(shoot *gardencorev1beta1.Shoot) bool {
	if shoot == nil {
		return false
	}

	value, ok := shoot.Annotations[v1beta1constants.AnnotationConfirmationForceDeletion]
	if !ok {
		return false
	}

	forceDelete, _ := strconv.ParseBool(value)
	return forceDelete
}

// ShootSchedulingProfile returns the scheduling profile of the given Shoot.
func ShootSchedulingProfile(shoot *gardencorev1beta1.Shoot) *gardencorev1beta1.SchedulingProfile {
	if shoot.Spec.Kubernetes.KubeScheduler != nil {
		return shoot.Spec.Kubernetes.KubeScheduler.Profile
	}
	return nil
}

// ShootConfinesSpecUpdateRollout returns a bool.
func ShootConfinesSpecUpdateRollout(maintenance *gardencorev1beta1.Maintenance) bool {
	return maintenance != nil && maintenance.ConfineSpecUpdateRollout != nil && *maintenance.ConfineSpecUpdateRollout
}

// SeedSettingExcessCapacityReservationEnabled returns true if the 'excess capacity reservation' setting is enabled.
func SeedSettingExcessCapacityReservationEnabled(settings *gardencorev1beta1.SeedSettings) bool {
	return settings == nil || settings.ExcessCapacityReservation == nil || ptr.Deref(settings.ExcessCapacityReservation.Enabled, true)
}

// SeedSettingVerticalPodAutoscalerEnabled returns true if the 'verticalPodAutoscaler' setting is enabled.
func SeedSettingVerticalPodAutoscalerEnabled(settings *gardencorev1beta1.SeedSettings) bool {
	return settings == nil || settings.VerticalPodAutoscaler == nil || settings.VerticalPodAutoscaler.Enabled
}

// SeedSettingDependencyWatchdogWeederEnabled returns true if the dependency-watchdog-weeder is enabled.
func SeedSettingDependencyWatchdogWeederEnabled(settings *gardencorev1beta1.SeedSettings) bool {
	return settings == nil || settings.DependencyWatchdog == nil || settings.DependencyWatchdog.Weeder == nil || settings.DependencyWatchdog.Weeder.Enabled
}

// SeedSettingDependencyWatchdogProberEnabled returns true if the dependency-watchdog-prober is enabled.
func SeedSettingDependencyWatchdogProberEnabled(settings *gardencorev1beta1.SeedSettings) bool {
	return settings == nil || settings.DependencyWatchdog == nil || settings.DependencyWatchdog.Prober == nil || settings.DependencyWatchdog.Prober.Enabled
}

// SeedSettingTopologyAwareRoutingEnabled returns true if the topology-aware routing is enabled.
func SeedSettingTopologyAwareRoutingEnabled(settings *gardencorev1beta1.SeedSettings) bool {
	return settings != nil && settings.TopologyAwareRouting != nil && settings.TopologyAwareRouting.Enabled
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
func FindMachineImageVersion(machineImages []gardencorev1beta1.MachineImage, name, version string) (gardencorev1beta1.MachineImageVersion, bool) {
	for _, image := range machineImages {
		if image.Name == name {
			for _, imageVersion := range image.Versions {
				if imageVersion.Version == version {
					return imageVersion, true
				}
			}
		}
	}

	return gardencorev1beta1.MachineImageVersion{}, false
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

// ToExpirableVersions returns the expirable versions from the given machine image versions.
func ToExpirableVersions(versions []gardencorev1beta1.MachineImageVersion) []gardencorev1beta1.ExpirableVersion {
	expVersions := []gardencorev1beta1.ExpirableVersion{}
	for _, version := range versions {
		expVersions = append(expVersions, version.ExpirableVersion)
	}
	return expVersions
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

// FindPrimaryDNSProvider finds the primary provider among the given `providers`.
// It returns the first provider if multiple candidates are found.
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

// GetLatestVersionForPatchAutoUpdate finds the latest patch version for a given <currentVersion> for the current minor version from a given slice of versions.
// The current version, preview and expired versions do not qualify.
// In case no newer patch version is found, returns false and an empty string. Otherwise, returns true and the found version.
func GetLatestVersionForPatchAutoUpdate(versions []gardencorev1beta1.ExpirableVersion, currentVersion string) (bool, string, error) {
	currentSemVerVersion, err := semver.NewVersion(currentVersion)
	if err != nil {
		return false, "", err
	}

	predicates := []VersionPredicate{FilterDifferentMajorMinorVersionAndLowerPatchVersionsOfSameMinor(*currentSemVerVersion)}

	return getVersionForAutoUpdate(versions, currentSemVerVersion, predicates)
}

// GetLatestVersionForMinorAutoUpdate finds the latest minor with the latest patch version higher than a given <currentVersion> for the current major version from a given slice of versions.
// Returns the highest patch version for the current minor in case the current version is not the highest patch version yet.
// The current version, preview and expired versions do not qualify.
// In case no newer version is found, returns false and an empty string. Otherwise, returns true and the found version.
func GetLatestVersionForMinorAutoUpdate(versions []gardencorev1beta1.ExpirableVersion, currentVersion string) (bool, string, error) {
	// always first check if there is a higher patch version available
	found, version, err := GetLatestVersionForPatchAutoUpdate(versions, currentVersion)
	if found {
		return found, version, nil
	}
	if err != nil {
		return false, version, err
	}

	currentSemVerVersion, err := semver.NewVersion(currentVersion)
	if err != nil {
		return false, "", err
	}

	predicates := []VersionPredicate{FilterDifferentMajorVersion(*currentSemVerVersion)}

	return getVersionForAutoUpdate(versions, currentSemVerVersion, predicates)
}

// GetOverallLatestVersionForAutoUpdate finds the overall latest version higher than a given <currentVersion> for the current major version from a given slice of versions.
// Returns the highest patch version for the current minor in case the current version is not the highest patch version yet.
// The current, preview and expired versions do not qualify.
// In case no newer version is found, returns false and an empty string. Otherwise, returns true and the found version.
func GetOverallLatestVersionForAutoUpdate(versions []gardencorev1beta1.ExpirableVersion, currentVersion string) (bool, string, error) {
	// always first check if there is a higher patch version available to update to
	found, version, err := GetLatestVersionForPatchAutoUpdate(versions, currentVersion)
	if found {
		return found, version, nil
	}
	if err != nil {
		return false, version, err
	}

	currentSemVerVersion, err := semver.NewVersion(currentVersion)
	if err != nil {
		return false, "", err
	}

	// if there is no higher patch version available, get the overall latest
	return getVersionForAutoUpdate(versions, currentSemVerVersion, []VersionPredicate{})
}

// getVersionForAutoUpdate finds the latest eligible version higher than a given <currentVersion> from a slice of versions.
// Versions <= the current version, preview and expired versions do not qualify for patch updates.
// First tries to find a non-deprecated version.
// In case no newer patch version is found, returns false and an empty string. Otherwise, returns true and the found version.
func getVersionForAutoUpdate(versions []gardencorev1beta1.ExpirableVersion, currentSemVerVersion *semver.Version, predicates []VersionPredicate) (bool, string, error) {
	versionPredicates := append([]VersionPredicate{FilterExpiredVersion(), FilterSameVersion(*currentSemVerVersion), FilterLowerVersion(*currentSemVerVersion)}, predicates...)

	// Try to find non-deprecated version first
	qualifyingVersionFound, latestNonDeprecatedImageVersion, err := GetLatestQualifyingVersion(versions, append(versionPredicates, FilterDeprecatedVersion())...)
	if err != nil {
		return false, "", err
	}
	if qualifyingVersionFound {
		return true, latestNonDeprecatedImageVersion.Version, nil
	}

	// otherwise, also consider deprecated versions
	qualifyingVersionFound, latestVersion, err := GetLatestQualifyingVersion(versions, versionPredicates...)
	if err != nil {
		return false, "", err
	}
	// latest version cannot be found. Do not return an error, but allow for forceful upgrade if Shoot's version is expired.
	if !qualifyingVersionFound {
		return false, "", nil
	}

	return true, latestVersion.Version, nil
}

// GetVersionForForcefulUpdateToConsecutiveMinor finds a version from a slice of expirable versions that qualifies for a minor level update given a <currentVersion>.
// A qualifying version is a non-preview version having the minor version increased by exactly one version (required for Kubernetes version upgrades).
// In case the consecutive minor version has only expired versions, picks the latest expired version (will try another update during the next maintenance time).
// If a version can be found, returns true and the qualifying patch version of the next minor version.
// In case it does not find a version, it returns false and an empty string.
func GetVersionForForcefulUpdateToConsecutiveMinor(versions []gardencorev1beta1.ExpirableVersion, currentVersion string) (bool, string, error) {
	currentSemVerVersion, err := semver.NewVersion(currentVersion)
	if err != nil {
		return false, "", err
	}

	// filters out any version that does not have minor version +1
	predicates := []VersionPredicate{FilterDifferentMajorVersion(*currentSemVerVersion), FilterNonConsecutiveMinorVersion(*currentSemVerVersion)}

	qualifyingVersionFound, latestVersion, err := GetLatestQualifyingVersion(versions, append(predicates, FilterExpiredVersion())...)
	if err != nil {
		return false, "", err
	}

	// if no qualifying version is found, allow force update to an expired version
	if !qualifyingVersionFound {
		qualifyingVersionFound, latestVersion, err = GetLatestQualifyingVersion(versions, predicates...)
		if err != nil {
			return false, "", err
		}
		if !qualifyingVersionFound {
			return false, "", nil
		}
	}

	return true, latestVersion.Version, nil
}

// GetVersionForForcefulUpdateToNextHigherMinor finds a version from a slice of expirable versions that qualifies for a minor level update given a <currentVersion>.
// A qualifying version is the highest non-preview version with the next higher minor version from the given slice of versions.
// In case the consecutive minor version has only expired versions, picks the latest expired version (will try another update during the next maintenance time).
// If a version can be found, returns true and the qualifying version.
// In case it does not find a version, it returns false and an empty string.
func GetVersionForForcefulUpdateToNextHigherMinor(versions []gardencorev1beta1.ExpirableVersion, currentVersion string) (bool, string, error) {
	currentSemVerVersion, err := semver.NewVersion(currentVersion)
	if err != nil {
		return false, "", err
	}

	predicates := []VersionPredicate{FilterDifferentMajorVersion(*currentSemVerVersion), FilterEqualAndSmallerMinorVersion(*currentSemVerVersion)}

	// prefer non-expired version
	return getVersionForMachineImageForceUpdate(versions, func(v semver.Version) int64 { return int64(v.Minor()) }, currentSemVerVersion, predicates)
}

// GetVersionForForcefulUpdateToNextHigherMajor finds a version from a slice of expirable versions that qualifies for a major level update given a <currentVersion>.
// A qualifying version is a non-preview version with the next (as defined in the CloudProfile for the image) higher major version.
// In case the next major version has only expired versions, picks the latest expired version (will try another update during the next maintenance time).
// If a version can be found, returns true and the qualifying version of the next major version.
// In case it does not find a version, it returns false and an empty string.
func GetVersionForForcefulUpdateToNextHigherMajor(versions []gardencorev1beta1.ExpirableVersion, currentVersion string) (bool, string, error) {
	currentSemVerVersion, err := semver.NewVersion(currentVersion)
	if err != nil {
		return false, "", err
	}

	predicates := []VersionPredicate{FilterEqualAndSmallerMajorVersion(*currentSemVerVersion)}

	// prefer non-expired version
	return getVersionForMachineImageForceUpdate(versions, func(v semver.Version) int64 { return int64(v.Major()) }, currentSemVerVersion, predicates)
}

// getVersionForMachineImageForceUpdate finds a version from a slice of expirable versions that qualifies for an update given a <currentVersion>.
// In contrast to determining a version for an auto-update, also allows update to an expired version in case a not-expired version cannot be determined.
// Used only for machine image updates, as finds a qualifying version from the next higher minor version, which is not necessarily consecutive (n+1).
func getVersionForMachineImageForceUpdate(versions []gardencorev1beta1.ExpirableVersion, getMajorOrMinor GetMajorOrMinor, currentSemVerVersion *semver.Version, predicates []VersionPredicate) (bool, string, error) {
	foundVersion, qualifyingVersion, nextMinorOrMajorVersion, err := GetQualifyingVersionForNextHigher(versions, getMajorOrMinor, currentSemVerVersion, append(predicates, FilterExpiredVersion())...)
	if err != nil {
		return false, "", err
	}

	skippedNextMajorMinor := false

	if foundVersion {
		parse, err := semver.NewVersion(qualifyingVersion.Version)
		if err != nil {
			return false, "", err
		}

		skippedNextMajorMinor = getMajorOrMinor(*parse) > nextMinorOrMajorVersion
	}

	// Two options when allowing updates to expired versions
	// 1) No higher non-expired qualifying version could be found at all
	// 2) Found a qualifying non-expired version, but we skipped the next minor/major.
	//    Potentially skipped expired versions in the next minor/major that qualify.
	//    Prefer update to expired version in next minor/major instead of skipping over minor/major altogether.
	//    Example: current version: 1.1.0, qualifying version : 1.4.1, next minor: 2. We skipped over the next minor which might have qualifying expired versions.
	if !foundVersion || skippedNextMajorMinor {
		foundVersion, qualifyingVersion, _, err = GetQualifyingVersionForNextHigher(versions, getMajorOrMinor, currentSemVerVersion, predicates...)
		if err != nil {
			return false, "", err
		}
		if !foundVersion {
			return false, "", nil
		}
	}

	return true, qualifyingVersion.Version, nil
}

// GetLatestQualifyingVersion returns the latest expirable version from a set of expirable versions.
// A version qualifies if its classification is not preview and the optional predicate does not filter out the version.
// If the predicate returns true, the version is not considered for the latest qualifying version.
func GetLatestQualifyingVersion(versions []gardencorev1beta1.ExpirableVersion, predicate ...VersionPredicate) (qualifyingVersionFound bool, latest *gardencorev1beta1.ExpirableVersion, err error) {
	var (
		latestSemanticVersion = &semver.Version{}
		latestVersion         *gardencorev1beta1.ExpirableVersion
	)
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

// GetMajorOrMinor returns either the major or the minor version from a semVer version.
type GetMajorOrMinor func(v semver.Version) int64

// GetQualifyingVersionForNextHigher returns the latest expirable version for the next higher {minor/major} (not necessarily consecutive n+1) version from a set of expirable versions.
// A version qualifies if its classification is not preview and the optional predicate does not filter out the version.
// If the predicate returns true, the version is not considered for the latest qualifying version.
func GetQualifyingVersionForNextHigher(versions []gardencorev1beta1.ExpirableVersion, majorOrMinor GetMajorOrMinor, currentSemVerVersion *semver.Version, predicates ...VersionPredicate) (qualifyingVersionFound bool, qualifyingVersion *gardencorev1beta1.ExpirableVersion, nextMinorOrMajor int64, err error) {
	// How to find the highest version with the next higher (not necessarily consecutive n+1) minor version (if the next higher minor version has no qualifying version, skip it to avoid consecutive updates)
	// 1) Sort the versions in ascending order
	// 2) Loop over the sorted array until the minor version changes (select all versions for the next higher minor)
	//    - predicates filter out version with minor/major <= current_minor/major
	// 3) Then select the last version in the array (that's the highest)

	slices.SortFunc(versions, func(a, b gardencorev1beta1.ExpirableVersion) int {
		return semver.MustParse(a.Version).Compare(semver.MustParse(b.Version))
	})

	var (
		highestVersionNextHigherMinorOrMajor   *semver.Version
		nextMajorOrMinorVersion                int64 = -1
		expirableVersionNextHigherMinorOrMajor       = gardencorev1beta1.ExpirableVersion{}
	)

OUTER:
	for _, v := range versions {
		parse, err := semver.NewVersion(v.Version)
		if err != nil {
			return false, nil, 0, err
		}

		// Determine the next higher minor/major version, even though all versions from that minor/major might be filtered (e.g, all expired)
		// That's required so that the caller can determine if the next minor/major version has been skipped or not.
		if majorOrMinor(*parse) > majorOrMinor(*currentSemVerVersion) && (majorOrMinor(*parse) < nextMajorOrMinorVersion || nextMajorOrMinorVersion == -1) {
			nextMajorOrMinorVersion = majorOrMinor(*parse)
		}

		// never update to preview versions
		if v.Classification != nil && *v.Classification == gardencorev1beta1.ClassificationPreview {
			continue
		}

		for _, p := range predicates {
			if p == nil {
				continue
			}

			shouldFilter, err := p(v, parse)
			if err != nil {
				return false, nil, nextMajorOrMinorVersion, fmt.Errorf("error while evaluation predicate: %w", err)
			}
			if shouldFilter {
				continue OUTER
			}
		}

		// last version is the highest version for next larger minor/major
		if highestVersionNextHigherMinorOrMajor != nil && majorOrMinor(*parse) > majorOrMinor(*highestVersionNextHigherMinorOrMajor) {
			break
		}
		highestVersionNextHigherMinorOrMajor = parse
		expirableVersionNextHigherMinorOrMajor = v
	}

	// unable to find qualified versions
	if highestVersionNextHigherMinorOrMajor == nil {
		return false, nil, nextMajorOrMinorVersion, nil
	}
	return true, &expirableVersionNextHigherMinorOrMajor, nextMajorOrMinorVersion, nil
}

// FilterDifferentMajorMinorVersionAndLowerPatchVersionsOfSameMinor returns a VersionPredicate(closure) that returns true if a given version v
//   - has a different major.minor version compared to the currentSemVerVersion
//   - has a lower patch version (acts as >= relational operator)
//
// Uses the tilde range operator.
func FilterDifferentMajorMinorVersionAndLowerPatchVersionsOfSameMinor(currentSemVerVersion semver.Version) VersionPredicate {
	return func(_ gardencorev1beta1.ExpirableVersion, v *semver.Version) (bool, error) {
		isWithinRange, err := versionutils.CompareVersions(v.String(), "~", currentSemVerVersion.String())
		if err != nil {
			return true, err
		}
		return !isWithinRange, nil
	}
}

// FilterNonConsecutiveMinorVersion returns a VersionPredicate(closure) that evaluates whether a given version v has a consecutive minor version compared to the currentSemVerVersion
//   - implicitly, therefore also versions cannot be smaller than the current version
//
// returns true if v does not have a consecutive minor version.
func FilterNonConsecutiveMinorVersion(currentSemVerVersion semver.Version) VersionPredicate {
	return func(_ gardencorev1beta1.ExpirableVersion, v *semver.Version) (bool, error) {
		if v.Major() != currentSemVerVersion.Major() {
			return true, nil
		}

		hasIncorrectMinor := currentSemVerVersion.Minor()+1 != v.Minor()
		return hasIncorrectMinor, nil
	}
}

// FilterDifferentMajorVersion returns a VersionPredicate(closure) that evaluates whether a given version v has the same major version compared to the currentSemVerVersion.
// Returns true if v does not have the same major version.
func FilterDifferentMajorVersion(currentSemVerVersion semver.Version) VersionPredicate {
	return func(_ gardencorev1beta1.ExpirableVersion, v *semver.Version) (bool, error) {
		return v.Major() != currentSemVerVersion.Major(), nil
	}
}

// FilterEqualAndSmallerMajorVersion returns a VersionPredicate(closure) that evaluates whether a given version v has a smaller major version compared to the currentSemVerVersion.
// Returns true if v has a smaller or equal major version.
func FilterEqualAndSmallerMajorVersion(currentSemVerVersion semver.Version) VersionPredicate {
	return func(_ gardencorev1beta1.ExpirableVersion, v *semver.Version) (bool, error) {
		return v.Major() <= currentSemVerVersion.Major(), nil
	}
}

// FilterEqualAndSmallerMinorVersion returns a VersionPredicate(closure) that evaluates whether a given version v has a smaller or equal minor version compared to the currentSemVerVersion.
// Returns true if v has a smaller or equal minor version.
func FilterEqualAndSmallerMinorVersion(currentSemVerVersion semver.Version) VersionPredicate {
	return func(_ gardencorev1beta1.ExpirableVersion, v *semver.Version) (bool, error) {
		return v.Minor() <= currentSemVerVersion.Minor(), nil
	}
}

// FilterSameVersion returns a VersionPredicate(closure) that evaluates whether a given version v is equal to the currentSemVerVersion.
// returns true if it is equal.
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

// GetResourceByName returns the NamedResourceReference with the given name in the given slice, or nil if not found.
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
		oldNames = sets.New[string]()
		newNames = sets.New[string]()
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

// ShootResourceReferencesEqual returns true when at least one of the Secret/ConfigMap resource references inside a Shoot
// has been changed.
func ShootResourceReferencesEqual(oldResources, newResources []gardencorev1beta1.NamedResourceReference) bool {
	var (
		oldNames = sets.New[string]()
		newNames = sets.New[string]()
	)

	for _, resource := range oldResources {
		if resource.ResourceRef.APIVersion == "v1" && sets.New("Secret", "ConfigMap").Has(resource.ResourceRef.Kind) {
			oldNames.Insert(resource.ResourceRef.Kind + "/" + resource.ResourceRef.Name)
		}
	}

	for _, resource := range newResources {
		if resource.ResourceRef.APIVersion == "v1" && sets.New("Secret", "ConfigMap").Has(resource.ResourceRef.Kind) {
			newNames.Insert(resource.ResourceRef.Kind + "/" + resource.ResourceRef.Name)
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

// GetShootAuthenticationConfigurationConfigMapName returns the Shoot's ConfigMap reference name for the aithentication configuration.
func GetShootAuthenticationConfigurationConfigMapName(apiServerConfig *gardencorev1beta1.KubeAPIServerConfig) string {
	if apiServerConfig != nil &&
		apiServerConfig.StructuredAuthentication != nil {
		return apiServerConfig.StructuredAuthentication.ConfigMapName
	}
	return ""
}

// AnonymousAuthenticationEnabled returns true if anonymous authentication is set explicitly to 'true' and false otherwise.
func AnonymousAuthenticationEnabled(kubeAPIServerConfig *gardencorev1beta1.KubeAPIServerConfig) bool {
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
func CalculateSeedUsage(shootList []*gardencorev1beta1.Shoot) map[string]int {
	m := map[string]int{}

	for _, shoot := range shootList {
		var (
			specSeed   = ptr.Deref(shoot.Spec.SeedName, "")
			statusSeed = ptr.Deref(shoot.Status.SeedName, "")
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

// CalculateEffectiveKubeletConfiguration returns the worker group specific kubelet configuration if available.
// Otherwise the shoot kubelet configuration is returned
func CalculateEffectiveKubeletConfiguration(shootKubelet *gardencorev1beta1.KubeletConfig, workerKubernetes *gardencorev1beta1.WorkerKubernetes) *gardencorev1beta1.KubeletConfig {
	if workerKubernetes != nil && workerKubernetes.Kubelet != nil {
		return workerKubernetes.Kubelet
	}
	return shootKubelet
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

	return sets.New(types...).Has(providerType)
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
	if !sets.New(types...).Has(providerType) {
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

// IsNodeLocalDNSEnabled indicates whether the node local DNS cache is enabled or not.
func IsNodeLocalDNSEnabled(systemComponents *gardencorev1beta1.SystemComponents) bool {
	return systemComponents != nil && systemComponents.NodeLocalDNS != nil && systemComponents.NodeLocalDNS.Enabled
}

// GetNodeLocalDNS returns a pointer to the NodeLocalDNS spec.
func GetNodeLocalDNS(systemComponents *gardencorev1beta1.SystemComponents) *gardencorev1beta1.NodeLocalDNS {
	if systemComponents != nil {
		return systemComponents.NodeLocalDNS
	}
	return nil
}

// GetShootCARotationPhase returns the specified shoot CA rotation phase or an empty string
func GetShootCARotationPhase(credentials *gardencorev1beta1.ShootCredentials) gardencorev1beta1.CredentialsRotationPhase {
	if credentials != nil && credentials.Rotation != nil && credentials.Rotation.CertificateAuthorities != nil {
		return credentials.Rotation.CertificateAuthorities.Phase
	}
	return ""
}

// MutateShootCARotation mutates the .status.credentials.rotation.certificateAuthorities field based on the provided
// mutation function. If the field is nil then it is initialized.
func MutateShootCARotation(shoot *gardencorev1beta1.Shoot, f func(*gardencorev1beta1.CARotation)) {
	if f == nil {
		return
	}

	if shoot.Status.Credentials == nil {
		shoot.Status.Credentials = &gardencorev1beta1.ShootCredentials{}
	}
	if shoot.Status.Credentials.Rotation == nil {
		shoot.Status.Credentials.Rotation = &gardencorev1beta1.ShootCredentialsRotation{}
	}
	if shoot.Status.Credentials.Rotation.CertificateAuthorities == nil {
		shoot.Status.Credentials.Rotation.CertificateAuthorities = &gardencorev1beta1.CARotation{}
	}

	f(shoot.Status.Credentials.Rotation.CertificateAuthorities)
}

// MutateShootKubeconfigRotation mutates the .status.credentials.rotation.kubeconfig field based on the provided
// mutation function. If the field is nil then it is initialized.
func MutateShootKubeconfigRotation(shoot *gardencorev1beta1.Shoot, f func(*gardencorev1beta1.ShootKubeconfigRotation)) {
	if f == nil {
		return
	}

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
	if f == nil {
		return
	}

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
func MutateObservabilityRotation(shoot *gardencorev1beta1.Shoot, f func(*gardencorev1beta1.ObservabilityRotation)) {
	if f == nil {
		return
	}

	if shoot.Status.Credentials == nil {
		shoot.Status.Credentials = &gardencorev1beta1.ShootCredentials{}
	}
	if shoot.Status.Credentials.Rotation == nil {
		shoot.Status.Credentials.Rotation = &gardencorev1beta1.ShootCredentialsRotation{}
	}
	if shoot.Status.Credentials.Rotation.Observability == nil {
		shoot.Status.Credentials.Rotation.Observability = &gardencorev1beta1.ObservabilityRotation{}
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
func GetShootServiceAccountKeyRotationPhase(credentials *gardencorev1beta1.ShootCredentials) gardencorev1beta1.CredentialsRotationPhase {
	if credentials != nil && credentials.Rotation != nil && credentials.Rotation.ServiceAccountKey != nil {
		return credentials.Rotation.ServiceAccountKey.Phase
	}
	return ""
}

// MutateShootServiceAccountKeyRotation mutates the .status.credentials.rotation.serviceAccountKey field based on the
// provided mutation function. If the field is nil then it is initialized.
func MutateShootServiceAccountKeyRotation(shoot *gardencorev1beta1.Shoot, f func(*gardencorev1beta1.ServiceAccountKeyRotation)) {
	if f == nil {
		return
	}

	if shoot.Status.Credentials == nil {
		shoot.Status.Credentials = &gardencorev1beta1.ShootCredentials{}
	}
	if shoot.Status.Credentials.Rotation == nil {
		shoot.Status.Credentials.Rotation = &gardencorev1beta1.ShootCredentialsRotation{}
	}
	if shoot.Status.Credentials.Rotation.ServiceAccountKey == nil {
		shoot.Status.Credentials.Rotation.ServiceAccountKey = &gardencorev1beta1.ServiceAccountKeyRotation{}
	}

	f(shoot.Status.Credentials.Rotation.ServiceAccountKey)
}

// GetShootETCDEncryptionKeyRotationPhase returns the specified shoot ETCD encryption key rotation phase or an empty
// string.
func GetShootETCDEncryptionKeyRotationPhase(credentials *gardencorev1beta1.ShootCredentials) gardencorev1beta1.CredentialsRotationPhase {
	if credentials != nil && credentials.Rotation != nil && credentials.Rotation.ETCDEncryptionKey != nil {
		return credentials.Rotation.ETCDEncryptionKey.Phase
	}
	return ""
}

// MutateShootETCDEncryptionKeyRotation mutates the .status.credentials.rotation.etcdEncryptionKey field based on the
// provided mutation function. If the field is nil then it is initialized.
func MutateShootETCDEncryptionKeyRotation(shoot *gardencorev1beta1.Shoot, f func(*gardencorev1beta1.ETCDEncryptionKeyRotation)) {
	if f == nil {
		return
	}

	if shoot.Status.Credentials == nil {
		shoot.Status.Credentials = &gardencorev1beta1.ShootCredentials{}
	}
	if shoot.Status.Credentials.Rotation == nil {
		shoot.Status.Credentials.Rotation = &gardencorev1beta1.ShootCredentialsRotation{}
	}
	if shoot.Status.Credentials.Rotation.ETCDEncryptionKey == nil {
		shoot.Status.Credentials.Rotation.ETCDEncryptionKey = &gardencorev1beta1.ETCDEncryptionKeyRotation{}
	}

	f(shoot.Status.Credentials.Rotation.ETCDEncryptionKey)
}

// GetAllZonesFromShoot returns the set of all availability zones defined in the worker pools of the Shoot specification.
func GetAllZonesFromShoot(shoot *gardencorev1beta1.Shoot) sets.Set[string] {
	out := sets.New[string]()
	for _, worker := range shoot.Spec.Provider.Workers {
		out.Insert(worker.Zones...)
	}
	return out
}

// IsFailureToleranceTypeZone returns true if failureToleranceType is zone else returns false.
func IsFailureToleranceTypeZone(failureToleranceType *gardencorev1beta1.FailureToleranceType) bool {
	return failureToleranceType != nil && *failureToleranceType == gardencorev1beta1.FailureToleranceTypeZone
}

// IsFailureToleranceTypeNode returns true if failureToleranceType is node else returns false.
func IsFailureToleranceTypeNode(failureToleranceType *gardencorev1beta1.FailureToleranceType) bool {
	return failureToleranceType != nil && *failureToleranceType == gardencorev1beta1.FailureToleranceTypeNode
}

// IsHAControlPlaneConfigured returns true if HA configuration for the shoot control plane has been set.
func IsHAControlPlaneConfigured(shoot *gardencorev1beta1.Shoot) bool {
	return shoot.Spec.ControlPlane != nil && shoot.Spec.ControlPlane.HighAvailability != nil
}

// IsMultiZonalShootControlPlane checks if the shoot should have a multi-zonal control plane.
func IsMultiZonalShootControlPlane(shoot *gardencorev1beta1.Shoot) bool {
	return shoot.Spec.ControlPlane != nil && shoot.Spec.ControlPlane.HighAvailability != nil && shoot.Spec.ControlPlane.HighAvailability.FailureTolerance.Type == gardencorev1beta1.FailureToleranceTypeZone
}

// IsWorkerless checks if the shoot has zero workers.
func IsWorkerless(shoot *gardencorev1beta1.Shoot) bool {
	return len(shoot.Spec.Provider.Workers) == 0
}

// ShootEnablesSSHAccess returns true if ssh access to worker nodes should be allowed for the given shoot.
func ShootEnablesSSHAccess(shoot *gardencorev1beta1.Shoot) bool {
	return !IsWorkerless(shoot) &&
		(shoot.Spec.Provider.WorkersSettings == nil || shoot.Spec.Provider.WorkersSettings.SSHAccess == nil || shoot.Spec.Provider.WorkersSettings.SSHAccess.Enabled)
}

// GetFailureToleranceType determines the failure tolerance type of the given shoot.
func GetFailureToleranceType(shoot *gardencorev1beta1.Shoot) *gardencorev1beta1.FailureToleranceType {
	if shoot.Spec.ControlPlane != nil && shoot.Spec.ControlPlane.HighAvailability != nil {
		return &shoot.Spec.ControlPlane.HighAvailability.FailureTolerance.Type
	}
	return nil
}

// IsTopologyAwareRoutingForShootControlPlaneEnabled returns whether the topology aware routing is enabled for the given Shoot control plane.
// Topology-aware routing is enabled when the corresponding Seed setting is enabled and the Shoot has a multi-zonal control plane.
func IsTopologyAwareRoutingForShootControlPlaneEnabled(seed *gardencorev1beta1.Seed, shoot *gardencorev1beta1.Shoot) bool {
	return SeedSettingTopologyAwareRoutingEnabled(seed.Spec.Settings) && IsMultiZonalShootControlPlane(shoot)
}

// ShootHasOperationType returns true when the 'type' in the last operation matches the provided type.
func ShootHasOperationType(lastOperation *gardencorev1beta1.LastOperation, lastOperationType gardencorev1beta1.LastOperationType) bool {
	return lastOperation != nil && lastOperation.Type == lastOperationType
}

// KubeAPIServerFeatureGateDisabled returns whether the given feature gate is explicitly disabled for the kube-apiserver for the given Shoot spec.
func KubeAPIServerFeatureGateDisabled(shoot *gardencorev1beta1.Shoot, featureGate string) bool {
	kubeAPIServer := shoot.Spec.Kubernetes.KubeAPIServer
	if kubeAPIServer == nil || kubeAPIServer.FeatureGates == nil {
		return false
	}

	value, ok := kubeAPIServer.FeatureGates[featureGate]
	if !ok {
		return false
	}
	return !value
}

// KubeControllerManagerFeatureGateDisabled returns whether the given feature gate is explicitly disabled for the kube-controller-manager for the given Shoot spec.
func KubeControllerManagerFeatureGateDisabled(shoot *gardencorev1beta1.Shoot, featureGate string) bool {
	kubeControllerManager := shoot.Spec.Kubernetes.KubeControllerManager
	if kubeControllerManager == nil || kubeControllerManager.FeatureGates == nil {
		return false
	}

	value, ok := kubeControllerManager.FeatureGates[featureGate]
	if !ok {
		return false
	}
	return !value
}

// KubeProxyFeatureGateDisabled returns whether the given feature gate is disabled for the kube-proxy for the given Shoot spec.
func KubeProxyFeatureGateDisabled(shoot *gardencorev1beta1.Shoot, featureGate string) bool {
	kubeProxy := shoot.Spec.Kubernetes.KubeProxy
	if kubeProxy == nil || kubeProxy.FeatureGates == nil {
		return false
	}

	value, ok := kubeProxy.FeatureGates[featureGate]
	if !ok {
		return false
	}
	return !value
}

// ConvertShootList converts a list of Shoots to a list of pointers to Shoots.
func ConvertShootList(list []gardencorev1beta1.Shoot) []*gardencorev1beta1.Shoot {
	var result []*gardencorev1beta1.Shoot
	for i := range list {
		result = append(result, &list[i])
	}
	return result
}

// HasManagedIssuer checks if the shoot has managed issuer enabled.
func HasManagedIssuer(shoot *gardencorev1beta1.Shoot) bool {
	return shoot.GetAnnotations()[v1beta1constants.AnnotationAuthenticationIssuer] == v1beta1constants.AnnotationAuthenticationIssuerManaged
}

// SumResourceReservations adds together the given *gardencorev1beta1.KubeletConfigReserved values.
// The func is suitable to calculate the sum of kubeReserved and systemReserved.
func SumResourceReservations(left, right *gardencorev1beta1.KubeletConfigReserved) *gardencorev1beta1.KubeletConfigReserved {
	if left == nil {
		return right
	} else if right == nil {
		return left
	}

	return &gardencorev1beta1.KubeletConfigReserved{
		CPU:              sumQuantities(left.CPU, right.CPU),
		Memory:           sumQuantities(left.Memory, right.Memory),
		PID:              sumQuantities(left.PID, right.PID),
		EphemeralStorage: sumQuantities(left.EphemeralStorage, right.EphemeralStorage),
	}
}

func sumQuantities(left, right *resource.Quantity) *resource.Quantity {
	if left == nil {
		return right
	} else if right == nil {
		return left
	}

	copy := left.DeepCopy()
	copy.Add(*right)
	return &copy
}
