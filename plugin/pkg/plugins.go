// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package pkg

import (
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/admission/plugin/namespace/lifecycle"
	"k8s.io/apiserver/pkg/admission/plugin/policy/mutating"
	"k8s.io/apiserver/pkg/admission/plugin/policy/validating"
	"k8s.io/apiserver/pkg/admission/plugin/resourcequota"
	mutatingwebhook "k8s.io/apiserver/pkg/admission/plugin/webhook/mutating"
	validatingwebhook "k8s.io/apiserver/pkg/admission/plugin/webhook/validating"

	backupbucketvalidator "github.com/gardener/gardener/plugin/pkg/backupbucket/validator"
	bastion "github.com/gardener/gardener/plugin/pkg/bastion/validator"
	"github.com/gardener/gardener/plugin/pkg/controllerregistration/resources"
	controllerregistrationresources "github.com/gardener/gardener/plugin/pkg/controllerregistration/resources"
	"github.com/gardener/gardener/plugin/pkg/global/customverbauthorizer"
	"github.com/gardener/gardener/plugin/pkg/global/deletionconfirmation"
	"github.com/gardener/gardener/plugin/pkg/global/extensionlabels"
	"github.com/gardener/gardener/plugin/pkg/global/extensionvalidation"
	"github.com/gardener/gardener/plugin/pkg/global/finalizerremoval"
	"github.com/gardener/gardener/plugin/pkg/global/resourcereferencemanager"
	managedseedshoot "github.com/gardener/gardener/plugin/pkg/managedseed/shoot"
	managedseed "github.com/gardener/gardener/plugin/pkg/managedseed/validator"
	namespacedcloudprofilevalidator "github.com/gardener/gardener/plugin/pkg/namespacedcloudprofile/validator"
	projectvalidator "github.com/gardener/gardener/plugin/pkg/project/validator"
	seedmutator "github.com/gardener/gardener/plugin/pkg/seed/mutator"
	seedvalidator "github.com/gardener/gardener/plugin/pkg/seed/validator"
	shootdns "github.com/gardener/gardener/plugin/pkg/shoot/dns"
	shootdnsrewriting "github.com/gardener/gardener/plugin/pkg/shoot/dnsrewriting"
	shootexposureclass "github.com/gardener/gardener/plugin/pkg/shoot/exposureclass"
	shootmanagedseed "github.com/gardener/gardener/plugin/pkg/shoot/managedseed"
	shootnodelocaldns "github.com/gardener/gardener/plugin/pkg/shoot/nodelocaldns"
	"github.com/gardener/gardener/plugin/pkg/shoot/oidc/clusteropenidconnectpreset"
	"github.com/gardener/gardener/plugin/pkg/shoot/oidc/openidconnectpreset"
	shootquotavalidator "github.com/gardener/gardener/plugin/pkg/shoot/quotavalidator"
	shootresourcereservation "github.com/gardener/gardener/plugin/pkg/shoot/resourcereservation"
	"github.com/gardener/gardener/plugin/pkg/shoot/tolerationrestriction"
	shootvalidator "github.com/gardener/gardener/plugin/pkg/shoot/validator"
	shootvpa "github.com/gardener/gardener/plugin/pkg/shoot/vpa"
)

// AllPluginNames returns the names of all plugins.
func AllPluginNames() []string {
	return []string{
		lifecycle.PluginName,                       // NamespaceLifecycle
		resourcereferencemanager.PluginName,        // ResourceReferenceManager
		extensionvalidation.PluginName,             // ExtensionValidator
		extensionlabels.PluginName,                 // ExtensionLabels
		tolerationrestriction.PluginName,           // ShootTolerationRestriction
		shootexposureclass.PluginName,              // ShootExposureClass
		shootdns.PluginName,                        // ShootDNS
		shootmanagedseed.PluginName,                // ShootManagedSeed
		shootnodelocaldns.PluginName,               // ShootNodeLocalDNSEnabledByDefault
		shootdnsrewriting.PluginName,               // ShootDNSRewriting
		shootquotavalidator.PluginName,             // ShootQuotaValidator
		shootvalidator.PluginName,                  // ShootValidator
		seedvalidator.PluginName,                   // SeedValidator
		seedmutator.PluginName,                     // SeedMutator
		resources.PluginName,                       // ControllerRegistrationResources
		namespacedcloudprofilevalidator.PluginName, // NamespacedCloudProfileValidator
		projectvalidator.PluginName,                // ProjectValidator
		deletionconfirmation.PluginName,            // DeletionConfirmation
		finalizerremoval.PluginName,                // FinalizerRemoval
		openidconnectpreset.PluginName,             // OpenIDConnectPreset
		clusteropenidconnectpreset.PluginName,      // ClusterOpenIDConnectPreset
		customverbauthorizer.PluginName,            // CustomVerbAuthorizer
		shootvpa.PluginName,                        // ShootVPAEnabledByDefault
		shootresourcereservation.PluginName,        // ShootResourceReservation
		managedseed.PluginName,                     // ManagedSeed
		managedseedshoot.PluginName,                // ManagedSeedShoot
		bastion.PluginName,                         // Bastion
		backupbucketvalidator.PluginName,           // BackupBucketValidator

		// new admission plugins should generally be inserted above here
		// webhook, and resourcequota plugins must go at the end

		mutating.PluginName,          // MutatingAdmissionPolicy
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
		lifecycle.PluginName,                       // NamespaceLifecycle
		resourcereferencemanager.PluginName,        // ResourceReferenceManager
		extensionvalidation.PluginName,             // ExtensionValidator
		extensionlabels.PluginName,                 // ExtensionLabels
		tolerationrestriction.PluginName,           // ShootTolerationRestriction
		shootexposureclass.PluginName,              // ShootExposureClass
		shootdns.PluginName,                        // ShootDNS
		shootmanagedseed.PluginName,                // ShootManagedSeed
		shootresourcereservation.PluginName,        // ShootResourceReservation
		shootquotavalidator.PluginName,             // ShootQuotaValidator
		shootvalidator.PluginName,                  // ShootValidator
		seedvalidator.PluginName,                   // SeedValidator
		seedmutator.PluginName,                     // SeedMutator
		controllerregistrationresources.PluginName, // ControllerRegistrationResources
		namespacedcloudprofilevalidator.PluginName, // NamespacedCloudProfileValidator
		projectvalidator.PluginName,                // ProjectValidator
		deletionconfirmation.PluginName,            // DeletionConfirmation
		finalizerremoval.PluginName,                // FinalizerRemoval
		openidconnectpreset.PluginName,             // OpenIDConnectPreset
		clusteropenidconnectpreset.PluginName,      // ClusterOpenIDConnectPreset
		customverbauthorizer.PluginName,            // CustomVerbAuthorizer
		managedseed.PluginName,                     // ManagedSeed
		managedseedshoot.PluginName,                // ManagedSeedShoot
		bastion.PluginName,                         // Bastion
		backupbucketvalidator.PluginName,           // BackupBucketValidator
		mutatingwebhook.PluginName,                 // MutatingAdmissionWebhook
		validatingwebhook.PluginName,               // ValidatingAdmissionWebhook
		// TODO(ary1992): Ennable the plugin once our base clusters are updated to k8s >= 1.30
		// validating.PluginName,                     // ValidatingAdmissionPolicy
		resourcequota.PluginName, // ResourceQuota
	)
}
