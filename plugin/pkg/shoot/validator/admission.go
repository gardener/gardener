// Copyright 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"strconv"
	"strings"
	"time"

	"github.com/Masterminds/semver"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	kubeinformers "k8s.io/client-go/informers"
	kubecorev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/utils/pointer"
	"k8s.io/utils/strings/slices"

	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apis/core/helper"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	admissioninitializer "github.com/gardener/gardener/pkg/apiserver/admission/initializer"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/internalversion"
	gardencorelisters "github.com/gardener/gardener/pkg/client/core/listers/core/internalversion"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	cidrvalidation "github.com/gardener/gardener/pkg/utils/validation/cidr"
	versionutils "github.com/gardener/gardener/pkg/utils/version"
	admissionutils "github.com/gardener/gardener/plugin/pkg/utils"
)

const (
	// PluginName is the name of this admission plugin.
	PluginName = "ShootValidator"

	internalVersionErrorMsg = "must not use apiVersion 'internal'"
)

// Register registers a plugin.
func Register(plugins *admission.Plugins) {
	plugins.Register(PluginName, func(config io.Reader) (admission.Interface, error) {
		return New()
	})
}

// ValidateShoot contains listers and admission handler.
type ValidateShoot struct {
	*admission.Handler
	authorizer          authorizer.Authorizer
	secretLister        kubecorev1listers.SecretLister
	cloudProfileLister  gardencorelisters.CloudProfileLister
	seedLister          gardencorelisters.SeedLister
	shootLister         gardencorelisters.ShootLister
	projectLister       gardencorelisters.ProjectLister
	secretBindingLister gardencorelisters.SecretBindingLister
	readyFunc           admission.ReadyFunc
}

var (
	_ = admissioninitializer.WantsInternalCoreInformerFactory(&ValidateShoot{})
	_ = admissioninitializer.WantsKubeInformerFactory(&ValidateShoot{})
	_ = admissioninitializer.WantsAuthorizer(&ValidateShoot{})

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

// SetAuthorizer gets the authorizer.
func (v *ValidateShoot) SetAuthorizer(authorizer authorizer.Authorizer) {
	v.authorizer = authorizer
}

// SetInternalCoreInformerFactory gets Lister from SharedInformerFactory.
func (v *ValidateShoot) SetInternalCoreInformerFactory(f gardencoreinformers.SharedInformerFactory) {
	seedInformer := f.Core().InternalVersion().Seeds()
	v.seedLister = seedInformer.Lister()

	shootInformer := f.Core().InternalVersion().Shoots()
	v.shootLister = shootInformer.Lister()

	cloudProfileInformer := f.Core().InternalVersion().CloudProfiles()
	v.cloudProfileLister = cloudProfileInformer.Lister()

	projectInformer := f.Core().InternalVersion().Projects()
	v.projectLister = projectInformer.Lister()

	secretBindingInformer := f.Core().InternalVersion().SecretBindings()
	v.secretBindingLister = secretBindingInformer.Lister()

	readyFuncs = append(
		readyFuncs,
		seedInformer.Informer().HasSynced,
		shootInformer.Informer().HasSynced,
		cloudProfileInformer.Informer().HasSynced,
		projectInformer.Informer().HasSynced,
		secretBindingInformer.Informer().HasSynced,
	)
}

// SetKubeInformerFactory gets Lister from SharedInformerFactory.
func (v *ValidateShoot) SetKubeInformerFactory(f kubeinformers.SharedInformerFactory) {
	secretInformer := f.Core().V1().Secrets()
	v.secretLister = secretInformer.Lister()

	readyFuncs = append(readyFuncs, secretInformer.Informer().HasSynced)
}

// ValidateInitialization checks whether the plugin was correctly initialized.
func (v *ValidateShoot) ValidateInitialization() error {
	if v.authorizer == nil {
		return errors.New("missing authorizer")
	}
	if v.secretLister == nil {
		return errors.New("missing secret lister")
	}
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

	// Ignore updates to all subresources, except for binding
	// Binding subresource is required because there are fields being set in the shoot
	// when it is scheduled and we want this plugin to be triggered.
	if a.GetSubresource() != "" && a.GetSubresource() != "binding" {
		return nil
	}

	var (
		shoot               = &core.Shoot{}
		oldShoot            = &core.Shoot{}
		convertIsSuccessful bool
		allErrs             field.ErrorList
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

		if a.GetSubresource() == "binding" && reflect.DeepEqual(oldShoot.Spec.SeedName, shoot.Spec.SeedName) {
			return fmt.Errorf("update of binding rejected, shoot is already assigned to the same seed")
		}

		// do not ignore metadata updates to detect and prevent removal of the gardener finalizer or unwanted changes to annotations
		if reflect.DeepEqual(shoot.Spec, oldShoot.Spec) && reflect.DeepEqual(shoot.ObjectMeta, oldShoot.ObjectMeta) {
			return nil
		}
	}

	cloudProfile, err := v.cloudProfileLister.Get(shoot.Spec.CloudProfileName)
	if err != nil {
		return apierrors.NewInternalError(fmt.Errorf("could not find referenced cloud profile: %+v", err.Error()))
	}

	var seed *core.Seed
	if shoot.Spec.SeedName != nil {
		seed, err = v.seedLister.Get(*shoot.Spec.SeedName)
		if err != nil {
			return apierrors.NewInternalError(fmt.Errorf("could not find referenced seed %q: %+v", *shoot.Spec.SeedName, err.Error()))
		}
	}

	project, err := admissionutils.ProjectForNamespaceFromInternalLister(v.projectLister, shoot.Namespace)
	if err != nil {
		return apierrors.NewInternalError(fmt.Errorf("could not find referenced project: %+v", err.Error()))
	}

	var secretBinding *core.SecretBinding
	if a.GetOperation() == admission.Create && shoot.Spec.SecretBindingName != nil {
		secretBinding, err = v.secretBindingLister.SecretBindings(shoot.Namespace).Get(*shoot.Spec.SecretBindingName)
		if err != nil {
			return apierrors.NewInternalError(fmt.Errorf("could not find referenced secret binding: %+v", err.Error()))
		}
	}

	// begin of validation code
	validationContext := &validationContext{
		cloudProfile:  cloudProfile,
		project:       project,
		seed:          seed,
		secretBinding: secretBinding,
		shoot:         shoot,
		oldShoot:      oldShoot,
	}

	if err := validationContext.validateProjectMembership(a); err != nil {
		return err
	}
	if err := validationContext.validateScheduling(ctx, a, v.authorizer, v.shootLister, v.seedLister); err != nil {
		return err
	}
	if err := validationContext.validateDeletion(a); err != nil {
		return err
	}
	if err := validationContext.validateShootHibernation(a); err != nil {
		return err
	}
	if allErrs = validationContext.ensureMachineImages(); len(allErrs) > 0 {
		return admission.NewForbidden(a, fmt.Errorf("%+v", allErrs))
	}

	validationContext.addMetadataAnnotations(a)

	allErrs = append(allErrs, validationContext.validateAPIVersionForRawExtensions()...)
	allErrs = append(allErrs, validationContext.validateShootNetworks(a, helper.IsWorkerless(shoot))...)
	allErrs = append(allErrs, validationContext.validateKubernetes(a)...)
	allErrs = append(allErrs, validationContext.validateRegion()...)
	allErrs = append(allErrs, validationContext.validateProvider(a)...)
	allErrs = append(allErrs, validationContext.validateAdmissionPlugins(a, v.secretLister)...)

	// Skip the validation if the operation is admission.Delete or the spec hasn't changed.
	if a.GetOperation() != admission.Delete && !reflect.DeepEqual(validationContext.shoot.Spec, validationContext.oldShoot.Spec) {
		dnsErrors, err := validationContext.validateDNSDomainUniqueness(v.shootLister)
		if err != nil {
			return apierrors.NewInternalError(err)
		}
		allErrs = append(allErrs, dnsErrors...)
	}

	if len(allErrs) > 0 {
		return admission.NewForbidden(a, fmt.Errorf("%+v", allErrs))
	}

	return nil
}

type validationContext struct {
	cloudProfile  *core.CloudProfile
	project       *core.Project
	seed          *core.Seed
	secretBinding *core.SecretBinding
	shoot         *core.Shoot
	oldShoot      *core.Shoot
}

func (c *validationContext) validateProjectMembership(a admission.Attributes) error {
	if a.GetOperation() != admission.Create {
		return nil
	}

	namePath := field.NewPath("name")

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
			fieldErr := field.Invalid(namePath, c.shoot.Name, fmt.Sprintf("the length of the shoot generateName and the project name must not exceed %d characters (project: %s; shoot with generateName: %s)", lengthLimit-randomLength, c.project.Name, c.shoot.GenerateName))
			return apierrors.NewInvalid(a.GetKind().GroupKind(), c.shoot.Name, field.ErrorList{fieldErr})
		}
	} else {
		if len(c.project.Name+c.shoot.Name) > lengthLimit {
			fieldErr := field.Invalid(namePath, c.shoot.Name, fmt.Sprintf("the length of the shoot name and the project name must not exceed %d characters (project: %s; shoot: %s)", lengthLimit, c.project.Name, c.shoot.Name))
			return apierrors.NewInvalid(a.GetKind().GroupKind(), c.shoot.Name, field.ErrorList{fieldErr})
		}
	}

	if c.project.DeletionTimestamp != nil {
		return admission.NewForbidden(a, fmt.Errorf("cannot create shoot '%s' in project '%s' that is already marked for deletion", c.shoot.Name, c.project.Name))
	}

	return nil
}

