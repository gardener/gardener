// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package pkg

import (
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/admission/plugin/namespace/lifecycle"
	"k8s.io/apiserver/pkg/admission/plugin/policy/validating"
	"k8s.io/apiserver/pkg/admission/plugin/resourcequota"
	mutatingwebhook "k8s.io/apiserver/pkg/admission/plugin/webhook/mutating"
	validatingwebhook "k8s.io/apiserver/pkg/admission/plugin/webhook/validating"
)

const (
	// PluginNameBastion is the name of the Bastion admission plugin.
	PluginNameBastion = "Bastion"
	// PluginNameControllerRegistrationResources is the name of the ControllerRegistrationResources admission plugin.
	PluginNameControllerRegistrationResources = "ControllerRegistrationResources"
	// PluginNameCustomVerbAuthorizer is the name of the CustomVerbAuthorizer admission plugin.
	PluginNameCustomVerbAuthorizer = "CustomVerbAuthorizer"
	// PluginNameDeletionConfirmation is the name of the DeletionConfirmation admission plugin.
	PluginNameDeletionConfirmation = "DeletionConfirmation"
	// PluginNameExtensionLabels is the name of the ExtensionLabels admission plugin.
	PluginNameExtensionLabels = "ExtensionLabels"
	// PluginNameExtensionValidator is the name of the ExtensionValidator admission plugin.
	PluginNameExtensionValidator = "ExtensionValidator"
	// PluginNameResourceReferenceManager is the name of the ResourceReferenceManager admission plugin.
	PluginNameResourceReferenceManager = "ResourceReferenceManager"
	// PluginNameManagedSeedShoot is the name of the ManagedSeedShoot admission plugin.
	PluginNameManagedSeedShoot = "ManagedSeedShoot"
	// PluginNameManagedSeed is the name of the ManagedSeed admission plugin.
	PluginNameManagedSeed = "ManagedSeed"
	// PluginNameNamespacedCloudProfileValidator is the name of the NamespacedCloudProfileValidator admission plugin.
	PluginNameNamespacedCloudProfileValidator = "NamespacedCloudProfileValidator"
	// PluginNameProjectValidator is the name of the ProjectValidator admission plugin.
	PluginNameProjectValidator = "ProjectValidator"
	// PluginNameSeedValidator is the name of the SeedValidator admission plugin.
	PluginNameSeedValidator = "SeedValidator"
	// PluginNameShootDNS is the name of the ShootDNS admission plugin.
	PluginNameShootDNS = "ShootDNS"
	// PluginNameShootDNSRewriting is the name of the ShootDNSRewriting admission plugin.
	PluginNameShootDNSRewriting = "ShootDNSRewriting"
	// PluginNameShootExposureClass is the name of the ShootExposureClass admission plugin.
	PluginNameShootExposureClass = "ShootExposureClass"
	// PluginNameShootManagedSeed is the name of the ShootManagedSeed admission plugin.
	PluginNameShootManagedSeed = "ShootManagedSeed"
	// PluginNameShootNodeLocalDNSEnabledByDefault is the name of the ShootNodeLocalDNSEnabledByDefault admission plugin.
	PluginNameShootNodeLocalDNSEnabledByDefault = "ShootNodeLocalDNSEnabledByDefault"
	// PluginNameClusterOpenIDConnectPreset is the name of the ClusterOpenIDConnectPreset admission plugin.
	PluginNameClusterOpenIDConnectPreset = "ClusterOpenIDConnectPreset"
	// PluginNameOpenIDConnectPreset is the name of the OpenIDConnectPreset admission plugin.
	PluginNameOpenIDConnectPreset = "OpenIDConnectPreset"
	// PluginNameShootQuotaValidator is the name of the ShootQuotaValidator admission plugin.
	PluginNameShootQuotaValidator = "ShootQuotaValidator"
	// PluginNameShootTolerationRestriction is the name of the ShootTolerationRestriction admission plugin.
	PluginNameShootTolerationRestriction = "ShootTolerationRestriction"
	// PluginNameShootValidator is the name of the ShootValidator admission plugin.
	PluginNameShootValidator = "ShootValidator"
	// PluginNameShootVPAEnabledByDefault is the name of the ShootVPAEnabledByDefault admission plugin.
	PluginNameShootVPAEnabledByDefault = "ShootVPAEnabledByDefault"
	// PluginNameShootResourceReservation is the name of the ShootResourceReservation admission plugin.
	PluginNameShootResourceReservation = "ShootResourceReservation"
)

