// Copyright 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	seedvalidator "github.com/gardener/gardener/plugin/pkg/seed/validator"
	shootdns "github.com/gardener/gardener/plugin/pkg/shoot/dns"
	shootdnsrewriting "github.com/gardener/gardener/plugin/pkg/shoot/dnsrewriting"
	shootexposureclass "github.com/gardener/gardener/plugin/pkg/shoot/exposureclass"
	shootmanagedseed "github.com/gardener/gardener/plugin/pkg/shoot/managedseed"
	shootnodelocaldns "github.com/gardener/gardener/plugin/pkg/shoot/nodelocaldns"
	"github.com/gardener/gardener/plugin/pkg/shoot/oidc/clusteropenidconnectpreset"
	"github.com/gardener/gardener/plugin/pkg/shoot/oidc/openidconnectpreset"
	shootquotavalidator "github.com/gardener/gardener/plugin/pkg/shoot/quotavalidator"
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
}
