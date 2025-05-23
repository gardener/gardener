// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorehelper "github.com/gardener/gardener/pkg/apis/core/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	seedmanagementv1alpha1helper "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1/helper"
	admissioninitializer "github.com/gardener/gardener/pkg/apiserver/admission/initializer"
	gardencoreclientset "github.com/gardener/gardener/pkg/client/core/clientset/versioned"
	seedmanagementclientset "github.com/gardener/gardener/pkg/client/seedmanagement/clientset/versioned"
	plugin "github.com/gardener/gardener/plugin/pkg"
	"github.com/gardener/gardener/plugin/pkg/utils"
)

// Register registers a plugin.
func Register(plugins *admission.Plugins) {
	plugins.Register(plugin.PluginNameShootManagedSeed, func(_ io.Reader) (admission.Interface, error) {
		return New()
	})
}

// ManagedSeed contains listers and admission handler.
type ManagedSeed struct {
	*admission.Handler
	coreClient           gardencoreclientset.Interface
	seedManagementClient seedmanagementclientset.Interface
	readyFunc            admission.ReadyFunc
}

var (
	_ = admissioninitializer.WantsCoreClientSet(&ManagedSeed{})
	_ = admissioninitializer.WantsSeedManagementClientSet(&ManagedSeed{})

	readyFuncs []admission.ReadyFunc
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

// SetCoreClientSet sets the garden core clientset.
func (v *ManagedSeed) SetCoreClientSet(c gardencoreclientset.Interface) {
	v.coreClient = c
}

// SetSeedManagementClientSet sets the garden seedmanagement clientset.
func (v *ManagedSeed) SetSeedManagementClientSet(c seedmanagementclientset.Interface) {
	v.seedManagementClient = c
}

// ValidateInitialization checks whether the plugin was correctly initialized.
func (v *ManagedSeed) ValidateInitialization() error {
	if v.coreClient == nil {
		return errors.New("missing garden core client")
	}
	if v.seedManagementClient == nil {
		return errors.New("missing garden seedmanagement client")
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

	seedTemplate, _, err := seedmanagementv1alpha1helper.ExtractSeedTemplateAndGardenletConfig(managedSeed.Name, &managedSeed.Spec.Gardenlet.Config)
	if err != nil {
		return apierrors.NewInternalError(fmt.Errorf("cannot extract the seed template: %w", err))
	}

	allErrs = append(allErrs, v.validateZoneRemovalFromShoot(field.NewPath("spec", "providers", "workers"), oldShoot, shoot, seedTemplate)...)

	if len(allErrs) > 0 {
		return apierrors.NewInvalid(a.GetKind().GroupKind(), shoot.Name, allErrs)
	}

	return nil
}

// validateZoneRemovalFromShoot returns an error if worker zones for the given shoot were changed
// while they are still registered in the ManagedSeed.
func (v *ManagedSeed) validateZoneRemovalFromShoot(fldPath *field.Path, oldShoot, newShoot *core.Shoot, seedTemplate *gardencorev1beta1.SeedTemplate) field.ErrorList {
	allErrs := field.ErrorList{}

	removedZones := gardencorehelper.GetAllZonesFromShoot(oldShoot).Difference(gardencorehelper.GetAllZonesFromShoot(newShoot))

	// Check if a zones change affect the configuration of the ManagedSeed.
	// In case of a removal, zone(s) must first be deselected in ManagedSeed before they can be removed in the shoot.
	// We only check removed zones here because Gardener used to tolerate a zone name mismatch, see https://github.com/gardener/gardener/pull/7024.
	if removedZones.HasAny(seedTemplate.Spec.Provider.Zones...) {
		allErrs = append(allErrs, field.Forbidden(fldPath, "shoot worker zone(s) must not be removed as long as registered in managedseed"))
	}

	return allErrs
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

func (v *ManagedSeed) getShoots(ctx context.Context, selector labels.Selector) ([]gardencorev1beta1.Shoot, error) {
	shootList, err := v.coreClient.CoreV1beta1().Shoots("").List(ctx, metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return nil, err
	}
	return shootList.Items, nil
}