// AllPluginNames returns the names of all plugins.
func AllPluginNames() []string {
	return []string{
		lifecycle.PluginName,                        // NamespaceLifecycle
		PluginNameResourceReferenceManager,          // ResourceReferenceManager
		PluginNameExtensionValidator,                // ExtensionValidator
		PluginNameExtensionLabels,                   // ExtensionLabels
		PluginNameShootTolerationRestriction,        // ShootTolerationRestriction
		PluginNameShootExposureClass,                // ShootExposureClass
		PluginNameShootDNS,                          // ShootDNS
		PluginNameShootManagedSeed,                  // ShootManagedSeed
		PluginNameShootNodeLocalDNSEnabledByDefault, // ShootNodeLocalDNSEnabledByDefault
		PluginNameShootDNSRewriting,                 // ShootDNSRewriting
		PluginNameShootQuotaValidator,               // ShootQuotaValidator
		PluginNameShootValidator,                    // ShootValidator
		PluginNameSeedValidator,                     // SeedValidator
		PluginNameControllerRegistrationResources,   // ControllerRegistrationResources
		PluginNameNamespacedCloudProfileValidator,   // NamespacedCloudProfileValidator
		PluginNameProjectValidator,                  // ProjectValidator
		PluginNameDeletionConfirmation,              // DeletionConfirmation
		PluginNameOpenIDConnectPreset,               // OpenIDConnectPreset
		PluginNameClusterOpenIDConnectPreset,        // ClusterOpenIDConnectPreset
		PluginNameCustomVerbAuthorizer,              // CustomVerbAuthorizer
		PluginNameShootVPAEnabledByDefault,          // ShootVPAEnabledByDefault
		PluginNameShootResourceReservation,          // ShootResourceReservation
		PluginNameManagedSeed,                       // ManagedSeed
		PluginNameManagedSeedShoot,                  // ManagedSeedShoot
		PluginNameBastion,                           // Bastion

		// new admission plugins should generally be inserted above here
		// webhook, and resourcequota plugins must go at the end

		mutatingwebhook.PluginName,   // MutatingAdmissionWebhook
		validating.PluginName,        // ValidatingAdmissionPolicy
		validatingwebhook.PluginName, // ValidatingAdmissionWebhook

		// This plugin must remain the last one in the list since it updates the quota usage
		// which can only happen reliably if previous plugins permitted the request.
		resourcequota.PluginName, // ResourceQuota
	}
}

// DefaultOnPlugins is the set of admission plugins that are enabled by default.
func DefaultOnPlugins() sets.Set[string] {
	return sets.New[string](
		lifecycle.PluginName,                      // NamespaceLifecycle
		PluginNameResourceReferenceManager,        // ResourceReferenceManager
		PluginNameExtensionValidator,              // ExtensionValidator
		PluginNameExtensionLabels,                 // ExtensionLabels
		PluginNameShootTolerationRestriction,      // ShootTolerationRestriction
		PluginNameShootExposureClass,              // ShootExposureClass
		PluginNameShootDNS,                        // ShootDNS
		PluginNameShootManagedSeed,                // ShootManagedSeed
		PluginNameShootResourceReservation,        // ShootResourceReservation
		PluginNameShootQuotaValidator,             // ShootQuotaValidator
		PluginNameShootValidator,                  // ShootValidator
		PluginNameSeedValidator,                   // SeedValidator
		PluginNameControllerRegistrationResources, // ControllerRegistrationResources
		PluginNameNamespacedCloudProfileValidator, // NamespacedCloudProfileValidator
		PluginNameProjectValidator,                // ProjectValidator
		PluginNameDeletionConfirmation,            // DeletionConfirmation
		PluginNameOpenIDConnectPreset,             // OpenIDConnectPreset
		PluginNameClusterOpenIDConnectPreset,      // ClusterOpenIDConnectPreset
		PluginNameCustomVerbAuthorizer,            // CustomVerbAuthorizer
		PluginNameManagedSeed,                     // ManagedSeed
		PluginNameManagedSeedShoot,                // ManagedSeedShoot
		PluginNameBastion,                         // Bastion
		mutatingwebhook.PluginName,                // MutatingAdmissionWebhook
		validatingwebhook.PluginName,              // ValidatingAdmissionWebhook
		validating.PluginName,                     // ValidatingAdmissionPolicy
		resourcequota.PluginName,                  // ResourceQuota
	)
}
