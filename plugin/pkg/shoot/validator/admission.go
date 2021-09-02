// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"reflect"
	"strings"
	"time"

	"github.com/Masterminds/semver"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/admission"

	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apis/core/helper"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	admissioninitializer "github.com/gardener/gardener/pkg/apiserver/admission/initializer"
	coreinformers "github.com/gardener/gardener/pkg/client/core/informers/internalversion"
	corelisters "github.com/gardener/gardener/pkg/client/core/listers/core/internalversion"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/utils"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	cidrvalidation "github.com/gardener/gardener/pkg/utils/validation/cidr"
)

const (
	// PluginName is the name of this admission plugin.
	PluginName = "ShootValidator"
)

// Register registers a plugin.
func Register(plugins *admission.Plugins) {
	plugins.Register(PluginName, func(config io.Reader) (admission.Interface, error) {
		return New()
	})
}

// ValidateShoot contains listers and and admission handler.
type ValidateShoot struct {
	*admission.Handler
	cloudProfileLister corelisters.CloudProfileLister
	seedLister         corelisters.SeedLister
	shootLister        corelisters.ShootLister
	projectLister      corelisters.ProjectLister
	readyFunc          admission.ReadyFunc
}

var (
	_ = admissioninitializer.WantsInternalCoreInformerFactory(&ValidateShoot{})

	readyFuncs = []admission.ReadyFunc{}
)

// New creates a new ValidateShoot admission plugin.
func New() (*ValidateShoot, error) {
	return &ValidateShoot{
		Handler: admission.NewHandler(admission.Create, admission.Update, admission.Delete),
	}, nil
}

// AssignReadyFunc assigns the ready function to the admission handler.
func (v *ValidateShoot) AssignReadyFunc(f admission.ReadyFunc) {
	v.readyFunc = f
	v.SetReadyFunc(f)
}

// SetInternalCoreInformerFactory gets Lister from SharedInformerFactory.
func (v *ValidateShoot) SetInternalCoreInformerFactory(f coreinformers.SharedInformerFactory) {
	seedInformer := f.Core().InternalVersion().Seeds()
	v.seedLister = seedInformer.Lister()

	shootInformer := f.Core().InternalVersion().Shoots()
	v.shootLister = shootInformer.Lister()

	cloudProfileInformer := f.Core().InternalVersion().CloudProfiles()
	v.cloudProfileLister = cloudProfileInformer.Lister()

	projectInformer := f.Core().InternalVersion().Projects()
	v.projectLister = projectInformer.Lister()

	readyFuncs = append(
		readyFuncs,
		seedInformer.Informer().HasSynced,
		shootInformer.Informer().HasSynced,
		cloudProfileInformer.Informer().HasSynced,
		projectInformer.Informer().HasSynced,
	)
}

// ValidateInitialization checks whether the plugin was correctly initialized.
func (v *ValidateShoot) ValidateInitialization() error {
	if v.cloudProfileLister == nil {
		return errors.New("missing cloudProfile lister")
	}
	if v.seedLister == nil {
		return errors.New("missing seed lister")
	}
	if v.shootLister == nil {
		return errors.New("missing shoot lister")
	}
	if v.projectLister == nil {
		return errors.New("missing project lister")
	}
	return nil
}

var _ admission.MutationInterface = &ValidateShoot{}

