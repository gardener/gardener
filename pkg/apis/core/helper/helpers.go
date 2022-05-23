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
	"strings"

	"github.com/Masterminds/semver"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/pointer"

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
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

// DetermineLatestMachineImageVersions determines the latest versions (semVer) of the given machine images from a slice of machine images
func DetermineLatestMachineImageVersions(images []core.MachineImage) (map[string]core.MachineImageVersion, error) {
	resultMapVersions := make(map[string]core.MachineImageVersion)

	for _, image := range images {
		latestMachineImageVersion, err := DetermineLatestMachineImageVersion(image.Versions, false)
		if err != nil {
			return nil, fmt.Errorf("failed to determine latest machine image version for image '%s': %w", image.Name, err)
		}
		resultMapVersions[image.Name] = latestMachineImageVersion
	}
	return resultMapVersions, nil
}

// DetermineLatestMachineImageVersion determines the latest MachineImageVersion from a slice of MachineImageVersion.
// When filterPreviewVersions is set, versions with classification preview are not considered.
// It will prefer older but non-deprecated versions over newer but deprecated versions.
func DetermineLatestMachineImageVersion(versions []core.MachineImageVersion, filterPreviewVersions bool) (core.MachineImageVersion, error) {
	latestVersion, latestNonDeprecatedVersion, err := DetermineLatestExpirableVersion(ToExpirableVersions(versions), filterPreviewVersions)
	if err != nil {
		return core.MachineImageVersion{}, err
	}

	// Try to find non-deprecated version first.
	for _, version := range versions {
		if version.Version == latestNonDeprecatedVersion.Version {
			return version, nil
		}
	}

	// It looks like there is no non-deprecated version, now look also into the deprecated versions
	for _, version := range versions {
		if version.Version == latestVersion.Version {
			return version, nil
		}
	}

	return core.MachineImageVersion{}, fmt.Errorf("the latest machine version has been removed")
}

// DetermineLatestExpirableVersion determines the latest expirable version and the latest non-deprecated version from a slice of ExpirableVersions.
// When filterPreviewVersions is set, versions with classification preview are not considered.
func DetermineLatestExpirableVersion(versions []core.ExpirableVersion, filterPreviewVersions bool) (core.ExpirableVersion, core.ExpirableVersion, error) {

	var (
		latestSemVerVersion              *semver.Version
		latestNonDeprecatedSemVerVersion *semver.Version

		latestExpirableVersion              core.ExpirableVersion
		latestNonDeprecatedExpirableVersion core.ExpirableVersion
	)

	for _, version := range versions {
		v, err := semver.NewVersion(version.Version)
		if err != nil {
			return core.ExpirableVersion{}, core.ExpirableVersion{}, fmt.Errorf("error while parsing expirable version '%s': %s", version.Version, err.Error())
		}

		if filterPreviewVersions && version.Classification != nil && *version.Classification == core.ClassificationPreview {
			continue
		}

		if latestSemVerVersion == nil || v.GreaterThan(latestSemVerVersion) {
			latestSemVerVersion = v
			latestExpirableVersion = version
		}

		if version.Classification != nil && *version.Classification != core.ClassificationDeprecated {
			if latestNonDeprecatedSemVerVersion == nil || v.GreaterThan(latestNonDeprecatedSemVerVersion) {
				latestNonDeprecatedSemVerVersion = v
				latestNonDeprecatedExpirableVersion = version
			}
		}
	}

	if latestSemVerVersion == nil {
		return core.ExpirableVersion{}, core.ExpirableVersion{}, fmt.Errorf("unable to determine latest expirable version")
	}

	return latestExpirableVersion, latestNonDeprecatedExpirableVersion, nil
}

