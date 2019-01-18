// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	coreclientset "github.com/gardener/gardener/pkg/client/core/clientset/internalversion"
	coreinformers "github.com/gardener/gardener/pkg/client/core/informers/internalversion"
	gardenclientset "github.com/gardener/gardener/pkg/client/garden/clientset/internalversion"
	gardeninformers "github.com/gardener/gardener/pkg/client/garden/informers/internalversion"

	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
)

// New constructs new instance of PluginInitializer
func New(coreInformers coreinformers.SharedInformerFactory, coreClient coreclientset.Interface, gardenInformers gardeninformers.SharedInformerFactory, gardenClient gardenclientset.Interface, kubeInformers kubeinformers.SharedInformerFactory, kubeClient kubernetes.Interface, authz authorizer.Authorizer) admission.PluginInitializer {
	return pluginInitializer{
		coreInformers: coreInformers,
		coreClient:    coreClient,

		gardenInformers: gardenInformers,
		gardenClient:    gardenClient,

		kubeInformers: kubeInformers,
		kubeClient:    kubeClient,

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

	if wants, ok := plugin.(WantsInternalGardenInformerFactory); ok {
		wants.SetInternalGardenInformerFactory(i.gardenInformers)
	}
	if wants, ok := plugin.(WantsInternalGardenClientset); ok {
		wants.SetInternalGardenClientset(i.gardenClient)
	}

	if wants, ok := plugin.(WantsKubeInformerFactory); ok {
		wants.SetKubeInformerFactory(i.kubeInformers)
	}
	if wants, ok := plugin.(WantsKubeClientset); ok {
		wants.SetKubeClientset(i.kubeClient)
	}

	if wants, ok := plugin.(WantsAuthorizer); ok {
		wants.SetAuthorizer(i.authorizer)
	}
}
