// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validator

import (
	"context"
	"errors"
	"fmt"
	"io"
	"reflect"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/extensions/pkg/util"
	"github.com/gardener/gardener/pkg/api"
	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorehelper "github.com/gardener/gardener/pkg/apis/core/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/apis/core/validation"
	admissioninitializer "github.com/gardener/gardener/pkg/apiserver/admission/initializer"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	gardencorev1beta1listers "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"
	"github.com/gardener/gardener/pkg/controllermanager/controller/namespacedcloudprofile"
	"github.com/gardener/gardener/pkg/utils"
	plugin "github.com/gardener/gardener/plugin/pkg"
)

// Register registers a plugin.
func Register(plugins *admission.Plugins) {
	plugins.Register(plugin.PluginNameNamespacedCloudProfileValidator, func(_ io.Reader) (admission.Interface, error) {
		return New()
	})
}

// ValidateNamespacedCloudProfile contains listers and admission handler.
type ValidateNamespacedCloudProfile struct {
	*admission.Handler
	cloudProfileLister gardencorev1beta1listers.CloudProfileLister
	readyFunc          admission.ReadyFunc
}

var (
	_          = admissioninitializer.WantsCoreInformerFactory(&ValidateNamespacedCloudProfile{})
	readyFuncs []admission.ReadyFunc
)

// New creates a new ValidateNamespacedCloudProfile admission plugin.
func New() (*ValidateNamespacedCloudProfile, error) {
	return &ValidateNamespacedCloudProfile{
		Handler: admission.NewHandler(admission.Create, admission.Update),
	}, nil
}

// AssignReadyFunc assigns the ready function to the admission handler.
func (v *ValidateNamespacedCloudProfile) AssignReadyFunc(f admission.ReadyFunc) {
	v.readyFunc = f
	v.SetReadyFunc(f)
}

// SetCoreInformerFactory gets Lister from SharedInformerFactory.
func (v *ValidateNamespacedCloudProfile) SetCoreInformerFactory(f gardencoreinformers.SharedInformerFactory) {
	cloudProfileInformer := f.Core().V1beta1().CloudProfiles()
	v.cloudProfileLister = cloudProfileInformer.Lister()

	readyFuncs = append(readyFuncs, cloudProfileInformer.Informer().HasSynced)
}

// ValidateInitialization checks whether the plugin was correctly initialized.
func (v *ValidateNamespacedCloudProfile) ValidateInitialization() error {
	if v.cloudProfileLister == nil {
		return errors.New("missing cloudProfile lister")
	}
	return nil
}

var _ admission.ValidationInterface = &ValidateNamespacedCloudProfile{}

// Validate validates the NamespacedCloudProfile.
func (v *ValidateNamespacedCloudProfile) Validate(ctx context.Context, a admission.Attributes, _ admission.ObjectInterfaces) error {
	// Wait until the caches have been synced
	if v.readyFunc == nil {
		v.AssignReadyFunc(func() bool {
			for _, readyFunc := range readyFuncs {
				if !readyFunc() {
					return false
				}
			}
			return true
		})
	}
	if !v.WaitForReady() {
		return admission.NewForbidden(a, errors.New("not yet ready to handle request"))
	}

	if a.GetKind().GroupKind() != gardencore.Kind("NamespacedCloudProfile") {
		return nil
	}

	if a.GetSubresource() != "" {
		return nil
	}

	var oldNamespacedCloudProfile = &gardencore.NamespacedCloudProfile{}

	namespacedCloudProfile, convertIsSuccessful := a.GetObject().(*gardencore.NamespacedCloudProfile)
	if !convertIsSuccessful {
		return apierrors.NewInternalError(errors.New("could not convert object to NamespacedCloudProfile"))
	}

	if a.GetOperation() == admission.Update || a.GetOperation() == admission.Delete {
		var ok bool
		oldNamespacedCloudProfile, ok = a.GetOldObject().(*gardencore.NamespacedCloudProfile)
		if !ok {
			return apierrors.NewInternalError(errors.New("could not convert old resource into NamespacedCloudProfile object"))
		}
	}

	// Exit early if the spec hasn't changed
	if a.GetOperation() == admission.Update {
		// do not ignore metadata updates to detect and prevent removal of the gardener finalizer or unwanted changes to annotations
		if reflect.DeepEqual(namespacedCloudProfile.Spec, oldNamespacedCloudProfile.Spec) &&
			(reflect.DeepEqual(namespacedCloudProfile.ObjectMeta, oldNamespacedCloudProfile.ObjectMeta) ||
				namespacedCloudProfile.DeletionTimestamp != nil) {
			return nil
		}
	}

	parentCloudProfileName := namespacedCloudProfile.Spec.Parent.Name
	parentCloudProfile, err := v.cloudProfileLister.Get(parentCloudProfileName)
	if err != nil {
		return apierrors.NewBadRequest("parent CloudProfile could not be found")
	}

	validationContext := &validationContext{
		parentCloudProfile:        parentCloudProfile,
		namespacedCloudProfile:    namespacedCloudProfile,
		oldNamespacedCloudProfile: oldNamespacedCloudProfile,
	}

	if err := validationContext.validateMachineTypes(a); err != nil {
		return err
	}
	if err := validationContext.validateKubernetesVersionOverrides(a); err != nil {
		return err
	}
	if err := validationContext.validateMachineImageOverrides(ctx, a); err != nil {
		return err
	}
	if err := validationContext.validateSimulatedCloudProfileStatusMergeResult(); err != nil {
		return err
	}

	return nil
}

