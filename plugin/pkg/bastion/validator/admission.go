// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package validator

import (
	"context"
	"errors"
	"fmt"
	"io"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/admission"

	gardencoreapi "github.com/gardener/gardener/pkg/api"
	v1beta1helper "github.com/gardener/gardener/pkg/api/core/v1beta1/helper"
	corevalidation "github.com/gardener/gardener/pkg/api/core/validation"
	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/apis/operations"
	admissioninitializer "github.com/gardener/gardener/pkg/apiserver/admission/initializer"
	gardencoreclientset "github.com/gardener/gardener/pkg/client/core/clientset/versioned"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	gardencorev1beta1listers "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/kubernetes"
	plugin "github.com/gardener/gardener/plugin/pkg"
)

// Register registers a plugin.
func Register(plugins *admission.Plugins) {
	plugins.Register(plugin.PluginNameBastion, func(_ io.Reader) (admission.Interface, error) {
		return New()
	})
}

// Bastion contains listers and admission handler.
type Bastion struct {
	*admission.Handler

	coreClient                   gardencoreclientset.Interface
	cloudProfileLister           gardencorev1beta1listers.CloudProfileLister
	namespacedCloudProfileLister gardencorev1beta1listers.NamespacedCloudProfileLister
	readyFunc                    admission.ReadyFunc
}

var (
	_ = admissioninitializer.WantsCoreClientSet(&Bastion{})
	_ = admissioninitializer.WantsCoreInformerFactory(&Bastion{})

	readyFuncs []admission.ReadyFunc
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

// SetCoreClientSet sets the garden core clientset.
func (v *Bastion) SetCoreClientSet(c gardencoreclientset.Interface) {
	v.coreClient = c
}

// SetCoreInformerFactory gets Lister from SharedInformerFactory.
func (v *Bastion) SetCoreInformerFactory(f gardencoreinformers.SharedInformerFactory) {
	cloudProfileInformer := f.Core().V1beta1().CloudProfiles()
	v.cloudProfileLister = cloudProfileInformer.Lister()

	namespacedCloudProfileInformer := f.Core().V1beta1().NamespacedCloudProfiles()
	v.namespacedCloudProfileLister = namespacedCloudProfileInformer.Lister()

	readyFuncs = append(
		readyFuncs,
		cloudProfileInformer.Informer().HasSynced,
		namespacedCloudProfileInformer.Informer().HasSynced,
	)
}

// ValidateInitialization checks whether the plugin was correctly initialized.
func (v *Bastion) ValidateInitialization() error {
	if v.coreClient == nil {
		return errors.New("missing garden core client")
	}
	if v.cloudProfileLister == nil {
		return errors.New("missing cloudProfile lister")
	}
	if v.namespacedCloudProfileLister == nil {
		return errors.New("missing namespacedCloudProfile lister")
	}
	return nil
}

var _ admission.MutationInterface = (*Bastion)(nil)

// Admit validates and if appropriate mutates the given bastion against the shoot that it references.
func (v *Bastion) Admit(ctx context.Context, a admission.Attributes, _ admission.ObjectInterfaces) error {
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
	shoot, err := v.coreClient.CoreV1beta1().Shoots(bastion.Namespace).Get(ctx, shootName, metav1.GetOptions{})
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

	// ensure shoot SSH access is not disabled
	if bastion.DeletionTimestamp == nil && !v1beta1helper.ShootEnablesSSHAccess(shoot) {
		fieldErr := field.Invalid(shootPath, shootName, "ssh access is disabled for worker nodes")
		return apierrors.NewInvalid(gk, bastion.Name, field.ErrorList{fieldErr})
	}

	// validate the machine override against the shoot's CloudProfile
	if bastion.DeletionTimestamp == nil && bastion.Spec.Machine != nil {
		if err := v.validateMachine(bastion, shoot); err != nil {
			return err
		}
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

// validateMachine validates the bastion's machine override against the CloudProfile referenced by the shoot.
func (v *Bastion) validateMachine(bastion *operations.Bastion, shoot *gardencorev1beta1.Shoot) error {
	gk := schema.GroupKind{Group: operations.GroupName, Kind: "Bastion"}

	coreShoot := &core.Shoot{}
	if err := gardencorev1beta1.Convert_v1beta1_Shoot_To_core_Shoot(shoot, coreShoot, nil); err != nil {
		return apierrors.NewInternalError(fmt.Errorf("could not convert shoot %s/%s: %w", shoot.Namespace, shoot.Name, err))
	}

	cloudProfileSpec, err := gardenerutils.GetCloudProfileSpec(v.cloudProfileLister, v.namespacedCloudProfileLister, coreShoot)
	if err != nil {
		return apierrors.NewInternalError(fmt.Errorf("could not find referenced cloud profile: %w", err))
	}
	if cloudProfileSpec == nil {
		return nil
	}

	coreCloudProfileSpec := &core.CloudProfileSpec{}
	if err := gardencoreapi.Scheme.Convert(cloudProfileSpec, coreCloudProfileSpec, nil); err != nil {
		return apierrors.NewInternalError(fmt.Errorf("could not convert cloud profile spec: %w", err))
	}

	machinePath := field.NewPath("spec", "machine")

	var (
		machineType  *core.BastionMachineType
		machineImage *core.BastionMachineImage
	)
	if bastion.Spec.Machine.Type != nil {
		machineType = &core.BastionMachineType{Name: *bastion.Spec.Machine.Type}
	}
	if image := bastion.Spec.Machine.Image; image != nil {
		machineImage = &core.BastionMachineImage{Name: image.Name, Version: image.Version}
	}

	if errs := corevalidation.ValidateBastionMachine(machineType, machineImage, coreCloudProfileSpec, machinePath.Child("type"), machinePath.Child("image")); len(errs) > 0 {
		return apierrors.NewInvalid(gk, bastion.Name, errs)
	}

	return nil
}
