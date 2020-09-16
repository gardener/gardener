// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package initializer

import (
	coreclientset "github.com/gardener/gardener/pkg/client/core/clientset/internalversion"
	externalcoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	coreinformers "github.com/gardener/gardener/pkg/client/core/informers/internalversion"
	settingsinformer "github.com/gardener/gardener/pkg/client/settings/informers/externalversions"

	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/client-go/dynamic"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
)

// New constructs new instance of PluginInitializer
func New(
	coreInformers coreinformers.SharedInformerFactory,
	coreClient coreclientset.Interface,
	externalCoreInformers externalcoreinformers.SharedInformerFactory,
	settingsInformers settingsinformer.SharedInformerFactory,
	kubeInformers kubeinformers.SharedInformerFactory,
	kubeClient kubernetes.Interface,
	dynamicClient dynamic.Interface,
	authz authorizer.Authorizer,
) admission.PluginInitializer {
	return pluginInitializer{
		coreInformers: coreInformers,
		coreClient:    coreClient,

		externalCoreInformers: externalCoreInformers,

		settingsInformers: settingsInformers,

		kubeInformers: kubeInformers,
		kubeClient:    kubeClient,

		dynamicClient: dynamicClient,

		authorizer: authz,
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
}