// Admit validates the Shoot details against the referenced CloudProfile.
func (v *ValidateShoot) Admit(ctx context.Context, a admission.Attributes, o admission.ObjectInterfaces) error {
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

	// Ignore updates to shoot status or other subresources
	if a.GetSubresource() != "" {
		return nil
	}

	var (
		shoot               = &core.Shoot{}
		oldShoot            = &core.Shoot{}
		convertIsSuccessful bool
	)

	if a.GetOperation() == admission.Delete {
		shoot, convertIsSuccessful = a.GetOldObject().(*core.Shoot)
	} else {
		shoot, convertIsSuccessful = a.GetObject().(*core.Shoot)
	}

	if !convertIsSuccessful {
		return apierrors.NewInternalError(errors.New("could not convert resource into Shoot object"))
	}

	// We only want to validate fields in the Shoot against the CloudProfile/Seed constraints which have changed.
	// On CREATE operations we just use an empty Shoot object, forcing the validator functions to always validate.
	// On UPDATE operations we fetch the current Shoot object.
	// On DELETE operations we want to verify that the Shoot is not in Restore or Migrate phase

	// Exit early if shoot spec hasn't changed
	if a.GetOperation() == admission.Update {
		old, ok := a.GetOldObject().(*core.Shoot)
		if !ok {
			return apierrors.NewInternalError(errors.New("could not convert old resource into Shoot object"))
		}
		oldShoot = old

		// do not ignore metadata updates to detect and prevent removal of the gardener finalizer or unwanted changes to annotations
		if reflect.DeepEqual(shoot.Spec, oldShoot.Spec) && reflect.DeepEqual(shoot.ObjectMeta, oldShoot.ObjectMeta) {
			return nil
		}
	}

	cloudProfile, err := v.cloudProfileLister.Get(shoot.Spec.CloudProfileName)
	if err != nil {
		return apierrors.NewBadRequest(fmt.Sprintf("could not find referenced cloud profile: %+v", err.Error()))
	}

	var seed *core.Seed
	if shoot.Spec.SeedName != nil {
		seed, err = v.seedLister.Get(*shoot.Spec.SeedName)
		if err != nil {
			return apierrors.NewBadRequest(fmt.Sprintf("could not find referenced seed %q: %+v", *shoot.Spec.SeedName, err.Error()))
		}
	}

	project, err := gutil.ProjectForNamespaceFromInternalLister(v.projectLister, shoot.Namespace)
	if err != nil {
		return apierrors.NewBadRequest(fmt.Sprintf("could not find referenced project: %+v", err.Error()))
	}

	// begin of validation code
	validationContext := &validationContext{
		cloudProfile: cloudProfile,
		project:      project,
		seed:         seed,
		shoot:        shoot,
		oldShoot:     oldShoot,
	}

	if err := validationContext.validateProjectMembership(a); err != nil {
		return err
	}
	if err := validationContext.validateScheduling(a, v.shootLister, v.seedLister); err != nil {
		return err
	}
	if err := validationContext.validateDeletion(a); err != nil {
		return err
	}
	if err := validationContext.validateShootHibernation(a); err != nil {
		return err
	}
	if err := validationContext.ensureMachineImages(); err != nil {
		return err
	}

	validationContext.addMetadataAnnotations(a)

	var allErrs field.ErrorList
	allErrs = append(allErrs, validationContext.validateShootNetworks()...)
	allErrs = append(allErrs, validationContext.validateKubernetes()...)
	allErrs = append(allErrs, validationContext.validateRegion()...)
	allErrs = append(allErrs, validationContext.validateProvider()...)

	dnsErrors, err := validationContext.validateDNSDomainUniqueness(v.shootLister)
	if err != nil {
		return apierrors.NewInternalError(err)
	}
	allErrs = append(allErrs, dnsErrors...)

	if len(allErrs) > 0 {
		return admission.NewForbidden(a, fmt.Errorf("%+v", allErrs))
	}

	return nil
}

type validationContext struct {
	cloudProfile *core.CloudProfile
	project      *core.Project
	seed         *core.Seed
	shoot        *core.Shoot
	oldShoot     *core.Shoot
}

func (c *validationContext) validateProjectMembership(a admission.Attributes) error {
	if a.GetOperation() != admission.Create {
		return nil
	}

	// We currently use the identifier "shoot-<project-name>-<shoot-name> in nearly all places for old Shoots, but have
	// changed that to "shoot--<project-name>-<shoot-name>": when creating infrastructure resources, Kubernetes resources,
	// DNS names, etc., then this identifier is used to tag/name the resources. Some of those resources have length
	// constraints that this identifier must not exceed 30 characters, thus we need to check whether Shoots do not exceed
	// this limit. The project name is a label on the namespace. If it is not found, the namespace name itself is used as
	// project name. These checks should only be performed for CREATE operations (we do not want to reject changes to existing
	// Shoots in case the limits are changed in the future).
	var lengthLimit = 21
	if len(c.shoot.Name) == 0 && len(c.shoot.GenerateName) > 0 {
		var randomLength = 5
		if len(c.project.Name+c.shoot.GenerateName) > lengthLimit-randomLength {
			return apierrors.NewBadRequest(fmt.Sprintf("the length of the shoot generateName and the project name must not exceed %d characters (project: %s; shoot with generateName: %s)", lengthLimit-randomLength, c.project.Name, c.shoot.GenerateName))
		}
	} else {
		if len(c.project.Name+c.shoot.Name) > lengthLimit {
			return apierrors.NewBadRequest(fmt.Sprintf("the length of the shoot name and the project name must not exceed %d characters (project: %s; shoot: %s)", lengthLimit, c.project.Name, c.shoot.Name))
		}
	}

	if c.project.DeletionTimestamp != nil {
		return admission.NewForbidden(a, fmt.Errorf("cannot create shoot '%s' in project '%s' that is already marked for deletion", c.shoot.Name, c.project.Name))
	}

	return nil
}

