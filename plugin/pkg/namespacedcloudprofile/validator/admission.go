// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
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

	"github.com/gardener/gardener/pkg/api"
	gardencore "github.com/gardener/gardener/pkg/apis/core"
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
func (v *ValidateNamespacedCloudProfile) Validate(_ context.Context, a admission.Attributes, _ admission.ObjectInterfaces) error {
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

	// Exit early if the spec hasn't changed
	if a.GetOperation() == admission.Update {
		old, ok := a.GetOldObject().(*gardencore.NamespacedCloudProfile)
		if !ok {
			return apierrors.NewInternalError(errors.New("could not convert old resource into NamespacedCloudProfile object"))
		}
		oldNamespacedCloudProfile = old

		// do not ignore metadata updates to detect and prevent removal of the gardener finalizer or unwanted changes to annotations
		if reflect.DeepEqual(namespacedCloudProfile.Spec, oldNamespacedCloudProfile.Spec) && reflect.DeepEqual(namespacedCloudProfile.ObjectMeta, oldNamespacedCloudProfile.ObjectMeta) {
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
	if err := validationContext.validateMachineImageOverrides(a); err != nil {
		return err
	}
	if err := validationContext.validateSimulatedCloudProfileStatusMergeResult(a); err != nil {
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
				return fmt.Errorf("expiration date of version '%s' is in the past", newVersion.Version)
			}
		}
	}
	return nil
}

func (c *validationContext) validateMachineImageOverrides(attr admission.Attributes) error {
	now := ptr.To(metav1.Now())
	parentImages := utils.CreateMapFromSlice(c.parentCloudProfile.Spec.MachineImages, func(mi gardencorev1beta1.MachineImage) string { return mi.Name })
	currentVersionsMerged := make(map[string]map[string]gardencore.MachineImageVersion)
	if attr.GetOperation() == admission.Update {
		for _, machineImage := range c.oldNamespacedCloudProfile.Status.CloudProfileSpec.MachineImages {
			currentVersionsMerged[machineImage.Name] = utils.CreateMapFromSlice(machineImage.Versions, func(version gardencore.MachineImageVersion) string { return version.Version })
		}
	}
	for _, newImage := range c.namespacedCloudProfile.Spec.MachineImages {
		if _, exists := parentImages[newImage.Name]; !exists {
			return fmt.Errorf("invalid machine image specified: '%s' does not exist in parent CloudProfile and thus cannot be overridden", newImage.Name)
		}
		parentVersions := utils.CreateMapFromSlice(parentImages[newImage.Name].Versions, func(v gardencorev1beta1.MachineImageVersion) string { return v.Version })
		for _, newVersion := range newImage.Versions {
			if _, exists := parentVersions[newVersion.Version]; !exists {
				return fmt.Errorf("invalid machine image specified: '%s@%s' does not exist in parent CloudProfile and thus cannot be overridden", newImage.Name, newVersion.Version)
			}
			if newVersion.ExpirationDate == nil {
				return fmt.Errorf("specified version '%s' does not set expiration date", newVersion.Version)
			}
			if attr.GetOperation() == admission.Update && newVersion.ExpirationDate.Before(now) {
				var override gardencore.MachineImageVersion
				exists := false
				if _, imageNameExists := currentVersionsMerged[newImage.Name]; imageNameExists {
					override, exists = currentVersionsMerged[newImage.Name][newVersion.Version]
				}
				if !exists || !override.ExpirationDate.Equal(newVersion.ExpirationDate) {
					return fmt.Errorf("expiration date of version '%s' is in the past", newVersion.Version)
				}
			}
		}
	}
	return nil
}

func (c *validationContext) validateSimulatedCloudProfileStatusMergeResult(_ admission.Attributes) error {
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
