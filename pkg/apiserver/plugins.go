// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package apiserver

import (
	bastionvalidator "github.com/gardener/gardener/plugin/pkg/bastion/validator"
	controllerregistrationresources "github.com/gardener/gardener/plugin/pkg/controllerregistration/resources"
	"github.com/gardener/gardener/plugin/pkg/global/customverbauthorizer"
	"github.com/gardener/gardener/plugin/pkg/global/deletionconfirmation"
	"github.com/gardener/gardener/plugin/pkg/global/extensionvalidation"
	"github.com/gardener/gardener/plugin/pkg/global/resourcereferencemanager"
	managedseedvalidator "github.com/gardener/gardener/plugin/pkg/managedseed/validator"
	plantvalidator "github.com/gardener/gardener/plugin/pkg/plant"
	seedvalidator "github.com/gardener/gardener/plugin/pkg/seed/validator"
	shootdns "github.com/gardener/gardener/plugin/pkg/shoot/dns"
	shootmanagedseed "github.com/gardener/gardener/plugin/pkg/shoot/managedseed"
	"github.com/gardener/gardener/plugin/pkg/shoot/oidc/clusteropenidconnectpreset"
	"github.com/gardener/gardener/plugin/pkg/shoot/oidc/openidconnectpreset"
	shootquotavalidator "github.com/gardener/gardener/plugin/pkg/shoot/quotavalidator"
	shoottolerationrestriction "github.com/gardener/gardener/plugin/pkg/shoot/tolerationrestriction"
	shootvalidator "github.com/gardener/gardener/plugin/pkg/shoot/validator"
	shootvpa "github.com/gardener/gardener/plugin/pkg/shoot/vpa"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/admission/plugin/namespace/lifecycle"
	"k8s.io/apiserver/pkg/admission/plugin/resourcequota"
	mutatingwebhook "k8s.io/apiserver/pkg/admission/plugin/webhook/mutating"
	validatingwebhook "k8s.io/apiserver/pkg/admission/plugin/webhook/validating"
)

var (
	// AllOrderedPlugins is the list of all the plugins in order.
	AllOrderedPlugins = []string{
		lifecycle.PluginName,                       // NamespaceLifecycle
		resourcereferencemanager.PluginName,        // ResourceReferenceManager
		extensionvalidation.PluginName,             // ExtensionValidator
		shoottolerationrestriction.PluginName,      // ShootTolerationRestriction
		shootdns.PluginName,                        // ShootDNS
		shootmanagedseed.PluginName,                // ShootManagedSeed
		shootquotavalidator.PluginName,             // ShootQuotaValidator
		shootvalidator.PluginName,                  // ShootValidator
		seedvalidator.PluginName,                   // SeedValidator
		controllerregistrationresources.PluginName, // ControllerRegistrationResources
		plantvalidator.PluginName,                  // PlantValidator
		deletionconfirmation.PluginName,            // DeletionConfirmation
		openidconnectpreset.PluginName,             // OpenIDConnectPreset
		clusteropenidconnectpreset.PluginName,      // ClusterOpenIDConnectPreset
		customverbauthorizer.PluginName,            // CustomVerbAuthorizer
		shootvpa.PluginName,                        // ShootVPAEnabledByDefault
		managedseedvalidator.PluginName,            // ManagedSeed
		bastionvalidator.PluginName,                // Bastion

		// new admission plugins should generally be inserted above here
		// webhook, and resourcequota plugins must go at the end

		mutatingwebhook.PluginName,   // MutatingAdmissionWebhook
		validatingwebhook.PluginName, // ValidatingAdmissionWebhook

		// This plugin must remain the last one in the list since it updates the quota usage
		// which can only happen reliably if previous plugins permitted the request.
		resourcequota.PluginName, // ResourceQuota
	}

	// DefaultOnPlugins is the set of admission plugins that are enabled by default.
	DefaultOnPlugins = sets.NewString(
		lifecycle.PluginName,                       // NamespaceLifecycle
		resourcereferencemanager.PluginName,        // ResourceReferenceManager
		extensionvalidation.PluginName,             // ExtensionValidator
		shoottolerationrestriction.PluginName,      // ShootTolerationRestriction
		shootdns.PluginName,                        // ShootDNS
		shootmanagedseed.PluginName,                // ShootManagedSeed
		shootquotavalidator.PluginName,             // ShootQuotaValidator
		shootvalidator.PluginName,                  // ShootValidator
		seedvalidator.PluginName,                   // SeedValidator
		controllerregistrationresources.PluginName, // ControllerRegistrationResources
		plantvalidator.PluginName,                  // PlantValidator
		deletionconfirmation.PluginName,            // DeletionConfirmation
		openidconnectpreset.PluginName,             // OpenIDConnectPreset
		clusteropenidconnectpreset.PluginName,      // ClusterOpenIDConnectPreset
		customverbauthorizer.PluginName,            // CustomVerbAuthorizer
		managedseedvalidator.PluginName,            // ManagedSeed
		bastionvalidator.PluginName,                // Bastion
		mutatingwebhook.PluginName,                 // MutatingAdmissionWebhook
		validatingwebhook.PluginName,               // ValidatingAdmissionWebhook
		resourcequota.PluginName,                   // ResourceQuota
	)

	// DefaultOffPlugins is the set of admission plugins that are disabled by default.
	DefaultOffPlugins = sets.NewString(AllOrderedPlugins...).Difference(DefaultOnPlugins)
)

// RegisterAllAdmissionPlugins registers all admission plugins.
func RegisterAllAdmissionPlugins(plugins *admission.Plugins) {
	resourcereferencemanager.Register(plugins)
	deletionconfirmation.Register(plugins)
	extensionvalidation.Register(plugins)
	shoottolerationrestriction.Register(plugins)
	shootquotavalidator.Register(plugins)
	shootdns.Register(plugins)
	shootmanagedseed.Register(plugins)
	shootvalidator.Register(plugins)
	seedvalidator.Register(plugins)
	controllerregistrationresources.Register(plugins)
	plantvalidator.Register(plugins)
	openidconnectpreset.Register(plugins)
	clusteropenidconnectpreset.Register(plugins)
	customverbauthorizer.Register(plugins)
	managedseedvalidator.Register(plugins)
	bastionvalidator.Register(plugins)
	resourcequota.Register(plugins)
	shootvpa.Register(plugins)
}