func (c *validationContext) validateScheduling(a admission.Attributes, shootLister corelisters.ShootLister, seedLister corelisters.SeedLister) error {
	var (
		shootIsBeingScheduled          = c.oldShoot.Spec.SeedName == nil && c.shoot.Spec.SeedName != nil
		shootIsBeingRescheduled        = c.oldShoot.Spec.SeedName != nil && c.shoot.Spec.SeedName != nil && *c.shoot.Spec.SeedName != *c.oldShoot.Spec.SeedName
		mustCheckSchedulingConstraints = shootIsBeingScheduled || shootIsBeingRescheduled
	)

	if mustCheckSchedulingConstraints {
		if c.seed.DeletionTimestamp != nil {
			return admission.NewForbidden(a, fmt.Errorf("cannot schedule shoot '%s' on seed '%s' that is already marked for deletion", c.shoot.Name, c.seed.Name))
		}

		if !helper.TaintsAreTolerated(c.seed.Spec.Taints, c.shoot.Spec.Tolerations) {
			return admission.NewForbidden(a, fmt.Errorf("forbidden to use a seeds whose taints are not tolerated by the shoot"))
		}

		if allocatableShoots, ok := c.seed.Status.Allocatable[core.ResourceShoots]; ok {
			scheduledShoots, err := getNumberOfShootsOnSeed(shootLister, c.seed.Name)
			if err != nil {
				return apierrors.NewInternalError(err)
			}

			if scheduledShoots >= allocatableShoots.Value() {
				return admission.NewForbidden(a, fmt.Errorf("cannot schedule shoot '%s' on seed '%s' that already has the maximum number of shoots scheduled on it (%d)", c.shoot.Name, c.seed.Name, allocatableShoots.Value()))
			}
		}
	}

	if shootIsBeingRescheduled {
		oldSeed, err := seedLister.Get(*c.oldShoot.Spec.SeedName)
		if err != nil {
			return apierrors.NewBadRequest(fmt.Sprintf("could not find referenced seed: %+v", err.Error()))
		}

		if oldSeed.Spec.Backup == nil {
			return admission.NewForbidden(a, fmt.Errorf("cannot change seed name because backup is not configured for old seed %q", oldSeed.Name))
		}
		if c.seed.Spec.Backup == nil {
			return admission.NewForbidden(a, fmt.Errorf("cannot change seed name because backup is not configured for seed %q", c.seed.Name))
		}

		if oldSeed.Spec.Provider.Type != c.seed.Spec.Provider.Type {
			return admission.NewForbidden(a, fmt.Errorf("cannot change Seed because cloud provider for new seed (%s) is not equal to cloud provider for old seed (%s)", c.seed.Spec.Provider.Type, oldSeed.Spec.Provider.Type))
		}
	}

	if c.seed != nil && c.seed.DeletionTimestamp != nil {
		newMeta := c.shoot.ObjectMeta
		oldMeta := *c.oldShoot.ObjectMeta.DeepCopy()

		// disallow any changes to the annotations of a shoot that references a seed which is already marked for deletion
		// except changes to the deletion confirmation annotation
		if !apiequality.Semantic.DeepEqual(newMeta.Annotations, oldMeta.Annotations) {
			newConfirmation, newHasConfirmation := newMeta.Annotations[gutil.ConfirmationDeletion]

			// copy the new confirmation value to the old annotations to see if
			// anything else was changed other than the confirmation annotation
			if newHasConfirmation {
				if oldMeta.Annotations == nil {
					oldMeta.Annotations = make(map[string]string)
				}
				oldMeta.Annotations[gutil.ConfirmationDeletion] = newConfirmation
			}

			if !apiequality.Semantic.DeepEqual(newMeta.Annotations, oldMeta.Annotations) {
				return admission.NewForbidden(a, fmt.Errorf("cannot update annotations of shoot '%s' on seed '%s' already marked for deletion: only the '%s' annotation can be changed", c.shoot.Name, c.seed.Name, gutil.ConfirmationDeletion))
			}
		}

		if !apiequality.Semantic.DeepEqual(c.shoot.Spec, c.oldShoot.Spec) {
			return admission.NewForbidden(a, fmt.Errorf("cannot update spec of shoot '%s' on seed '%s' already marked for deletion", c.shoot.Name, c.seed.Name))
		}
	}

	return nil
}

func getNumberOfShootsOnSeed(shootLister corelisters.ShootLister, seedName string) (int64, error) {
	allShoots, err := shootLister.Shoots(metav1.NamespaceAll).List(labels.Everything())
	if err != nil {
		return 0, fmt.Errorf("could not list all shoots: %w", err)
	}

	seedUsage := helper.CalculateSeedUsage(allShoots)
	return int64(seedUsage[seedName]), nil
}

func (c *validationContext) validateDeletion(a admission.Attributes) error {
	if a.GetOperation() == admission.Delete {
		if c.shoot.Status.LastOperation != nil &&
			((c.shoot.Status.LastOperation.Type == core.LastOperationTypeRestore && c.shoot.Status.LastOperation.State != core.LastOperationStateSucceeded) ||
				c.shoot.Status.LastOperation.Type == core.LastOperationTypeMigrate) {
			return admission.NewForbidden(a, fmt.Errorf("cannot mark shoot for deletion during %s operation that is in state %s", c.shoot.Status.LastOperation.Type, c.shoot.Status.LastOperation.State))
		}
	}

	// Allow removal of `gardener` finalizer only if the Shoot deletion has completed successfully
	if len(c.shoot.Status.TechnicalID) > 0 && c.shoot.Status.LastOperation != nil {
		oldFinalizers := sets.NewString(c.oldShoot.Finalizers...)
		newFinalizers := sets.NewString(c.shoot.Finalizers...)

		if oldFinalizers.Has(core.GardenerName) && !newFinalizers.Has(core.GardenerName) {
			lastOperation := c.shoot.Status.LastOperation
			deletionSucceeded := lastOperation.Type == core.LastOperationTypeDelete && lastOperation.State == core.LastOperationStateSucceeded && lastOperation.Progress == 100

			if !deletionSucceeded {
				return admission.NewForbidden(a, fmt.Errorf("finalizer %q cannot be removed because shoot deletion has not completed successfully yet", core.GardenerName))
			}
		}
	}

	return nil
}

