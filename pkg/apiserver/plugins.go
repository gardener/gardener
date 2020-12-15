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
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/admission/plugin/namespace/lifecycle"
	mutatingwebhook "k8s.io/apiserver/pkg/admission/plugin/webhook/mutating"
	validatingwebhook "k8s.io/apiserver/pkg/admission/plugin/webhook/validating"

	controllerregistrationresources "github.com/gardener/gardener/plugin/pkg/controllerregistration/resources"
	"github.com/gardener/gardener/plugin/pkg/global/customverbauthorizer"
	"github.com/gardener/gardener/plugin/pkg/global/deletionconfirmation"
	"github.com/gardener/gardener/plugin/pkg/global/extensionvalidation"
	"github.com/gardener/gardener/plugin/pkg/global/resourcereferencemanager"
	plantvalidator "github.com/gardener/gardener/plugin/pkg/plant"
	seedvalidator "github.com/gardener/gardener/plugin/pkg/seed/validator"
	shootdns "github.com/gardener/gardener/plugin/pkg/shoot/dns"
	"github.com/gardener/gardener/plugin/pkg/shoot/oidc/clusteropenidconnectpreset"
	"github.com/gardener/gardener/plugin/pkg/shoot/oidc/openidconnectpreset"
	shootquotavalidator "github.com/gardener/gardener/plugin/pkg/shoot/quotavalidator"
	shoottolerationrestriction "github.com/gardener/gardener/plugin/pkg/shoot/tolerationrestriction"
	shootvalidator "github.com/gardener/gardener/plugin/pkg/shoot/validator"
	shootstatedeletionvalidator "github.com/gardener/gardener/plugin/pkg/shootstate/validator"
	"github.com/gardener/gardener/third_party/forked/kubernetes/plugin/pkg/admission/resourcequota"
)

// AllOrderedPlugins is the list of all the plugins in order.
var AllOrderedPlugins = []string{
	lifecycle.PluginName, // NamespaceLifecycle
	resourcereferencemanager.PluginName,
	extensionvalidation.PluginName,
	shoottolerationrestriction.PluginName,
	shootdns.PluginName,
	shootquotavalidator.PluginName,
	shootvalidator.PluginName,
	seedvalidator.PluginName,
	controllerregistrationresources.PluginName,
	plantvalidator.PluginName,
	deletionconfirmation.PluginName,
	openidconnectpreset.PluginName,
	clusteropenidconnectpreset.PluginName,
	shootstatedeletionvalidator.PluginName,
	customverbauthorizer.PluginName,

	// new admission plugins should generally be inserted above here
	// webhook, and resourcequota plugins must go at the end

	mutatingwebhook.PluginName,   // MutatingAdmissionWebhook
	validatingwebhook.PluginName, // ValidatingAdmissionWebhook

	// This plugin must remain the last one in the list since it updates the quota usage
	// which can only happen reliably if previous plugins permitted the request.
	resourcequota.PluginName, // ResourceQuota
}

// RegisterAllAdmissionPlugins registers all admission plugins.
func RegisterAllAdmissionPlugins(plugins *admission.Plugins) {
	resourcereferencemanager.Register(plugins)
	deletionconfirmation.Register(plugins)
	extensionvalidation.Register(plugins)
	shoottolerationrestriction.Register(plugins)
	shootquotavalidator.Register(plugins)
	shootdns.Register(plugins)
	shootvalidator.Register(plugins)
	seedvalidator.Register(plugins)
	controllerregistrationresources.Register(plugins)
	plantvalidator.Register(plugins)
	openidconnectpreset.Register(plugins)
	clusteropenidconnectpreset.Register(plugins)
	shootstatedeletionvalidator.Register(plugins)
	customverbauthorizer.Register(plugins)
	resourcequota.Register(plugins)
}