// validateSeedSelectionForMultiZonalShoot ensures that the selected Seed has at least 3 zones when the Shoot is highly
// available and specifies failure tolerance zone.
func (c *validationContext) validateSeedSelectionForMultiZonalShoot() error {
	if helper.IsMultiZonalShootControlPlane(c.shoot) && len(c.seed.Spec.Provider.Zones) < 3 {
		return fmt.Errorf("cannot schedule shoot '%s' with failure tolerance of zone on seed '%s' with only %d zones, at least 3 zones are required", c.shoot.Name, c.seed.Name, len(c.seed.Spec.Provider.Zones))
	}
	return nil
}

func (c *validationContext) validateScheduling(ctx context.Context, a admission.Attributes, authorizer authorizer.Authorizer, shootLister gardencorelisters.ShootLister, seedLister gardencorelisters.SeedLister) error {
	var (
		shootIsBeingScheduled          = c.oldShoot.Spec.SeedName == nil && c.shoot.Spec.SeedName != nil
		shootIsBeingRescheduled        = c.oldShoot.Spec.SeedName != nil && c.shoot.Spec.SeedName != nil && *c.shoot.Spec.SeedName != *c.oldShoot.Spec.SeedName
		mustCheckSchedulingConstraints = shootIsBeingScheduled || shootIsBeingRescheduled
	)

	switch a.GetOperation() {
	case admission.Create:
		if shootIsBeingScheduled {
			if err := authorize(ctx, a, authorizer, "set .spec.seedName"); err != nil {
				return err
			}
		}
	case admission.Update:
		if a.GetSubresource() == "binding" {
			if c.oldShoot.Spec.SeedName != nil && c.shoot.Spec.SeedName == nil {
				return admission.NewForbidden(a, fmt.Errorf("spec.seedName cannot be set to nil"))
			}

			if shootIsBeingRescheduled {
				newShootSpec := c.shoot.Spec
				newShootSpec.SeedName = c.oldShoot.Spec.SeedName
				if !reflect.DeepEqual(newShootSpec, c.oldShoot.Spec) {
					return admission.NewForbidden(a, fmt.Errorf("only spec.seedName can be changed using the binding subresource when the shoot is being rescheduled to a new seed"))
				}
			}
		} else if !reflect.DeepEqual(c.shoot.Spec.SeedName, c.oldShoot.Spec.SeedName) {
			return admission.NewForbidden(a, fmt.Errorf("spec.seedName cannot be changed by patching the shoot, please use the shoots/binding subresource"))
		}
	case admission.Delete:
		return nil
	}

	if mustCheckSchedulingConstraints {
		if c.seed.DeletionTimestamp != nil {
			return admission.NewForbidden(a, fmt.Errorf("cannot schedule shoot '%s' on seed '%s' that is already marked for deletion", c.shoot.Name, c.seed.Name))
		}

		if !helper.TaintsAreTolerated(c.seed.Spec.Taints, c.shoot.Spec.Tolerations) {
			return admission.NewForbidden(a, fmt.Errorf("forbidden to use a seed whose taints are not tolerated by the shoot"))
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

		if seedSelector := c.cloudProfile.Spec.SeedSelector; seedSelector != nil {
			selector, err := metav1.LabelSelectorAsSelector(&seedSelector.LabelSelector)
			if err != nil {
				return apierrors.NewInternalError(fmt.Errorf("label selector conversion failed: %v for seedSelector: %w", seedSelector.LabelSelector, err))
			}
			if !selector.Matches(labels.Set(c.seed.Labels)) {
				return admission.NewForbidden(a, fmt.Errorf("cannot schedule shoot '%s' on seed '%s' because the seed selector of cloud profile '%s' is not matching the labels of the seed", c.shoot.Name, c.seed.Name, c.cloudProfile.Name))
			}

			if seedSelector.ProviderTypes != nil {
				if !sets.New(seedSelector.ProviderTypes...).HasAny(c.seed.Spec.Provider.Type, "*") {
					return admission.NewForbidden(a, fmt.Errorf("cannot schedule shoot '%s' on seed '%s' because none of the provider types in the seed selector of cloud profile '%s' is matching the provider type of the seed", c.shoot.Name, c.seed.Name, c.cloudProfile.Name))
				}
			}
		}
	}

	if shootIsBeingRescheduled {
		oldSeed, err := seedLister.Get(*c.oldShoot.Spec.SeedName)
		if err != nil {
			return apierrors.NewInternalError(fmt.Errorf("could not find referenced seed: %+v", err.Error()))
		}

		if oldSeed.Spec.Backup == nil {
			return admission.NewForbidden(a, fmt.Errorf("cannot change seed name because backup is not configured for old seed %q", oldSeed.Name))
		}
		if c.seed.Spec.Backup == nil {
			return admission.NewForbidden(a, fmt.Errorf("cannot change seed name because backup is not configured for seed %q", c.seed.Name))
		}

		if oldSeed.Spec.Provider.Type != c.seed.Spec.Provider.Type {
			return admission.NewForbidden(a, fmt.Errorf("cannot change seed because cloud provider for new seed (%s) is not equal to cloud provider for old seed (%s)", c.seed.Spec.Provider.Type, oldSeed.Spec.Provider.Type))
		}
	} else if !reflect.DeepEqual(c.oldShoot.Spec, c.shoot.Spec) {
		if wasShootRescheduledToNewSeed(c.shoot) {
			return admission.NewForbidden(a, fmt.Errorf("shoot spec cannot be changed because shoot has been rescheduled to a new seed"))
		}
		if isShootInMigrationOrRestorePhase(c.shoot) && !reflect.DeepEqual(c.oldShoot.Spec, c.shoot.Spec) {
			return admission.NewForbidden(a, fmt.Errorf("cannot change shoot spec during %s operation that is in state %s", c.shoot.Status.LastOperation.Type, c.shoot.Status.LastOperation.State))
		}
	}

	if c.seed != nil {
		if err := c.validateSeedSelectionForMultiZonalShoot(); err != nil {
			return admission.NewForbidden(a, err)
		}

		if c.seed.DeletionTimestamp != nil {
			newMeta := c.shoot.ObjectMeta
			oldMeta := *c.oldShoot.ObjectMeta.DeepCopy()

			// disallow any changes to the annotations of a shoot that references a seed which is already marked for deletion
			// except changes to the deletion confirmation annotation
			if !apiequality.Semantic.DeepEqual(newMeta.Annotations, oldMeta.Annotations) {
				newConfirmation, newHasConfirmation := newMeta.Annotations[gardenerutils.ConfirmationDeletion]

				// copy the new confirmation value to the old annotations to see if
				// anything else was changed other than the confirmation annotation
				if newHasConfirmation {
					if oldMeta.Annotations == nil {
						oldMeta.Annotations = make(map[string]string)
					}
					oldMeta.Annotations[gardenerutils.ConfirmationDeletion] = newConfirmation
				}

				if !apiequality.Semantic.DeepEqual(newMeta.Annotations, oldMeta.Annotations) {
					return admission.NewForbidden(a, fmt.Errorf("cannot update annotations of shoot '%s' on seed '%s' already marked for deletion: only the '%s' annotation can be changed", c.shoot.Name, c.seed.Name, gardenerutils.ConfirmationDeletion))
				}
			}

			if !apiequality.Semantic.DeepEqual(c.shoot.Spec, c.oldShoot.Spec) {
				return admission.NewForbidden(a, fmt.Errorf("cannot update spec of shoot '%s' on seed '%s' already marked for deletion", c.shoot.Name, c.seed.Name))
			}
		}
	}

	return nil
}

func getNumberOfShootsOnSeed(shootLister gardencorelisters.ShootLister, seedName string) (int64, error) {
	allShoots, err := shootLister.Shoots(metav1.NamespaceAll).List(labels.Everything())
	if err != nil {
		return 0, fmt.Errorf("could not list all shoots: %w", err)
	}

	seedUsage := helper.CalculateSeedUsage(allShoots)
	return int64(seedUsage[seedName]), nil
}

func authorize(ctx context.Context, a admission.Attributes, auth authorizer.Authorizer, operation string) error {
	var (
		userInfo  = a.GetUserInfo()
		resource  = a.GetResource()
		namespace = a.GetNamespace()
		name      = a.GetName()
	)

	decision, _, err := auth.Authorize(ctx, authorizer.AttributesRecord{
		User:            userInfo,
		APIGroup:        resource.Group,
		Resource:        resource.Resource,
		Subresource:     "binding",
		Namespace:       namespace,
		Name:            name,
		Verb:            "update",
		ResourceRequest: true,
	})

	if err != nil {
		return err
	}

	if decision != authorizer.DecisionAllow {
		return admission.NewForbidden(a, fmt.Errorf("user %q is not allowed to %s for %q", userInfo.GetName(), operation, resource.Resource))
	}

	return nil
}

func (c *validationContext) validateDeletion(a admission.Attributes) error {
	if a.GetOperation() == admission.Delete {
		if isShootInMigrationOrRestorePhase(c.shoot) {
			return admission.NewForbidden(a, fmt.Errorf("cannot mark shoot for deletion during %s operation that is in state %s", c.shoot.Status.LastOperation.Type, c.shoot.Status.LastOperation.State))
		}
	}

	// Allow removal of `gardener` finalizer only if the Shoot deletion has completed successfully
	if len(c.shoot.Status.TechnicalID) > 0 && c.shoot.Status.LastOperation != nil {
		oldFinalizers := sets.New(c.oldShoot.Finalizers...)
		newFinalizers := sets.New(c.shoot.Finalizers...)

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
				err := fmt.Errorf("'%s' constraint is '%s': %s", core.ShootHibernationPossible, hibernationConstraint.Status, hibernationConstraint.Message)
				return admission.NewForbidden(a, err)
			}
		}
	}

	if !newIsHibernated && oldIsHibernated {
		addInfrastructureDeploymentTask(c.shoot)
		addDNSRecordDeploymentTasks(c.shoot)
	}

	return nil
}

