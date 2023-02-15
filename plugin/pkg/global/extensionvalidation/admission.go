// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package extensionvalidation

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/hashicorp/go-multierror"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/admission"

	"github.com/gardener/gardener/pkg/apis/core"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	admissioninitializer "github.com/gardener/gardener/pkg/apiserver/admission/initializer"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/internalversion"
	gardencorelisters "github.com/gardener/gardener/pkg/client/core/listers/core/internalversion"
)

const (
	// PluginName is the name of this admission plugin.
	PluginName = "ExtensionValidator"
)

// Register registers a plugin.
func Register(plugins *admission.Plugins) {
	plugins.Register(PluginName, NewFactory)
}

// NewFactory creates a new PluginFactory.
func NewFactory(config io.Reader) (admission.Interface, error) {
	return New()
}

// ExtensionValidator contains listers and admission handler.
type ExtensionValidator struct {
	*admission.Handler
	controllerRegistrationLister gardencorelisters.ControllerRegistrationLister
	backupBucketLister           gardencorelisters.BackupBucketLister
	readyFunc                    admission.ReadyFunc
}

var (
	_ = admissioninitializer.WantsInternalCoreInformerFactory(&ExtensionValidator{})

	readyFuncs []admission.ReadyFunc
)

// New creates a new ExtensionValidator admission plugin.
func New() (*ExtensionValidator, error) {
	return &ExtensionValidator{
		Handler: admission.NewHandler(admission.Create, admission.Update),
	}, nil
}

// AssignReadyFunc assigns the ready function to the admission handler.
func (e *ExtensionValidator) AssignReadyFunc(f admission.ReadyFunc) {
	e.readyFunc = f
	e.SetReadyFunc(f)
}

// SetInternalCoreInformerFactory gets Lister from SharedInformerFactory.
func (e *ExtensionValidator) SetInternalCoreInformerFactory(f gardencoreinformers.SharedInformerFactory) {
	controllerRegistrationInformer := f.Core().InternalVersion().ControllerRegistrations()
	e.controllerRegistrationLister = controllerRegistrationInformer.Lister()

	backupBucketInformer := f.Core().InternalVersion().BackupBuckets()
	e.backupBucketLister = backupBucketInformer.Lister()

	readyFuncs = append(readyFuncs, controllerRegistrationInformer.Informer().HasSynced, backupBucketInformer.Informer().HasSynced)
}