type validationContext struct {
	parentCloudProfile        *gardencorev1beta1.CloudProfile
	namespacedCloudProfile    *gardencore.NamespacedCloudProfile
	oldNamespacedCloudProfile *gardencore.NamespacedCloudProfile
}

func (c *validationContext) validateMachineTypes(a admission.Attributes) error {
	if c.namespacedCloudProfile.Spec.MachineTypes == nil || c.parentCloudProfile.Spec.MachineTypes == nil {
		return nil
	}

	for _, machineType := range c.namespacedCloudProfile.Spec.MachineTypes {
		for _, parentMachineType := range c.parentCloudProfile.Spec.MachineTypes {
			if parentMachineType.Name != machineType.Name {
				continue
			}
			// If a machineType is already present in the NamespacedCloudProfile and just got added to the parent CloudProfile,
			// it should still be allowed to remain in the NamespacedCloudProfile.
			if a.GetOperation() == admission.Update && isMachineTypePresentInNamespacedCloudProfile(machineType, c.oldNamespacedCloudProfile) {
				continue
			}
			return apierrors.NewBadRequest(fmt.Sprintf("NamespacedCloudProfile attempts to overwrite parent CloudProfile with machineType: %+v", machineType))
		}
	}

	return nil
}

func isMachineTypePresentInNamespacedCloudProfile(machineType gardencore.MachineType, cloudProfile *gardencore.NamespacedCloudProfile) bool {
	for _, cloudProfileMachineType := range cloudProfile.Spec.MachineTypes {
		if cloudProfileMachineType.Name == machineType.Name {
			return true
		}
	}
	return false
}

func (c *validationContext) validateKubernetesVersionOverrides(attr admission.Attributes) error {
	if c.namespacedCloudProfile.Spec.Kubernetes == nil {
		return nil
	}

	now := ptr.To(metav1.Now())
	parentVersions := utils.CreateMapFromSlice(c.parentCloudProfile.Spec.Kubernetes.Versions, func(version gardencorev1beta1.ExpirableVersion) string { return version.Version })
	currentVersionsMerged := make(map[string]gardencore.ExpirableVersion)
	if attr.GetOperation() == admission.Update {
		currentVersionsMerged = utils.CreateMapFromSlice(c.oldNamespacedCloudProfile.Status.CloudProfileSpec.Kubernetes.Versions, func(version gardencore.ExpirableVersion) string { return version.Version })
	}
	for _, newVersion := range c.namespacedCloudProfile.Spec.Kubernetes.Versions {
		if _, exists := parentVersions[newVersion.Version]; !exists {
			return fmt.Errorf("invalid kubernetes version specified: '%s' does not exist in parent CloudProfile and thus cannot be overridden", newVersion.Version)
		}
		if newVersion.ExpirationDate == nil {
			return fmt.Errorf("specified version '%s' does not set expiration date", newVersion.Version)
		}
		if attr.GetOperation() == admission.Update && newVersion.ExpirationDate.Before(now) {
			if override, exists := currentVersionsMerged[newVersion.Version]; !exists || !override.ExpirationDate.Equal(newVersion.ExpirationDate) {
				return fmt.Errorf("expiration date for version %q is in the past", newVersion.Version)
			}
		}
	}
	return nil
}