// ToExpirableVersions converts MachineImageVersion to ExpirableVersion
func ToExpirableVersions(versions []core.MachineImageVersion) []core.ExpirableVersion {
	expirableVersions := []core.ExpirableVersion{}
	for _, version := range versions {
		expirableVersions = append(expirableVersions, version.ExpirableVersion)
	}
	return expirableVersions
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

// TaintsAreTolerated returns true when all the given taints are tolerated by the given tolerations.
func TaintsAreTolerated(taints []core.SeedTaint, tolerations []core.Toleration) bool {
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

// SeedSettingExcessCapacityReservationEnabled returns true if the 'excess capacity reservation' setting is enabled.
func SeedSettingExcessCapacityReservationEnabled(settings *core.SeedSettings) bool {
	return settings == nil || settings.ExcessCapacityReservation == nil || settings.ExcessCapacityReservation.Enabled
}

// SeedSettingSchedulingVisible returns true if the 'scheduling' setting is set to 'visible'.
func SeedSettingSchedulingVisible(settings *core.SeedSettings) bool {
	return settings == nil || settings.Scheduling == nil || settings.Scheduling.Visible
}

// SeedSettingShootDNSEnabled returns true if the 'shoot dns' setting is enabled.
func SeedSettingShootDNSEnabled(settings *core.SeedSettings) bool {
	return settings == nil || settings.ShootDNS == nil || settings.ShootDNS.Enabled
}

// SeedSettingOwnerChecksEnabled returns true if the 'ownerChecks' setting is enabled.
func SeedSettingOwnerChecksEnabled(settings *core.SeedSettings) bool {
	return settings == nil || settings.OwnerChecks == nil || settings.OwnerChecks.Enabled
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

// GetShootAuditPolicyConfigMapName returns the Shoot's ConfigMap reference name for the audit policy.
func GetShootAuditPolicyConfigMapName(apiServerConfig *core.KubeAPIServerConfig) string {
	if ref := GetShootAuditPolicyConfigMapRef(apiServerConfig); ref != nil {
		return ref.Name
	}
	return ""
}

// GetShootAuditPolicyConfigMapRef returns the Shoot's ConfigMap reference for the audit policy.
func GetShootAuditPolicyConfigMapRef(apiServerConfig *core.KubeAPIServerConfig) *corev1.ObjectReference {
	if apiServerConfig != nil &&
		apiServerConfig.AuditConfig != nil &&
		apiServerConfig.AuditConfig.AuditPolicy != nil {
		return apiServerConfig.AuditConfig.AuditPolicy.ConfigMapRef
	}
	return nil
}

// HibernationIsEnabled checks if the given shoot's desired state is hibernated.
func HibernationIsEnabled(shoot *core.Shoot) bool {
	return shoot.Spec.Hibernation != nil && shoot.Spec.Hibernation.Enabled != nil && *shoot.Spec.Hibernation.Enabled
}

// SystemComponentsAllowed checks if the given worker allows system components to be scheduled onto it
func SystemComponentsAllowed(worker *core.Worker) bool {
	return worker.SystemComponents == nil || worker.SystemComponents.Allow
}

// KubernetesDashboardEnabled returns true if the kubernetes-dashboard addon is enabled in the Shoot manifest.
func KubernetesDashboardEnabled(addons *core.Addons) bool {
	return addons != nil && addons.KubernetesDashboard != nil && addons.KubernetesDashboard.Enabled
}

// NginxIngressEnabled returns true if the nginx-ingress addon is enabled in the Shoot manifest.
func NginxIngressEnabled(addons *core.Addons) bool {
	return addons != nil && addons.NginxIngress != nil && addons.NginxIngress.Enabled
}

// ShootWantsVerticalPodAutoscaler checks if the given Shoot needs a VPA.
func ShootWantsVerticalPodAutoscaler(shoot *core.Shoot) bool {
	return shoot.Spec.Kubernetes.VerticalPodAutoscaler != nil && shoot.Spec.Kubernetes.VerticalPodAutoscaler.Enabled
}

// GetShootCARotationPhase returns the specified shoot CA rotation phase or an empty string
func GetShootCARotationPhase(credentials *core.ShootCredentials) core.ShootCredentialsRotationPhase {
	if credentials != nil && credentials.Rotation != nil && credentials.Rotation.CertificateAuthorities != nil {
		return credentials.Rotation.CertificateAuthorities.Phase
	}
	return ""
}

// GetShootServiceAccountKeyRotationPhase returns the specified shoot service account key rotation phase or an empty
// string.
func GetShootServiceAccountKeyRotationPhase(credentials *core.ShootCredentials) core.ShootCredentialsRotationPhase {
	if credentials != nil && credentials.Rotation != nil && credentials.Rotation.ServiceAccountKey != nil {
		return credentials.Rotation.ServiceAccountKey.Phase
	}
	return ""
}

// GetShootETCDEncryptionKeyRotationPhase returns the specified shoot ETCD encryption key rotation phase or an empty
// string.
func GetShootETCDEncryptionKeyRotationPhase(credentials *core.ShootCredentials) core.ShootCredentialsRotationPhase {
	if credentials != nil && credentials.Rotation != nil && credentials.Rotation.ETCDEncryptionKey != nil {
		return credentials.Rotation.ETCDEncryptionKey.Phase
	}
	return ""
}

var scheme *runtime.Scheme

func init() {
	scheme = runtime.NewScheme()
	utilruntime.Must(core.AddToScheme(scheme))
	utilruntime.Must(gardencorev1beta1.AddToScheme(scheme))
}

// ConvertSeed converts the given external Seed version to an internal version.
func ConvertSeed(obj runtime.Object) (*core.Seed, error) {
	obj, err := scheme.ConvertToVersion(obj, core.SchemeGroupVersion)
	if err != nil {
		return nil, err
	}
	result, ok := obj.(*core.Seed)
	if !ok {
		return nil, fmt.Errorf("could not convert Seed to internal version")
	}
	return result, nil
}

// ConvertSeedExternal converts the given internal Seed version to an external version.
func ConvertSeedExternal(obj runtime.Object) (*gardencorev1beta1.Seed, error) {
	obj, err := scheme.ConvertToVersion(obj, gardencorev1beta1.SchemeGroupVersion)
	if err != nil {
		return nil, err
	}
	result, ok := obj.(*gardencorev1beta1.Seed)
	if !ok {
		return nil, fmt.Errorf("could not convert Seed to version %s", gardencorev1beta1.SchemeGroupVersion.String())
	}
	return result, nil
}

// CalculateSeedUsage returns a map representing the number of shoots per seed from the given list of shoots.
// It takes both spec.seedName and status.seedName into account.
func CalculateSeedUsage(shootList []*core.Shoot) map[string]int {
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

// GetSecretBindingTypes returns the SecretBinding provider types.
func GetSecretBindingTypes(secretBinding *core.SecretBinding) []string {
	return strings.Split(secretBinding.Provider.Type, ",")
}

// SecretBindingHasType checks if the given SecretBinding has the given provider type.
func SecretBindingHasType(secretBinding *core.SecretBinding, providerType string) bool {
	if secretBinding.Provider == nil {
		return false
	}

	types := GetSecretBindingTypes(secretBinding)
	if len(types) == 0 {
		return false
	}

	return sets.NewString(types...).Has(providerType)
}
