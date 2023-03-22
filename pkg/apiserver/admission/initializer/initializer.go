// Copyright 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package initializer

import (
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	quotav1 "k8s.io/apiserver/pkg/quota/v1"
	"k8s.io/client-go/dynamic"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"

	gardencoreclientset "github.com/gardener/gardener/pkg/client/core/clientset/internalversion"
	gardencoreversionedclientset "github.com/gardener/gardener/pkg/client/core/clientset/versioned"
	gardencoreexternalinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/internalversion"
	seedmanagementclientset "github.com/gardener/gardener/pkg/client/seedmanagement/clientset/versioned"
	seedmanagementinformers "github.com/gardener/gardener/pkg/client/seedmanagement/informers/externalversions"
	settingsinformers "github.com/gardener/gardener/pkg/client/settings/informers/externalversions"
)

// New constructs new instance of PluginInitializer
func New(
	coreInformers gardencoreinformers.SharedInformerFactory,
	coreClient gardencoreclientset.Interface,
	externalCoreInformers gardencoreexternalinformers.SharedInformerFactory,
	externalCoreClient gardencoreversionedclientset.Interface,
	seedManagementInformers seedmanagementinformers.SharedInformerFactory,
	seedManagementClient seedmanagementclientset.Interface,
	settingsInformers settingsinformers.SharedInformerFactory,
	kubeInformers kubeinformers.SharedInformerFactory,
	kubeClient kubernetes.Interface,
	dynamicClient dynamic.Interface,
	authz authorizer.Authorizer,
	quotaConfiguration quotav1.Configuration,
) admission.PluginInitializer {
	return pluginInitializer{
		coreInformers: coreInformers,
		coreClient:    coreClient,

		externalCoreInformers: externalCoreInformers,
		externalCoreClient:    externalCoreClient,

		seedManagementInformers: seedManagementInformers,
		seedManagementClient:    seedManagementClient,

		settingsInformers: settingsInformers,

		kubeInformers: kubeInformers,
		kubeClient:    kubeClient,

		dynamicClient: dynamicClient,

		authorizer: authz,

		quotaConfiguration: quotaConfiguration,
	}
}

// Initialize checks the initialization interfaces implemented by each plugin
// and provide the appropriate initialization data
func (i pluginInitializer) Initialize(plugin admission.Interface) {
	if wants, ok := plugin.(WantsInternalCoreInformerFactory); ok {
		wants.SetInternalCoreInformerFactory(i.coreInformers)
	}
	if wants, ok := plugin.(WantsInternalCoreClientset); ok {
		wants.SetInternalCoreClientset(i.coreClient)
	}

	if wants, ok := plugin.(WantsExternalCoreInformerFactory); ok {
		wants.SetExternalCoreInformerFactory(i.externalCoreInformers)
	}
	if wants, ok := plugin.(WantsExternalCoreClientset); ok {
		wants.SetExternalCoreClientset(i.externalCoreClient)
	}

	if wants, ok := plugin.(WantsSeedManagementInformerFactory); ok {
		wants.SetSeedManagementInformerFactory(i.seedManagementInformers)
	}
	if wants, ok := plugin.(WantsSeedManagementClientset); ok {
		wants.SetSeedManagementClientset(i.seedManagementClient)
	}

	if wants, ok := plugin.(WantsSettingsInformerFactory); ok {
		wants.SetSettingsInformerFactory(i.settingsInformers)
	}

	if wants, ok := plugin.(WantsKubeInformerFactory); ok {
		wants.SetKubeInformerFactory(i.kubeInformers)
	}
	if wants, ok := plugin.(WantsKubeClientset); ok {
		wants.SetKubeClientset(i.kubeClient)
	}

	if wants, ok := plugin.(WantsDynamicClient); ok {
		wants.SetDynamicClient(i.dynamicClient)
	}

	if wants, ok := plugin.(WantsAuthorizer); ok {
		wants.SetAuthorizer(i.authorizer)
	}

	if wants, ok := plugin.(WantsQuotaConfiguration); ok {
		wants.SetQuotaConfiguration(i.quotaConfiguration)
	}
}