func (c *validationContext) ensureMachineImages() field.ErrorList {
	allErrs := field.ErrorList{}

	if c.shoot.DeletionTimestamp == nil {
		for idx, worker := range c.shoot.Spec.Provider.Workers {
			fldPath := field.NewPath("spec", "provider", "workers").Index(idx)
			image, err := ensureMachineImage(c.oldShoot.Spec.Provider.Workers, worker, c.cloudProfile.Spec.MachineImages, fldPath)
			if err != nil {
				allErrs = append(allErrs, err)
				continue
			}
			c.shoot.Spec.Provider.Workers[idx].Machine.Image = image
		}
	}

	return allErrs
}

func (c *validationContext) addMetadataAnnotations(a admission.Attributes) {
	if a.GetOperation() == admission.Create {
		addInfrastructureDeploymentTask(c.shoot)
		addDNSRecordDeploymentTasks(c.shoot)
	}

	if !reflect.DeepEqual(c.oldShoot.Spec.Provider.InfrastructureConfig, c.shoot.Spec.Provider.InfrastructureConfig) {
		addInfrastructureDeploymentTask(c.shoot)
	}

	// We rely that SSHAccess is defaulted in the shoot creation, that is why we do not check for nils for the new shoot object.
	if c.oldShoot.Spec.Provider.WorkersSettings != nil &&
		c.oldShoot.Spec.Provider.WorkersSettings.SSHAccess != nil &&
		c.oldShoot.Spec.Provider.WorkersSettings.SSHAccess.Enabled != c.shoot.Spec.Provider.WorkersSettings.SSHAccess.Enabled {
		addInfrastructureDeploymentTask(c.shoot)
	}

	if !reflect.DeepEqual(c.oldShoot.Spec.DNS, c.shoot.Spec.DNS) {
		addDNSRecordDeploymentTasks(c.shoot)
	}

	if c.shoot.ObjectMeta.Annotations[v1beta1constants.GardenerOperation] == v1beta1constants.ShootOperationRotateSSHKeypair ||
		c.shoot.ObjectMeta.Annotations[v1beta1constants.GardenerOperation] == v1beta1constants.OperationRotateCredentialsStart {
		addInfrastructureDeploymentTask(c.shoot)
	}

	if c.shoot.Spec.Maintenance != nil &&
		pointer.BoolDeref(c.shoot.Spec.Maintenance.ConfineSpecUpdateRollout, false) &&
		!apiequality.Semantic.DeepEqual(c.oldShoot.Spec, c.shoot.Spec) &&
		c.shoot.Status.LastOperation != nil &&
		c.shoot.Status.LastOperation.State == core.LastOperationStateFailed {

		metav1.SetMetaDataAnnotation(&c.shoot.ObjectMeta, v1beta1constants.FailedShootNeedsRetryOperation, "true")
	}
}