func (e *ExtensionValidator) waitUntilReady(attrs admission.Attributes) error {
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
func (e *ExtensionValidator) ValidateInitialization() error {
	if e.controllerRegistrationLister == nil {
		return errors.New("missing ControllerRegistration lister")
	}
	if e.backupBucketLister == nil {
		return errors.New("missing BackupBucket lister")
	}
	return nil
}

var _ admission.ValidationInterface = &ExtensionValidator{}

// Validate makes admissions decisions based on the extension types.
func (e *ExtensionValidator) Validate(ctx context.Context, a admission.Attributes, o admission.ObjectInterfaces) error {
	if err := e.waitUntilReady(a); err != nil {
		return fmt.Errorf("err while waiting for ready %w", err)
	}

	switch a.GetKind().GroupKind() {
	case core.Kind("BackupBucket"), core.Kind("BackupEntry"), core.Kind("Seed"), core.Kind("Shoot"):
	default:
		return nil
	}

	controllerRegistrationList, err := e.controllerRegistrationLister.List(labels.Everything())
	if err != nil {
		return err
	}

	var (
		kindToTypesMap  = computeRegisteredPrimaryExtensionKindTypes(controllerRegistrationList)
		validationError error
	)

	switch a.GetKind().GroupKind() {
	case core.Kind("BackupBucket"):
		backupBucket, ok := a.GetObject().(*core.BackupBucket)
		if !ok {
			return apierrors.NewBadRequest("could not convert object into BackupBucket object")
		}

		oldBackupBucket := &core.BackupBucket{}
		if oldObj := a.GetOldObject(); oldObj != nil {
			oldBackupBucket, ok = oldObj.(*core.BackupBucket)
			if !ok {
				return apierrors.NewBadRequest("could not convert old object into BackupBucket object")
			}
		}

		if !apiequality.Semantic.DeepEqual(backupBucket.Spec, oldBackupBucket.Spec) {
			validationError = e.validateBackupBucket(kindToTypesMap, backupBucket.Spec)
		}

	case core.Kind("BackupEntry"):
		backupEntry, ok := a.GetObject().(*core.BackupEntry)
		if !ok {
			return apierrors.NewBadRequest("could not convert object into BackupEntry object")
		}

		oldBackupEntry := &core.BackupEntry{}
		if oldObj := a.GetOldObject(); oldObj != nil {
			oldBackupEntry, ok = oldObj.(*core.BackupEntry)
			if !ok {
				return apierrors.NewBadRequest("could not convert old object into BackupEntry object")
			}
		}

		if !apiequality.Semantic.DeepEqual(backupEntry.Spec, oldBackupEntry.Spec) {
			backupBucket, err := e.backupBucketLister.Get(backupEntry.Spec.BucketName)
			if err != nil {
				return err
			}
			validationError = e.validateBackupEntry(kindToTypesMap, backupBucket.Spec.Provider.Type)
		}

	case core.Kind("Seed"):
		seed, ok := a.GetObject().(*core.Seed)
		if !ok {
			return apierrors.NewBadRequest("could not convert object into Seed object")
		}

		oldSeed := &core.Seed{}
		if oldObj := a.GetOldObject(); oldObj != nil {
			oldSeed, ok = oldObj.(*core.Seed)
			if !ok {
				return apierrors.NewBadRequest("could not convert old object into Seed object")
			}
		}

		if !apiequality.Semantic.DeepEqual(seed.Spec, oldSeed.Spec) {
			validationError = e.validateSeed(kindToTypesMap, seed.Spec)
		}

	case core.Kind("Shoot"):
		shoot, ok := a.GetObject().(*core.Shoot)
		if !ok {
			return apierrors.NewBadRequest("could not convert object into Shoot object")
		}

		oldShoot := &core.Shoot{}
		if oldObj := a.GetOldObject(); oldObj != nil {
			oldShoot, ok = oldObj.(*core.Shoot)
			if !ok {
				return apierrors.NewBadRequest("could not convert old object into Shoot object")
			}
		}

		if !apiequality.Semantic.DeepEqual(shoot.Spec, oldShoot.Spec) {
			validationError = e.validateShoot(kindToTypesMap, shoot.Spec)
		}
	}

	if validationError != nil {
		return admission.NewForbidden(a, validationError)
	}

	return nil
}

func (e *ExtensionValidator) validateBackupBucket(kindToTypesMap map[string]sets.Set[string], spec core.BackupBucketSpec) error {
	return isExtensionRegistered(
		kindToTypesMap,
		extensionsv1alpha1.BackupBucketResource,
		spec.Provider.Type,
		"given BackupBucket uses non-registered provider type: "+field.NewPath("spec", "provider", "type").String(),
	)
}

func (e *ExtensionValidator) validateBackupEntry(kindToTypesMap map[string]sets.Set[string], bucketType string) error {
	return isExtensionRegistered(
		kindToTypesMap,
		extensionsv1alpha1.BackupEntryResource,
		bucketType,
		fmt.Sprintf("given BackupEntry references bucket (%s) using non-registered provider type", field.NewPath("spec", "bucketName")),
	)
}

func (e *ExtensionValidator) validateSeed(kindToTypesMap map[string]sets.Set[string], spec core.SeedSpec) error {
	var (
		message = "given Seed uses non-registered"

		requiredExtensions = requiredExtensions{
			{extensionsv1alpha1.ControlPlaneResource, spec.Provider.Type, fmt.Sprintf("%s provider type: %s", message, field.NewPath("spec", "provider", "type"))},
		}
	)

	if spec.Backup != nil {
		msg := fmt.Sprintf("%s backup provider type: %s", message, field.NewPath("spec", "backup", "provider"))
		requiredExtensions = append(
			requiredExtensions,
			requiredExtension{extensionsv1alpha1.BackupBucketResource, spec.Backup.Provider, msg},
			requiredExtension{extensionsv1alpha1.BackupEntryResource, spec.Backup.Provider, msg},
		)
	}

	if spec.Ingress != nil && spec.DNS.Provider != nil {
		provider := spec.DNS.Provider
		requiredExtensions = append(requiredExtensions, requiredExtension{extensionsv1alpha1.DNSRecordResource, provider.Type, fmt.Sprintf("%s extension type: %s", message, field.NewPath("spec", "dns", "provider").Child("type"))})
	}

	return requiredExtensions.areRegistered(kindToTypesMap)
}

func (e *ExtensionValidator) validateShoot(kindToTypesMap map[string]sets.Set[string], spec core.ShootSpec) error {
	var (
		message         = "given Shoot uses non-registered"
		providerTypeMsg = fmt.Sprintf("%s provider type: %s", message, field.NewPath("spec", "provider", "type"))

		requiredExtensions = requiredExtensions{
			{extensionsv1alpha1.ControlPlaneResource, spec.Provider.Type, providerTypeMsg},
			{extensionsv1alpha1.InfrastructureResource, spec.Provider.Type, providerTypeMsg},
			{extensionsv1alpha1.WorkerResource, spec.Provider.Type, providerTypeMsg},
			{extensionsv1alpha1.NetworkResource, spec.Networking.Type, fmt.Sprintf("%s networking type: %s", message, field.NewPath("spec", "networking", "type"))},
		}
	)

	if spec.DNS != nil {
		for i, provider := range spec.DNS.Providers {
			if provider.Type == nil || *provider.Type == core.DNSUnmanaged {
				continue
			}

			if provider.Primary != nil && *provider.Primary {
				requiredExtensions = append(requiredExtensions, requiredExtension{extensionsv1alpha1.DNSRecordResource, *provider.Type, fmt.Sprintf("%s extension type: %s", message, field.NewPath("spec", "dns", "providers").Index(i).Child("type"))})
			}
		}
	}

	for i, extension := range spec.Extensions {
		requiredExtensions = append(requiredExtensions, requiredExtension{extensionsv1alpha1.ExtensionResource, extension.Type, fmt.Sprintf("%s extension type: %s", message, field.NewPath("spec", "extensions").Index(i).Child("type"))})
	}

	for i, worker := range spec.Provider.Workers {
		if worker.CRI != nil {
			for j, cr := range worker.CRI.ContainerRuntimes {
				requiredExtensions = append(requiredExtensions, requiredExtension{extensionsv1alpha1.ContainerRuntimeResource, cr.Type, fmt.Sprintf("%s container runtime type: %s", message, field.NewPath("spec", "provider", "workers").Index(i).Child("cri", "containerRuntimes").Index(j).Child("type"))})
			}
		}

		if worker.Machine.Image == nil {
			continue
		}

		requiredExtensions = append(requiredExtensions, requiredExtension{extensionsv1alpha1.OperatingSystemConfigResource, worker.Machine.Image.Name, fmt.Sprintf("%s operating system type: %s", message, field.NewPath("spec", "provider", "workers").Index(i).Child("machine", "image", "name"))})
	}

	return requiredExtensions.areRegistered(kindToTypesMap)
}

// Helper functions

type requiredExtension struct {
	extensionKind string
	extensionType string
	message       string
}

type requiredExtensions []requiredExtension

func (r requiredExtensions) areRegistered(kindToTypesMap map[string]sets.Set[string]) error {
	var result error

	for _, requiredExtension := range r {
		if err := isExtensionRegistered(kindToTypesMap, requiredExtension.extensionKind, requiredExtension.extensionType, requiredExtension.message); err != nil {
			result = multierror.Append(result, err)
		}
	}

	return result
}

// isExtensionRegistered takes a map of registered kinds to a set of types and a kind/type to verify. If the provided
// kind/type combination is registered then it returns nil, otherwise it returns an error with the given message.
func isExtensionRegistered(kindToTypesMap map[string]sets.Set[string], extensionKind, extensionType, message string) error {
	if types, ok := kindToTypesMap[extensionKind]; !ok || !types.Has(extensionType) {
		return fmt.Errorf("%s (%q)", message, extensionType)
	}
	return nil
}

// computeRegisteredPrimaryExtensionKindTypes computes a map that maps the extension kind to the set of types that are
// registered (only if primary=true), e.g. {ControlPlane=>{foo,bar,baz}, Network=>{a,b,c}}.
func computeRegisteredPrimaryExtensionKindTypes(controllerRegistrationList []*core.ControllerRegistration) map[string]sets.Set[string] {
	out := map[string]sets.Set[string]{}

	for _, controllerRegistration := range controllerRegistrationList {
		for _, resource := range controllerRegistration.Spec.Resources {
			if resource.Primary != nil && !*resource.Primary {
				continue
			}

			if _, ok := out[resource.Kind]; !ok {
				out[resource.Kind] = sets.New[string]()
			}

			out[resource.Kind].Insert(resource.Type)
		}
	}

	return out
}
