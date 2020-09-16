// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package rest

import (
	"github.com/gardener/gardener/pkg/api"
	"github.com/gardener/gardener/pkg/apis/settings"
	settingsv1alpha1 "github.com/gardener/gardener/pkg/apis/settings/v1alpha1"
	clusteroidcpresetstore "github.com/gardener/gardener/pkg/registry/settings/clusteropenidconnectpreset/storage"
	oidcpresetstore "github.com/gardener/gardener/pkg/registry/settings/openidconnectpreset/storage"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"
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

	oidcPresetStorage := oidcpresetstore.NewStorage(restOptionsGetter)
	clusterOIDCStorage := clusteroidcpresetstore.NewStorage(restOptionsGetter)

	storage["openidconnectpresets"] = oidcPresetStorage.OpenIDConnectPreset
	storage["clusteropenidconnectpresets"] = clusterOIDCStorage.ClusterOpenIDConnectPreset

	return storage
}
