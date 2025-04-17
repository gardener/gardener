// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package extensionlabels

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apiserver/pkg/admission"

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorehelper "github.com/gardener/gardener/pkg/apis/core/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/apis/security"
	admissioninitializer "github.com/gardener/gardener/pkg/apiserver/admission/initializer"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	gardencorev1beta1listers "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	plugin "github.com/gardener/gardener/plugin/pkg"
)

// Register registers a plugin.
func Register(plugins *admission.Plugins) {
	plugins.Register(plugin.PluginNameExtensionLabels, NewFactory)
}

// NewFactory creates a new PluginFactory.
func NewFactory(_ io.Reader) (admission.Interface, error) {
	return New()
}

// ExtensionLabels contains the admission handler
type ExtensionLabels struct {
	*admission.Handler
	backupBucketLister           gardencorev1beta1listers.BackupBucketLister
	cloudProfileLister           gardencorev1beta1listers.CloudProfileLister
	controllerRegistrationLister gardencorev1beta1listers.ControllerRegistrationLister
	readyFunc                    admission.ReadyFunc
}

var (
	_          = admissioninitializer.WantsCoreInformerFactory(&ExtensionLabels{})
	readyFuncs []admission.ReadyFunc
)

// New creates a new ExtensionLabels admission plugin.
func New() (*ExtensionLabels, error) {
	return &ExtensionLabels{
		Handler: admission.NewHandler(admission.Create, admission.Update),
	}, nil
}

// AssignReadyFunc assigns the ready function to the admission handler.
func (e *ExtensionLabels) AssignReadyFunc(f admission.ReadyFunc) {
	e.readyFunc = f
	e.SetReadyFunc(f)
}

// SetCoreInformerFactory sets the garden core informer factory.
func (e *ExtensionLabels) SetCoreInformerFactory(f gardencoreinformers.SharedInformerFactory) {
	backupBucketInformer := f.Core().V1beta1().BackupBuckets()
	e.backupBucketLister = backupBucketInformer.Lister()
	cloudProfileInformer := f.Core().V1beta1().CloudProfiles()
	e.cloudProfileLister = cloudProfileInformer.Lister()
	controllerRegistrationInformer := f.Core().V1beta1().ControllerRegistrations()
	e.controllerRegistrationLister = controllerRegistrationInformer.Lister()

	readyFuncs = append(readyFuncs,
		backupBucketInformer.Informer().HasSynced,
		cloudProfileInformer.Informer().HasSynced,
		controllerRegistrationInformer.Informer().HasSynced,
	)
}

func (e *ExtensionLabels) waitUntilReady(attrs admission.Attributes) error {
	// Wait until the caches have been synced
	if e.readyFunc == nil {
		e.AssignReadyFunc(func() bool {
			for _, readyFunc := range readyFuncs {
				if !readyFunc() {
					return false
				}
			}
			return true
		})
	}

	if !e.WaitForReady() {
		return admission.NewForbidden(attrs, errors.New("not yet ready to handle request"))
	}

	return nil
}

// ValidateInitialization checks whether the plugin was correctly initialized.
func (e *ExtensionLabels) ValidateInitialization() error {
	if e.backupBucketLister == nil {
		return errors.New("missing BackupBucket lister")
	}
	if e.cloudProfileLister == nil {
		return errors.New("missing CloudProfile lister")
	}
	if e.controllerRegistrationLister == nil {
		return errors.New("missing ControllerRegistration lister")
	}
	return nil
}

var _ admission.MutationInterface = &ExtensionLabels{}

