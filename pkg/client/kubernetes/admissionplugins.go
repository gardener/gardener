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

package kubernetes

import (
	"fmt"

	"github.com/Masterminds/semver"
	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
)

var (
	defaultPlugins = []gardenv1beta1.AdmissionPlugin{
		{Name: "Priority"},
		{Name: "NamespaceLifecycle"},
		{Name: "LimitRanger"},
		{Name: "PodSecurityPolicy"},
		{Name: "ServiceAccount"},
		{Name: "NodeRestriction"},
		{Name: "DefaultStorageClass"},
		{Name: "DefaultTolerationSeconds"},
		{Name: "ResourceQuota"},
		{Name: "StorageObjectInUseProtection"},
		{Name: "MutatingAdmissionWebhook"},
		{Name: "ValidatingAdmissionWebhook"},
	}
	defaultPluginsWithInitializers = append(defaultPlugins, gardenv1beta1.AdmissionPlugin{Name: "Initializers"})

	lowestSupportedKubernetesVersionMajorMinor = "1.10"
	lowestSupportedKubernetesVersion, _        = semver.NewVersion(lowestSupportedKubernetesVersionMajorMinor)

	admissionPlugins = map[string][]gardenv1beta1.AdmissionPlugin{
		"1.10": defaultPluginsWithInitializers,
		"1.11": defaultPluginsWithInitializers,
		"1.12": defaultPluginsWithInitializers,
		"1.13": defaultPluginsWithInitializers,
		"1.14": defaultPlugins,
	}
)

// GetAdmissionPluginsForVersion returns the set of default admission plugins for the given Kubernetes version.
// If the given Kubernetes version does not explicitly define admission plugins the set of names for the next
// available version will be returned (e.g., for version X not defined the set of version X-1 will be returned).
func GetAdmissionPluginsForVersion(v string) []gardenv1beta1.AdmissionPlugin {
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

func formatMajorMinor(major, minor int64) string {
	return fmt.Sprintf("%d.%d", major, minor)
}
