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
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"

	"github.com/gardener/gardener/pkg/api"
	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	backupbucketstore "github.com/gardener/gardener/pkg/registry/core/backupbucket/storage"
	backupentrystore "github.com/gardener/gardener/pkg/registry/core/backupentry/storage"
	cloudprofilestore "github.com/gardener/gardener/pkg/registry/core/cloudprofile/storage"
	controllerdeploymentstore "github.com/gardener/gardener/pkg/registry/core/controllerdeployment/storage"
	controllerinstallationstore "github.com/gardener/gardener/pkg/registry/core/controllerinstallation/storage"
	controllerregistrationstore "github.com/gardener/gardener/pkg/registry/core/controllerregistration/storage"
	exposureclassstore "github.com/gardener/gardener/pkg/registry/core/exposureclass/storage"
	projectstore "github.com/gardener/gardener/pkg/registry/core/project/storage"
	quotastore "github.com/gardener/gardener/pkg/registry/core/quota/storage"
	secretbindingstore "github.com/gardener/gardener/pkg/registry/core/secretbinding/storage"
	seedstore "github.com/gardener/gardener/pkg/registry/core/seed/storage"
	shootstore "github.com/gardener/gardener/pkg/registry/core/shoot/storage"
	shootstatestore "github.com/gardener/gardener/pkg/registry/core/shootstate/storage"
)

// StorageProvider contains configurations related to the core resources.
type StorageProvider struct {
	AdminKubeconfigMaxExpiration time.Duration
	CredentialsRotationInterval  time.Duration
}

// NewRESTStorage creates a new API group info object and registers the v1alpha1 core storage.
func (p StorageProvider) NewRESTStorage(restOptionsGetter generic.RESTOptionsGetter) genericapiserver.APIGroupInfo {
	apiGroupInfo := genericapiserver.NewDefaultAPIGroupInfo(core.GroupName, api.Scheme, metav1.ParameterCodec, api.Codecs)
	apiGroupInfo.VersionedResourcesStorageMap[gardencorev1beta1.SchemeGroupVersion.Version] = p.v1beta1Storage(restOptionsGetter)
	return apiGroupInfo
}

// GroupName returns the core group name.
func (p StorageProvider) GroupName() string {
	return core.GroupName
}

func (p StorageProvider) v1beta1Storage(restOptionsGetter generic.RESTOptionsGetter) map[string]rest.Storage {
	storage := map[string]rest.Storage{}

	backupBucketStorage := backupbucketstore.NewStorage(restOptionsGetter)
	storage["backupbuckets"] = backupBucketStorage.BackupBucket
	storage["backupbuckets/status"] = backupBucketStorage.Status

	backupEntryStorage := backupentrystore.NewStorage(restOptionsGetter)
	storage["backupentries"] = backupEntryStorage.BackupEntry
	storage["backupentries/status"] = backupEntryStorage.Status

	cloudprofileStorage := cloudprofilestore.NewStorage(restOptionsGetter)
	storage["cloudprofiles"] = cloudprofileStorage.CloudProfile

	controllerDeploymentStorage := controllerdeploymentstore.NewStorage(restOptionsGetter)
	storage["controllerdeployments"] = controllerDeploymentStorage.ControllerDeployment

	controllerRegistrationStorage := controllerregistrationstore.NewStorage(restOptionsGetter)
	storage["controllerregistrations"] = controllerRegistrationStorage.ControllerRegistration

	controllerInstallationStorage := controllerinstallationstore.NewStorage(restOptionsGetter)
	storage["controllerinstallations"] = controllerInstallationStorage.ControllerInstallation
	storage["controllerinstallations/status"] = controllerInstallationStorage.Status

	exposureClassStorage := exposureclassstore.NewStorage(restOptionsGetter)
	storage["exposureclasses"] = exposureClassStorage.ExposureClass

	projectStorage := projectstore.NewStorage(restOptionsGetter)
	storage["projects"] = projectStorage.Project
	storage["projects/status"] = projectStorage.Status

	quotaStorage := quotastore.NewStorage(restOptionsGetter)
	storage["quotas"] = quotaStorage.Quota

	secretBindingStorage := secretbindingstore.NewStorage(restOptionsGetter)
	storage["secretbindings"] = secretBindingStorage.SecretBinding

	seedStorage := seedstore.NewStorage(restOptionsGetter)
	storage["seeds"] = seedStorage.Seed
	storage["seeds/status"] = seedStorage.Status

	shootStateStorage := shootstatestore.NewStorage(restOptionsGetter)
	storage["shootstates"] = shootStateStorage.ShootState

	shootStorage := shootstore.NewStorage(restOptionsGetter, shootstatestore.NewStorage(restOptionsGetter).ShootState.Store, p.AdminKubeconfigMaxExpiration, p.CredentialsRotationInterval)
	storage["shoots"] = shootStorage.Shoot
	storage["shoots/status"] = shootStorage.Status

	storage["shoots/binding"] = shootStorage.Binding

	if shootStorage.AdminKubeconfig != nil {
		storage["shoots/adminkubeconfig"] = shootStorage.AdminKubeconfig
	}

	return storage
}
