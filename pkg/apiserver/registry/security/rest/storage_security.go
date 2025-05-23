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
	"github.com/gardener/gardener/pkg/apis/security"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	credentialsbindingstore "github.com/gardener/gardener/pkg/apiserver/registry/security/credentialsbinding/storage"
	workloadidentitystore "github.com/gardener/gardener/pkg/apiserver/registry/security/workloadidentity/storage"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	"github.com/gardener/gardener/pkg/utils/workloadidentity"
)

// StorageProvider is an empty struct.
type StorageProvider struct {
	TokenIssuer         workloadidentity.TokenIssuer
	CoreInformerFactory gardencoreinformers.SharedInformerFactory
}

// NewRESTStorage creates a new API group info object and registers the v1alpha1 Garden storage.
func (p StorageProvider) NewRESTStorage(restOptionsGetter generic.RESTOptionsGetter) genericapiserver.APIGroupInfo {
	apiGroupInfo := genericapiserver.NewDefaultAPIGroupInfo(security.GroupName, api.Scheme, metav1.ParameterCodec, api.Codecs)
	apiGroupInfo.VersionedResourcesStorageMap[securityv1alpha1.SchemeGroupVersion.Version] = p.v1alpha1Storage(restOptionsGetter)
	return apiGroupInfo
}

// GroupName returns the garden group name.
func (p StorageProvider) GroupName() string {
	return security.GroupName
}

func (p StorageProvider) v1alpha1Storage(restOptionsGetter generic.RESTOptionsGetter) map[string]rest.Storage {
	storage := map[string]rest.Storage{}

	credentialsBindingStorage := credentialsbindingstore.NewStorage(restOptionsGetter)
	storage["credentialsbindings"] = credentialsBindingStorage.CredentialsBinding

	workloadIdentityStorage := workloadidentitystore.NewStorage(
		restOptionsGetter,
		p.TokenIssuer,
		p.CoreInformerFactory,
	)
	storage["workloadidentities"] = workloadIdentityStorage.WorkloadIdentity
	storage["workloadidentities/token"] = workloadIdentityStorage.TokenRequest

	return storage
}