func (c *validationContext) validateAdmissionPlugins(a admission.Attributes, secretLister kubecorev1listers.SecretLister) field.ErrorList {
	var (
		allErrs           field.ErrorList
		referencedSecrets = sets.New[string]()
		path              = field.NewPath("spec", "kubernetes", "kubeAPIServer", "admissionPlugins")
	)

	if a.GetOperation() == admission.Delete {
		return nil
	}

	if c.shoot.Spec.Kubernetes.KubeAPIServer == nil {
		return nil
	}

	for _, referencedResource := range c.shoot.Spec.Resources {
		if referencedResource.ResourceRef.APIVersion == "v1" && referencedResource.ResourceRef.Kind == "Secret" {
			referencedSecrets.Insert(referencedResource.ResourceRef.Name)
		}
	}

	for i, admissionPlugin := range c.shoot.Spec.Kubernetes.KubeAPIServer.AdmissionPlugins {
		if admissionPlugin.KubeconfigSecretName != nil {
			if !referencedSecrets.Has(*admissionPlugin.KubeconfigSecretName) {
				allErrs = append(allErrs, field.Invalid(path.Index(i).Child("kubeconfigSecretName"), *admissionPlugin.KubeconfigSecretName, "secret should be referenced in shoot .spec.resources"))
				continue
			}
			if err := c.validateReferencedSecret(secretLister, *admissionPlugin.KubeconfigSecretName, c.shoot.Namespace, path.Index(i).Child("kubeconfigSecretName")); err != nil {
				allErrs = append(allErrs, err)
			}
		}
	}

	return allErrs
}

func (c *validationContext) validateReferencedSecret(secretLister kubecorev1listers.SecretLister, secretName, namespace string, fldPath *field.Path) *field.Error {
	var (
		secret *corev1.Secret
		err    error
	)

	if secret, err = secretLister.Secrets(namespace).Get(secretName); err != nil {
		return field.InternalError(fldPath, fmt.Errorf("unable to get the referenced secret: namespace: %s, name: %s, error: %w", namespace, secretName, err))
	}

	if _, ok := secret.Data[kubernetes.KubeConfig]; !ok {
		return field.Invalid(fldPath, secretName, fmt.Sprintf("referenced kubeconfig secret doesn't contain kubeconfig: namespace: %s, name: %s", namespace, secretName))
	}

	return nil
}

func (c *validationContext) validateShootNetworks(a admission.Attributes, workerless bool) field.ErrorList {
	var (
		allErrs field.ErrorList
		path    = field.NewPath("spec", "networking")
	)

	if a.GetOperation() == admission.Delete {
		return nil
	}

	if c.seed != nil {
		if c.shoot.Spec.Networking.Pods == nil && !workerless {
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

		// validate network disjointedness within shoot network
		allErrs = append(allErrs, cidrvalidation.ValidateShootNetworkDisjointedness(
			path,
			c.shoot.Spec.Networking.Nodes,
			c.shoot.Spec.Networking.Pods,
			c.shoot.Spec.Networking.Services,
			workerless,
		)...)

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
				workerless,
			)...)
		}
	}

	return allErrs
}

func (c *validationContext) validateKubernetes(a admission.Attributes) field.ErrorList {
	var (
		allErrs field.ErrorList
		path    = field.NewPath("spec", "kubernetes")
	)

	if a.GetOperation() == admission.Delete {
		return nil
	}

	defaultVersion, errList := defaultKubernetesVersion(c.cloudProfile.Spec.Kubernetes.Versions, c.shoot.Spec.Kubernetes.Version, path.Child("version"))
	if len(errList) > 0 {
		allErrs = append(allErrs, errList...)
	}

	if defaultVersion != nil {
		c.shoot.Spec.Kubernetes.Version = *defaultVersion
	} else {
		// We assume that the 'defaultVersion' is already calculated correctly, so only run validation if the verion was not defaulted.
		allErrs = append(allErrs, validateKubernetesVersionConstraints(c.cloudProfile.Spec.Kubernetes.Versions, c.shoot.Spec.Kubernetes.Version, c.oldShoot.Spec.Kubernetes.Version, path.Child("version"))...)
	}

	if c.shoot.DeletionTimestamp == nil {
		performKubernetesDefaulting(c.shoot, c.oldShoot)
	}

	return allErrs
}

func performKubernetesDefaulting(newShoot, oldShoot *core.Shoot) {
	if newShoot.Spec.Kubernetes.EnableStaticTokenKubeconfig == nil {
		// Error is ignored here because we cannot do anything meaningful with it - variable will default to "false".
		if k8sLessThan126, _ := versionutils.CheckVersionMeetsConstraint(newShoot.Spec.Kubernetes.Version, "< 1.26"); k8sLessThan126 {
			newShoot.Spec.Kubernetes.EnableStaticTokenKubeconfig = pointer.Bool(true)
		} else {
			newShoot.Spec.Kubernetes.EnableStaticTokenKubeconfig = pointer.Bool(false)
		}
	}

	if len(newShoot.Spec.Provider.Workers) > 0 {
		// Error is ignored here because we cannot do anything meaningful with them - variables will default to `false`.
		k8sLess125, _ := versionutils.CheckVersionMeetsConstraint(newShoot.Spec.Kubernetes.Version, "< 1.25")
		if newShoot.Spec.Kubernetes.AllowPrivilegedContainers == nil && k8sLess125 && !isPSPDisabled(newShoot) {
			newShoot.Spec.Kubernetes.AllowPrivilegedContainers = pointer.Bool(true)
		}

		k8sLess127, _ := versionutils.CheckVersionMeetsConstraint(newShoot.Spec.Kubernetes.Version, "< 1.27")
		if newShoot.Spec.Kubernetes.KubeControllerManager.NodeMonitorGracePeriod == nil {
			if k8sLess127 {
				newShoot.Spec.Kubernetes.KubeControllerManager.NodeMonitorGracePeriod = &metav1.Duration{Duration: 2 * time.Minute}
			} else {
				newShoot.Spec.Kubernetes.KubeControllerManager.NodeMonitorGracePeriod = &metav1.Duration{Duration: 40 * time.Second}
			}
		} else if upgradeToKubernetes127(newShoot, oldShoot) && defaultNodeGracePeriod(oldShoot) {
			newShoot.Spec.Kubernetes.KubeControllerManager.NodeMonitorGracePeriod = &metav1.Duration{Duration: 40 * time.Second}
		}
	}
}

func defaultNodeGracePeriod(shoot *core.Shoot) bool {
	return shoot.Spec.Kubernetes.KubeControllerManager != nil &&
		reflect.DeepEqual(shoot.Spec.Kubernetes.KubeControllerManager.NodeMonitorGracePeriod, &metav1.Duration{Duration: 2 * time.Minute})
}

func upgradeToKubernetes127(newShoot, oldShoot *core.Shoot) bool {
	var oldShootK8sLess127 bool
	if oldShoot.Spec.Kubernetes.Version != "" {
		oldShootK8sLess127, _ = versionutils.CheckVersionMeetsConstraint(oldShoot.Spec.Kubernetes.Version, "< 1.27")
	}
	newShootK8sGreaterEqual127, _ := versionutils.CheckVersionMeetsConstraint(newShoot.Spec.Kubernetes.Version, ">= 1.27")

	return oldShootK8sLess127 && newShootK8sGreaterEqual127
}