func (c *validationContext) validateMachineImageOverrides(ctx context.Context, attr admission.Attributes) error {
	var (
		allErrs      = field.ErrorList{}
		now          = ptr.To(metav1.Now())
		parentImages = util.NewV1beta1ImagesContext(c.parentCloudProfile.Spec.MachineImages)

		oldVersionsSpec, oldVersionsMerged *util.ImagesContext[gardencore.MachineImage, gardencore.MachineImageVersion]
	)

	if attr.GetOperation() == admission.Update {
		oldVersionsSpec = util.NewCoreImagesContext(c.oldNamespacedCloudProfile.Spec.MachineImages)
		oldVersionsMerged = util.NewCoreImagesContext(c.oldNamespacedCloudProfile.Status.CloudProfileSpec.MachineImages)
	}

	for imageIndex, image := range c.namespacedCloudProfile.Spec.MachineImages {
		imageIndexPath := field.NewPath("spec", "machineImages").Index(imageIndex)
		_, isExistingImage := parentImages.GetImage(image.Name)

		if isExistingImage {
			// If in the meantime an image specified only in the NamespacedCloudProfile has been
			// added to the parent CloudProfile, then ignore already existing fields otherwise invalid for a new override.
			var imageAlreadyExistsInNamespacedCloudProfile bool
			if oldVersionsSpec != nil {
				var currentImage gardencore.MachineImage
				currentImage, imageAlreadyExistsInNamespacedCloudProfile = oldVersionsSpec.GetImage(image.Name)

				if imageAlreadyExistsInNamespacedCloudProfile && ptr.Deref(image.UpdateStrategy, "") != ptr.Deref(currentImage.UpdateStrategy, "") {
					allErrs = append(allErrs, field.Forbidden(imageIndexPath.Child("updateStrategy"), fmt.Sprintf("cannot update the machine image update strategy of %q, as this version has been added to the parent CloudProfile by now", image.Name)))
				}
			}

			for imageVersionIndex, imageVersion := range image.Versions {
				if _, isExistingVersion := parentImages.GetImageVersion(image.Name, imageVersion.Version); isExistingVersion {
					// An image with the specified version is already present in the parent CloudProfile.
					// Ensure that only the expiration date is overridden.
					// For new versions added to an existing image, the validation will be done on the simulated merge result.
					imageVersionIndexPath := imageIndexPath.Child("versions").Index(imageVersionIndex)

					// If in the meantime an image version specified only in the NamespacedCloudProfile has been
					// added to the parent CloudProfile, then ignore already existing fields otherwise invalid for a new override.
					var imageVersionAlreadyInNamespacedCloudProfile bool
					if imageAlreadyExistsInNamespacedCloudProfile {
						var oldMachineImageVersion gardencore.MachineImageVersion
						oldMachineImageVersion, imageVersionAlreadyInNamespacedCloudProfile = oldVersionsSpec.GetImageVersion(image.Name, imageVersion.Version)

						if imageVersionAlreadyInNamespacedCloudProfile && !reflect.DeepEqual(oldMachineImageVersion, imageVersion) {
							allErrs = append(allErrs, field.Forbidden(imageVersionIndexPath, fmt.Sprintf("cannot update the machine image version spec of \"%s@%s\", as this version has been added to the parent CloudProfile by now", image.Name, imageVersion.Version)))
						}
					}

					if !imageVersionAlreadyInNamespacedCloudProfile {
						allErrs = append(allErrs, validateNamespacedCloudProfileExtendedMachineImages(imageVersion, imageVersionIndexPath)...)

						if imageVersion.ExpirationDate == nil {
							allErrs = append(allErrs, field.Invalid(imageVersionIndexPath.Child("expirationDate"), imageVersion.ExpirationDate, fmt.Sprintf("expiration date for version %q must be set", imageVersion.Version)))
						}
					}

					if attr.GetOperation() == admission.Update && imageVersion.ExpirationDate.Before(now) {
						var (
							override gardencore.MachineImageVersion
							exists   bool
						)
						if oldVersionsMerged != nil {
							override, exists = oldVersionsMerged.GetImageVersion(image.Name, imageVersion.Version)
						}
						if !exists || !override.ExpirationDate.Equal(imageVersion.ExpirationDate) {
							allErrs = append(allErrs, field.Invalid(imageVersionIndexPath.Child("expirationDate"), imageVersion.ExpirationDate, fmt.Sprintf("expiration date for version %q is in the past", imageVersion.Version)))
						}
					}
				}
			}
		} else {
			var parentCloudProfileSpecCore gardencore.CloudProfileSpec
			if err := api.Scheme.Convert(&c.parentCloudProfile.Spec, &parentCloudProfileSpecCore, ctx); err != nil {
				allErrs = append(allErrs, field.InternalError(imageIndexPath, err))
			}

			// There is no entry for this image in the parent CloudProfile yet.
			capabilities := gardencorehelper.CapabilityDefinitionsToCapabilities(parentCloudProfileSpecCore.Capabilities)
			allErrs = append(allErrs, validation.ValidateMachineImages([]gardencore.MachineImage{image}, capabilities, imageIndexPath, false)...)
			allErrs = append(allErrs, validation.ValidateCloudProfileMachineImages([]gardencore.MachineImage{image}, capabilities, imageIndexPath)...)
		}
	}
	return allErrs.ToAggregate()
}

