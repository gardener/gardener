// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	"slices"
)

// SetDefaults_ExtensionProfile sets default values for ExtensionProfile objects.
func SetDefaults_ExtensionProfile(extensionProfile *ExtensionProfile) {
	defaultExtensionUpdateStrategy(&extensionProfile.Spec.UpdateStrategy)

	for i := range extensionProfile.Spec.Components {
		component := &extensionProfile.Spec.Components[i]
		if component.UpdateStrategy != nil {
			defaultExtensionUpdateStrategy(component.UpdateStrategy)
		}
	}
}

// defaultExtensionUpdateStrategy defaults the strategy's Default to "patch" (the least-surprising
// automatic policy, per GEP-0078) and ensures Default is contained in Supported.
func defaultExtensionUpdateStrategy(updateStrategy *ExtensionUpdateStrategy) {
	if len(updateStrategy.Default) == 0 {
		updateStrategy.Default = UpdateStrategyPatch
	}
	if !slices.Contains(updateStrategy.Supported, updateStrategy.Default) {
		updateStrategy.Supported = append(updateStrategy.Supported, updateStrategy.Default)
	}
}