func (c *validationContext) validateShootHibernation(a admission.Attributes) error {
	// Prevent Shoots from getting hibernated in case they have problematic webhooks.
	// Otherwise, we can never wake up this shoot cluster again.
	oldIsHibernated := c.oldShoot.Spec.Hibernation != nil && c.oldShoot.Spec.Hibernation.Enabled != nil && *c.oldShoot.Spec.Hibernation.Enabled
	newIsHibernated := c.shoot.Spec.Hibernation != nil && c.shoot.Spec.Hibernation.Enabled != nil && *c.shoot.Spec.Hibernation.Enabled

	if !oldIsHibernated && newIsHibernated {
		if hibernationConstraint := helper.GetCondition(c.shoot.Status.Constraints, core.ShootHibernationPossible); hibernationConstraint != nil {
			if hibernationConstraint.Status != core.ConditionTrue {
				return admission.NewForbidden(a, fmt.Errorf(hibernationConstraint.Message))
			}
		}
	}

	if !newIsHibernated && oldIsHibernated {
		addInfrastructureDeploymentTask(c.shoot)
	}

	return nil
}

func (c *validationContext) ensureMachineImages() error {
	if c.shoot.DeletionTimestamp == nil {
		for idx, worker := range c.shoot.Spec.Provider.Workers {
			image, err := ensureMachineImage(c.oldShoot.Spec.Provider.Workers, worker, c.cloudProfile.Spec.MachineImages)
			if err != nil {
				return err
			}
			c.shoot.Spec.Provider.Workers[idx].Machine.Image = image
		}
	}

	return nil
}

func (c *validationContext) addMetadataAnnotations(a admission.Attributes) {
	if a.GetOperation() == admission.Create {
		addInfrastructureDeploymentTask(c.shoot)
	}

	if !reflect.DeepEqual(c.oldShoot.Spec.Provider.InfrastructureConfig, c.shoot.Spec.Provider.InfrastructureConfig) {
		addInfrastructureDeploymentTask(c.shoot)
	}

	if c.shoot.ObjectMeta.Annotations[v1beta1constants.GardenerOperation] == v1beta1constants.ShootOperationRotateSSHKeypair {
		addInfrastructureDeploymentTask(c.shoot)
	}

	if c.shoot.Spec.Maintenance != nil && utils.IsTrue(c.shoot.Spec.Maintenance.ConfineSpecUpdateRollout) &&
		!apiequality.Semantic.DeepEqual(c.oldShoot.Spec, c.shoot.Spec) &&
		c.shoot.Status.LastOperation != nil && c.shoot.Status.LastOperation.State == core.LastOperationStateFailed {
		metav1.SetMetaDataAnnotation(&c.shoot.ObjectMeta, v1beta1constants.FailedShootNeedsRetryOperation, "true")
	}
}

func (c *validationContext) validateShootNetworks() field.ErrorList {
	var (
		allErrs field.ErrorList
		path    = field.NewPath("spec", "networking")
	)

	if c.seed != nil {
		if c.shoot.Spec.Networking.Pods == nil {
			if c.seed.Spec.Networks.ShootDefaults != nil {
				c.shoot.Spec.Networking.Pods = c.seed.Spec.Networks.ShootDefaults.Pods
			} else {
				allErrs = append(allErrs, field.Required(path.Child("pods"), "pods is required"))
			}
		}

		if c.shoot.Spec.Networking.Services == nil {
			if c.seed.Spec.Networks.ShootDefaults != nil {
				c.shoot.Spec.Networking.Services = c.seed.Spec.Networks.ShootDefaults.Services
			} else {
				allErrs = append(allErrs, field.Required(path.Child("services"), "services is required"))
			}
		}

		// validate network disjointedness with seed networks if shoot is being (re)scheduled
		if !apiequality.Semantic.DeepEqual(c.oldShoot.Spec.SeedName, c.shoot.Spec.SeedName) {
			allErrs = append(allErrs, cidrvalidation.ValidateNetworkDisjointedness(
				path,
				c.shoot.Spec.Networking.Nodes,
				c.shoot.Spec.Networking.Pods,
				c.shoot.Spec.Networking.Services,
				c.seed.Spec.Networks.Nodes,
				c.seed.Spec.Networks.Pods,
				c.seed.Spec.Networks.Services,
			)...)
		}
	}

	return allErrs
}

