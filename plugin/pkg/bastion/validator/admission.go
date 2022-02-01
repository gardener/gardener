// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"io"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/apis/operations"
	admissioninitializer "github.com/gardener/gardener/pkg/apiserver/admission/initializer"
	coreclientset "github.com/gardener/gardener/pkg/client/core/clientset/internalversion"
	"github.com/gardener/gardener/pkg/utils/kubernetes"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/admission"
)

const (
	// PluginName is the name of this admission plugin.
	PluginName = "Bastion"
)

// Register registers a plugin.
func Register(plugins *admission.Plugins) {
	plugins.Register(PluginName, func(config io.Reader) (admission.Interface, error) {
		return New()
	})
}

// Bastion contains listers and and admission handler.
type Bastion struct {
	*admission.Handler
	coreClient coreclientset.Interface
	readyFunc  admission.ReadyFunc
}

var (
	_ = admissioninitializer.WantsInternalCoreClientset(&Bastion{})

	readyFuncs = []admission.ReadyFunc{}
)

// New creates a new Bastion admission plugin.
func New() (*Bastion, error) {
	return &Bastion{
		Handler: admission.NewHandler(admission.Create, admission.Update),
	}, nil
}

// AssignReadyFunc assigns the ready function to the admission handler.
func (v *Bastion) AssignReadyFunc(f admission.ReadyFunc) {
	v.readyFunc = f
	v.SetReadyFunc(f)
}

// SetInternalCoreClientset sets the garden core clientset.
func (v *Bastion) SetInternalCoreClientset(c coreclientset.Interface) {
	v.coreClient = c
}

// ValidateInitialization checks whether the plugin was correctly initialized.
func (v *Bastion) ValidateInitialization() error {
	if v.coreClient == nil {
		return errors.New("missing garden core client")
	}
	return nil
}

var _ admission.MutationInterface = &Bastion{}

// Admit validates and if appropriate mutates the given bastion against the shoot that it references.
func (v *Bastion) Admit(ctx context.Context, a admission.Attributes, o admission.ObjectInterfaces) error {
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

	// Ignore all kinds other than Bastion
	if a.GetKind().GroupKind() != operations.Kind("Bastion") {
		return nil
	}

	// Ignore updates to status or other subresources
	if a.GetSubresource() != "" {
		return nil
	}

	// Convert object to Bastion
	bastion, ok := a.GetObject().(*operations.Bastion)
	if !ok {
		return apierrors.NewBadRequest("could not convert object to Bastion")
	}

	gk := schema.GroupKind{Group: operations.GroupName, Kind: "Bastion"}

	// ensure shoot name is specified
	shootPath := field.NewPath("spec", "shootRef", "name")
	if bastion.Spec.ShootRef.Name == "" {
		return apierrors.NewInvalid(gk, bastion.Name, field.ErrorList{field.Required(shootPath, "shoot is required")})
	}

	shootName := bastion.Spec.ShootRef.Name

	// ensure shoot exists
	shoot, err := v.coreClient.Core().Shoots(bastion.Namespace).Get(ctx, shootName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			fieldErr := field.Invalid(shootPath, shootName, fmt.Sprintf("shoot %s/%s not found", bastion.Namespace, shootName))
			return apierrors.NewInvalid(gk, bastion.Name, field.ErrorList{fieldErr})
		}

		return apierrors.NewInternalError(fmt.Errorf("could not get shoot %s/%s: %v", bastion.Namespace, shootName, err))
	}

	// ensure shoot is alive
	if a.GetOperation() == admission.Create && shoot.DeletionTimestamp != nil {
		fieldErr := field.Invalid(shootPath, shootName, "shoot is in deletion")
		return apierrors.NewInvalid(gk, bastion.Name, field.ErrorList{fieldErr})
	}

	// ensure shoot is already assigned to a seed
	if shoot.Spec.SeedName == nil || len(*shoot.Spec.SeedName) == 0 {
		fieldErr := field.Invalid(shootPath, shootName, "shoot is not yet assigned to a seed")
		return apierrors.NewInvalid(gk, bastion.Name, field.ErrorList{fieldErr})
	}

	// update bastion
	bastion.Spec.SeedName = shoot.Spec.SeedName
	bastion.Spec.ProviderType = &shoot.Spec.Provider.Type

	if userInfo := a.GetUserInfo(); a.GetOperation() == admission.Create && userInfo != nil {
		metav1.SetMetaDataAnnotation(&bastion.ObjectMeta, v1beta1constants.GardenCreatedBy, userInfo.GetName())
	}

	// ensure bastions are cleaned up when shoots are deleted
	ownerRef := *metav1.NewControllerRef(shoot, gardencorev1beta1.SchemeGroupVersion.WithKind("Shoot"))
	bastion.OwnerReferences = kubernetes.MergeOwnerReferences(bastion.OwnerReferences, ownerRef)

	return nil
}
