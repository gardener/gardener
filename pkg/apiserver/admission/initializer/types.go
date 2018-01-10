// Copyright 2018 The Gardener Authors.
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
	gardeninformers "github.com/gardener/gardener/pkg/client/garden/informers/internalversion"
	"k8s.io/apiserver/pkg/admission"
	kubeinformers "k8s.io/client-go/informers"
)

// WantsInternalGardenInformerFactory defines a function which sets InformerFactory for admission plugins that need it
type WantsInternalGardenInformerFactory interface {
	SetInternalGardenInformerFactory(gardeninformers.SharedInformerFactory)
	admission.InitializationValidator
}

// WantsKubeInformerFactory defines a function which sets InformerFactory for admission plugins that need it
type WantsKubeInformerFactory interface {
	SetKubeInformerFactory(kubeinformers.SharedInformerFactory)
	admission.InitializationValidator
}

type pluginInitializer struct {
	gardenInformers gardeninformers.SharedInformerFactory
	kubeInformers   kubeinformers.SharedInformerFactory
}

var _ admission.PluginInitializer = pluginInitializer{}