func (c *validationContext) validateProvider(a admission.Attributes) field.ErrorList {
	var (
		allErrs       field.ErrorList
		path          = field.NewPath("spec", "provider")
		kubeletConfig = c.shoot.Spec.Kubernetes.Kubelet
	)

	if a.GetOperation() == admission.Delete {
		return nil
	}

	if c.shoot.Spec.Provider.Type != c.cloudProfile.Spec.Type {
		allErrs = append(allErrs, field.Invalid(path.Child("type"), c.shoot.Spec.Provider.Type, fmt.Sprintf("provider type in shoot must equal provider type of referenced CloudProfile: %q", c.cloudProfile.Spec.Type)))
		// exit early, all other validation errors will be misleading
		return allErrs
	}

	if a.GetOperation() == admission.Create && c.secretBinding != nil {
		if !helper.SecretBindingHasType(c.secretBinding, c.shoot.Spec.Provider.Type) {
			var secretBindingProviderType string
			if c.secretBinding.Provider != nil {
				secretBindingProviderType = c.secretBinding.Provider.Type
			}

			allErrs = append(allErrs, field.Invalid(path.Child("type"), c.shoot.Spec.Provider.Type, fmt.Sprintf("provider type in shoot must match provider type of referenced SecretBinding: %q", secretBindingProviderType)))
			// exit early, all other validation errors will be misleading
			return allErrs
		}
	}

	controlPlaneVersion, err := semver.NewVersion(c.shoot.Spec.Kubernetes.Version)
	if err != nil {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "kubernetes", "version"), c.shoot.Spec.Kubernetes.Version, fmt.Sprintf("cannot parse the kubernetes version: %s", err.Error())))
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
		if worker.Machine.Architecture != nil && !slices.Contains(v1beta1constants.ValidArchitectures, *worker.Machine.Architecture) {
			allErrs = append(allErrs, field.NotSupported(idxPath.Child("machine", "architecture"), *worker.Machine.Architecture, v1beta1constants.ValidArchitectures))
		} else {
			var (
				isMachinePresentInCloudprofile, architectureSupported, availableInAllZones, isUsableMachine, supportedMachineTypes = validateMachineTypes(c.cloudProfile.Spec.MachineTypes, worker.Machine, oldWorker.Machine, c.cloudProfile.Spec.Regions, c.shoot.Spec.Region, worker.Zones)
				detail                                                                                                             = fmt.Sprintf("machine type %q ", worker.Machine.Type)
			)

			if !isMachinePresentInCloudprofile {
				allErrs = append(allErrs, field.NotSupported(idxPath.Child("machine", "type"), worker.Machine.Type, supportedMachineTypes))
			} else if !architectureSupported || !availableInAllZones || !isUsableMachine {
				if !isUsableMachine {
					detail += "is unusable, "
				}
				if !availableInAllZones {
					detail += "is unavailable in at least one zone, "
				}
				if !architectureSupported {
					detail += fmt.Sprintf("does not support CPU architecture %q, ", *worker.Machine.Architecture)
				}
				allErrs = append(allErrs, field.Invalid(idxPath.Child("machine", "type"), worker.Machine.Type, fmt.Sprintf("%ssupported types are %+v", detail, supportedMachineTypes)))
			}

			isMachineImagePresentInCloudprofile, architectureSupported, activeMachineImageVersion, validMachineImageversions := validateMachineImagesConstraints(c.cloudProfile.Spec.MachineImages, worker.Machine, oldWorker.Machine)
			if !isMachineImagePresentInCloudprofile {
				allErrs = append(allErrs, field.Invalid(idxPath.Child("machine", "image"), worker.Machine.Image, fmt.Sprintf("machine image version is not supported, supported machine image versions are: %+v", validMachineImageversions)))
			} else if !architectureSupported || !activeMachineImageVersion {
				detail := fmt.Sprintf("machine image version '%s:%s' ", worker.Machine.Image.Name, worker.Machine.Image.Version)
				if !architectureSupported {
					detail += fmt.Sprintf("does not support CPU architecture %q, ", *worker.Machine.Architecture)
				}
				if !activeMachineImageVersion {
					detail += "is expired, "
				}
				allErrs = append(allErrs, field.Invalid(idxPath.Child("machine", "image"), worker.Machine.Image, fmt.Sprintf("%ssupported machine image versions are: %+v", detail, validMachineImageversions)))
			} else {
				allErrs = append(allErrs, validateContainerRuntimeConstraints(c.cloudProfile.Spec.MachineImages, worker, oldWorker, idxPath.Child("cri"))...)

				kubeletVersion, err := helper.CalculateEffectiveKubernetesVersion(controlPlaneVersion, worker.Kubernetes)
				if err != nil {
					allErrs = append(allErrs, field.Invalid(idxPath.Child("kubernetes", "version"), worker.Kubernetes.Version, "cannot determine effective Kubernetes version for worker pool"))
					// exit early, all other validation errors will be misleading
					return allErrs
				}
				if err := validateKubeletVersionConstraint(c.cloudProfile.Spec.MachineImages, worker, kubeletVersion, idxPath); err != nil {
					allErrs = append(allErrs, err)
				}
			}
		}
		isVolumePresentInCloudprofile, availableInAllZones, isUsableVolume, supportedVolumeTypes := validateVolumeTypes(c.cloudProfile.Spec.VolumeTypes, worker.Volume, oldWorker.Volume, c.cloudProfile.Spec.Regions, c.shoot.Spec.Region, worker.Zones)
		if !isVolumePresentInCloudprofile {
			allErrs = append(allErrs, field.NotSupported(idxPath.Child("volume", "type"), pointer.StringDeref(worker.Volume.Type, ""), supportedVolumeTypes))
		} else if !availableInAllZones || !isUsableVolume {
			detail := fmt.Sprintf("volume type %q ", *worker.Volume.Type)
			if !isUsableVolume {
				detail += "is unusable, "
			}
			if !availableInAllZones {
				detail += "is unavailable in at least one zone, "
			}
			allErrs = append(allErrs, field.Invalid(idxPath.Child("volume", "type"), *worker.Volume.Type, fmt.Sprintf("%ssupported types are %+v", detail, supportedVolumeTypes)))
		}
		if ok, minSize := validateVolumeSize(c.cloudProfile.Spec.VolumeTypes, c.cloudProfile.Spec.MachineTypes, worker.Machine.Type, worker.Volume); !ok {
			allErrs = append(allErrs, field.Invalid(idxPath.Child("volume", "size"), worker.Volume.VolumeSize, fmt.Sprintf("size must be >= %s", minSize)))
		}
		if worker.Kubernetes != nil {
			if worker.Kubernetes.Kubelet != nil {
				kubeletConfig = worker.Kubernetes.Kubelet
			}
			allErrs = append(allErrs, validateKubeletConfig(idxPath.Child("kubernetes").Child("kubelet"), c.cloudProfile.Spec.MachineTypes, worker.Machine.Type, kubeletConfig)...)

			if worker.Kubernetes.Version != nil {
				oldWorkerKubernetesVersion := c.oldShoot.Spec.Kubernetes.Version
				if oldWorker.Kubernetes != nil && oldWorker.Kubernetes.Version != nil {
					oldWorkerKubernetesVersion = *oldWorker.Kubernetes.Version
				}

				defaultVersion, errList := defaultKubernetesVersion(c.cloudProfile.Spec.Kubernetes.Versions, *worker.Kubernetes.Version, idxPath.Child("kubernetes", "version"))
				if len(errList) > 0 {
					allErrs = append(allErrs, errList...)
				}

				if defaultVersion != nil {
					worker.Kubernetes.Version = defaultVersion
				} else {
					// We assume that the 'defaultVersion' is already calculated correctly, so only run validation if the verion was not defaulted.
					allErrs = append(allErrs, validateKubernetesVersionConstraints(c.cloudProfile.Spec.Kubernetes.Versions, *worker.Kubernetes.Version, oldWorkerKubernetesVersion, idxPath.Child("kubernetes", "version"))...)
				}
			}
		}

		allErrs = append(allErrs, validateZones(c.cloudProfile.Spec.Regions, c.shoot.Spec.Region, c.oldShoot.Spec.Region, worker, oldWorker, idxPath)...)
	}

	return allErrs
}

