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

// WantsInternalCoreInformerFactory defines a function which sets InformerFactory for admission plugins that need it.
type WantsInternalCoreInformerFactory interface {
	SetInternalCoreInformerFactory(coreinformers.SharedInformerFactory)
	admission.InitializationValidator
}

// WantsInternalCoreClientset defines a function which sets Core Clientset for admission plugins that need it.
type WantsInternalCoreClientset interface {
	SetInternalCoreClientset(coreclientset.Interface)
	admission.InitializationValidator
}

// WantsExternalCoreInformerFactory defines a function which sets external Core InformerFactory for admission plugins that need it.
type WantsExternalCoreInformerFactory interface {
	SetExternalCoreInformerFactory(externalcoreinformers.SharedInformerFactory)
	admission.InitializationValidator
}

// WantsKubeInformerFactory defines a function which sets InformerFactory for admission plugins that need it.
type WantsKubeInformerFactory interface {
	SetKubeInformerFactory(kubeinformers.SharedInformerFactory)
	admission.InitializationValidator
}

// WantsSettingsInformerFactory defines a function which sets InformerFactory for admission plugins that need it.
type WantsSettingsInformerFactory interface {
	SetSettingsInformerFactory(settingsinformer.SharedInformerFactory)
	admission.InitializationValidator
}

// WantsKubeClientset defines a function which sets Kubernetes Clientset for admission plugins that need it.
type WantsKubeClientset interface {
	SetKubeClientset(kubernetes.Interface)
	admission.InitializationValidator
}

// WantsDynamicClient defines a function which sets a dynamic client for admission plugins that need it.
type WantsDynamicClient interface {
	SetDynamicClient(dynamic.Interface)
	admission.InitializationValidator
}

// WantsAuthorizer defines a function which sets an authorizer for admission plugins that need it.
type WantsAuthorizer interface {
	SetAuthorizer(authorizer.Authorizer)
	admission.InitializationValidator
}

type pluginInitializer struct {
	coreInformers coreinformers.SharedInformerFactory
	coreClient    coreclientset.Interface

	externalCoreInformers externalcoreinformers.SharedInformerFactory

	settingsInformers settingsinformer.SharedInformerFactory

	kubeInformers kubeinformers.SharedInformerFactory
	kubeClient    kubernetes.Interface

	dynamicClient dynamic.Interface

	authorizer authorizer.Authorizer
}

var _ admission.PluginInitializer = pluginInitializer{}