func (c *validationContext) validateKubernetes() field.ErrorList {
	var (
		allErrs field.ErrorList
		path    = field.NewPath("spec", "kubernetes")
	)

	ok, isDefaulted, validKubernetesVersions, versionDefault := validateKubernetesVersionConstraints(c.cloudProfile.Spec.Kubernetes.Versions, c.shoot.Spec.Kubernetes.Version, c.oldShoot.Spec.Kubernetes.Version)
	if !ok {
		err := field.NotSupported(path.Child("version"), c.shoot.Spec.Kubernetes.Version, validKubernetesVersions)
		if isDefaulted {
			err.Detail = fmt.Sprintf("unable to default version - couldn't find a suitable patch version for %s. Suitable patch versions have a non-expired expiration date and are no 'preview' versions. 'Preview'-classified versions have to be selected explicitly -  %s", c.shoot.Spec.Kubernetes.Version, err.Detail)
		}
		allErrs = append(allErrs, err)
	} else if versionDefault != nil {
		c.shoot.Spec.Kubernetes.Version = versionDefault.String()
	}

	return allErrs
}

func (c *validationContext) validateProvider() field.ErrorList {
	var (
		allErrs field.ErrorList
		path    = field.NewPath("spec", "provider")
	)

	if c.shoot.Spec.Provider.Type != c.cloudProfile.Spec.Type {
		allErrs = append(allErrs, field.Invalid(path.Child("type"), c.shoot.Spec.Provider.Type, fmt.Sprintf("provider type in shoot must equal provider type of referenced CloudProfile: %q", c.cloudProfile.Spec.Type)))
		// exit early, all other validation errors will be misleading
		return allErrs
	}

	for i, worker := range c.shoot.Spec.Provider.Workers {
		var oldWorker = core.Worker{Machine: core.Machine{Image: &core.ShootMachineImage{}}}
		for _, ow := range c.oldShoot.Spec.Provider.Workers {
			if ow.Name == worker.Name {
				oldWorker = ow
				break
			}
		}

		idxPath := path.Child("workers").Index(i)
		if ok, validMachineTypes := validateMachineTypes(c.cloudProfile.Spec.MachineTypes, worker.Machine.Type, oldWorker.Machine.Type, c.cloudProfile.Spec.Regions, c.shoot.Spec.Region, worker.Zones); !ok {
			allErrs = append(allErrs, field.NotSupported(idxPath.Child("machine", "type"), worker.Machine.Type, validMachineTypes))
		}
		if ok, validMachineImages := validateMachineImagesConstraints(c.cloudProfile.Spec.MachineImages, worker.Machine.Image, oldWorker.Machine.Image); !ok {
			allErrs = append(allErrs, field.NotSupported(idxPath.Child("machine", "image"), worker.Machine.Image, validMachineImages))
		} else {
			allErrs = append(allErrs, validateContainerRuntimeConstraints(c.cloudProfile.Spec.MachineImages, worker, oldWorker, idxPath.Child("cri"))...)
		}
		if ok, validVolumeTypes := validateVolumeTypes(c.cloudProfile.Spec.VolumeTypes, worker.Volume, oldWorker.Volume, c.cloudProfile.Spec.Regions, c.shoot.Spec.Region, worker.Zones); !ok {
			allErrs = append(allErrs, field.NotSupported(idxPath.Child("volume", "type"), worker.Volume, validVolumeTypes))
		}
		if ok, minSize := validateVolumeSize(c.cloudProfile.Spec.VolumeTypes, c.cloudProfile.Spec.MachineTypes, worker.Machine.Type, worker.Volume); !ok {
			allErrs = append(allErrs, field.Invalid(idxPath.Child("volume", "size"), worker.Volume.VolumeSize, fmt.Sprintf("size must be >= %s", minSize)))
		}

		allErrs = append(allErrs, validateZones(c.cloudProfile.Spec.Regions, c.shoot.Spec.Region, c.oldShoot.Spec.Region, worker, oldWorker, idxPath)...)
	}

	return allErrs
}

func validateVolumeSize(volumeTypeConstraints []core.VolumeType, machineTypeConstraints []core.MachineType, machineType string, volume *core.Volume) (bool, string) {
	if volume == nil {
		return true, ""
	}

	volSize, err := resource.ParseQuantity(volume.VolumeSize)
	if err != nil {
		// don't fail here, this is the shoot validator's job
		return true, ""
	}
	volType := volume.Type
	if volType == nil {
		return true, ""
	}

	// Check machine type constraints first since they override any other constraint for volume types.
	for _, machineTypeConstraint := range machineTypeConstraints {
		if machineType != machineTypeConstraint.Name {
			continue
		}
		if machineTypeConstraint.Storage == nil || machineTypeConstraint.Storage.MinSize == nil {
			continue
		}
		if machineTypeConstraint.Storage.Type != *volType {
			continue
		}
		if volSize.Cmp(*machineTypeConstraint.Storage.MinSize) < 0 {
			return false, machineTypeConstraint.Storage.MinSize.String()
		}
	}

	// Now check more common volume type constraints.
	for _, volumeTypeConstraint := range volumeTypeConstraints {
		if volumeTypeConstraint.Name == *volType && volumeTypeConstraint.MinSize != nil {
			if volSize.Cmp(*volumeTypeConstraint.MinSize) < 0 {
				return false, volumeTypeConstraint.MinSize.String()
			}
		}
	}
	return true, ""
}

