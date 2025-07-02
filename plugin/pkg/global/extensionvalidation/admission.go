// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package extensionvalidation

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"

	"github.com/hashicorp/go-multierror"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorehelper "github.com/gardener/gardener/pkg/apis/core/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	admissioninitializer "github.com/gardener/gardener/pkg/apiserver/admission/initializer"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	gardencorev1beta1listers "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"
	plugin "github.com/gardener/gardener/plugin/pkg"
)

// Register registers a plugin.
func Register(plugins *admission.Plugins) {
	plugins.Register(plugin.PluginNameExtensionValidator, NewFactory)
}

// NewFactory creates a new PluginFactory.
func NewFactory(_ io.Reader) (admission.Interface, error) {
	return New()
}

// ExtensionValidator contains listers and admission handler.
type ExtensionValidator struct {
	*admission.Handler

	controllerRegistrationLister gardencorev1beta1listers.ControllerRegistrationLister
	backupBucketLister           gardencorev1beta1listers.BackupBucketLister
	readyFunc                    admission.ReadyFunc
}

var (
	_ = admissioninitializer.WantsCoreInformerFactory(&ExtensionValidator{})

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

// SetCoreInformerFactory gets Lister from SharedInformerFactory.
func (e *ExtensionValidator) SetCoreInformerFactory(f gardencoreinformers.SharedInformerFactory) {
	controllerRegistrationInformer := f.Core().V1beta1().ControllerRegistrations()
	e.controllerRegistrationLister = controllerRegistrationInformer.Lister()

	backupBucketInformer := f.Core().V1beta1().BackupBuckets()
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
func (e *ExtensionValidator) Validate(_ context.Context, a admission.Attributes, _ admission.ObjectInterfaces) error {
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
		kindToExtensions = computeRegisteredPrimaryExtensionKindTypes(controllerRegistrationList)
		validationError  error
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
			validationError = e.validateBackupBucket(kindToExtensions, backupBucket.Spec)
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
			validationError = e.validateBackupEntry(kindToExtensions, backupBucket.Spec.Provider.Type)
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
			validationError = e.validateSeed(kindToExtensions, seed.Spec)
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
			validationError = e.validateShoot(kindToExtensions, computeWorkerlessSupportedExtensionTypes(controllerRegistrationList), shoot.Spec, gardencorehelper.IsWorkerless(shoot))
		}
	}

	if validationError != nil {
		return admission.NewForbidden(a, validationError)
	}

	return nil
}

func (e *ExtensionValidator) validateBackupBucket(kindToExtensions map[string][]extension, spec core.BackupBucketSpec) error {
	return isExtensionRegistered(
		kindToExtensions,
		extensionsv1alpha1.BackupBucketResource,
		spec.Provider.Type,
		"given BackupBucket uses non-registered provider type",
		field.NewPath("spec", "provider", "type"),
		nil,
	)
}

func (e *ExtensionValidator) validateBackupEntry(kindToExtensions map[string][]extension, bucketType string) error {
	return isExtensionRegistered(
		kindToExtensions,
		extensionsv1alpha1.BackupEntryResource,
		bucketType,
		"given BackupEntry references bucket using non-registered provider type",
		field.NewPath("spec", "bucketName"),
		nil,
	)
}

