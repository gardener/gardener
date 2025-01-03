// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubernetes

import (
	"fmt"

	"github.com/Masterminds/semver/v3"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

var (
	defaultPlugins = []gardencorev1beta1.AdmissionPlugin{
		{Name: "Priority"},
		{Name: "NamespaceLifecycle"},
		{Name: "LimitRanger"},
		{Name: "PodSecurity"},
		{Name: "ServiceAccount"},
		{Name: "NodeRestriction"},
		{Name: "DefaultStorageClass"},
		{Name: "DefaultTolerationSeconds"},
		{Name: "ResourceQuota"},
		{Name: "StorageObjectInUseProtection"},
		{Name: "MutatingAdmissionWebhook"},
		{Name: "ValidatingAdmissionWebhook"},
	}

	lowestSupportedKubernetesVersionMajorMinor = "1.27"
	lowestSupportedKubernetesVersion, _        = semver.NewVersion(lowestSupportedKubernetesVersionMajorMinor)

	admissionPlugins = map[string][]gardencorev1beta1.AdmissionPlugin{
		"1.27": defaultPlugins,
	}
)

// GetAdmissionPluginsForVersion returns the set of default admission plugins for the given Kubernetes version.
// If the given Kubernetes version does not explicitly define admission plugins the set of names for the next
// available version will be returned (e.g., for version X not defined the set of version X-1 will be returned).
func GetAdmissionPluginsForVersion(v string) []gardencorev1beta1.AdmissionPlugin {
	return copyPlugins(getAdmissionPluginsForVersionInternal(v))
}

func getAdmissionPluginsForVersionInternal(v string) []gardencorev1beta1.AdmissionPlugin {
	version, err := semver.NewVersion(v)
	if err != nil {
		return admissionPlugins[lowestSupportedKubernetesVersionMajorMinor]
	}

	if version.LessThan(lowestSupportedKubernetesVersion) {
		return admissionPlugins[lowestSupportedKubernetesVersionMajorMinor]
	}

	majorMinor := formatMajorMinor(version.Major(), version.Minor())
	if pluginsForVersion, ok := admissionPlugins[majorMinor]; ok {
		return pluginsForVersion
	}

	// We do not handle decrementing the major part of the version. The reason for this is that we would have to set
	// the minor part to some higher value which we don't know (assume we go from 2.2->2.1->2.0->1.?). We decided not
	// to handle decrementing the major part at all, as if Gardener supports Kubernetes 2.X (independent of the fact
	// that it's anyway unclear when/whether that will come) many parts have to be adapted anyway.
	return GetAdmissionPluginsForVersion(formatMajorMinor(version.Major(), version.Minor()-1))
}

func formatMajorMinor(major, minor uint64) string {
	return fmt.Sprintf("%d.%d", major, minor)
}

func copyPlugins(admissionPlugins []gardencorev1beta1.AdmissionPlugin) []gardencorev1beta1.AdmissionPlugin {
	dst := make([]gardencorev1beta1.AdmissionPlugin, 0)
	for _, plugin := range admissionPlugins {
		pluginPointer := &plugin
		dst = append(dst, *pluginPointer.DeepCopy())
	}
	return dst
}