func (c *validationContext) validateDNSDomainUniqueness(shootLister corelisters.ShootLister) (field.ErrorList, error) {
	var (
		allErrs field.ErrorList
		dns     = c.shoot.Spec.DNS
		dnsPath = field.NewPath("spec", "dns", "domain")
	)

	if dns == nil || dns.Domain == nil {
		return allErrs, nil
	}

	shoots, err := shootLister.Shoots(metav1.NamespaceAll).List(labels.Everything())
	if err != nil {
		return allErrs, err
	}

	for _, shoot := range shoots {
		if shoot.Name == c.shoot.Name {
			continue
		}

		var domain *string
		if shoot.Spec.DNS != nil {
			domain = shoot.Spec.DNS.Domain
		}
		if domain == nil {
			continue
		}

		// Prevent that this shoot uses the exact same domain of any other shoot in the system.
		if *domain == *dns.Domain {
			allErrs = append(allErrs, field.Duplicate(dnsPath, *dns.Domain))
			break
		}

		// Prevent that this shoot uses a subdomain of the domain of any other shoot in the system.
		if hasDomainIntersection(*domain, *dns.Domain) {
			allErrs = append(allErrs, field.Forbidden(dnsPath, "the domain is already used by another shoot or it is a subdomain of an already used domain"))
			break
		}
	}

	return allErrs, nil
}

// hasDomainIntersection checks if domainA is a suffix of domainB or domainB is a suffix of domainA.
func hasDomainIntersection(domainA, domainB string) bool {
	if domainA == domainB {
		return true
	}

	var short, long string
	if len(domainA) > len(domainB) {
		short = domainB
		long = domainA
	} else {
		short = domainA
		long = domainB
	}

	if !strings.HasPrefix(short, ".") {
		short = fmt.Sprintf(".%s", short)
	}

	return strings.HasSuffix(long, short)
}

func validateKubernetesVersionConstraints(constraints []core.ExpirableVersion, shootVersion, oldShootVersion string) (bool, bool, []string, *semver.Version) {
	if shootVersion == oldShootVersion {
		return true, false, nil, nil
	}

	shootVersionSplit := strings.Split(shootVersion, ".")
	var (
		shootVersionMajor, shootVersionMinor int64
		defaultToLatestPatchVersion          bool
	)
	if len(shootVersionSplit) == 2 {
		// add a fake patch version to avoid manual parsing
		fakeShootVersion := shootVersion + ".0"
		version, err := semver.NewVersion(fakeShootVersion)
		if err == nil {
			defaultToLatestPatchVersion = true
			shootVersionMajor = version.Major()
			shootVersionMinor = version.Minor()
		}
	}

	var validValues []string
	var latestVersion *semver.Version
	for _, versionConstraint := range constraints {
		if versionConstraint.ExpirationDate != nil && versionConstraint.ExpirationDate.Time.UTC().Before(time.Now().UTC()) {
			continue
		}

		// filter preview versions for defaulting
		if defaultToLatestPatchVersion && versionConstraint.Classification != nil && *versionConstraint.Classification == core.ClassificationPreview {
			validValues = append(validValues, fmt.Sprintf("%s (preview)", versionConstraint.Version))
			continue
		}

		validValues = append(validValues, versionConstraint.Version)

		if versionConstraint.Version == shootVersion {
			return true, false, nil, nil
		}

		if defaultToLatestPatchVersion {
			// CloudProfile cannot contain invalid semVer shootVersion
			cpVersion, _ := semver.NewVersion(versionConstraint.Version)

			// defaulting on patch level: version has to have the same major and minor kubernetes version
			if cpVersion.Major() != shootVersionMajor || cpVersion.Minor() != shootVersionMinor {
				continue
			}

			if latestVersion == nil || cpVersion.GreaterThan(latestVersion) {
				latestVersion = cpVersion
			}
		}
	}

	if latestVersion != nil {
		return true, defaultToLatestPatchVersion, nil, latestVersion
	}

	return false, defaultToLatestPatchVersion, validValues, nil
}

