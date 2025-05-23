// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package initializer

import (
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	quotav1 "k8s.io/apiserver/pkg/quota/v1"
	"k8s.io/client-go/dynamic"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"

	gardencoreclientset "github.com/gardener/gardener/pkg/client/core/clientset/versioned"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	securityclientset "github.com/gardener/gardener/pkg/client/security/clientset/versioned"
	securityinformers "github.com/gardener/gardener/pkg/client/security/informers/externalversions"
	seedmanagementclientset "github.com/gardener/gardener/pkg/client/seedmanagement/clientset/versioned"
	seedmanagementinformers "github.com/gardener/gardener/pkg/client/seedmanagement/informers/externalversions"
	settingsinformers "github.com/gardener/gardener/pkg/client/settings/informers/externalversions"
)

// WantsCoreInformerFactory defines a function which sets Core InformerFactory for admission plugins that need it.
type WantsCoreInformerFactory interface {
	SetCoreInformerFactory(gardencoreinformers.SharedInformerFactory)
	admission.InitializationValidator
}

// WantsCoreClientSet defines a function which sets Core Clientset for admission plugins that need it.
type WantsCoreClientSet interface {
	SetCoreClientSet(gardencoreclientset.Interface)
	admission.InitializationValidator
}

// WantsKubeInformerFactory defines a function which sets InformerFactory for admission plugins that need it.
type WantsKubeInformerFactory interface {
	SetKubeInformerFactory(kubeinformers.SharedInformerFactory)
	admission.InitializationValidator
}

// WantsSeedManagementInformerFactory defines a function which sets InformerFactory for admission plugins that need it.
type WantsSeedManagementInformerFactory interface {
	SetSeedManagementInformerFactory(seedmanagementinformers.SharedInformerFactory)
	admission.InitializationValidator
}

// WantsSeedManagementClientSet defines a function which sets SeedManagement Clientset for admission plugins that need it.
type WantsSeedManagementClientSet interface {
	SetSeedManagementClientSet(seedmanagementclientset.Interface)
	admission.InitializationValidator
}

// WantsSecurityInformerFactory defines a function which sets security InformerFactory for admission plugins that need it.
type WantsSecurityInformerFactory interface {
	SetSecurityInformerFactory(securityinformers.SharedInformerFactory)
	admission.InitializationValidator
}

// WantsSecurityClientSet defines a function which sets Security Clientset for admission plugins that need it.
type WantsSecurityClientSet interface {
	SetSecurityClientSet(securityclientset.Interface)
	admission.InitializationValidator
}

// WantsSettingsInformerFactory defines a function which sets InformerFactory for admission plugins that need it.
type WantsSettingsInformerFactory interface {
	SetSettingsInformerFactory(settingsinformers.SharedInformerFactory)
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

// WantsQuotaConfiguration defines a function which sets quota configuration for admission plugins that need it.
type WantsQuotaConfiguration interface {
	SetQuotaConfiguration(quotav1.Configuration)
	admission.InitializationValidator
}

type pluginInitializer struct {
	coreInformers gardencoreinformers.SharedInformerFactory
	coreClient    gardencoreclientset.Interface

	seedManagementInformers seedmanagementinformers.SharedInformerFactory
	seedManagementClient    seedmanagementclientset.Interface

	settingsInformers settingsinformers.SharedInformerFactory

	securityInformers securityinformers.SharedInformerFactory
	securityClient    securityclientset.Interface

	kubeInformers kubeinformers.SharedInformerFactory
	kubeClient    kubernetes.Interface

	dynamicClient dynamic.Interface

	authorizer authorizer.Authorizer

	quotaConfiguration quotav1.Configuration
}

var _ admission.PluginInitializer = pluginInitializer{}
