// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package managedseed

import (
	"context"
	"errors"
	"fmt"
	"io"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/utils/pointer"

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorehelper "github.com/gardener/gardener/pkg/apis/core/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	seedmanagementv1alpha1helper "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1/helper"
	admissioninitializer "github.com/gardener/gardener/pkg/apiserver/admission/initializer"
	gardencoreclientset "github.com/gardener/gardener/pkg/client/core/clientset/internalversion"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/internalversion"
	gardencorelisters "github.com/gardener/gardener/pkg/client/core/listers/core/internalversion"
	seedmanagementclientset "github.com/gardener/gardener/pkg/client/seedmanagement/clientset/versioned"
	"github.com/gardener/gardener/plugin/pkg/utils"
	admissionutils "github.com/gardener/gardener/plugin/pkg/utils"
)

const (
	// PluginName is the name of this admission plugin.
	PluginName = "ShootManagedSeed"
)

// Register registers a plugin.
func Register(plugins *admission.Plugins) {
	plugins.Register(PluginName, func(config io.Reader) (admission.Interface, error) {
		return New()
	})
}

// ManagedSeed contains listers and admission handler.
type ManagedSeed struct {
	*admission.Handler
	coreClient           gardencoreclientset.Interface
	seedManagementClient seedmanagementclientset.Interface
	shootLister          gardencorelisters.ShootLister
	readyFunc            admission.ReadyFunc
}

var (
	_ = admissioninitializer.WantsInternalCoreClientset(&ManagedSeed{})
	_ = admissioninitializer.WantsSeedManagementClientset(&ManagedSeed{})

	readyFuncs = []admission.ReadyFunc{}
)

// New creates a new ManagedSeed admission plugin.
func New() (*ManagedSeed, error) {
	return &ManagedSeed{
		Handler: admission.NewHandler(admission.Update, admission.Delete),
	}, nil
}

// AssignReadyFunc assigns the ready function to the admission handler.
func (v *ManagedSeed) AssignReadyFunc(f admission.ReadyFunc) {
	v.readyFunc = f
	v.SetReadyFunc(f)
}

// SetInternalCoreClientset sets the garden core clientset.
func (v *ManagedSeed) SetInternalCoreClientset(c gardencoreclientset.Interface) {
	v.coreClient = c
}

// SetSeedManagementClientset sets the garden seedmanagement clientset.
func (v *ManagedSeed) SetSeedManagementClientset(c seedmanagementclientset.Interface) {
	v.seedManagementClient = c
}

// SetInternalCoreInformerFactory gets Lister from SharedInformerFactory.
func (v *ManagedSeed) SetInternalCoreInformerFactory(f gardencoreinformers.SharedInformerFactory) {
	shootInformer := f.Core().InternalVersion().Shoots()
	v.shootLister = shootInformer.Lister()

	readyFuncs = append(readyFuncs, shootInformer.Informer().HasSynced)
}

// ValidateInitialization checks whether the plugin was correctly initialized.
func (v *ManagedSeed) ValidateInitialization() error {
	if v.coreClient == nil {
		return errors.New("missing garden core client")
	}
	if v.seedManagementClient == nil {
		return errors.New("missing garden seedmanagement client")
	}
	if v.shootLister == nil {
		return errors.New("missing shoot lister")
	}
	return nil
}

var _ admission.ValidationInterface = &ManagedSeed{}

// Validate validates changes to the Shoot referenced by a ManagedSeed.
func (v *ManagedSeed) Validate(ctx context.Context, a admission.Attributes, _ admission.ObjectInterfaces) error {
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

	// Ignore all kinds other than Shoot
	if a.GetKind().GroupKind() != core.Kind("Shoot") {
		return nil
	}

	// Ignore updates to status or other subresources
	if a.GetSubresource() != "" {
		return nil
	}

	if a.GetOperation() == admission.Update {
		return v.validateUpdate(ctx, a)
	} else if a.GetOperation() == admission.Delete {
		switch {
		case a.GetName() == "":
			return v.validateDeleteCollection(ctx, a)
		default:
			return v.validateDelete(ctx, a)
		}
	}

	return nil
}

