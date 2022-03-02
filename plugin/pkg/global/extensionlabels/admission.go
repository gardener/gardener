// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package extensionlabels

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/gardener/gardener/pkg/apis/core"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	admissioninitializer "github.com/gardener/gardener/pkg/apiserver/admission/initializer"
	internalcoreinformers "github.com/gardener/gardener/pkg/client/core/informers/internalversion"
	corelisters "github.com/gardener/gardener/pkg/client/core/listers/core/internalversion"
	"github.com/gardener/gardener/pkg/features"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/admission"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
)

const (
	// PluginName is the name of this admission plugin.
	PluginName = "ExtensionLabels"
)

// Register registers a plugin.
func Register(plugins *admission.Plugins) {
	plugins.Register(PluginName, NewFactory)
}

// NewFactory creates a new PluginFactory.
func NewFactory(config io.Reader) (admission.Interface, error) {
	return New()
}

// ExtensionLabels contains the admission handler
type ExtensionLabels struct {
	*admission.Handler
	backupBucketLister corelisters.BackupBucketLister
	readyFunc          admission.ReadyFunc
}

var (
	_          = admissioninitializer.WantsInternalCoreInformerFactory(&ExtensionLabels{})
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

// SetInternalCoreInformerFactory sets the external garden core informer factory.
func (e *ExtensionLabels) SetInternalCoreInformerFactory(f internalcoreinformers.SharedInformerFactory) {
	backupBucketInformer := f.Core().InternalVersion().BackupBuckets()
	e.backupBucketLister = backupBucketInformer.Lister()

	readyFuncs = append(readyFuncs, backupBucketInformer.Informer().HasSynced)
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
	return nil
}

var _ admission.MutationInterface = &ExtensionLabels{}

// Admit add extension labels to resources.
func (e *ExtensionLabels) Admit(ctx context.Context, a admission.Attributes, o admission.ObjectInterfaces) error {
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

	case core.Kind("Shoot"):
		shoot, ok := a.GetObject().(*core.Shoot)
		if !ok {
			return apierrors.NewBadRequest("could not convert resource into Shoot object")
		}

		removeLabels(&shoot.ObjectMeta)
		addMetaDataLabelsShoot(shoot)

	case core.Kind("CloudProfile"):
		cloudProfile, ok := a.GetObject().(*core.CloudProfile)
		if !ok {
			return apierrors.NewBadRequest("could not convert resource into CloudProfile object")
		}

		removeLabels(&cloudProfile.ObjectMeta)
		addMetaDataLabelsCloudProfile(cloudProfile)

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
			return err
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

	if seed.Spec.DNS.Provider != nil && utilfeature.DefaultFeatureGate.Enabled(features.UseDNSRecords) {
		metav1.SetMetaDataLabel(&seed.ObjectMeta, v1beta1constants.LabelExtensionDNSRecordTypePrefix+seed.Spec.DNS.Provider.Type, "true")
	}
}

func addMetaDataLabelsShoot(shoot *core.Shoot) {
	for _, extension := range shoot.Spec.Extensions {
		if extension.Disabled != nil && *extension.Disabled {
			continue
		}
		metav1.SetMetaDataLabel(&shoot.ObjectMeta, v1beta1constants.LabelExtensionExtensionTypePrefix+extension.Type, "true")
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
			if utilfeature.DefaultFeatureGate.Enabled(features.UseDNSRecords) {
				metav1.SetMetaDataLabel(&shoot.ObjectMeta, v1beta1constants.LabelExtensionDNSRecordTypePrefix+*provider.Type, "true")
			}
		}
	}
	metav1.SetMetaDataLabel(&shoot.ObjectMeta, v1beta1constants.LabelExtensionProviderTypePrefix+shoot.Spec.Provider.Type, "true")
	metav1.SetMetaDataLabel(&shoot.ObjectMeta, v1beta1constants.LabelExtensionNetworkingTypePrefix+shoot.Spec.Networking.Type, "true")
}

func addMetaDataLabelsCloudProfile(cloudProfile *core.CloudProfile) {
	metav1.SetMetaDataLabel(&cloudProfile.ObjectMeta, v1beta1constants.LabelExtensionProviderTypePrefix+cloudProfile.Spec.Type, "true")
}

func addMetaDataLabelsBackupBucket(backupBucket *core.BackupBucket) {
	metav1.SetMetaDataLabel(&backupBucket.ObjectMeta, v1beta1constants.LabelExtensionProviderTypePrefix+backupBucket.Spec.Provider.Type, "true")
}

func addMetaDataLabelsBackupEntry(backupEntry *core.BackupEntry, backupBucket *core.BackupBucket) {
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
