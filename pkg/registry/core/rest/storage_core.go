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

package rest

import (
	"github.com/gardener/gardener/pkg/api"
	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	controllerinstallationstore "github.com/gardener/gardener/pkg/registry/core/controllerinstallation/storage"
	controllerregistrationstore "github.com/gardener/gardener/pkg/registry/core/controllerregistration/storage"
	plantstore "github.com/gardener/gardener/pkg/registry/core/plant/storage"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"
)

// StorageProvider is an empty struct.
type StorageProvider struct{}

// NewRESTStorage creates a new API group info object and registers the v1alpha1 core storage.
func (p StorageProvider) NewRESTStorage(restOptionsGetter generic.RESTOptionsGetter) genericapiserver.APIGroupInfo {
	apiGroupInfo := genericapiserver.NewDefaultAPIGroupInfo(core.GroupName, api.Scheme, metav1.ParameterCodec, api.Codecs)
	apiGroupInfo.VersionedResourcesStorageMap[gardencorev1alpha1.SchemeGroupVersion.Version] = p.v1alpha1Storage(restOptionsGetter)
	return apiGroupInfo
}

// GroupName returns the core group name.
func (p StorageProvider) GroupName() string {
	return core.GroupName
}

func (p StorageProvider) v1alpha1Storage(restOptionsGetter generic.RESTOptionsGetter) map[string]rest.Storage {
	storage := map[string]rest.Storage{}

	controllerRegistrationStorage := controllerregistrationstore.NewStorage(restOptionsGetter)
	storage["controllerregistrations"] = controllerRegistrationStorage.ControllerRegistration

	controllerInstallationStorage := controllerinstallationstore.NewStorage(restOptionsGetter)
	storage["controllerinstallations"] = controllerInstallationStorage.ControllerInstallation
	storage["controllerinstallations/status"] = controllerInstallationStorage.Status

	plantStorage := plantstore.NewStorage(restOptionsGetter)
	storage["plants"] = plantStorage.Plant
	storage["plants/status"] = plantStorage.Status

	return storage
}
