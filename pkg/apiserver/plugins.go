// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package apiserver

import (
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/admission/plugin/resourcequota"

	bastionvalidator "github.com/gardener/gardener/plugin/pkg/bastion/validator"
	controllerregistrationresources "github.com/gardener/gardener/plugin/pkg/controllerregistration/resources"
	"github.com/gardener/gardener/plugin/pkg/global/customverbauthorizer"
	"github.com/gardener/gardener/plugin/pkg/global/deletionconfirmation"
	"github.com/gardener/gardener/plugin/pkg/global/extensionlabels"
	"github.com/gardener/gardener/plugin/pkg/global/extensionvalidation"
	"github.com/gardener/gardener/plugin/pkg/global/resourcereferencemanager"
	managedseedshoot "github.com/gardener/gardener/plugin/pkg/managedseed/shoot"
	managedseedvalidator "github.com/gardener/gardener/plugin/pkg/managedseed/validator"
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
	shoottolerationrestriction "github.com/gardener/gardener/plugin/pkg/shoot/tolerationrestriction"
	shootvalidator "github.com/gardener/gardener/plugin/pkg/shoot/validator"
	shootvpa "github.com/gardener/gardener/plugin/pkg/shoot/vpa"
)

// RegisterAllAdmissionPlugins registers all admission plugins.
func RegisterAllAdmissionPlugins(plugins *admission.Plugins) {
	resourcereferencemanager.Register(plugins)
	deletionconfirmation.Register(plugins)
	extensionvalidation.Register(plugins)
	extensionlabels.Register(plugins)
	shoottolerationrestriction.Register(plugins)
	shootexposureclass.Register(plugins)
	shootquotavalidator.Register(plugins)
	shootdns.Register(plugins)
	shootmanagedseed.Register(plugins)
	shootnodelocaldns.Register(plugins)
	shootdnsrewriting.Register(plugins)
	shootvalidator.Register(plugins)
	seedvalidator.Register(plugins)
	seedmutator.Register(plugins)
	controllerregistrationresources.Register(plugins)
	namespacedcloudprofilevalidator.Register(plugins)
	projectvalidator.Register(plugins)
	openidconnectpreset.Register(plugins)
	clusteropenidconnectpreset.Register(plugins)
	customverbauthorizer.Register(plugins)
	managedseedvalidator.Register(plugins)
	managedseedshoot.Register(plugins)
	bastionvalidator.Register(plugins)
	resourcequota.Register(plugins)
	shootvpa.Register(plugins)
	shootresourcereservation.Register(plugins)
}