func (e *ExtensionValidator) validateSeed(kindToExtensions map[string][]extension, spec core.SeedSpec) error {
	var (
		message = "given Seed uses non-registered"

		requiredExtensions = requiredExtensions{
			{extensionsv1alpha1.ControlPlaneResource, spec.Provider.Type, fmt.Sprintf("%s provider type", message), field.NewPath("spec", "provider", "type")},
		}
	)

	if spec.Backup != nil {
		msg := fmt.Sprintf("%s backup provider type", message)
		requiredExtensions = append(
			requiredExtensions,
			requiredExtension{extensionsv1alpha1.BackupBucketResource, spec.Backup.Provider, msg, field.NewPath("spec", "backup", "provider")},
			requiredExtension{extensionsv1alpha1.BackupEntryResource, spec.Backup.Provider, msg, field.NewPath("spec", "backup", "provider")},
		)
	}

	if spec.Ingress != nil && spec.DNS.Provider != nil {
		provider := spec.DNS.Provider
		requiredExtensions = append(requiredExtensions, requiredExtension{extensionsv1alpha1.DNSRecordResource, provider.Type, fmt.Sprintf("%s extension type", message), field.NewPath("spec", "dns", "provider").Child("type")})
	}

	for i, ext := range spec.Extensions {
		requiredExtensions = append(requiredExtensions, requiredExtension{extensionsv1alpha1.ExtensionResource, ext.Type, fmt.Sprintf("%s extension type", message), field.NewPath("spec", "extensions").Index(i).Child("type")})
	}

	return requiredExtensions.areRegistered(kindToExtensions, gardencorev1beta1.ClusterTypeSeed)
}

func (e *ExtensionValidator) validateShoot(kindToExtensions map[string][]extension, workerlessSupportedExtensionTypes sets.Set[string], spec core.ShootSpec, workerless bool) error {
	var (
		message            = "given Shoot uses non-registered"
		providerTypeMsg    = fmt.Sprintf("%s provider type", message)
		fldPath            = field.NewPath("spec", "provider", "type")
		requiredExtensions = requiredExtensions{}
		result             error
	)

	if !workerless {
		requiredExtensions = append(requiredExtensions,
			requiredExtension{extensionsv1alpha1.ControlPlaneResource, spec.Provider.Type, providerTypeMsg, fldPath},
			requiredExtension{extensionsv1alpha1.InfrastructureResource, spec.Provider.Type, providerTypeMsg, fldPath},
			requiredExtension{extensionsv1alpha1.WorkerResource, spec.Provider.Type, providerTypeMsg, fldPath},
		)
		if spec.Networking != nil && spec.Networking.Type != nil {
			requiredExtensions = append(requiredExtensions, requiredExtension{extensionsv1alpha1.NetworkResource, *spec.Networking.Type, fmt.Sprintf("%s networking type", message), field.NewPath("spec", "networking", "type")})
		}
	}

	if spec.DNS != nil {
		for i, provider := range spec.DNS.Providers {
			if provider.Type == nil || *provider.Type == core.DNSUnmanaged {
				continue
			}

			if provider.Primary != nil && *provider.Primary {
				requiredExtensions = append(requiredExtensions, requiredExtension{extensionsv1alpha1.DNSRecordResource, *provider.Type, fmt.Sprintf("%s extension type", message), field.NewPath("spec", "dns", "providers").Index(i).Child("type")})
			}
		}
	}

	for i, ext := range spec.Extensions {
		requiredExtensions = append(requiredExtensions, requiredExtension{extensionsv1alpha1.ExtensionResource, ext.Type, fmt.Sprintf("%s extension type", message), field.NewPath("spec", "extensions").Index(i).Child("type")})
	}

	for i, worker := range spec.Provider.Workers {
		if worker.CRI != nil {
			for j, cr := range worker.CRI.ContainerRuntimes {
				requiredExtensions = append(requiredExtensions, requiredExtension{extensionsv1alpha1.ContainerRuntimeResource, cr.Type, fmt.Sprintf("%s container runtime type", message), field.NewPath("spec", "provider", "workers").Index(i).Child("cri", "containerRuntimes").Index(j).Child("type")})
			}
		}

		if worker.Machine.Image == nil {
			continue
		}

		requiredExtensions = append(requiredExtensions, requiredExtension{extensionsv1alpha1.OperatingSystemConfigResource, worker.Machine.Image.Name, fmt.Sprintf("%s operating system type", message), field.NewPath("spec", "provider", "workers").Index(i).Child("machine", "image", "name")})
	}

	if err := requiredExtensions.areRegistered(kindToExtensions, gardencorev1beta1.ClusterTypeShoot); err != nil {
		result = multierror.Append(result, err)
	}

	if workerless {
		if err := requiredExtensions.areSupportedForWorkerlessShoots(workerlessSupportedExtensionTypes); err != nil {
			result = multierror.Append(result, err)
		}
	}

	return result
}