// Admit adds extension labels to resources.
func (e *ExtensionLabels) Admit(_ context.Context, a admission.Attributes, _ admission.ObjectInterfaces) error {
	if err := e.waitUntilReady(a); err != nil {
		return fmt.Errorf("err while waiting for ready %w", err)
	}

	switch a.GetKind().GroupKind() {
	case core.Kind("Seed"):
		seed, ok := a.GetObject().(*core.Seed)
		if !ok {
			return apierrors.NewBadRequest("could not convert resource into Seed object")
		}

		removeLabels(&seed.ObjectMeta)
		addMetaDataLabelsSeed(seed)

	case core.Kind("SecretBinding"):
		secretBinding, ok := a.GetObject().(*core.SecretBinding)
		if !ok {
			return apierrors.NewBadRequest("could not convert resource into SecretBinding object")
		}

		removeLabels(&secretBinding.ObjectMeta)
		addMetaDataLabelsSecretBinding(secretBinding)

	case security.Kind("CredentialsBinding"):
		credentialsBinding, ok := a.GetObject().(*security.CredentialsBinding)
		if !ok {
			return apierrors.NewBadRequest("could not convert resource into CredentialsBinding object")
		}

		removeLabels(&credentialsBinding.ObjectMeta)
		providerType := credentialsBinding.Provider.Type
		metav1.SetMetaDataLabel(&credentialsBinding.ObjectMeta, v1beta1constants.LabelExtensionProviderTypePrefix+providerType, "true")

	case security.Kind("WorkloadIdentity"):
		workloadIdentity, ok := a.GetObject().(*security.WorkloadIdentity)
		if !ok {
			return apierrors.NewBadRequest("could not convert resource into WorkloadIdentity object")
		}

		removeLabels(&workloadIdentity.ObjectMeta)
		providerType := workloadIdentity.Spec.TargetSystem.Type
		metav1.SetMetaDataLabel(&workloadIdentity.ObjectMeta, v1beta1constants.LabelExtensionProviderTypePrefix+providerType, "true")

	case core.Kind("Shoot"):
		shoot, ok := a.GetObject().(*core.Shoot)
		if !ok {
			return apierrors.NewBadRequest("could not convert resource into Shoot object")
		}

		controllerRegistrations, err := e.controllerRegistrationLister.List(labels.Everything())
		if err != nil {
			return apierrors.NewInternalError(err)
		}

		removeLabels(&shoot.ObjectMeta)
		if err := addMetaDataLabelsShoot(shoot, controllerRegistrations); err != nil {
			return fmt.Errorf("failed to add metadata labels to shoot: %w", err)
		}

	case core.Kind("CloudProfile"):
		cloudProfile, ok := a.GetObject().(*core.CloudProfile)
		if !ok {
			return apierrors.NewBadRequest("could not convert resource into CloudProfile object")
		}

		removeLabels(&cloudProfile.ObjectMeta)
		addMetaDataLabelsCloudProfile(cloudProfile)

	case core.Kind("NamespacedCloudProfile"):
		namespacedCloudProfile, ok := a.GetObject().(*core.NamespacedCloudProfile)
		if !ok {
			return apierrors.NewBadRequest("could not convert resource into NamespacedCloudProfile object")
		}

		if namespacedCloudProfile.Spec.Parent.Kind != v1beta1constants.CloudProfileReferenceKindCloudProfile {
			return apierrors.NewBadRequest("invalid parent kind")
		}
		parentCloudProfile, err := e.cloudProfileLister.Get(namespacedCloudProfile.Spec.Parent.Name)
		if err != nil {
			return apierrors.NewInternalError(err)
		}

		removeLabels(&namespacedCloudProfile.ObjectMeta)
		addMetaDataLabelsNamespacedCloudProfile(namespacedCloudProfile, parentCloudProfile)

	case core.Kind("BackupBucket"):
		backupBucket, ok := a.GetObject().(*core.BackupBucket)
		if !ok {
			return apierrors.NewBadRequest("could not convert object into BackupBucket object")
		}

		removeLabels(&backupBucket.ObjectMeta)
		addMetaDataLabelsBackupBucket(backupBucket)

	case core.Kind("BackupEntry"):
		backupEntry, ok := a.GetObject().(*core.BackupEntry)
		if !ok {
			return apierrors.NewBadRequest("could not convert object into BackupEntry object")
		}

		backupBucket, err := e.backupBucketLister.Get(backupEntry.Spec.BucketName)
		if err != nil {
			return apierrors.NewInternalError(err)
		}

		removeLabels(&backupEntry.ObjectMeta)
		addMetaDataLabelsBackupEntry(backupEntry, backupBucket)
	}
	return nil
}

func addMetaDataLabelsSeed(seed *core.Seed) {
	metav1.SetMetaDataLabel(&seed.ObjectMeta, v1beta1constants.LabelExtensionProviderTypePrefix+seed.Spec.Provider.Type, "true")
	if seed.Spec.Backup != nil {
		metav1.SetMetaDataLabel(&seed.ObjectMeta, v1beta1constants.LabelExtensionProviderTypePrefix+seed.Spec.Backup.Provider, "true")
	}

	if seed.Spec.DNS.Provider != nil {
		metav1.SetMetaDataLabel(&seed.ObjectMeta, v1beta1constants.LabelExtensionDNSRecordTypePrefix+seed.Spec.DNS.Provider.Type, "true")
	}
}

func addMetaDataLabelsSecretBinding(secretBinding *core.SecretBinding) {
	types := gardencorehelper.GetSecretBindingTypes(secretBinding)
	for _, t := range types {
		metav1.SetMetaDataLabel(&secretBinding.ObjectMeta, v1beta1constants.LabelExtensionProviderTypePrefix+t, "true")
	}
}