func isPSPDisabled(shoot *core.Shoot) bool {
	if shoot.Spec.Kubernetes.KubeAPIServer != nil {
		for _, plugin := range shoot.Spec.Kubernetes.KubeAPIServer.AdmissionPlugins {
			if plugin.Name == "PodSecurityPolicy" && pointer.BoolDeref(plugin.Disabled, false) {
				return true
			}
		}
	}
	return false
}

func (c *validationContext) validateAPIVersionForRawExtensions() field.ErrorList {
	var allErrs field.ErrorList

	if ok, gvk := usesInternalVersion(c.shoot.Spec.Provider.InfrastructureConfig); ok {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "provider", "infrastructureConfig"), gvk, internalVersionErrorMsg))
	}

	if ok, gvk := usesInternalVersion(c.shoot.Spec.Provider.ControlPlaneConfig); ok {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "provider", "controlPlaneConfig"), gvk, internalVersionErrorMsg))
	}

	if c.shoot.Spec.Networking != nil {
		if ok, gvk := usesInternalVersion(c.shoot.Spec.Networking.ProviderConfig); ok {
			allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "networking", "providerConfig"), gvk, internalVersionErrorMsg))
		}
	}

	for i, worker := range c.shoot.Spec.Provider.Workers {
		workerPath := field.NewPath("spec", "provider", "workers").Index(i)
		if ok, gvk := usesInternalVersion(worker.ProviderConfig); ok {
			allErrs = append(allErrs, field.Invalid(workerPath.Child("providerConfig"), gvk, internalVersionErrorMsg))
		}

		if ok, gvk := usesInternalVersion(worker.Machine.Image.ProviderConfig); ok {
			allErrs = append(allErrs, field.Invalid(workerPath.Child("machine", "image", "providerConfig"), gvk, internalVersionErrorMsg))
		}

		if worker.CRI != nil && worker.CRI.ContainerRuntimes != nil {
			for j, cr := range worker.CRI.ContainerRuntimes {
				if ok, gvk := usesInternalVersion(cr.ProviderConfig); ok {
					allErrs = append(allErrs, field.Invalid(workerPath.Child("cri", "containerRuntimes").Index(j).Child("providerConfig"), gvk, internalVersionErrorMsg))
				}
			}
		}
	}
	return allErrs
}

func usesInternalVersion(ext *runtime.RawExtension) (bool, string) {
	if ext == nil {
		return false, ""
	}

	// we ignore any errors while trying to parse the GVK from the RawExtension, because the RawExtension could contain arbitrary json.
	// However, *if* the RawExtension is a k8s-like object, we want to ensure that only external APIs can be used.
	_, gvk, _ := unstructured.UnstructuredJSONScheme.Decode(ext.Raw, nil, nil)
	if gvk != nil && gvk.Version == runtime.APIVersionInternal {
		return true, gvk.String()
	}
	return false, ""
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

func (c *validationContext) validateDNSDomainUniqueness(shootLister gardencorelisters.ShootLister) (field.ErrorList, error) {
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

func defaultKubernetesVersion(constraints []core.ExpirableVersion, shootVersion string, fldPath *field.Path) (*string, field.ErrorList) {
	var (
		allErrs           = field.ErrorList{}
		shootVersionMajor *int64
		shootVersionMinor *int64
		versionParts      = strings.Split(shootVersion, ".")
	)

	if len(versionParts) == 3 {
		return nil, allErrs
	}
	if len(versionParts) == 2 && len(versionParts[1]) > 0 {
		v, err := strconv.Atoi(versionParts[1])
		if err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath, versionParts[1], "must be a semantic version"))
			return nil, allErrs
		}
		shootVersionMinor = pointer.Int64(int64(v))
	}
	if len(versionParts) >= 1 && len(versionParts[0]) > 0 {
		v, err := strconv.Atoi(versionParts[0])
		if err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath, versionParts[0], "must be a semantic version"))
			return nil, allErrs
		}
		shootVersionMajor = pointer.Int64(int64(v))
	}

	if latestVersion := findLatestVersion(constraints, shootVersionMajor, shootVersionMinor); latestVersion != nil {
		return pointer.String(latestVersion.String()), nil
	}

	allErrs = append(allErrs, field.Invalid(fldPath, shootVersion, fmt.Sprintf("couldn't find a suitable version for %s. Suitable versions have a non-expired expiration date and are no 'preview' versions. 'Preview'-classified versions have to be selected explicitly", shootVersion)))
	return nil, allErrs
}

func findLatestVersion(constraints []core.ExpirableVersion, major, minor *int64) *semver.Version {
	var latestVersion *semver.Version
	for _, versionConstraint := range constraints {
		// ignore expired versions
		if versionConstraint.ExpirationDate != nil && versionConstraint.ExpirationDate.Time.UTC().Before(time.Now().UTC()) {
			continue
		}

		// filter preview versions for defaulting
		if versionConstraint.Classification != nil && *versionConstraint.Classification == core.ClassificationPreview {
			continue
		}

		// CloudProfile cannot contain invalid semVer shootVersion
		cpVersion := semver.MustParse(versionConstraint.Version)

		// defaulting on patch level: version has to have the same major and minor kubernetes version
		if major != nil && cpVersion.Major() != *major {
			continue
		}

		if minor != nil && cpVersion.Minor() != *minor {
			continue
		}

		if latestVersion == nil || cpVersion.GreaterThan(latestVersion) {
			latestVersion = cpVersion
		}
	}

	return latestVersion
}

func validateKubernetesVersionConstraints(constraints []core.ExpirableVersion, shootVersion, oldShootVersion string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if shootVersion == oldShootVersion {
		return allErrs
	}

	var validValues []string
	for _, versionConstraint := range constraints {
		if versionConstraint.ExpirationDate != nil && versionConstraint.ExpirationDate.Time.UTC().Before(time.Now().UTC()) {
			continue
		}

		if versionConstraint.Version == shootVersion {
			return allErrs
		}

		versionStr := versionConstraint.Version
		if versionConstraint.Classification != nil && *versionConstraint.Classification == core.ClassificationPreview {
			versionStr += " (preview)"
		}
		validValues = append(validValues, versionStr)
	}

	allErrs = append(allErrs, field.NotSupported(fldPath, shootVersion, validValues))

	return allErrs
}