// Helper functions

type requiredExtension struct {
	extensionKind string
	extensionType string
	message       string
	fldPath       *field.Path
}

type requiredExtensions []requiredExtension

func (r requiredExtensions) areRegistered(kindToExtensions map[string][]extension, clusterType gardencorev1beta1.ClusterType) error {
	var result error

	for _, requiredExtension := range r {
		if err := isExtensionRegistered(kindToExtensions, requiredExtension.extensionKind, requiredExtension.extensionType, requiredExtension.message, requiredExtension.fldPath, &clusterType); err != nil {
			result = multierror.Append(result, err)
		}
	}

	return result
}

// isExtensionRegistered takes a map of registered kinds to a set of types and a kind/type to verify. If the provided
// kind/type combination is registered then it returns nil, otherwise it returns an error with the given message.
func isExtensionRegistered(kindToExtensions map[string][]extension, extensionKind, extensionType, message string, fldPath *field.Path, clusterType *gardencorev1beta1.ClusterType) error {
	if !slices.ContainsFunc(kindToExtensions[extensionKind], func(ext extension) bool {
		if clusterType != nil && len(ext.clusterCompatibility) > 0 && !sets.New(ext.clusterCompatibility...).Has(*clusterType) {
			return false
		}
		return ext.extensionType == extensionType
	}) {
		return fmt.Errorf("%s: %s (%q)", message, fldPath, extensionType)
	}
	return nil
}

type extension struct {
	extensionType        string
	clusterCompatibility []gardencorev1beta1.ClusterType
}

// computeRegisteredPrimaryExtensionKindTypes computes a map that maps the extension kind to the set of types that are
// registered (only if primary=true), e.g. {ControlPlane=>{foo,bar,baz}, Network=>{a,b,c}}.
func computeRegisteredPrimaryExtensionKindTypes(controllerRegistrationList []*gardencorev1beta1.ControllerRegistration) map[string][]extension {
	out := map[string][]extension{}

	for _, controllerRegistration := range controllerRegistrationList {
		for _, resource := range controllerRegistration.Spec.Resources {
			if resource.Primary != nil && !*resource.Primary {
				continue
			}

			out[resource.Kind] = append(out[resource.Kind], extension{extensionType: resource.Type, clusterCompatibility: resource.ClusterCompatibility})
		}
	}

	return out
}

// computeWorkerlessSupportedExtensionTypes computes Extension types that are supported for workerless Shoots.
func computeWorkerlessSupportedExtensionTypes(controllerRegistrationList []*gardencorev1beta1.ControllerRegistration) sets.Set[string] {
	out := sets.Set[string]{}

	for _, controllerRegistration := range controllerRegistrationList {
		for _, resource := range controllerRegistration.Spec.Resources {
			if resource.Kind != extensionsv1alpha1.ExtensionResource || !ptr.Deref(resource.WorkerlessSupported, false) {
				continue
			}

			out.Insert(resource.Type)
		}
	}

	return out
}

func (r requiredExtensions) areSupportedForWorkerlessShoots(workerlessSupportedExtensionTypes sets.Set[string]) error {
	var result error

	for _, requiredExtension := range r {
		if requiredExtension.extensionKind != extensionsv1alpha1.ExtensionResource {
			continue
		}

		if !workerlessSupportedExtensionTypes.Has(requiredExtension.extensionType) {
			result = multierror.Append(result, fmt.Errorf("given Shoot is workerless and uses non-supported extension type: %s (%q)", requiredExtension.fldPath, requiredExtension.extensionType))
		}
	}

	return result
}
