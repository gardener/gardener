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
	"github.com/gardener/gardener/pkg/apis/operations"
	operationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
	bastionstore "github.com/gardener/gardener/pkg/registry/operations/bastion/storage"
)

// StorageProvider is an empty struct.
type StorageProvider struct{}

// NewRESTStorage creates a new API group info object and registers the v1alpha1 operations storage.
func (p StorageProvider) NewRESTStorage(restOptionsGetter generic.RESTOptionsGetter) genericapiserver.APIGroupInfo {
	apiGroupInfo := genericapiserver.NewDefaultAPIGroupInfo(operations.GroupName, api.Scheme, metav1.ParameterCodec, api.Codecs)
	apiGroupInfo.VersionedResourcesStorageMap[operationsv1alpha1.SchemeGroupVersion.Version] = p.v1alpha1Storage(restOptionsGetter)
	return apiGroupInfo
}

// GroupName returns the operations group name.
func (p StorageProvider) GroupName() string {
	return operations.GroupName
}

func (p StorageProvider) v1alpha1Storage(restOptionsGetter generic.RESTOptionsGetter) map[string]rest.Storage {
	storage := map[string]rest.Storage{}

	bastionStorage := bastionstore.NewStorage(restOptionsGetter)
	storage["bastions"] = bastionStorage.Bastion
	storage["bastions/status"] = bastionStorage.Status

	return storage
}