func validateMachineTypes(constraints []core.MachineType, machine, oldMachine core.Machine, regions []core.Region, region string, zones []string) (bool, bool, bool, bool, []string) {
	if machine.Type == oldMachine.Type && pointer.StringEqual(machine.Architecture, oldMachine.Architecture) {
		return true, true, true, true, nil
	}

	var (
		isMachinePresentInCloudprofile    = false
		machinesWithSupportedArchitecture = sets.New[string]()
		machinesAvailableInAllZones       = sets.New[string]()
		usableMachines                    = sets.New[string]()
	)

	for _, t := range constraints {
		if pointer.StringEqual(t.Architecture, machine.Architecture) {
			machinesWithSupportedArchitecture.Insert(t.Name)
		}
		if pointer.BoolDeref(t.Usable, false) {
			usableMachines.Insert(t.Name)
		}
		if !isUnavailableInAtleastOneZone(regions, region, zones, t.Name, func(zone core.AvailabilityZone) []string { return zone.UnavailableMachineTypes }) {
			machinesAvailableInAllZones.Insert(t.Name)
		}
		if t.Name == machine.Type {
			isMachinePresentInCloudprofile = true
		}
	}

	return isMachinePresentInCloudprofile,
		machinesWithSupportedArchitecture.Has(machine.Type),
		machinesAvailableInAllZones.Has(machine.Type),
		usableMachines.Has(machine.Type),
		sets.List(machinesWithSupportedArchitecture.Intersection(machinesAvailableInAllZones).Intersection(usableMachines))
}

func isUnavailableInAtleastOneZone(regions []core.Region, region string, zones []string, t string, unavailableTypes func(zone core.AvailabilityZone) []string) bool {
	for _, r := range regions {
		if r.Name != region {
			continue
		}

		for _, zoneName := range zones {
			for _, z := range r.Zones {
				if z.Name != zoneName {
					continue
				}

				for _, unavailableType := range unavailableTypes(z) {
					if t == unavailableType {
						return true
					}
				}
			}
		}
	}
	return false
}

func validateKubeletConfig(fldPath *field.Path, machineTypes []core.MachineType, workerMachineType string, kubeletConfig *core.KubeletConfig) field.ErrorList {
	var allErrs field.ErrorList

	if kubeletConfig == nil {
		return allErrs
	}

	reservedCPU := *resource.NewQuantity(0, resource.DecimalSI)
	reservedMemory := *resource.NewQuantity(0, resource.DecimalSI)

	if kubeletConfig.KubeReserved != nil {
		if kubeletConfig.KubeReserved.CPU != nil {
			reservedCPU.Add(*kubeletConfig.KubeReserved.CPU)
		}
		if kubeletConfig.KubeReserved.Memory != nil {
			reservedMemory.Add(*kubeletConfig.KubeReserved.Memory)
		}
	}

	if kubeletConfig.SystemReserved != nil {
		if kubeletConfig.SystemReserved.CPU != nil {
			reservedCPU.Add(*kubeletConfig.SystemReserved.CPU)
		}
		if kubeletConfig.SystemReserved.Memory != nil {
			reservedMemory.Add(*kubeletConfig.SystemReserved.Memory)
		}
	}

	for _, machineType := range machineTypes {
		if machineType.Name == workerMachineType {
			capacityCPU := machineType.CPU
			capacityMemory := machineType.Memory

			if cmp := reservedCPU.Cmp(capacityCPU); cmp >= 0 {
				allErrs = append(allErrs, field.Invalid(fldPath, fmt.Sprintf("kubeReserved CPU + systemReserved CPU: %s", reservedCPU.String()), fmt.Sprintf("total reserved CPU (kubeReserved + systemReserved) cannot be more than the Node's CPU capacity '%s'", capacityCPU.String())))
			}

			if cmp := reservedMemory.Cmp(capacityMemory); cmp >= 0 {
				allErrs = append(allErrs, field.Invalid(fldPath, fmt.Sprintf("kubeReserved memory + systemReserved memory: %s", reservedMemory.String()), fmt.Sprintf("total reserved memory (kubeReserved + systemReserved) cannot be more than the Node's memory capacity '%s'", capacityMemory.String())))
			}
		}
	}

	return allErrs
}