func validateMachineTypes(constraints []core.MachineType, machineType, oldMachineType string, regions []core.Region, region string, zones []string) (bool, []string) {
	if machineType == oldMachineType {
		return true, nil
	}

	validValues := []string{}

	var unavailableInAtLeastOneZone bool
top:
	for _, r := range regions {
		if r.Name != region {
			continue
		}

		for _, zoneName := range zones {
			for _, z := range r.Zones {
				if z.Name != zoneName {
					continue
				}

				for _, t := range z.UnavailableMachineTypes {
					if t == machineType {
						unavailableInAtLeastOneZone = true
						break top
					}
				}
			}
		}
	}

	for _, t := range constraints {
		if t.Usable != nil && !*t.Usable {
			continue
		}
		if unavailableInAtLeastOneZone {
			continue
		}
		validValues = append(validValues, t.Name)
		if t.Name == machineType {
			return true, nil
		}
	}

	return false, validValues
}

func validateVolumeTypes(constraints []core.VolumeType, volume, oldVolume *core.Volume, regions []core.Region, region string, zones []string) (bool, []string) {
	if volume == nil || volume.Type == nil || (volume != nil && oldVolume != nil && volume.Type != nil && oldVolume.Type != nil && *volume.Type == *oldVolume.Type) {
		return true, nil
	}

	var volumeType string
	if volume != nil && volume.Type != nil {
		volumeType = *volume.Type
	}

	validValues := []string{}

	var unavailableInAtLeastOneZone bool
top:
	for _, r := range regions {
		if r.Name != region {
			continue
		}

		for _, zoneName := range zones {
			for _, z := range r.Zones {
				if z.Name != zoneName {
					continue
				}

				for _, t := range z.UnavailableVolumeTypes {
					if t == volumeType {
						unavailableInAtLeastOneZone = true
						break top
					}
				}
			}
		}
	}

	for _, v := range constraints {
		if v.Usable != nil && !*v.Usable {
			continue
		}
		if unavailableInAtLeastOneZone {
			continue
		}
		validValues = append(validValues, v.Name)
		if v.Name == volumeType {
			return true, nil
		}
	}

	return false, validValues
}

func (c *validationContext) validateRegion() field.ErrorList {
	var (
		fldPath     = field.NewPath("spec", "region")
		validValues []string
		region      = c.shoot.Spec.Region
		oldRegion   = c.oldShoot.Spec.Region
	)

	if region == oldRegion {
		return nil
	}

	for _, r := range c.cloudProfile.Spec.Regions {
		validValues = append(validValues, r.Name)
		if r.Name == region {
			return nil
		}
	}

	return field.ErrorList{field.NotSupported(fldPath, region, validValues)}
}

func validateZones(constraints []core.Region, region, oldRegion string, worker, oldWorker core.Worker, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if region == oldRegion && reflect.DeepEqual(worker.Zones, oldWorker.Zones) {
		return allErrs
	}

	for j, zone := range worker.Zones {
		jdxPath := fldPath.Child("zones").Index(j)
		if ok, validZones := validateZone(constraints, region, zone); !ok {
			if len(validZones) == 0 {
				allErrs = append(allErrs, field.Invalid(jdxPath, region, "this region does not support availability zones, please do not configure them"))
			} else {
				allErrs = append(allErrs, field.NotSupported(jdxPath, zone, validZones))
			}
		}
	}

	return allErrs
}

func validateZone(constraints []core.Region, region, zone string) (bool, []string) {
	validValues := []string{}

	for _, r := range constraints {
		if r.Name == region {
			for _, z := range r.Zones {
				validValues = append(validValues, z.Name)
				if z.Name == zone {
					return true, nil
				}
			}
			break
		}
	}

	return false, validValues
}

// getDefaultMachineImage determines the latest non-preview machine image version from the first machine image in the CloudProfile and considers that as the default image
func getDefaultMachineImage(machineImages []core.MachineImage, imageName string) (*core.ShootMachineImage, error) {
	if len(machineImages) == 0 {
		return nil, errors.New("the cloud profile does not contain any machine image - cannot create shoot cluster")
	}

	var defaultImage *core.MachineImage

	if len(imageName) != 0 {
		for _, machineImage := range machineImages {
			if machineImage.Name == imageName {
				defaultImage = &machineImage
				break
			}
		}
		if defaultImage == nil {
			return nil, fmt.Errorf("image name %q is not supported", imageName)
		}
	} else {
		defaultImage = &machineImages[0]
	}

	latestMachineImageVersion, err := helper.DetermineLatestMachineImageVersion(defaultImage.Versions, true)
	if err != nil {
		return nil, fmt.Errorf("failed to determine latest machine image from cloud profile: %s", err.Error())
	}
	return &core.ShootMachineImage{Name: defaultImage.Name, Version: latestMachineImageVersion.Version}, nil
}