func validateNamespacedCloudProfileExtendedMachineImages(machineVersion gardencore.MachineImageVersion, versionsPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(ptr.Deref(machineVersion.Classification, "")) > 0 {
		allErrs = append(allErrs, field.Forbidden(versionsPath.Child("classification"), "must not provide a classification to an extended machine image in NamespacedCloudProfile"))
	}
	if len(machineVersion.CRI) > 0 {
		allErrs = append(allErrs, field.Forbidden(versionsPath.Child("cri"), "must not provide a cri to an extended machine image in NamespacedCloudProfile"))
	}
	if len(machineVersion.Architectures) > 0 {
		allErrs = append(allErrs, field.Forbidden(versionsPath.Child("architectures"), "must not provide an architecture to an extended machine image in NamespacedCloudProfile"))
	}
	if len(machineVersion.CapabilitySets) > 0 {
		allErrs = append(allErrs, field.Forbidden(versionsPath.Child("capabilitySets"), "must not provide capabilities to an extended machine image in NamespacedCloudProfile"))
	}
	if len(ptr.Deref(machineVersion.KubeletVersionConstraint, "")) > 0 {
		allErrs = append(allErrs, field.Forbidden(versionsPath.Child("kubeletVersionConstraint"), "must not provide a kubelet version constraint to an extended machine image in NamespacedCloudProfile"))
	}

	return allErrs
}

func (c *validationContext) validateSimulatedCloudProfileStatusMergeResult() error {
	namespacedCloudProfile := &gardencorev1beta1.NamespacedCloudProfile{}
	if err := api.Scheme.Convert(c.namespacedCloudProfile, namespacedCloudProfile, nil); err != nil {
		return err
	}
	errs := ValidateSimulatedNamespacedCloudProfileStatus(c.parentCloudProfile, namespacedCloudProfile)
	if len(errs) > 0 {
		return fmt.Errorf("error while validating merged NamespacedCloudProfile: %+v", errs)
	}
	return nil
}

// ValidateSimulatedNamespacedCloudProfileStatus merges the parent CloudProfile and the created or updated NamespacedCloudProfile
// to generate and validate the NamespacedCloudProfile status.
func ValidateSimulatedNamespacedCloudProfileStatus(originalParentCloudProfile *gardencorev1beta1.CloudProfile, originalNamespacedCloudProfile *gardencorev1beta1.NamespacedCloudProfile) field.ErrorList {
	parentCloudProfile := originalParentCloudProfile.DeepCopy()
	namespacedCloudProfile := originalNamespacedCloudProfile.DeepCopy()

	namespacedcloudprofile.MergeCloudProfiles(namespacedCloudProfile, parentCloudProfile)

	coreNamespacedCloudProfile := &gardencore.NamespacedCloudProfile{}
	if err := api.Scheme.Convert(namespacedCloudProfile, coreNamespacedCloudProfile, nil); err != nil {
		return field.ErrorList{{
			Type:     field.ErrorTypeInternal,
			Field:    "",
			BadValue: nil,
			Detail:   "could not convert NamespacedCloudProfile from type core.gardener.cloud/v1beta1 to the internal core type",
		}}
	}
	return validation.ValidateNamespacedCloudProfileStatus(&coreNamespacedCloudProfile.Status.CloudProfileSpec, field.NewPath("status.cloudProfileSpec"))
}