func validateVolumeTypes(constraints []core.VolumeType, volume, oldVolume *core.Volume, regions []core.Region, region string, zones []string) (bool, bool, bool, []string) {
	if volume == nil || volume.Type == nil || (volume != nil && oldVolume != nil && volume.Type != nil && oldVolume.Type != nil && *volume.Type == *oldVolume.Type) {
		return true, true, true, nil
	}

	var volumeType string
	if volume != nil && volume.Type != nil {
		volumeType = *volume.Type
	}

	var (
		isVolumePresentInCloudprofile = false
		volumesAvailableInAllZones    = sets.New[string]()
		usableVolumes                 = sets.New[string]()
	)

	for _, v := range constraints {
		if pointer.BoolDeref(v.Usable, false) {
			usableVolumes.Insert(v.Name)
		}
		if !isUnavailableInAtleastOneZone(regions, region, zones, v.Name, func(zone core.AvailabilityZone) []string { return zone.UnavailableVolumeTypes }) {
			volumesAvailableInAllZones.Insert(v.Name)
		}
		if v.Name == volumeType {
			isVolumePresentInCloudprofile = true
		}
	}

	return isVolumePresentInCloudprofile,
		volumesAvailableInAllZones.Has(volumeType),
		usableVolumes.Has(volumeType),
		sets.List(usableVolumes.Intersection(volumesAvailableInAllZones))
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

	usedZones := sets.New[string]()
	for j, zone := range worker.Zones {
		jdxPath := fldPath.Child("zones").Index(j)
		if ok, validZones := validateZone(constraints, region, zone); !ok {
			if len(validZones) == 0 {
				allErrs = append(allErrs, field.Invalid(jdxPath, region, "this region does not support availability zones, please do not configure them"))
			} else {
				allErrs = append(allErrs, field.NotSupported(jdxPath, zone, validZones))
			}
		}
		if usedZones.Has(zone) {
			allErrs = append(allErrs, field.Duplicate(jdxPath, zone))
		}
		usedZones.Insert(zone)
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
func getDefaultMachineImage(machineImages []core.MachineImage, imageName string, arch *string, fldPath *field.Path) (*core.ShootMachineImage, *field.Error) {
	if len(machineImages) == 0 {
		return nil, field.Invalid(fldPath, imageName, "the cloud profile does not contain any machine image - cannot create shoot cluster")
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
			return nil, field.Invalid(fldPath, imageName, "image is not supported")
		}
	} else {
		// select the first image which support the required architecture type
		for _, machineImage := range machineImages {
			for _, version := range machineImage.Versions {
				if slices.Contains(version.Architectures, *arch) {
					defaultImage = &machineImage
					break
				}
			}
			if defaultImage != nil {
				break
			}
		}
		if defaultImage == nil {
			return nil, field.Invalid(fldPath, imageName, fmt.Sprintf("no valid machine image found that support architecture `%s`", *arch))
		}
	}

	var validVersions []core.MachineImageVersion

	for _, version := range defaultImage.Versions {
		if slices.Contains(version.Architectures, *arch) {
			validVersions = append(validVersions, version)
		}
	}

	latestMachineImageVersion, err := helper.DetermineLatestMachineImageVersion(validVersions, true)
	if err != nil {
		return nil, field.Invalid(fldPath, imageName, fmt.Sprintf("failed to determine latest machine image from cloud profile: %s", err.Error()))
	}
	return &core.ShootMachineImage{Name: defaultImage.Name, Version: latestMachineImageVersion.Version}, nil
}

func validateMachineImagesConstraints(constraints []core.MachineImage, machine, oldMachine core.Machine) (bool, bool, bool, []string) {
	if apiequality.Semantic.DeepEqual(machine.Image, oldMachine.Image) && pointer.StringEqual(machine.Architecture, oldMachine.Architecture) {
		return true, true, true, nil
	}

	var (
		machineImageVersionsInCloudProfile            = sets.New[string]()
		activeMachineImageVersions                    = sets.New[string]()
		machineImageVersionsWithSupportedArchitecture = sets.New[string]()
	)

	for _, machineImage := range constraints {
		for _, machineVersion := range machineImage.Versions {
			machineImageVersion := fmt.Sprintf("%s:%s", machineImage.Name, machineVersion.Version)

			if machineVersion.ExpirationDate == nil || machineVersion.ExpirationDate.Time.UTC().After(time.Now().UTC()) {
				activeMachineImageVersions.Insert(machineImageVersion)
			}
			if slices.Contains(machineVersion.Architectures, *machine.Architecture) {
				machineImageVersionsWithSupportedArchitecture.Insert(machineImageVersion)
			}
			machineImageVersionsInCloudProfile.Insert(machineImageVersion)
		}
	}

	supportedMachineImageVersions := sets.List(activeMachineImageVersions.Intersection(machineImageVersionsWithSupportedArchitecture))
	if machine.Image == nil || len(machine.Image.Version) == 0 {
		return false, false, false, supportedMachineImageVersions
	}

	shootMachineImageVersion := fmt.Sprintf("%s:%s", machine.Image.Name, machine.Image.Version)
	return machineImageVersionsInCloudProfile.Has(shootMachineImageVersion),
		machineImageVersionsWithSupportedArchitecture.Has(shootMachineImageVersion),
		activeMachineImageVersions.Has(shootMachineImageVersion),
		supportedMachineImageVersions
}

func validateContainerRuntimeConstraints(constraints []core.MachineImage, worker, oldWorker core.Worker, fldPath *field.Path) field.ErrorList {
	if worker.CRI == nil || worker.Machine.Image == nil {
		return nil
	}

	if apiequality.Semantic.DeepEqual(worker.CRI, oldWorker.CRI) &&
		apiequality.Semantic.DeepEqual(worker.Machine.Image, oldWorker.Machine.Image) {
		return nil
	}

	machineImageVersion, ok := helper.FindMachineImageVersion(constraints, worker.Machine.Image.Name, worker.Machine.Image.Version)
	if !ok {
		return nil
	}

	return validateCRI(machineImageVersion.CRI, worker, fldPath)
}

func validateCRI(constraints []core.CRI, worker core.Worker, fldPath *field.Path) field.ErrorList {
	if worker.CRI == nil {
		return nil
	}

	var (
		allErrors = field.ErrorList{}
		validCRIs = []string{}
		foundCRI  *core.CRI
	)

	for _, criConstraint := range constraints {
		validCRIs = append(validCRIs, string(criConstraint.Name))
		if worker.CRI.Name == criConstraint.Name {
			foundCRI = &criConstraint
			break
		}
	}
	if foundCRI == nil {
		detail := fmt.Sprintf("machine image '%s@%s' does not support CRI '%s', supported values: %+v", worker.Machine.Image.Name, worker.Machine.Image.Version, worker.CRI.Name, validCRIs)
		allErrors = append(allErrors, field.Invalid(fldPath.Child("name"), worker.CRI.Name, detail))
		return allErrors
	}

	for j, cr := range worker.CRI.ContainerRuntimes {
		jdxPath := fldPath.Child("containerRuntimes").Index(j)
		if ok, validValues := validateCRMembership(foundCRI.ContainerRuntimes, cr.Type); !ok {
			detail := fmt.Sprintf("machine image '%s@%s' does not support container runtime '%s', supported values: %+v", worker.Machine.Image.Name, worker.Machine.Image.Version, cr.Type, validValues)
			allErrors = append(allErrors, field.Invalid(jdxPath.Child("type"), cr.Type, detail))
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

func validateKubeletVersionConstraint(constraints []core.MachineImage, worker core.Worker, kubeletVersion *semver.Version, fldPath *field.Path) *field.Error {
	if worker.Machine.Image == nil {
		return nil
	}

	machineImageVersion, ok := helper.FindMachineImageVersion(constraints, worker.Machine.Image.Name, worker.Machine.Image.Version)
	if !ok {
		return nil
	}

	if machineImageVersion.KubeletVersionConstraint != nil {
		// CloudProfile cannot contain an invalid kubeletVersionConstraint
		constraint, _ := semver.NewConstraint(*machineImageVersion.KubeletVersionConstraint)
		if !constraint.Check(kubeletVersion) {
			detail := fmt.Sprintf("machine image '%s@%s' does not support kubelet version '%s', supported kubelet versions by this machine image version: '%+v'", worker.Machine.Image.Name, worker.Machine.Image.Version, kubeletVersion, *machineImageVersion.KubeletVersionConstraint)
			return field.Invalid(fldPath.Child("machine", "image"), worker.Machine.Image, detail)
		}
	}

	return nil
}

func ensureMachineImage(oldWorkers []core.Worker, worker core.Worker, images []core.MachineImage, fldPath *field.Path) (*core.ShootMachineImage, *field.Error) {
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

	return getDefaultMachineImage(images, imageName, worker.Machine.Architecture, fldPath)
}

func addInfrastructureDeploymentTask(shoot *core.Shoot) {
	addDeploymentTasks(shoot, v1beta1constants.ShootTaskDeployInfrastructure)
}

func addDNSRecordDeploymentTasks(shoot *core.Shoot) {
	addDeploymentTasks(shoot,
		v1beta1constants.ShootTaskDeployDNSRecordInternal,
		v1beta1constants.ShootTaskDeployDNSRecordExternal,
		v1beta1constants.ShootTaskDeployDNSRecordIngress,
	)
}

func addDeploymentTasks(shoot *core.Shoot, tasks ...string) {
	if shoot.ObjectMeta.Annotations == nil {
		shoot.ObjectMeta.Annotations = make(map[string]string)
	}
	controllerutils.AddTasks(shoot.ObjectMeta.Annotations, tasks...)
}

// wasShootRescheduledToNewSeed returns true if the shoot.Spec.SeedName has been changed, but the migration operation has not started yet.
func wasShootRescheduledToNewSeed(shoot *core.Shoot) bool {
	return shoot.Status.LastOperation != nil &&
		shoot.Status.LastOperation.Type != core.LastOperationTypeMigrate &&
		shoot.Spec.SeedName != nil &&
		shoot.Status.SeedName != nil &&
		*shoot.Spec.SeedName != *shoot.Status.SeedName
}

// isShootInMigrationOrRestorePhase returns true if the shoot is currently being migrated or restored.
func isShootInMigrationOrRestorePhase(shoot *core.Shoot) bool {
	return shoot.Status.LastOperation != nil &&
		(shoot.Status.LastOperation.Type == core.LastOperationTypeRestore &&
			shoot.Status.LastOperation.State != core.LastOperationStateSucceeded ||
			shoot.Status.LastOperation.Type == core.LastOperationTypeMigrate)
}
