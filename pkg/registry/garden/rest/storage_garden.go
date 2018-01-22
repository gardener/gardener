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

package rest

import (
	"github.com/gardener/gardener/pkg/api"
	"github.com/gardener/gardener/pkg/apis/garden"
	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	cloudprofilestore "github.com/gardener/gardener/pkg/registry/garden/cloudprofile/storage"
	crosssecretbinding "github.com/gardener/gardener/pkg/registry/garden/crosssecretbinding/storage"
	privatesecretbinding "github.com/gardener/gardener/pkg/registry/garden/privatesecretbinding/storage"
	quotastore "github.com/gardener/gardener/pkg/registry/garden/quota/storage"
	seedstore "github.com/gardener/gardener/pkg/registry/garden/seed/storage"
	shootstore "github.com/gardener/gardener/pkg/registry/garden/shoot/storage"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"
)

// StorageProvider is an empty struct.
type StorageProvider struct{}

// NewRESTStorage creates a new API group info object and registers the v1beta1 Garden storage.
func (p StorageProvider) NewRESTStorage(restOptionsGetter generic.RESTOptionsGetter) genericapiserver.APIGroupInfo {
	apiGroupInfo := genericapiserver.NewDefaultAPIGroupInfo(garden.GroupName, api.Registry, api.Scheme, api.ParameterCodec, api.Codecs)

	apiGroupInfo.VersionedResourcesStorageMap[gardenv1beta1.SchemeGroupVersion.Version] = p.v1beta1Storage(restOptionsGetter)
	apiGroupInfo.GroupMeta.GroupVersion = gardenv1beta1.SchemeGroupVersion

	return apiGroupInfo
}

// GroupName returns the garden group name.
func (p StorageProvider) GroupName() string {
	return garden.GroupName
}

func (p StorageProvider) v1beta1Storage(restOptionsGetter generic.RESTOptionsGetter) map[string]rest.Storage {
	storage := map[string]rest.Storage{}

	cloudprofileStorage := cloudprofilestore.NewStorage(restOptionsGetter)
	storage["cloudprofiles"] = cloudprofileStorage.CloudProfile

	seedStorage := seedstore.NewStorage(restOptionsGetter)
	storage["seeds"] = seedStorage.Seed
	storage["seeds/status"] = seedStorage.Status

	privateSecretBindingStorage := privatesecretbinding.NewStorage(restOptionsGetter)
	storage["privatesecretbindings"] = privateSecretBindingStorage.PrivateSecretBinding

	crossSecretBindingStorage := crosssecretbinding.NewStorage(restOptionsGetter)
	storage["crosssecretbindings"] = crossSecretBindingStorage.CrossSecretBinding

	quotaStorage := quotastore.NewStorage(restOptionsGetter)
	storage["quotas"] = quotaStorage.Quota
	storage["quotas/status"] = quotaStorage.Status

	shootStorage := shootstore.NewStorage(restOptionsGetter)
	storage["shoots"] = shootStorage.Shoot
	storage["shoots/status"] = shootStorage.Status

	return storage
}
