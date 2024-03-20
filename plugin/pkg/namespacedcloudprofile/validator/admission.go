// Copyright 2024 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package validator

import (
	"context"
	"errors"
	"fmt"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"io"
	"reflect"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apiserver/pkg/admission"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	admissioninitializer "github.com/gardener/gardener/pkg/apiserver/admission/initializer"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	gardencorev1beta1listers "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"
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
			// If a machineType is already present in the namespacedCloudProfile and just got added to the parentCloudProfile,
			// it should still be allowed to remain in the namespacedCloudProfile.
			if a.GetOperation() == admission.Update && isMachineTypePresentInNamespacedCloudProfile(machineType, c.oldNamespacedCloudProfile) {
				continue
			}
			return apierrors.NewBadRequest(fmt.Sprintf("NamespacedCloudProfile attempts to rewrite MachineType of parent CloudProfile with machineType: %+v", machineType))
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
