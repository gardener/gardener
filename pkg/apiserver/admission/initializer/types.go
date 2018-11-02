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
	"github.com/gardener/gardener/pkg/client/garden/clientset/internalversion"
	gardeninformers "github.com/gardener/gardener/pkg/client/garden/informers/internalversion"

	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
)

// WantsInternalGardenInformerFactory defines a function which sets InformerFactory for admission plugins that need it.
type WantsInternalGardenInformerFactory interface {
	SetInternalGardenInformerFactory(gardeninformers.SharedInformerFactory)
	admission.InitializationValidator
}

// WantsInternalGardenClientset defines a function which sets Garden Clientset for admission plugins that need it.
type WantsInternalGardenClientset interface {
	SetInternalGardenClientset(internalversion.Interface)
	admission.InitializationValidator
}

// WantsKubeInformerFactory defines a function which sets InformerFactory for admission plugins that need it.
type WantsKubeInformerFactory interface {
	SetKubeInformerFactory(kubeinformers.SharedInformerFactory)
	admission.InitializationValidator
}

// WantsKubeClientset defines a function which sets Kubernetes Clientset for admission plugins that need it.
type WantsKubeClientset interface {
	SetKubeClientset(kubernetes.Interface)
	admission.InitializationValidator
}

// WantsAuthorizer defines a function which sets an authorizer for admission plugins that need it.
type WantsAuthorizer interface {
	SetAuthorizer(authorizer.Authorizer)
	admission.InitializationValidator
}

type pluginInitializer struct {
	gardenInformers gardeninformers.SharedInformerFactory
	gardenClient    internalversion.Interface

	kubeInformers kubeinformers.SharedInformerFactory
	kubeClient    kubernetes.Interface

	authorizer authorizer.Authorizer
}

var _ admission.PluginInitializer = pluginInitializer{}
