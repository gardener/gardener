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

package rest

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"

	"github.com/gardener/gardener/pkg/api"
	"github.com/gardener/gardener/pkg/apis/settings"
	settingsv1alpha1 "github.com/gardener/gardener/pkg/apis/settings/v1alpha1"
	clusteropenidconnectpresetstore "github.com/gardener/gardener/pkg/registry/settings/clusteropenidconnectpreset/storage"
	openidconnectpresetstore "github.com/gardener/gardener/pkg/registry/settings/openidconnectpreset/storage"
)

// StorageProvider is an empty struct.
type StorageProvider struct{}

// NewRESTStorage creates a new API group info object and registers the v1beta1 Garden storage.
func (p StorageProvider) NewRESTStorage(restOptionsGetter generic.RESTOptionsGetter) genericapiserver.APIGroupInfo {
	apiGroupInfo := genericapiserver.NewDefaultAPIGroupInfo(settings.GroupName, api.Scheme, metav1.ParameterCodec, api.Codecs)
	apiGroupInfo.VersionedResourcesStorageMap[settingsv1alpha1.SchemeGroupVersion.Version] = p.v1alpha1Storage(restOptionsGetter)
	return apiGroupInfo
}

// GroupName returns the garden group name.
func (p StorageProvider) GroupName() string {
	return settings.GroupName
}

func (p StorageProvider) v1alpha1Storage(restOptionsGetter generic.RESTOptionsGetter) map[string]rest.Storage {
	storage := map[string]rest.Storage{}

	oidcPresetStorage := openidconnectpresetstore.NewStorage(restOptionsGetter)
	clusterOIDCStorage := clusteropenidconnectpresetstore.NewStorage(restOptionsGetter)

	storage["openidconnectpresets"] = oidcPresetStorage.OpenIDConnectPreset
	storage["clusteropenidconnectpresets"] = clusterOIDCStorage.ClusterOpenIDConnectPreset

	return storage
}
