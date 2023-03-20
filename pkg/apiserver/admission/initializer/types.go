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

// WantsInternalCoreInformerFactory defines a function which sets InformerFactory for admission plugins that need it.
type WantsInternalCoreInformerFactory interface {
	SetInternalCoreInformerFactory(gardencoreinformers.SharedInformerFactory)
	admission.InitializationValidator
}

// WantsInternalCoreClientset defines a function which sets Core Clientset for admission plugins that need it.
type WantsInternalCoreClientset interface {
	SetInternalCoreClientset(gardencoreclientset.Interface)
	admission.InitializationValidator
}

// WantsExternalCoreInformerFactory defines a function which sets external Core InformerFactory for admission plugins that need it.
type WantsExternalCoreInformerFactory interface {
	SetExternalCoreInformerFactory(gardencoreexternalinformers.SharedInformerFactory)
	admission.InitializationValidator
}

// WantsExternalCoreClientset defines a function which sets external Core Clientset for admission plugins that need it.
type WantsExternalCoreClientset interface {
	SetExternalCoreClientset(gardencoreversionedclientset.Interface)
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

// WantsSeedManagementClientset defines a function which sets SeedManagement Clientset for admission plugins that need it.
type WantsSeedManagementClientset interface {
	SetSeedManagementClientset(seedmanagementclientset.Interface)
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

	externalCoreInformers gardencoreexternalinformers.SharedInformerFactory
	externalCoreClient    gardencoreversionedclientset.Interface

	seedManagementInformers seedmanagementinformers.SharedInformerFactory
	seedManagementClient    seedmanagementclientset.Interface

	settingsInformers settingsinformers.SharedInformerFactory

	kubeInformers kubeinformers.SharedInformerFactory
	kubeClient    kubernetes.Interface

	dynamicClient dynamic.Interface

	authorizer authorizer.Authorizer

	quotaConfiguration quotav1.Configuration
}

var _ admission.PluginInitializer = pluginInitializer{}
