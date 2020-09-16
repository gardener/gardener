// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package rest

import (
	"github.com/gardener/gardener/pkg/api"
	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	backupbucketstore "github.com/gardener/gardener/pkg/registry/core/backupbucket/storage"
	backupentrystore "github.com/gardener/gardener/pkg/registry/core/backupentry/storage"
	cloudprofilestore "github.com/gardener/gardener/pkg/registry/core/cloudprofile/storage"
	controllerinstallationstore "github.com/gardener/gardener/pkg/registry/core/controllerinstallation/storage"
	controllerregistrationstore "github.com/gardener/gardener/pkg/registry/core/controllerregistration/storage"
	plantstore "github.com/gardener/gardener/pkg/registry/core/plant/storage"
	projectstore "github.com/gardener/gardener/pkg/registry/core/project/storage"
	quotastore "github.com/gardener/gardener/pkg/registry/core/quota/storage"
	secretbindingstore "github.com/gardener/gardener/pkg/registry/core/secretbinding/storage"
	seedstore "github.com/gardener/gardener/pkg/registry/core/seed/storage"
	shootstore "github.com/gardener/gardener/pkg/registry/core/shoot/storage"
	shootstatestore "github.com/gardener/gardener/pkg/registry/core/shootstate/storage"

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
	apiGroupInfo.VersionedResourcesStorageMap[gardencorev1beta1.SchemeGroupVersion.Version] = p.v1beta1Storage(restOptionsGetter)
	return apiGroupInfo
}

// GroupName returns the core group name.
func (p StorageProvider) GroupName() string {
	return core.GroupName
}

func (p StorageProvider) v1alpha1Storage(restOptionsGetter generic.RESTOptionsGetter) map[string]rest.Storage {
	storage := map[string]rest.Storage{}

	backupBucketStorage := backupbucketstore.NewStorage(restOptionsGetter)
	storage["backupbuckets"] = backupBucketStorage.BackupBucket
	storage["backupbuckets/status"] = backupBucketStorage.Status

	backupEntryStorage := backupentrystore.NewStorage(restOptionsGetter)
	storage["backupentries"] = backupEntryStorage.BackupEntry
	storage["backupentries/status"] = backupEntryStorage.Status

	cloudprofileStorage := cloudprofilestore.NewStorage(restOptionsGetter)
	storage["cloudprofiles"] = cloudprofileStorage.CloudProfile

	controllerRegistrationStorage := controllerregistrationstore.NewStorage(restOptionsGetter)
	storage["controllerregistrations"] = controllerRegistrationStorage.ControllerRegistration

	controllerInstallationStorage := controllerinstallationstore.NewStorage(restOptionsGetter)
	storage["controllerinstallations"] = controllerInstallationStorage.ControllerInstallation
	storage["controllerinstallations/status"] = controllerInstallationStorage.Status

	plantStorage := plantstore.NewStorage(restOptionsGetter)
	storage["plants"] = plantStorage.Plant
	storage["plants/status"] = plantStorage.Status

	projectStorage := projectstore.NewStorage(restOptionsGetter)
	storage["projects"] = projectStorage.Project
	storage["projects/status"] = projectStorage.Status

	quotaStorage := quotastore.NewStorage(restOptionsGetter)
	storage["quotas"] = quotaStorage.Quota

	secretBindingStorage := secretbindingstore.NewStorage(restOptionsGetter)
	storage["secretbindings"] = secretBindingStorage.SecretBinding

	seedStorage := seedstore.NewStorage(restOptionsGetter, cloudprofileStorage.CloudProfile)
	storage["seeds"] = seedStorage.Seed
	storage["seeds/status"] = seedStorage.Status

	shootStorage := shootstore.NewStorage(restOptionsGetter)
	storage["shoots"] = shootStorage.Shoot
	storage["shoots/status"] = shootStorage.Status

	shootStateStorage := shootstatestore.NewStorage(restOptionsGetter)
	storage["shootstates"] = shootStateStorage.ShootState

	return storage
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

	controllerRegistrationStorage := controllerregistrationstore.NewStorage(restOptionsGetter)
	storage["controllerregistrations"] = controllerRegistrationStorage.ControllerRegistration

	controllerInstallationStorage := controllerinstallationstore.NewStorage(restOptionsGetter)
	storage["controllerinstallations"] = controllerInstallationStorage.ControllerInstallation
	storage["controllerinstallations/status"] = controllerInstallationStorage.Status

	plantStorage := plantstore.NewStorage(restOptionsGetter)
	storage["plants"] = plantStorage.Plant
	storage["plants/status"] = plantStorage.Status

	projectStorage := projectstore.NewStorage(restOptionsGetter)
	storage["projects"] = projectStorage.Project
	storage["projects/status"] = projectStorage.Status

	quotaStorage := quotastore.NewStorage(restOptionsGetter)
	storage["quotas"] = quotaStorage.Quota

	secretBindingStorage := secretbindingstore.NewStorage(restOptionsGetter)
	storage["secretbindings"] = secretBindingStorage.SecretBinding

	seedStorage := seedstore.NewStorage(restOptionsGetter, cloudprofileStorage.CloudProfile)
	storage["seeds"] = seedStorage.Seed
	storage["seeds/status"] = seedStorage.Status

	shootStorage := shootstore.NewStorage(restOptionsGetter)
	storage["shoots"] = shootStorage.Shoot
	storage["shoots/status"] = shootStorage.Status

	return storage
}
