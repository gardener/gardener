// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"

	"github.com/gardener/gardener/pkg/api"
	"github.com/gardener/gardener/pkg/apis/seedmanagement"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	managedseedstore "github.com/gardener/gardener/pkg/registry/seedmanagement/managedseed/storage"
	managedseedsetstore "github.com/gardener/gardener/pkg/registry/seedmanagement/managedseedset/storage"
)

// StorageProvider is an empty struct.
type StorageProvider struct{}

// NewRESTStorage creates a new API group info object and registers the v1alpha1 Garden storage.
func (p StorageProvider) NewRESTStorage(restOptionsGetter generic.RESTOptionsGetter) genericapiserver.APIGroupInfo {
	apiGroupInfo := genericapiserver.NewDefaultAPIGroupInfo(seedmanagement.GroupName, api.Scheme, metav1.ParameterCodec, api.Codecs)
	apiGroupInfo.VersionedResourcesStorageMap[seedmanagementv1alpha1.SchemeGroupVersion.Version] = p.v1alpha1Storage(restOptionsGetter)
	return apiGroupInfo
}

// GroupName returns the garden group name.
func (p StorageProvider) GroupName() string {
	return seedmanagement.GroupName
}

func (p StorageProvider) v1alpha1Storage(restOptionsGetter generic.RESTOptionsGetter) map[string]rest.Storage {
	storage := map[string]rest.Storage{}

	managedSeedStorage := managedseedstore.NewStorage(restOptionsGetter)
	managedSeedSetStorage := managedseedsetstore.NewStorage(restOptionsGetter)

	storage["managedseeds"] = managedSeedStorage.ManagedSeed
	storage["managedseeds/status"] = managedSeedStorage.Status
	storage["managedseedsets"] = managedSeedSetStorage.ManagedSeedSet
	storage["managedseedsets/status"] = managedSeedSetStorage.Status
	storage["managedseedsets/scale"] = managedSeedSetStorage.Scale

	return storage
}
