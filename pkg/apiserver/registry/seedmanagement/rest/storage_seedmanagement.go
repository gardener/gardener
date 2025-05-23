// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package rest

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"

	"github.com/gardener/gardener/pkg/api"
	"github.com/gardener/gardener/pkg/apis/seedmanagement"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	gardenletstore "github.com/gardener/gardener/pkg/apiserver/registry/seedmanagement/gardenlet/storage"
	managedseedstore "github.com/gardener/gardener/pkg/apiserver/registry/seedmanagement/managedseed/storage"
	managedseedsetstore "github.com/gardener/gardener/pkg/apiserver/registry/seedmanagement/managedseedset/storage"
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

	gardenletStorage := gardenletstore.NewStorage(restOptionsGetter)
	managedSeedStorage := managedseedstore.NewStorage(restOptionsGetter)
	managedSeedSetStorage := managedseedsetstore.NewStorage(restOptionsGetter)

	storage["gardenlets"] = gardenletStorage.Gardenlet
	storage["gardenlets/status"] = gardenletStorage.Status
	storage["managedseeds"] = managedSeedStorage.ManagedSeed
	storage["managedseeds/status"] = managedSeedStorage.Status
	storage["managedseedsets"] = managedSeedSetStorage.ManagedSeedSet
	storage["managedseedsets/status"] = managedSeedSetStorage.Status
	storage["managedseedsets/scale"] = managedSeedSetStorage.Scale

	return storage
}