func (v *ManagedSeed) validateUpdate(ctx context.Context, a admission.Attributes) error {
	managedSeed, err := utils.GetManagedSeed(ctx, v.seedManagementClient, a.GetNamespace(), a.GetName())
	if err != nil {
		return apierrors.NewInternalError(fmt.Errorf("could not get ManagedSeed for shoot '%s/%s': %v", a.GetNamespace(), a.GetName(), err))
	}
	if managedSeed == nil {
		return nil
	}

	shoot, ok := a.GetObject().(*core.Shoot)
	if !ok {
		return apierrors.NewInternalError(errors.New("could not convert resource into Shoot object"))
	}

	oldShoot, ok := a.GetOldObject().(*core.Shoot)
	if !ok {
		return apierrors.NewInternalError(errors.New("could not convert resource into Shoot object"))
	}

	var allErrs field.ErrorList
	if nginxIngressEnabled := gardencorehelper.NginxIngressEnabled(shoot.Spec.Addons); nginxIngressEnabled {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "addons", "nginxIngress", "enabled"), nginxIngressEnabled, "shoot ingress addon is not supported for managed seeds - use the managed seed ingress controller"))
	}
	if vpaEnabled := gardencorehelper.ShootWantsVerticalPodAutoscaler(shoot); !vpaEnabled {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "kubernetes", "verticalPodAutoscaler", "enabled"), vpaEnabled, "shoot VPA has to be enabled for managed seeds"))
	}

	if oldShoot.Spec.Networking.Nodes != nil && *oldShoot.Spec.Networking.Nodes != *shoot.Spec.Networking.Nodes {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "networking", "nodes"), shoot.Spec.Networking.Nodes, "field is immutable for managed seeds"))
	}

	seedTemplate, _, err := seedmanagementv1alpha1helper.ExtractSeedTemplateAndGardenletConfig(managedSeed)
	if err != nil {
		return apierrors.NewInternalError(fmt.Errorf("cannot extract the seed template: %w", err))
	}

	if seedTemplate.Spec.SecretRef != nil && !pointer.BoolDeref(shoot.Spec.Kubernetes.EnableStaticTokenKubeconfig, false) {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "kubernetes", "enableStaticTokenKubeconfig"), shoot.Spec.Kubernetes.EnableStaticTokenKubeconfig, "shoot static token kubeconfig cannot be disabled when the seed secretRef is set"))
	}

	zoneValidationErrrs, err := v.validateWorkerZoneChanges(ctx, field.NewPath("spec", "providers", "workers"), shoot, oldShoot, seedTemplate)
	if err != nil {
		return apierrors.NewInternalError(err)
	}
	allErrs = append(allErrs, zoneValidationErrrs...)

	if len(allErrs) > 0 {
		return apierrors.NewInvalid(a.GetKind().GroupKind(), shoot.Name, allErrs)
	}

	return nil
}

// validateWorkerZoneChanges returns an error if worker zones for the given shoot were changed
// while it still hosts shoot control-planes.
func (v *ManagedSeed) validateWorkerZoneChanges(ctx context.Context, fldPath *field.Path, shoot, oldShoot *core.Shoot, seedTemplate *gardencorev1beta1.SeedTemplate) (field.ErrorList, error) {
	allErrs := field.ErrorList{}

	shootZones := gardencorehelper.GetAllZonesFromShoot(shoot)

	// return if zones in shoot workers are unchanged
	if shootZones.Equal(gardencorehelper.GetAllZonesFromShoot(oldShoot)) {
		return allErrs, nil
	}

	// return if all zones in seedTemplate are available in shoot workers, i.e. no zone previously available
	// in seed was removed from shoot workers.
	if shootZones.HasAll(seedTemplate.Spec.Provider.Zones...) {
		return allErrs, nil
	}

	managedSeed, err := utils.GetManagedSeed(ctx, v.seedManagementClient, shoot.GetNamespace(), shoot.GetName())
	if err != nil {
		return allErrs, apierrors.NewInternalError(fmt.Errorf("could not get ManagedSeed for shoot '%s/%s': %v", shoot.GetNamespace(), shoot.GetName(), err))
	}

	if managedSeed == nil {
		return allErrs, nil
	}

	shoots, err := v.shootLister.List(labels.Everything())
	if err != nil {
		return allErrs, err
	}

	if admissionutils.IsSeedUsedByShoot(managedSeed.Name, shoots) {
		allErrs = append(allErrs, field.Forbidden(fldPath, "cannot change zone information since shoot is registered as seed which is still used by shoot(s)"))
	}

	return allErrs, nil
}

func (v *ManagedSeed) validateDeleteCollection(ctx context.Context, a admission.Attributes) error {
	shoots, err := v.getShoots(ctx, labels.Everything())
	if err != nil {
		return err
	}
	for _, shoot := range shoots {
		if err := v.validateDelete(ctx, utils.NewAttributesWithName(a, shoot.Name)); err != nil {
			return err
		}
	}

	return nil
}

func (v *ManagedSeed) validateDelete(ctx context.Context, a admission.Attributes) error {
	managedSeed, err := utils.GetManagedSeed(ctx, v.seedManagementClient, a.GetNamespace(), a.GetName())
	if err != nil {
		return apierrors.NewInternalError(fmt.Errorf("could not get ManagedSeed for shoot '%s/%s': %v", a.GetNamespace(), a.GetName(), err))
	}
	if managedSeed == nil {
		return nil
	}

	return admission.NewForbidden(a, fmt.Errorf("cannot delete shoot %s/%s since it is still referenced by a managed seed", a.GetNamespace(), a.GetName()))
}

func (v *ManagedSeed) getShoots(ctx context.Context, selector labels.Selector) ([]core.Shoot, error) {
	shootList, err := v.coreClient.Core().Shoots("").List(ctx, metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return nil, err
	}
	return shootList.Items, nil
}
