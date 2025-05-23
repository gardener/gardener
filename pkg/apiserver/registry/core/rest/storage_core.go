// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package rest

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"
	kubeinformers "k8s.io/client-go/informers"

	"github.com/gardener/gardener/pkg/api"
	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	backupbucketstore "github.com/gardener/gardener/pkg/apiserver/registry/core/backupbucket/storage"
	backupentrystore "github.com/gardener/gardener/pkg/apiserver/registry/core/backupentry/storage"
	cloudprofilestore "github.com/gardener/gardener/pkg/apiserver/registry/core/cloudprofile/storage"
	controllerdeploymentstore "github.com/gardener/gardener/pkg/apiserver/registry/core/controllerdeployment/storage"
	controllerinstallationstore "github.com/gardener/gardener/pkg/apiserver/registry/core/controllerinstallation/storage"
	controllerregistrationstore "github.com/gardener/gardener/pkg/apiserver/registry/core/controllerregistration/storage"
	exposureclassstore "github.com/gardener/gardener/pkg/apiserver/registry/core/exposureclass/storage"
	internalsecretstore "github.com/gardener/gardener/pkg/apiserver/registry/core/internalsecret/storage"
	namespacedcloudprofilestore "github.com/gardener/gardener/pkg/apiserver/registry/core/namespacedcloudprofile/storage"
	projectstore "github.com/gardener/gardener/pkg/apiserver/registry/core/project/storage"
	quotastore "github.com/gardener/gardener/pkg/apiserver/registry/core/quota/storage"
	secretbindingstore "github.com/gardener/gardener/pkg/apiserver/registry/core/secretbinding/storage"
	seedstore "github.com/gardener/gardener/pkg/apiserver/registry/core/seed/storage"
	shootstore "github.com/gardener/gardener/pkg/apiserver/registry/core/shoot/storage"
	shootstatestore "github.com/gardener/gardener/pkg/apiserver/registry/core/shootstate/storage"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
)

// StorageProvider contains configurations related to the core resources.
type StorageProvider struct {
	AdminKubeconfigMaxExpiration  time.Duration
	ViewerKubeconfigMaxExpiration time.Duration
	CredentialsRotationInterval   time.Duration
	KubeInformerFactory           kubeinformers.SharedInformerFactory
	CoreInformerFactory           gardencoreinformers.SharedInformerFactory
}

// NewRESTStorage creates a new API group info object and registers the v1beta1 core storage.
func (p StorageProvider) NewRESTStorage(restOptionsGetter generic.RESTOptionsGetter) genericapiserver.APIGroupInfo {
	apiGroupInfo := genericapiserver.NewDefaultAPIGroupInfo(core.GroupName, api.Scheme, metav1.ParameterCodec, api.Codecs)
	apiGroupInfo.VersionedResourcesStorageMap[gardencorev1.SchemeGroupVersion.Version] = p.v1Storage(restOptionsGetter)
	apiGroupInfo.VersionedResourcesStorageMap[gardencorev1beta1.SchemeGroupVersion.Version] = p.v1beta1Storage(restOptionsGetter)
	return apiGroupInfo
}

// GroupName returns the core group name.
func (p StorageProvider) GroupName() string {
	return core.GroupName
}

func (p StorageProvider) v1Storage(restOptionsGetter generic.RESTOptionsGetter) map[string]rest.Storage {
	storage := map[string]rest.Storage{}

	controllerDeploymentStorage := controllerdeploymentstore.NewStorage(restOptionsGetter)
	storage["controllerdeployments"] = controllerDeploymentStorage.ControllerDeployment

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

	namespacedcloudprofileStorage := namespacedcloudprofilestore.NewStorage(restOptionsGetter)
	storage["namespacedcloudprofiles"] = namespacedcloudprofileStorage.NamespacedCloudProfile
	storage["namespacedcloudprofiles/status"] = namespacedcloudprofileStorage.Status

	controllerDeploymentStorage := controllerdeploymentstore.NewStorage(restOptionsGetter)
	storage["controllerdeployments"] = controllerDeploymentStorage.ControllerDeployment

	controllerRegistrationStorage := controllerregistrationstore.NewStorage(restOptionsGetter)
	storage["controllerregistrations"] = controllerRegistrationStorage.ControllerRegistration

	controllerInstallationStorage := controllerinstallationstore.NewStorage(restOptionsGetter)
	storage["controllerinstallations"] = controllerInstallationStorage.ControllerInstallation
	storage["controllerinstallations/status"] = controllerInstallationStorage.Status

	exposureClassStorage := exposureclassstore.NewStorage(restOptionsGetter)
	storage["exposureclasses"] = exposureClassStorage.ExposureClass

	storage["internalsecrets"] = internalsecretstore.NewREST(restOptionsGetter)

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

	shootStorage := shootstore.NewStorage(
		restOptionsGetter,
		p.CoreInformerFactory.Core().V1beta1().InternalSecrets().Lister(),
		p.KubeInformerFactory.Core().V1().Secrets().Lister(),
		p.KubeInformerFactory.Core().V1().ConfigMaps().Lister(),
		p.AdminKubeconfigMaxExpiration,
		p.ViewerKubeconfigMaxExpiration,
		p.CredentialsRotationInterval,
	)
	storage["shoots"] = shootStorage.Shoot
	storage["shoots/status"] = shootStorage.Status
	storage["shoots/binding"] = shootStorage.Binding
	storage["shoots/adminkubeconfig"] = shootStorage.AdminKubeconfig
	storage["shoots/viewerkubeconfig"] = shootStorage.ViewerKubeconfig

	return storage
}
