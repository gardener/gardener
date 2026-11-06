// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	"slices"
)

// SetDefaults_ExtensionProfile sets default values for ExtensionProfile objects.
func SetDefaults_ExtensionProfile(extensionProfile *ExtensionProfile) {
	defaultExtensionUpdateStrategy(&extensionProfile.Spec.UpdateStrategy, nil)

	for i := range extensionProfile.Spec.Components {
		component := &extensionProfile.Spec.Components[i]
		if component.UpdateStrategy != nil {
			// A component without its own default inherits the profile-level default, so pass the
			// already-defaulted profile default as the fallback instead of defaulting to patch again.
			defaultExtensionUpdateStrategy(component.UpdateStrategy, extensionProfile.Spec.UpdateStrategy.Default)
		}
	}
}

// defaultExtensionUpdateStrategy defaults the strategy's Default and ensures it is contained in Supported.
// When Default is unset it falls back to inheritedDefault (the profile-level default for a component), or to
// "patch" when there is nothing to inherit.
func defaultExtensionUpdateStrategy(updateStrategy *ExtensionUpdateStrategy, inheritedDefault *UpdateStrategy) {
	if updateStrategy.Default == nil {
		if inheritedDefault != nil {
			updateStrategy.Default = new(*inheritedDefault)
		} else {
			updateStrategy.Default = new(UpdateStrategyPatch)
		}
	}
	if !slices.Contains(updateStrategy.Supported, *updateStrategy.Default) {
		updateStrategy.Supported = append(updateStrategy.Supported, *updateStrategy.Default)
	}
}