func validateMachineImagesConstraints(constraints []core.MachineImage, image, oldImage *core.ShootMachineImage) (bool, []string) {
	if oldImage == nil || apiequality.Semantic.DeepEqual(image, oldImage) {
		return true, nil
	}

	validValues := []string{}
	if image == nil {
		for _, machineImage := range constraints {
			for _, machineVersion := range machineImage.Versions {
				if machineVersion.ExpirationDate != nil && machineVersion.ExpirationDate.Time.UTC().Before(time.Now().UTC()) {
					continue
				}
				validValues = append(validValues, fmt.Sprintf("machineImage(%s:%s)", machineImage.Name, machineVersion.Version))
			}
		}

		return false, validValues
	}

	if len(image.Version) == 0 {
		return true, nil
	}

	for _, machineImage := range constraints {
		if machineImage.Name == image.Name {
			for _, machineVersion := range machineImage.Versions {
				if machineVersion.ExpirationDate != nil && machineVersion.ExpirationDate.Time.UTC().Before(time.Now().UTC()) {
					continue
				}
				validValues = append(validValues, fmt.Sprintf("machineImage(%s:%s)", machineImage.Name, machineVersion.Version))

				if machineVersion.Version == image.Version {
					return true, nil
				}
			}
		}
	}
	return false, validValues
}

func validateContainerRuntimeConstraints(constraints []core.MachineImage, worker, oldWorker core.Worker, fldPath *field.Path) field.ErrorList {
	if worker.CRI == nil || worker.Machine.Image == nil {
		return nil
	}

	if apiequality.Semantic.DeepEqual(worker.CRI, oldWorker.CRI) &&
		apiequality.Semantic.DeepEqual(worker.Machine.Image, oldWorker.Machine.Image) {
		return nil
	}

	var machineImage *core.MachineImage
	var machineVersion *core.MachineImageVersion

	for _, image := range constraints {
		if image.Name == worker.Machine.Image.Name {
			machineImage = &image
			break
		}
	}

	if machineImage == nil {
		return nil
	}

	for _, version := range machineImage.Versions {
		if version.Version == worker.Machine.Image.Version {
			machineVersion = &version
			break
		}
	}
	if machineVersion == nil {
		return nil
	}
	return validateCRI(machineVersion.CRI, worker.CRI, fldPath)
}

func validateCRI(constraints []core.CRI, cri *core.CRI, fldPath *field.Path) field.ErrorList {
	if cri == nil {
		return nil
	}

	var (
		allErrors = field.ErrorList{}
		validCRIs = []string{}
		foundCRI  *core.CRI
	)

	for _, criConstraint := range constraints {
		validCRIs = append(validCRIs, string(criConstraint.Name))
		if cri.Name == criConstraint.Name {
			foundCRI = &criConstraint
			break
		}
	}
	if foundCRI == nil {
		allErrors = append(allErrors, field.NotSupported(fldPath.Child("name"), cri.Name, validCRIs))
		return allErrors
	}

	for j, runtime := range cri.ContainerRuntimes {
		jdxPath := fldPath.Child("containerRuntimes").Index(j)
		if ok, validValues := validateCRMembership(foundCRI.ContainerRuntimes, runtime.Type); !ok {
			allErrors = append(allErrors, field.NotSupported(jdxPath.Child("type"), runtime, validValues))
		}
	}

	return allErrors
}

func validateCRMembership(constraints []core.ContainerRuntime, cr string) (bool, []string) {
	validValues := []string{}
	for _, constraint := range constraints {
		validValues = append(validValues, constraint.Type)
		if constraint.Type == cr {
			return true, nil
		}
	}
	return false, validValues
}

func ensureMachineImage(oldWorkers []core.Worker, worker core.Worker, images []core.MachineImage) (*core.ShootMachineImage, error) {
	// General approach with machine image defaulting in this code: Try to keep the machine image
	// from the old shoot object to not accidentally update it to the default machine image.
	// This should only happen in the maintenance time window of shoots and is performed by the
	// shoot maintenance controller.

	oldWorker := helper.FindWorkerByName(oldWorkers, worker.Name)
	if oldWorker != nil && oldWorker.Machine.Image != nil {
		// worker is already existing -> keep the machine image if name/version is unspecified
		if worker.Machine.Image == nil {
			// machine image completely unspecified in new worker -> keep the old one
			return oldWorker.Machine.Image, nil
		}

		if oldWorker.Machine.Image.Name == worker.Machine.Image.Name {
			// image name was not changed -> keep version from the new worker if specified, otherwise use the old worker image version
			if len(worker.Machine.Image.Version) != 0 {
				return worker.Machine.Image, nil
			}
			return oldWorker.Machine.Image, nil
		} else {
			// image name was changed -> keep version from new worker if specified, otherwise default the image version
			if len(worker.Machine.Image.Version) != 0 {
				return worker.Machine.Image, nil
			}
		}
	}

	imageName := ""
	if worker.Machine.Image != nil {
		if len(worker.Machine.Image.Version) != 0 {
			return worker.Machine.Image, nil
		}
		imageName = worker.Machine.Image.Name
	}

	return getDefaultMachineImage(images, imageName)
}

func addInfrastructureDeploymentTask(shoot *core.Shoot) {
	if shoot.ObjectMeta.Annotations == nil {
		shoot.ObjectMeta.Annotations = make(map[string]string)
	}
	controllerutils.AddTasks(shoot.ObjectMeta.Annotations, v1beta1constants.ShootTaskDeployInfrastructure)
}