func addMetaDataLabelsShoot(shoot *core.Shoot, controllerRegistrations []*gardencorev1beta1.ControllerRegistration) error {
	v1beta1Shoot := &gardencorev1beta1.Shoot{}
	if err := kubernetes.GardenScheme.Convert(shoot, v1beta1Shoot, nil); err != nil {
		return fmt.Errorf("could not convert Shoot to v1beta1.Shoot: %v", err)
	}

	controllerRegistrationList := &gardencorev1beta1.ControllerRegistrationList{
		Items: slices.Collect[gardencorev1beta1.ControllerRegistration](func(yield func(gardencorev1beta1.ControllerRegistration) bool) {
			for _, registration := range controllerRegistrations {
				if !yield(*registration) {
					return
				}
			}
		}),
	}

	for extensionType := range gardenerutils.ComputeEnabledTypesForKindExtension(v1beta1Shoot, controllerRegistrationList) {
		metav1.SetMetaDataLabel(&shoot.ObjectMeta, v1beta1constants.LabelExtensionExtensionTypePrefix+extensionType, "true")
	}

	for _, pool := range shoot.Spec.Provider.Workers {
		if pool.CRI != nil {
			for _, cr := range pool.CRI.ContainerRuntimes {
				metav1.SetMetaDataLabel(&shoot.ObjectMeta, v1beta1constants.LabelExtensionContainerRuntimeTypePrefix+cr.Type, "true")
			}
		}
		if pool.Machine.Image != nil {
			metav1.SetMetaDataLabel(&shoot.ObjectMeta, v1beta1constants.LabelExtensionOperatingSystemConfigTypePrefix+pool.Machine.Image.Name, "true")
		}
	}

	if shoot.Spec.DNS != nil {
		for _, provider := range shoot.Spec.DNS.Providers {
			if provider.Type == nil || *provider.Type == core.DNSUnmanaged {
				continue
			}
			metav1.SetMetaDataLabel(&shoot.ObjectMeta, v1beta1constants.LabelExtensionDNSRecordTypePrefix+*provider.Type, "true")
		}
	}

	metav1.SetMetaDataLabel(&shoot.ObjectMeta, v1beta1constants.LabelExtensionProviderTypePrefix+shoot.Spec.Provider.Type, "true")

	if shoot.Spec.Networking != nil && shoot.Spec.Networking.Type != nil {
		metav1.SetMetaDataLabel(&shoot.ObjectMeta, v1beta1constants.LabelExtensionNetworkingTypePrefix+*shoot.Spec.Networking.Type, "true")
	}

	return nil
}

func addMetaDataLabelsCloudProfile(cloudProfile *core.CloudProfile) {
	metav1.SetMetaDataLabel(&cloudProfile.ObjectMeta, v1beta1constants.LabelExtensionProviderTypePrefix+cloudProfile.Spec.Type, "true")
}

func addMetaDataLabelsNamespacedCloudProfile(namespacedCloudProfile *core.NamespacedCloudProfile, parentCloudProfile *gardencorev1beta1.CloudProfile) {
	metav1.SetMetaDataLabel(&namespacedCloudProfile.ObjectMeta, v1beta1constants.LabelExtensionProviderTypePrefix+parentCloudProfile.Spec.Type, "true")
}

func addMetaDataLabelsBackupBucket(backupBucket *core.BackupBucket) {
	metav1.SetMetaDataLabel(&backupBucket.ObjectMeta, v1beta1constants.LabelExtensionProviderTypePrefix+backupBucket.Spec.Provider.Type, "true")
}

func addMetaDataLabelsBackupEntry(backupEntry *core.BackupEntry, backupBucket *gardencorev1beta1.BackupBucket) {
	metav1.SetMetaDataLabel(&backupEntry.ObjectMeta, v1beta1constants.LabelExtensionProviderTypePrefix+backupBucket.Spec.Provider.Type, "true")
}

func removeLabels(objectMeta *metav1.ObjectMeta) {
	extensionLabels := []string{
		v1beta1constants.LabelExtensionExtensionTypePrefix,
		v1beta1constants.LabelExtensionProviderTypePrefix,
		v1beta1constants.LabelExtensionDNSRecordTypePrefix,
		v1beta1constants.LabelExtensionNetworkingTypePrefix,
		v1beta1constants.LabelExtensionOperatingSystemConfigTypePrefix,
		v1beta1constants.LabelExtensionContainerRuntimeTypePrefix,
	}
	for k := range objectMeta.Labels {
		for _, label := range extensionLabels {
			if strings.HasPrefix(k, label) {
				delete(objectMeta.Labels, k)
			}
		}
	}
}
