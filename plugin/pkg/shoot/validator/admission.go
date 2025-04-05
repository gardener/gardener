// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validator

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
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
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apis/core/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	admissioninitializer "github.com/gardener/gardener/pkg/apiserver/admission/initializer"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	gardencorev1beta1listers "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	securityinformers "github.com/gardener/gardener/pkg/client/security/informers/externalversions"
	securityv1alpha1listers "github.com/gardener/gardener/pkg/client/security/listers/security/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
	cidrvalidation "github.com/gardener/gardener/pkg/utils/validation/cidr"
	versionutils "github.com/gardener/gardener/pkg/utils/version"
	plugin "github.com/gardener/gardener/plugin/pkg"
	admissionutils "github.com/gardener/gardener/plugin/pkg/utils"
)

const internalVersionErrorMsg = "must not use apiVersion 'internal'"

// Register registers a plugin.
func Register(plugins *admission.Plugins) {
	plugins.Register(plugin.PluginNameShootValidator, func(_ io.Reader) (admission.Interface, error) {
		return New()
	})
}

// ValidateShoot contains listers and admission handler.
type ValidateShoot struct {
	*admission.Handler
	authorizer                   authorizer.Authorizer
	secretLister                 kubecorev1listers.SecretLister
	cloudProfileLister           gardencorev1beta1listers.CloudProfileLister
	namespacedCloudProfileLister gardencorev1beta1listers.NamespacedCloudProfileLister
	seedLister                   gardencorev1beta1listers.SeedLister
	shootLister                  gardencorev1beta1listers.ShootLister
	projectLister                gardencorev1beta1listers.ProjectLister
	secretBindingLister          gardencorev1beta1listers.SecretBindingLister
	credentialsBindingLister     securityv1alpha1listers.CredentialsBindingLister
	readyFunc                    admission.ReadyFunc
}

var (
	_ = admissioninitializer.WantsCoreInformerFactory(&ValidateShoot{})
	_ = admissioninitializer.WantsKubeInformerFactory(&ValidateShoot{})
	_ = admissioninitializer.WantsAuthorizer(&ValidateShoot{})
	_ = admissioninitializer.WantsSecurityInformerFactory(&ValidateShoot{})

	readyFuncs []admission.ReadyFunc
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

// SetCoreInformerFactory gets Lister from SharedInformerFactory.
func (v *ValidateShoot) SetCoreInformerFactory(f gardencoreinformers.SharedInformerFactory) {
	seedInformer := f.Core().V1beta1().Seeds()
	v.seedLister = seedInformer.Lister()

	shootInformer := f.Core().V1beta1().Shoots()
	v.shootLister = shootInformer.Lister()

	cloudProfileInformer := f.Core().V1beta1().CloudProfiles()
	v.cloudProfileLister = cloudProfileInformer.Lister()

	namespacedCloudProfileInformer := f.Core().V1beta1().NamespacedCloudProfiles()
	v.namespacedCloudProfileLister = namespacedCloudProfileInformer.Lister()

	projectInformer := f.Core().V1beta1().Projects()
	v.projectLister = projectInformer.Lister()

	secretBindingInformer := f.Core().V1beta1().SecretBindings()
	v.secretBindingLister = secretBindingInformer.Lister()

	readyFuncs = append(
		readyFuncs,
		seedInformer.Informer().HasSynced,
		shootInformer.Informer().HasSynced,
		cloudProfileInformer.Informer().HasSynced,
		namespacedCloudProfileInformer.Informer().HasSynced,
		projectInformer.Informer().HasSynced,
		secretBindingInformer.Informer().HasSynced,
	)
}

// SetSecurityInformerFactory gets Lister from SharedInformerFactory.
func (v *ValidateShoot) SetSecurityInformerFactory(f securityinformers.SharedInformerFactory) {
	credentialsBindingInformer := f.Security().V1alpha1().CredentialsBindings()
	v.credentialsBindingLister = credentialsBindingInformer.Lister()

	readyFuncs = append(readyFuncs, credentialsBindingInformer.Informer().HasSynced)
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
	if v.namespacedCloudProfileLister == nil {
		return errors.New("missing namespacedCloudProfile lister")
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
	if v.credentialsBindingLister == nil {
		return errors.New("missing credentials binding lister")
	}
	return nil
}

var _ admission.MutationInterface = &ValidateShoot{}

// Admit validates the Shoot details against the referenced CloudProfile.
func (v *ValidateShoot) Admit(ctx context.Context, a admission.Attributes, _ admission.ObjectInterfaces) error {
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
	if a.GetOperation() == admission.Update || a.GetOperation() == admission.Delete {
		var ok bool
		oldShoot, ok = a.GetOldObject().(*core.Shoot)
		if !ok {
			return apierrors.NewInternalError(errors.New("could not convert old resource into Shoot object"))
		}
	}

	if a.GetOperation() == admission.Update {
		if a.GetSubresource() == "binding" && reflect.DeepEqual(oldShoot.Spec.SeedName, shoot.Spec.SeedName) {
			return fmt.Errorf("update of binding rejected, shoot is already assigned to the same seed")
		}

		// do not ignore metadata updates to detect and prevent removal of the gardener finalizer or unwanted changes to annotations
		if reflect.DeepEqual(shoot.Spec, oldShoot.Spec) && reflect.DeepEqual(shoot.ObjectMeta, oldShoot.ObjectMeta) {
			return nil
		}
	}

	cloudProfileSpec, err := admissionutils.GetCloudProfileSpec(v.cloudProfileLister, v.namespacedCloudProfileLister, shoot)
	if err != nil {
		return apierrors.NewInternalError(fmt.Errorf("could not find referenced cloud profile: %+v", err.Error()))
	}

	if a.GetOperation() == admission.Create && len(ptr.Deref(shoot.Spec.CloudProfileName, "")) > 0 && shoot.Spec.CloudProfile != nil {
		return fmt.Errorf("new shoot can only specify either cloudProfileName or cloudProfile reference")
	}

	if err := admissionutils.ValidateCloudProfileChanges(v.cloudProfileLister, v.namespacedCloudProfileLister, shoot, oldShoot); err != nil {
		return err
	}

	var seed *gardencorev1beta1.Seed
	if shoot.Spec.SeedName != nil {
		seed, err = v.seedLister.Get(*shoot.Spec.SeedName)
		if err != nil {
			return apierrors.NewInternalError(fmt.Errorf("could not find referenced seed %q: %+v", *shoot.Spec.SeedName, err.Error()))
		}
	}

	project, err := admissionutils.ProjectForNamespaceFromLister(v.projectLister, shoot.Namespace)
	if err != nil {
		return apierrors.NewInternalError(fmt.Errorf("could not find referenced project: %+v", err.Error()))
	}

	var secretBinding *gardencorev1beta1.SecretBinding
	if shoot.Spec.SecretBindingName != nil {
		secretBinding, err = v.secretBindingLister.SecretBindings(shoot.Namespace).Get(*shoot.Spec.SecretBindingName)
		if err != nil {
			return apierrors.NewInternalError(fmt.Errorf("could not find referenced secret binding: %+v", err.Error()))
		}
	}

	var credentialsBinding *securityv1alpha1.CredentialsBinding
	if shoot.Spec.CredentialsBindingName != nil {
		credentialsBinding, err = v.credentialsBindingLister.CredentialsBindings(shoot.Namespace).Get(*shoot.Spec.CredentialsBindingName)
		if err != nil {
			return apierrors.NewInternalError(fmt.Errorf("could not find referenced credentials binding: %+v", err.Error()))
		}
	}

	// begin of validation code
	validationContext := &validationContext{
		cloudProfileSpec:   cloudProfileSpec,
		project:            project,
		seed:               seed,
		secretBinding:      secretBinding,
		credentialsBinding: credentialsBinding,
		shoot:              shoot,
		oldShoot:           oldShoot,
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
	if err := validationContext.validateManagedServiceAccountIssuer(a, v.secretLister); err != nil {
		return err
	}
	if err := validationContext.validateSecretBindingToCredentialsBindingMigration(a, v.secretBindingLister, v.credentialsBindingLister); err != nil {
		return err
	}
	if err := validationContext.validateCredentialsBindingChange(ctx, a, v.authorizer, v.credentialsBindingLister); err != nil {
		return err
	}
	if allErrs = validationContext.ensureMachineImages(); len(allErrs) > 0 {
		return admission.NewForbidden(a, allErrs.ToAggregate())
	}

	validationContext.addMetadataAnnotations(a)

	allErrs = append(allErrs, validationContext.validateAPIVersionForRawExtensions()...)
	allErrs = append(allErrs, validationContext.validateShootNetworks(a, helper.IsWorkerless(shoot))...)
	allErrs = append(allErrs, validationContext.validateKubernetes(a)...)
	allErrs = append(allErrs, validationContext.validateRegion()...)
	allErrs = append(allErrs, validationContext.validateAccessRestrictions()...)
	allErrs = append(allErrs, validationContext.validateProvider(a)...)
	allErrs = append(allErrs, validationContext.validateAdmissionPlugins(a, v.secretLister)...)
	allErrs = append(allErrs, validationContext.validateLimits(a)...)

	// Skip the validation if the operation is admission.Delete or the spec hasn't changed.
	if a.GetOperation() != admission.Delete && !reflect.DeepEqual(validationContext.shoot.Spec, validationContext.oldShoot.Spec) {
		dnsErrors, err := validationContext.validateDNSDomainUniqueness(v.shootLister)
		if err != nil {
			return apierrors.NewInternalError(err)
		}
		allErrs = append(allErrs, dnsErrors...)
	}

	if len(allErrs) > 0 {
		return admission.NewForbidden(a, allErrs.ToAggregate())
	}

	return nil
}

type validationContext struct {
	cloudProfileSpec   *gardencorev1beta1.CloudProfileSpec
	project            *gardencorev1beta1.Project
	seed               *gardencorev1beta1.Seed
	secretBinding      *gardencorev1beta1.SecretBinding
	credentialsBinding *securityv1alpha1.CredentialsBinding
	shoot              *core.Shoot
	oldShoot           *core.Shoot
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

func (c *validationContext) validateScheduling(ctx context.Context, a admission.Attributes, authorizer authorizer.Authorizer, shootLister gardencorev1beta1listers.ShootLister, seedLister gardencorev1beta1listers.SeedLister) error {
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
		if c.oldShoot.Spec.SeedName != nil && c.shoot.Spec.SeedName == nil {
			return admission.NewForbidden(a, fmt.Errorf("spec.seedName is already set to '%s' and cannot be changed to 'nil'", *c.oldShoot.Spec.SeedName))
		} else if a.GetSubresource() == "binding" {
			if shootIsBeingRescheduled {
				newShootSpec := c.shoot.Spec
				newShootSpec.SeedName = c.oldShoot.Spec.SeedName
				if !reflect.DeepEqual(newShootSpec, c.oldShoot.Spec) {
					return admission.NewForbidden(a, fmt.Errorf("only spec.seedName can be changed using the binding subresource when the shoot is being rescheduled to a new seed"))
				}
			}
		} else if !reflect.DeepEqual(c.shoot.Spec.SeedName, c.oldShoot.Spec.SeedName) {
			oldShootNameStr := ptr.Deref(c.oldShoot.Spec.SeedName, "nil")
			return admission.NewForbidden(a, fmt.Errorf("spec.seedName '%s' cannot be changed to '%s' by patching the shoot, please use the shoots/binding subresource", oldShootNameStr, *c.shoot.Spec.SeedName))
		}
	case admission.Delete:
		return nil
	}

	if mustCheckSchedulingConstraints {
		if c.seed.DeletionTimestamp != nil {
			return admission.NewForbidden(a, fmt.Errorf("cannot schedule shoot '%s' on seed '%s' that is already marked for deletion", c.shoot.Name, c.seed.Name))
		}

		var seedTaints []core.SeedTaint
		if c.seed.Spec.Taints != nil {
			for _, taint := range c.seed.Spec.Taints {
				seedTaints = append(seedTaints, core.SeedTaint{
					Key:   taint.Key,
					Value: taint.Value,
				})
			}
		}

		if !helper.TaintsAreTolerated(seedTaints, c.shoot.Spec.Tolerations) {
			return admission.NewForbidden(a, fmt.Errorf("forbidden to use a seed whose taints are not tolerated by the shoot"))
		}

		var seedAccessRestrictions []core.AccessRestriction
		if c.seed.Spec.AccessRestrictions != nil {
			for _, accessRestriction := range c.seed.Spec.AccessRestrictions {
				seedAccessRestrictions = append(seedAccessRestrictions, core.AccessRestriction{
					Name: accessRestriction.Name,
				})
			}
		}

		if !helper.AccessRestrictionsAreSupported(seedAccessRestrictions, c.shoot.Spec.AccessRestrictions) {
			return admission.NewForbidden(a, fmt.Errorf("forbidden to use a seed which doesn't support the access restrictions of the shoot"))
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

		if seedSelector := c.cloudProfileSpec.SeedSelector; seedSelector != nil {
			selector, err := metav1.LabelSelectorAsSelector(&seedSelector.LabelSelector)
			if err != nil {
				return apierrors.NewInternalError(fmt.Errorf("label selector conversion failed: %v for seedSelector: %w", seedSelector.LabelSelector, err))
			}
			if !selector.Matches(labels.Set(c.seed.Labels)) {
				return admission.NewForbidden(a, fmt.Errorf("cannot schedule shoot '%s' on seed '%s' because the cloud profile seed selector is not matching the labels of the seed", c.shoot.Name, c.seed.Name))
			}

			if seedSelector.ProviderTypes != nil {
				if !sets.New(seedSelector.ProviderTypes...).HasAny(c.seed.Spec.Provider.Type, "*") {
					return admission.NewForbidden(a, fmt.Errorf("cannot schedule shoot '%s' on seed '%s' because none of the provider types in the cloud profile seed selector is matching the provider type of the seed", c.shoot.Name, c.seed.Name))
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
				newConfirmation, newHasConfirmation := newMeta.Annotations[v1beta1constants.ConfirmationDeletion]

				// copy the new confirmation value to the old annotations to see if
				// anything else was changed other than the confirmation annotation
				if newHasConfirmation {
					if oldMeta.Annotations == nil {
						oldMeta.Annotations = make(map[string]string)
					}
					oldMeta.Annotations[v1beta1constants.ConfirmationDeletion] = newConfirmation
				}

				if !apiequality.Semantic.DeepEqual(newMeta.Annotations, oldMeta.Annotations) {
					return admission.NewForbidden(a, fmt.Errorf("cannot update annotations of shoot '%s' on seed '%s' already marked for deletion: only the '%s' annotation can be changed", c.shoot.Name, c.seed.Name, v1beta1constants.ConfirmationDeletion))
				}
			}

			if !apiequality.Semantic.DeepEqual(c.shoot.Spec, c.oldShoot.Spec) {
				return admission.NewForbidden(a, fmt.Errorf("cannot update spec of shoot '%s' on seed '%s' already marked for deletion", c.shoot.Name, c.seed.Name))
			}
		}
	}

	return nil
}

func getNumberOfShootsOnSeed(shootLister gardencorev1beta1listers.ShootLister, seedName string) (int64, error) {
	allShoots, err := shootLister.Shoots(metav1.NamespaceAll).List(labels.Everything())
	if err != nil {
		return 0, fmt.Errorf("could not list all shoots: %w", err)
	}

	seedUsage := v1beta1helper.CalculateSeedUsage(allShoots)
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
		return apierrors.NewInternalError(fmt.Errorf("could not authorize update request for shoot binding subresource: %+v", err.Error()))
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

func (c *validationContext) validateSecretBindingToCredentialsBindingMigration(
	a admission.Attributes,
	secretBindingLister gardencorev1beta1listers.SecretBindingLister,
	credentialsBindingLister securityv1alpha1listers.CredentialsBindingLister,
) error {
	secretBindingNameProgressedToEmpty := c.oldShoot.Spec.SecretBindingName != nil && c.shoot.Spec.SecretBindingName == nil
	credentialsBindingNameProgressedToSet := c.oldShoot.Spec.CredentialsBindingName == nil && c.shoot.Spec.CredentialsBindingName != nil

	if secretBindingNameProgressedToEmpty && credentialsBindingNameProgressedToSet {
		secretBinding, err := secretBindingLister.SecretBindings(c.oldShoot.Namespace).Get(*c.oldShoot.Spec.SecretBindingName)
		if err != nil {
			return apierrors.NewInternalError(fmt.Errorf("could not retrieve previously referenced secret binding: %+v", err.Error()))
		}
		credentialsBinding, err := credentialsBindingLister.CredentialsBindings(c.shoot.Namespace).Get(*c.shoot.Spec.CredentialsBindingName)
		if err != nil {
			return apierrors.NewInternalError(fmt.Errorf("could not retrieve newly referenced credentials binding: %+v", err.Error()))
		}

		// during migration the newly referenced credential should be
		// the exact same one that was referenced by the secret binding
		if credentialsBinding.CredentialsRef.Kind != "Secret" ||
			credentialsBinding.CredentialsRef.APIVersion != corev1.SchemeGroupVersion.String() ||
			credentialsBinding.CredentialsRef.Name != secretBinding.SecretRef.Name ||
			credentialsBinding.CredentialsRef.Namespace != secretBinding.SecretRef.Namespace {
			return admission.NewForbidden(a, errors.New("it is not allowed to change the referenced Secret when migrating from SecretBindingName to CredentialsBindingName"))
		}
	}

	return nil
}

func (c *validationContext) validateCredentialsBindingChange(
	ctx context.Context,
	a admission.Attributes,
	auth authorizer.Authorizer,
	credentialsBindingLister securityv1alpha1listers.CredentialsBindingLister,
) error {
	getAttributesRecord := func(credentialsBinding *securityv1alpha1.CredentialsBinding) (authorizer.AttributesRecord, error) {
		var (
			credentialsAPIGroup   string
			credentialsAPIVersion string
			credentialsResource   string
		)
		if credentialsBinding.CredentialsRef.APIVersion == corev1.SchemeGroupVersion.String() {
			credentialsAPIGroup = corev1.SchemeGroupVersion.Group
			credentialsAPIVersion = corev1.SchemeGroupVersion.Version
			credentialsResource = "secrets"
		} else if credentialsBinding.CredentialsRef.APIVersion == securityv1alpha1.SchemeGroupVersion.String() {
			credentialsAPIGroup = securityv1alpha1.SchemeGroupVersion.Group
			credentialsAPIVersion = securityv1alpha1.SchemeGroupVersion.Version
			credentialsResource = "workloadidentities"
		} else {
			return authorizer.AttributesRecord{}, errors.New("unknown credentials ref: CredentialsBinding is referencing neither a Secret nor a WorkloadIdentity")
		}
		return authorizer.AttributesRecord{
			User:            a.GetUserInfo(),
			Verb:            "get",
			APIGroup:        credentialsAPIGroup,
			APIVersion:      credentialsAPIVersion,
			Resource:        credentialsResource,
			Namespace:       credentialsBinding.CredentialsRef.Namespace,
			Name:            credentialsBinding.CredentialsRef.Name,
			ResourceRequest: true,
		}, nil
	}

	// Prevent users from changing the credentials binding unless they have read permissions for both old and new credentials.
	// This ensures that if a user has access to a shoot that references a binding in another namespace controlled by another party
	// the said user cannot reference another binding and potentially change the underlying cloud provider account
	// and leave orphaned resources in the other party's account.
	if c.oldShoot.Spec.CredentialsBindingName != nil && c.shoot.Spec.CredentialsBindingName != nil &&
		*c.oldShoot.Spec.CredentialsBindingName != *c.shoot.Spec.CredentialsBindingName {
		oldCredentialsBinding, err := credentialsBindingLister.CredentialsBindings(c.oldShoot.Namespace).Get(*c.oldShoot.Spec.CredentialsBindingName)
		if err != nil {
			return apierrors.NewInternalError(fmt.Errorf("could not retrieve previously referenced credentials binding: %+v", err.Error()))
		}

		oldCredentialsBindingAttributesRecord, err := getAttributesRecord(oldCredentialsBinding)
		if err != nil {
			return admission.NewForbidden(a, err)
		}

		if decision, _, err := auth.Authorize(ctx, oldCredentialsBindingAttributesRecord); err != nil {
			return apierrors.NewInternalError(fmt.Errorf("could not authorize read request for old credentials: %+v", err.Error()))
		} else if decision != authorizer.DecisionAllow {
			return admission.NewForbidden(a, fmt.Errorf("user %q is not allowed to read the previously referenced %s %q", a.GetUserInfo().GetName(), oldCredentialsBinding.CredentialsRef.Kind, oldCredentialsBinding.CredentialsRef.Namespace+"/"+oldCredentialsBinding.CredentialsRef.Name))
		}

		newCredentialsBinding, err := credentialsBindingLister.CredentialsBindings(c.shoot.Namespace).Get(*c.shoot.Spec.CredentialsBindingName)
		if err != nil {
			return apierrors.NewInternalError(fmt.Errorf("could not retrieve newly referenced credentials binding: %+v", err.Error()))
		}

		newCredentialsBindingAttributesRecord, err := getAttributesRecord(newCredentialsBinding)
		if err != nil {
			return admission.NewForbidden(a, err)
		}

		if decision, _, err := auth.Authorize(ctx, newCredentialsBindingAttributesRecord); err != nil {
			return apierrors.NewInternalError(fmt.Errorf("could not authorize read request for new credentials: %+v", err.Error()))
		} else if decision != authorizer.DecisionAllow {
			return admission.NewForbidden(a, fmt.Errorf("user %q is not allowed to read the newly referenced %s %q", a.GetUserInfo().GetName(), newCredentialsBinding.CredentialsRef.Kind, newCredentialsBinding.CredentialsRef.Namespace+"/"+newCredentialsBinding.CredentialsRef.Name))
		}
	}
	return nil
}

func (c *validationContext) ensureMachineImages() field.ErrorList {
	allErrs := field.ErrorList{}

	if c.shoot.DeletionTimestamp == nil {
		for idx, worker := range c.shoot.Spec.Provider.Workers {
			fldPath := field.NewPath("spec", "provider", "workers").Index(idx)

			image, err := ensureMachineImage(c.oldShoot.Spec.Provider.Workers, worker, c.cloudProfileSpec.MachineImages, fldPath)
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

	if !reflect.DeepEqual(c.oldShoot.Spec.Provider.InfrastructureConfig, c.shoot.Spec.Provider.InfrastructureConfig) ||
		c.oldShoot.Spec.Networking != nil && c.oldShoot.Spec.Networking.IPFamilies != nil && !reflect.DeepEqual(c.oldShoot.Spec.Networking.IPFamilies, c.shoot.Spec.Networking.IPFamilies) {
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

	if sets.New(
		v1beta1constants.ShootOperationRotateSSHKeypair,
		v1beta1constants.OperationRotateCredentialsStart,
		v1beta1constants.OperationRotateCredentialsStartWithoutWorkersRollout,
	).Has(c.shoot.Annotations[v1beta1constants.GardenerOperation]) {
		addInfrastructureDeploymentTask(c.shoot)
	}

	if c.shoot.Spec.Maintenance != nil &&
		ptr.Deref(c.shoot.Spec.Maintenance.ConfineSpecUpdateRollout, false) &&
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

// For backwards-compatibility, we want to validate the oidc config only for newly created Shoot clusters.
// Performing the validation for all Shoots would prevent already existing Shoots with the wrong spec to be updated/deleted.
// There is additional oidc config validation in the static API validation.
func (c *validationContext) validateKubeAPIServerOIDCConfig(a admission.Attributes) field.ErrorList {
	var (
		allErrs field.ErrorList
		path    = field.NewPath("spec", "kubernetes", "kubeAPIServer", "oidcConfig")
	)

	if a.GetOperation() != admission.Create {
		return nil
	}

	if c.shoot.Spec.Kubernetes.KubeAPIServer == nil || c.shoot.Spec.Kubernetes.KubeAPIServer.OIDCConfig == nil {
		return nil
	}

	oidc := c.shoot.Spec.Kubernetes.KubeAPIServer.OIDCConfig
	if oidc.ClientID == nil {
		allErrs = append(allErrs, field.Required(path.Child("clientID"), "clientID must be set when oidcConfig is provided"))
	} else if len(*oidc.ClientID) == 0 {
		allErrs = append(allErrs, field.Required(path.Child("clientID"), "clientID cannot be empty"))
	}

	if oidc.IssuerURL == nil {
		allErrs = append(allErrs, field.Required(path.Child("issuerURL"), "issuerURL must be set when oidcConfig is provided"))
	} else if len(*oidc.IssuerURL) == 0 {
		allErrs = append(allErrs, field.Required(path.Child("issuerURL"), "issuerURL cannot be empty"))
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

func cidrMatchesIPFamily(cidr string, ipfamilies []core.IPFamily) bool {
	ip, _, _ := net.ParseCIDR(cidr)
	return ip != nil && (ip.To4() != nil && slices.Contains(ipfamilies, core.IPFamilyIPv4) || ip.To4() == nil && slices.Contains(ipfamilies, core.IPFamilyIPv6))
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
				if cidrMatchesIPFamily(*c.seed.Spec.Networks.ShootDefaults.Pods, c.shoot.Spec.Networking.IPFamilies) {
					c.shoot.Spec.Networking.Pods = c.seed.Spec.Networks.ShootDefaults.Pods
				}
			} else if slices.Contains(c.shoot.Spec.Networking.IPFamilies, core.IPFamilyIPv4) {
				allErrs = append(allErrs, field.Required(path.Child("pods"), "pods is required"))
			}
		}

		if c.shoot.Spec.Networking.Services == nil {
			if c.seed.Spec.Networks.ShootDefaults != nil {
				if cidrMatchesIPFamily(*c.seed.Spec.Networks.ShootDefaults.Services, c.shoot.Spec.Networking.IPFamilies) {
					c.shoot.Spec.Networking.Services = c.seed.Spec.Networks.ShootDefaults.Services
				}
			} else if slices.Contains(c.shoot.Spec.Networking.IPFamilies, core.IPFamilyIPv4) {
				allErrs = append(allErrs, field.Required(path.Child("services"), "services is required"))
			}
		}

		if slices.Contains(c.shoot.Spec.Networking.IPFamilies, core.IPFamilyIPv4) {
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

				// validate network disjointedness with seed networks if networking status is non-empty
				if c.shoot.Status.Networking != nil {
					networkingStatus := c.shoot.Status.Networking.DeepCopy()
					networkingStatus.EgressCIDRs = nil
					if !apiequality.Semantic.DeepEqual(networkingStatus, &core.NetworkingStatus{}) {
						allErrs = append(allErrs, cidrvalidation.ValidateMultiNetworkDisjointedness(
							field.NewPath("status", "networking"),
							c.shoot.Status.Networking.Nodes,
							c.shoot.Status.Networking.Pods,
							c.shoot.Status.Networking.Services,
							c.seed.Spec.Networks.Nodes,
							c.seed.Spec.Networks.Pods,
							c.seed.Spec.Networks.Services,
							workerless,
						)...)
					}
				}
			}
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

	defaultVersion, errList := defaultKubernetesVersion(c.cloudProfileSpec.Kubernetes.Versions, c.shoot.Spec.Kubernetes.Version, path.Child("version"))
	if len(errList) > 0 {
		allErrs = append(allErrs, errList...)
	}

	if defaultVersion != nil {
		c.shoot.Spec.Kubernetes.Version = *defaultVersion
	} else {
		// We assume that the 'defaultVersion' is already calculated correctly, so only run validation if the version was not defaulted.
		allErrs = append(allErrs, validateKubernetesVersionConstraints(a, c.cloudProfileSpec.Kubernetes.Versions, c.shoot.Spec.Kubernetes.Version, c.oldShoot.Spec.Kubernetes.Version, false, path.Child("version"))...)
	}

	allErrs = append(allErrs, c.validateKubeAPIServerOIDCConfig(a)...)

	return allErrs
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

	if err := c.validateProviderType(path); err != nil {
		return append(allErrs, err)
	}

	controlPlaneVersion, err := semver.NewVersion(c.shoot.Spec.Kubernetes.Version)
	if err != nil {
		return append(allErrs, field.Invalid(field.NewPath("spec", "kubernetes", "version"), c.shoot.Spec.Kubernetes.Version, fmt.Sprintf("cannot parse the kubernetes version: %v", err)))
	}

	for i, worker := range c.shoot.Spec.Provider.Workers {
		idxPath := path.Child("workers").Index(i)
		oldWorker, isNewWorkerPool := c.getOldWorker(worker.Name)

		workerErr := c.validateWorkerMachine(idxPath, worker, oldWorker, isNewWorkerPool, a)
		if workerErr != nil {
			allErrs = append(allErrs, workerErr)
		} else {
			allErrs = append(allErrs, validateContainerRuntimeInterface(c.cloudProfileSpec.MachineImages, worker, oldWorker, idxPath.Child("cri"))...)
			kubeletVersion, err := helper.CalculateEffectiveKubernetesVersion(controlPlaneVersion, worker.Kubernetes)
			if err != nil {
				// exit early, all other validation errors will be misleading
				return append(allErrs, field.Invalid(idxPath.Child("kubernetes", "version"), worker.Kubernetes.Version, "cannot determine effective Kubernetes version for worker pool"))
			}
			if err := validateKubeletVersion(c.cloudProfileSpec.MachineImages, worker, kubeletVersion, idxPath); err != nil {
				allErrs = append(allErrs, err)
			}
		}

		if err := c.validateWorkerVolume(idxPath, worker, oldWorker); err != nil {
			allErrs = append(allErrs, err)
		}

		if worker.Kubernetes != nil {
			if worker.Kubernetes.Kubelet != nil {
				kubeletConfig = worker.Kubernetes.Kubelet
			}
			allErrs = append(allErrs, validateKubeletConfig(idxPath.Child("kubernetes").Child("kubelet"), c.cloudProfileSpec.MachineTypes, worker.Machine.Type, kubeletConfig)...)

			if errList := c.validateWorkerKubernetesVersion(idxPath, worker, oldWorker, isNewWorkerPool, a); len(errList) > 0 {
				allErrs = append(allErrs, errList...)
			}
		}

		allErrs = append(allErrs, validateZones(c.cloudProfileSpec.Regions, c.shoot.Spec.Region, c.oldShoot.Spec.Region, worker, oldWorker, idxPath)...)
	}

	return allErrs
}

func (c *validationContext) validateProviderType(path *field.Path) *field.Error {
	if c.shoot.Spec.Provider.Type != c.cloudProfileSpec.Type {
		return field.Invalid(path.Child("type"), c.shoot.Spec.Provider.Type, fmt.Sprintf("provider type in shoot must equal provider type of referenced CloudProfile: %q", c.cloudProfileSpec.Type))
	}

	if c.secretBinding != nil && !v1beta1helper.SecretBindingHasType(c.secretBinding, c.shoot.Spec.Provider.Type) {
		var secretBindingProviderType string
		if c.secretBinding.Provider != nil {
			secretBindingProviderType = c.secretBinding.Provider.Type
		}
		return field.Invalid(path.Child("type"), c.shoot.Spec.Provider.Type, fmt.Sprintf("provider type in shoot must match provider type of referenced SecretBinding: %q", secretBindingProviderType))
	}

	if c.credentialsBinding != nil && c.credentialsBinding.Provider.Type != c.shoot.Spec.Provider.Type {
		return field.Invalid(path.Child("type"), c.shoot.Spec.Provider.Type, fmt.Sprintf("provider type in shoot must match provider type of referenced CredentialsBinding: %q", c.credentialsBinding.Provider.Type))
	}

	return nil
}

func (c *validationContext) getOldWorker(workerName string) (core.Worker, bool) {
	for _, ow := range c.oldShoot.Spec.Provider.Workers {
		if ow.Name == workerName {
			return ow, false
		}
	}
	return core.Worker{Machine: core.Machine{Image: &core.ShootMachineImage{}}}, true
}

func (c *validationContext) validateWorkerMachine(idxPath *field.Path, worker, oldWorker core.Worker, isNewWorkerPool bool, a admission.Attributes) *field.Error {
	if worker.Machine.Architecture != nil && !slices.Contains(v1beta1constants.ValidArchitectures, *worker.Machine.Architecture) {
		return field.NotSupported(idxPath.Child("machine", "architecture"), *worker.Machine.Architecture, v1beta1constants.ValidArchitectures)
	}

	isMachinePresentInCloudprofile, architectureSupported, availableInAllZones, isUsableMachine, supportedMachineTypes := validateMachineTypes(c.cloudProfileSpec.MachineTypes, worker.Machine, oldWorker.Machine, c.cloudProfileSpec.Regions, c.shoot.Spec.Region, worker.Zones)
	if !isMachinePresentInCloudprofile {
		return field.NotSupported(idxPath.Child("machine", "type"), worker.Machine.Type, supportedMachineTypes)
	}

	if !architectureSupported || !availableInAllZones || !isUsableMachine {
		detail := fmt.Sprintf("machine type %q ", worker.Machine.Type)
		if !isUsableMachine {
			detail += "is unusable, "
		}
		if !availableInAllZones {
			detail += "is unavailable in at least one zone, "
		}
		if !architectureSupported {
			detail += fmt.Sprintf("does not support CPU architecture %q, ", *worker.Machine.Architecture)
		}
		return field.Invalid(idxPath.Child("machine", "type"), worker.Machine.Type, fmt.Sprintf("%ssupported types are %+v", detail, supportedMachineTypes))
	}

	isUpdateStrategyInPlace := helper.IsUpdateStrategyInPlace(worker.UpdateStrategy)
	isMachineImagePresentInCloudprofile, architectureSupported, activeMachineImageVersion, inPlaceUpdateSupported, validMachineImageVersions := validateMachineImagesConstraints(a, c.cloudProfileSpec.MachineImages, isNewWorkerPool, isUpdateStrategyInPlace, worker.Machine, oldWorker.Machine)
	if !isMachineImagePresentInCloudprofile {
		return field.Invalid(idxPath.Child("machine", "image"), worker.Machine.Image, fmt.Sprintf("machine image version is not supported, supported machine image versions are: %+v", validMachineImageVersions))
	}

	if !architectureSupported || !activeMachineImageVersion || (isUpdateStrategyInPlace && !inPlaceUpdateSupported) {
		detail := fmt.Sprintf("machine image version '%s:%s' ", worker.Machine.Image.Name, worker.Machine.Image.Version)
		if !architectureSupported {
			detail += fmt.Sprintf("does not support CPU architecture %q, ", *worker.Machine.Architecture)
		}
		if !activeMachineImageVersion {
			detail += "is expired, "
		}
		if isUpdateStrategyInPlace && !inPlaceUpdateSupported {
			if a.GetOperation() == admission.Update && !isNewWorkerPool {
				detail += "cannot be in-place updated from the current version, "
			} else {
				detail += "does not support in-place updates, "
			}
		}
		return field.Invalid(idxPath.Child("machine", "image"), worker.Machine.Image, fmt.Sprintf("%ssupported machine image versions are: %+v", detail, validMachineImageVersions))
	}

	return nil
}

func (c *validationContext) validateWorkerVolume(idxPath *field.Path, worker, oldWorker core.Worker) *field.Error {
	isVolumePresentInCloudprofile, availableInAllZones, isUsableVolume, supportedVolumeTypes := validateVolumeTypes(c.cloudProfileSpec.VolumeTypes, worker.Volume, oldWorker.Volume, c.cloudProfileSpec.Regions, c.shoot.Spec.Region, worker.Zones)
	if !isVolumePresentInCloudprofile {
		return field.NotSupported(idxPath.Child("volume", "type"), ptr.Deref(worker.Volume.Type, ""), supportedVolumeTypes)
	}

	if !availableInAllZones || !isUsableVolume {
		detail := fmt.Sprintf("volume type %q ", *worker.Volume.Type)
		if !isUsableVolume {
			detail += "is unusable, "
		}
		if !availableInAllZones {
			detail += "is unavailable in at least one zone, "
		}
		return field.Invalid(idxPath.Child("volume", "type"), *worker.Volume.Type, fmt.Sprintf("%ssupported types are %+v", detail, supportedVolumeTypes))
	}

	if ok, minSize := validateVolumeSize(c.cloudProfileSpec.VolumeTypes, c.cloudProfileSpec.MachineTypes, worker.Machine.Type, worker.Volume); !ok {
		return field.Invalid(idxPath.Child("volume", "size"), worker.Volume.VolumeSize, fmt.Sprintf("size must be >= %s", minSize))
	}

	return nil
}

func (c *validationContext) validateWorkerKubernetesVersion(idxPath *field.Path, worker, oldWorker core.Worker, isNewWorkerPool bool, a admission.Attributes) field.ErrorList {
	if worker.Kubernetes.Version == nil {
		return nil
	}
	oldWorkerKubernetesVersion := c.oldShoot.Spec.Kubernetes.Version
	if oldWorker.Kubernetes != nil && oldWorker.Kubernetes.Version != nil {
		oldWorkerKubernetesVersion = *oldWorker.Kubernetes.Version
	}

	defaultVersion, errList := defaultKubernetesVersion(c.cloudProfileSpec.Kubernetes.Versions, *worker.Kubernetes.Version, idxPath.Child("kubernetes", "version"))
	if len(errList) > 0 {
		return errList
	}

	if defaultVersion == nil {
		return validateKubernetesVersionConstraints(a, c.cloudProfileSpec.Kubernetes.Versions, *worker.Kubernetes.Version, oldWorkerKubernetesVersion, isNewWorkerPool, idxPath.Child("kubernetes", "version"))
	}
	worker.Kubernetes.Version = defaultVersion
	return nil
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

func validateVolumeSize(volumeTypeConstraints []gardencorev1beta1.VolumeType, machineTypeConstraints []gardencorev1beta1.MachineType, machineType string, volume *core.Volume) (bool, string) {
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

func (c *validationContext) validateDNSDomainUniqueness(shootLister gardencorev1beta1listers.ShootLister) (field.ErrorList, error) {
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
		if shoot.Name == c.shoot.Name &&
			shoot.Namespace == c.shoot.Namespace {
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

func defaultKubernetesVersion(constraints []gardencorev1beta1.ExpirableVersion, shootVersion string, fldPath *field.Path) (*string, field.ErrorList) {
	var (
		allErrs           = field.ErrorList{}
		shootVersionMajor *uint64
		shootVersionMinor *uint64
		versionParts      = strings.Split(shootVersion, ".")
	)

	if len(versionParts) == 3 {
		return nil, allErrs
	}
	if len(versionParts) == 2 && len(versionParts[1]) > 0 {
		v, err := strconv.ParseUint(versionParts[1], 10, 0)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath, versionParts[1], "must be a semantic version"))
			return nil, allErrs
		}
		shootVersionMinor = ptr.To(v)
	}
	if len(versionParts) >= 1 && len(versionParts[0]) > 0 {
		v, err := strconv.ParseUint(versionParts[0], 10, 0)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath, versionParts[0], "must be a semantic version"))
			return nil, allErrs
		}
		shootVersionMajor = ptr.To(v)
	}

	if latestVersion := findLatestVersion(constraints, shootVersionMajor, shootVersionMinor); latestVersion != nil {
		return ptr.To(latestVersion.String()), nil
	}

	allErrs = append(allErrs, field.Invalid(fldPath, shootVersion, fmt.Sprintf("couldn't find a suitable version for %s. Suitable versions have a non-expired expiration date and are no 'preview' versions. 'Preview'-classified versions have to be selected explicitly", shootVersion)))
	return nil, allErrs
}

func findLatestVersion(constraints []gardencorev1beta1.ExpirableVersion, major, minor *uint64) *semver.Version {
	var latestVersion *semver.Version
	for _, versionConstraint := range constraints {
		// ignore expired versions
		if versionConstraint.ExpirationDate != nil && versionConstraint.ExpirationDate.Time.UTC().Before(time.Now().UTC()) {
			continue
		}

		// filter preview versions for defaulting
		if ptr.Deref(versionConstraint.Classification, "") == gardencorev1beta1.ClassificationPreview {
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

func validateKubernetesVersionConstraints(a admission.Attributes, constraints []gardencorev1beta1.ExpirableVersion, shootVersion, oldShootVersion string, isNewWorkerPool bool, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if shootVersion == oldShootVersion {
		return allErrs
	}

	var validValues []string
	for _, versionConstraint := range constraints {
		// Disallow usage of an expired Kubernetes version on Shoot creation and new worker pool creation
		// Updating an existing worker to a higher (ensured by validation) expired Kubernetes version is necessary for consecutive maintenance force updates
		if a.GetOperation() == admission.Create || isNewWorkerPool {
			if versionConstraint.ExpirationDate != nil && versionConstraint.ExpirationDate.Time.UTC().Before(time.Now().UTC()) {
				continue
			}
		}

		if versionConstraint.Version == shootVersion {
			return allErrs
		}

		versionStr := versionConstraint.Version
		if ptr.Deref(versionConstraint.Classification, "") == gardencorev1beta1.ClassificationPreview {
			versionStr += " (preview)"
		}
		validValues = append(validValues, versionStr)
	}

	allErrs = append(allErrs, field.NotSupported(fldPath, shootVersion, validValues))

	return allErrs
}

func validateMachineTypes(constraints []gardencorev1beta1.MachineType, machine, oldMachine core.Machine, regions []gardencorev1beta1.Region, region string, zones []string) (bool, bool, bool, bool, []string) {
	if machine.Type == oldMachine.Type && ptr.Equal(machine.Architecture, oldMachine.Architecture) {
		return true, true, true, true, nil
	}

	var (
		isMachinePresentInCloudprofile    = false
		machinesWithSupportedArchitecture = sets.New[string]()
		machinesAvailableInAllZones       = sets.New[string]()
		usableMachines                    = sets.New[string]()
	)

	for _, t := range constraints {
		if ptr.Equal(t.Architecture, machine.Architecture) {
			machinesWithSupportedArchitecture.Insert(t.Name)
		}
		if ptr.Deref(t.Usable, false) {
			usableMachines.Insert(t.Name)
		}
		if !isUnavailableInAtleastOneZone(regions, region, zones, t.Name, func(zone gardencorev1beta1.AvailabilityZone) []string { return zone.UnavailableMachineTypes }) {
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

func isUnavailableInAtleastOneZone(regions []gardencorev1beta1.Region, region string, zones []string, t string, unavailableTypes func(zone gardencorev1beta1.AvailabilityZone) []string) bool {
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

func validateKubeletConfig(fldPath *field.Path, machineTypes []gardencorev1beta1.MachineType, workerMachineType string, kubeletConfig *core.KubeletConfig) field.ErrorList {
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

func validateVolumeTypes(constraints []gardencorev1beta1.VolumeType, volume, oldVolume *core.Volume, regions []gardencorev1beta1.Region, region string, zones []string) (bool, bool, bool, []string) {
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
		if ptr.Deref(v.Usable, false) {
			usableVolumes.Insert(v.Name)
		}
		if !isUnavailableInAtleastOneZone(regions, region, zones, v.Name, func(zone gardencorev1beta1.AvailabilityZone) []string { return zone.UnavailableVolumeTypes }) {
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

	for _, r := range c.cloudProfileSpec.Regions {
		validValues = append(validValues, r.Name)
		if r.Name == region {
			return nil
		}
	}

	return field.ErrorList{field.NotSupported(fldPath, region, validValues)}
}

func (c *validationContext) validateAccessRestrictions() field.ErrorList {
	var (
		allErrs     field.ErrorList
		fldPath     = field.NewPath("spec", "accessRestrictions")
		validValues = sets.New[string]()

		accessRestrictions    = sets.New[string]()
		oldAccessRestrictions = sets.New[string]()
	)

	for _, restriction := range c.shoot.Spec.AccessRestrictions {
		accessRestrictions.Insert(restriction.Name)
	}
	for _, restriction := range c.oldShoot.Spec.AccessRestrictions {
		oldAccessRestrictions.Insert(restriction.Name)
	}

	if accessRestrictions.Equal(oldAccessRestrictions) {
		return nil
	}

	// verify that the access restrictions are present for this region in the CloudProfile
	regionIndex := slices.IndexFunc(c.cloudProfileSpec.Regions, func(region gardencorev1beta1.Region) bool {
		return region.Name == c.shoot.Spec.Region
	})

	if regionIndex == -1 {
		return field.ErrorList{field.Invalid(field.NewPath("spec", "region"), c.shoot.Spec.Region, "region not present in CloudProfile")}
	}

	for _, restriction := range c.cloudProfileSpec.Regions[regionIndex].AccessRestrictions {
		validValues.Insert(restriction.Name)
	}

	for i, restriction := range c.shoot.Spec.AccessRestrictions {
		if !validValues.Has(restriction.Name) {
			allErrs = append(allErrs, field.NotSupported(fldPath.Index(i), restriction.Name, validValues.UnsortedList()))
		}
	}

	// verify that the access restrictions are supported by the seed
	if c.seed != nil {
		supportedAccessRestrictionsOfSeed := sets.New[string]()
		for _, restriction := range c.seed.Spec.AccessRestrictions {
			supportedAccessRestrictionsOfSeed.Insert(restriction.Name)
		}

		for i, restriction := range c.shoot.Spec.AccessRestrictions {
			if !supportedAccessRestrictionsOfSeed.Has(restriction.Name) {
				allErrs = append(allErrs, field.Forbidden(fldPath.Index(i), fmt.Sprintf("access restriction %q is not supported by the seed", restriction.Name)))
			}
		}
	}

	return allErrs
}

func validateZones(constraints []gardencorev1beta1.Region, region, oldRegion string, worker, oldWorker core.Worker, fldPath *field.Path) field.ErrorList {
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

func validateZone(constraints []gardencorev1beta1.Region, region, zone string) (bool, []string) {
	var validValues []string

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
func getDefaultMachineImage(machineImages []gardencorev1beta1.MachineImage, image *core.ShootMachineImage, arch *string, isUpdateStrategyInPlace bool, fldPath *field.Path) (*core.ShootMachineImage, *field.Error) {
	var imageReference string
	if image != nil {
		imageReference = fmt.Sprintf("%s@%s", image.Name, image.Version)
	}

	if len(machineImages) == 0 {
		return nil, field.Invalid(fldPath, imageReference, "the cloud profile does not contain any machine image - cannot create shoot cluster")
	}

	var defaultImage *core.MachineImage

	if image != nil && len(image.Name) != 0 {
		for _, mi := range machineImages {
			machineImage := mi
			if machineImage.Name == image.Name {
				coreMachineImage := &core.MachineImage{}
				if err := gardencorev1beta1.Convert_v1beta1_MachineImage_To_core_MachineImage(&machineImage, coreMachineImage, nil); err != nil {
					return nil, field.Invalid(fldPath, machineImage.Name, fmt.Sprintf("failed to convert machine image from cloud profile: %s", err.Error()))
				}
				defaultImage = coreMachineImage

				break
			}
		}
		if defaultImage == nil {
			return nil, field.Invalid(fldPath, image.Name, "image is not supported")
		}
	} else {
		// select the first image which supports the required architecture type
		for _, mi := range machineImages {
			machineImage := mi
			for _, version := range machineImage.Versions {
				coreMachineImage := &core.MachineImage{}
				if err := gardencorev1beta1.Convert_v1beta1_MachineImage_To_core_MachineImage(&machineImage, coreMachineImage, nil); err != nil {
					return nil, field.Invalid(fldPath, machineImage.Name, fmt.Sprintf("failed to convert machine image from cloud profile: %s", err.Error()))
				}

				if slices.Contains(version.Architectures, *arch) {
					defaultImage = coreMachineImage
					break
				}
			}
			if defaultImage != nil {
				break
			}
		}
		if defaultImage == nil {
			return nil, field.Invalid(fldPath, imageReference, fmt.Sprintf("no valid machine image found that supports architecture `%s`", *arch))
		}
	}

	var (
		machineImageVersionMajor *uint64
		machineImageVersionMinor *uint64
	)

	if image != nil {
		var err error
		versionParts := strings.Split(strings.TrimPrefix(image.Version, "v"), ".")
		if len(versionParts) == 3 {
			return image, nil
		}
		if len(versionParts) == 2 && len(versionParts[1]) > 0 {
			if machineImageVersionMinor, err = parseSemanticVersionPart(versionParts[1]); err != nil {
				return nil, field.Invalid(fldPath, image.Version, err.Error())
			}
		}
		if len(versionParts) >= 1 && len(versionParts[0]) > 0 {
			if machineImageVersionMajor, err = parseSemanticVersionPart(versionParts[0]); err != nil {
				return nil, field.Invalid(fldPath, image.Version, err.Error())
			}
		}
	}

	var validVersions []core.MachineImageVersion

	for _, version := range defaultImage.Versions {
		if !slices.Contains(version.Architectures, *arch) {
			continue
		}

		// if InPlace update is true, only consider versions that support in-place updates
		if isUpdateStrategyInPlace && (version.InPlaceUpdates == nil || !version.InPlaceUpdates.Supported) {
			continue
		}

		// CloudProfile cannot contain invalid semVer machine image version
		parsedVersion := semver.MustParse(version.Version)
		if machineImageVersionMajor != nil && parsedVersion.Major() != *machineImageVersionMajor ||
			machineImageVersionMinor != nil && parsedVersion.Minor() != *machineImageVersionMinor {
			continue
		}
		validVersions = append(validVersions, version)
	}

	latestMachineImageVersion, err := helper.DetermineLatestMachineImageVersion(validVersions, true)
	if err != nil {
		return nil, field.Invalid(fldPath, imageReference, fmt.Sprintf("failed to determine latest machine image from cloud profile: %s", err.Error()))
	}
	var providerConfig *runtime.RawExtension
	if image != nil {
		providerConfig = image.ProviderConfig
	}
	return &core.ShootMachineImage{Name: defaultImage.Name, ProviderConfig: providerConfig, Version: latestMachineImageVersion.Version}, nil
}

func parseSemanticVersionPart(part string) (*uint64, error) {
	v, err := strconv.ParseUint(part, 10, 0)
	if err != nil {
		return nil, fmt.Errorf("%s must be a semantic version: %w", part, err)
	}
	return ptr.To(v), nil
}

func validateMachineImagesConstraints(a admission.Attributes, constraints []gardencorev1beta1.MachineImage, isNewWorkerPool, isUpdateStrategyInPlace bool, machine, oldMachine core.Machine) (bool, bool, bool, bool, []string) {
	if apiequality.Semantic.DeepEqual(machine.Image, oldMachine.Image) && ptr.Equal(machine.Architecture, oldMachine.Architecture) {
		return true, true, true, true, nil
	}

	var (
		machineImageVersionsInCloudProfile            = sets.New[string]()
		activeMachineImageVersions                    = sets.New[string]()
		machineImageVersionsWithSupportedArchitecture = sets.New[string]()
		machineImageVersionsWithInPlaceUpdateSupport  = sets.New[string]()
	)

	for _, machineImage := range constraints {
		if machine.Image == nil || machine.Image.Name == machineImage.Name {
			for _, machineVersion := range machineImage.Versions {
				machineImageVersion := fmt.Sprintf("%s:%s", machineImage.Name, machineVersion.Version)

				if machineVersion.ExpirationDate == nil || machineVersion.ExpirationDate.Time.UTC().After(time.Now().UTC()) {
					activeMachineImageVersions.Insert(machineImageVersion)
				} else if machineVersion.ExpirationDate != nil && machineVersion.ExpirationDate.Time.UTC().Before(time.Now().UTC()) && a.GetOperation() == admission.Update && !isNewWorkerPool {
					// An already expired machine image version is a viable machine image version for the worker pool if-and-only-if:
					//  - this is an update call (no new Shoot creation)
					//  - updates an existing worker pool (not for a new worker pool)
					//  - the expired version is higher than the old machine's version
					// Reason: updating an existing worker pool to an expired machine image version is required for maintenance force updates
					downgrade, _ := versionutils.CompareVersions(machineVersion.Version, "<", oldMachine.Image.Version)
					if !downgrade {
						activeMachineImageVersions.Insert(machineImageVersion)
					}
				}

				if slices.Contains(machineVersion.Architectures, *machine.Architecture) {
					machineImageVersionsWithSupportedArchitecture.Insert(machineImageVersion)
				}

				if isUpdateStrategyInPlace && machineVersion.InPlaceUpdates != nil {
					switch {
					case a.GetOperation() == admission.Create || isNewWorkerPool:
						if machineVersion.InPlaceUpdates.Supported {
							machineImageVersionsWithInPlaceUpdateSupport.Insert(machineImageVersion)
						}
					case a.GetOperation() == admission.Update && !isNewWorkerPool && machine.Image != nil && machine.Image.Name == machineImage.Name:
						if machineVersion.InPlaceUpdates.Supported && machineVersion.InPlaceUpdates.MinVersionForUpdate != nil {
							// This checks if the MinVersionForInPlaceUpdate (the minimum version which can be in-place updated to the current version)
							// is less than or equal to the old machine's version. If the condition is true, the version is considered valid
							// for performing the in-place update on the machine.
							if validVersion, _ := versionutils.CompareVersions(*machineVersion.InPlaceUpdates.MinVersionForUpdate, "<=", oldMachine.Image.Version); validVersion {
								machineImageVersionsWithInPlaceUpdateSupport.Insert(machineImageVersion)
							}
						}
					}
				}

				machineImageVersionsInCloudProfile.Insert(machineImageVersion)
			}
		}
	}

	// valid machine image versions are all versions that can be used by this worker pool
	validMachineImageVersions := activeMachineImageVersions.Intersection(machineImageVersionsWithSupportedArchitecture)
	if isUpdateStrategyInPlace {
		validMachineImageVersions = validMachineImageVersions.Intersection(machineImageVersionsWithInPlaceUpdateSupport)
	}

	if machine.Image == nil || len(machine.Image.Version) == 0 {
		return false, false, false, false, sets.List(validMachineImageVersions)
	}

	shootMachineImageVersion := fmt.Sprintf("%s:%s", machine.Image.Name, machine.Image.Version)
	return machineImageVersionsInCloudProfile.Has(shootMachineImageVersion),
		machineImageVersionsWithSupportedArchitecture.Has(shootMachineImageVersion),
		activeMachineImageVersions.Has(shootMachineImageVersion),
		machineImageVersionsWithInPlaceUpdateSupport.Has(shootMachineImageVersion),
		sets.List(validMachineImageVersions)
}

func validateContainerRuntimeInterface(constraints []gardencorev1beta1.MachineImage, worker, oldWorker core.Worker, fldPath *field.Path) field.ErrorList {
	if worker.CRI == nil || worker.Machine.Image == nil {
		return nil
	}

	if apiequality.Semantic.DeepEqual(worker.CRI, oldWorker.CRI) &&
		apiequality.Semantic.DeepEqual(worker.Machine.Image, oldWorker.Machine.Image) {
		return nil
	}

	machineImageVersion, ok := v1beta1helper.FindMachineImageVersion(constraints, worker.Machine.Image.Name, worker.Machine.Image.Version)
	if !ok {
		return nil
	}

	return validateCRI(machineImageVersion.CRI, worker, fldPath)
}

func validateCRI(constraints []gardencorev1beta1.CRI, worker core.Worker, fldPath *field.Path) field.ErrorList {
	if worker.CRI == nil {
		return nil
	}

	var (
		allErrors = field.ErrorList{}
		validCRIs = []string{}
		foundCRI  *core.CRI
	)

	for _, c := range constraints {
		criConstraint := c
		validCRIs = append(validCRIs, string(criConstraint.Name))

		coreCRIConstraint := &core.CRI{}
		if err := gardencorev1beta1.Convert_v1beta1_CRI_To_core_CRI(&criConstraint, coreCRIConstraint, nil); err != nil {
			return append(allErrors, field.Invalid(fldPath, worker.CRI.Name, fmt.Sprintf("failed to convert CRI from cloud profile: %s", err.Error())))
		}

		if worker.CRI.Name == coreCRIConstraint.Name {
			foundCRI = coreCRIConstraint
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
	var validValues []string
	for _, constraint := range constraints {
		validValues = append(validValues, constraint.Type)
		if constraint.Type == cr {
			return true, nil
		}
	}
	return false, validValues
}

func validateKubeletVersion(constraints []gardencorev1beta1.MachineImage, worker core.Worker, kubeletVersion *semver.Version, fldPath *field.Path) *field.Error {
	if worker.Machine.Image == nil {
		return nil
	}

	machineImageVersion, ok := v1beta1helper.FindMachineImageVersion(constraints, worker.Machine.Image.Name, worker.Machine.Image.Version)
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

func ensureMachineImage(oldWorkers []core.Worker, worker core.Worker, images []gardencorev1beta1.MachineImage, fldPath *field.Path) (*core.ShootMachineImage, *field.Error) {
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

	return getDefaultMachineImage(images, worker.Machine.Image, worker.Machine.Architecture, helper.IsUpdateStrategyInPlace(worker.UpdateStrategy), fldPath)
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
	if shoot.Annotations == nil {
		shoot.Annotations = make(map[string]string)
	}
	controllerutils.AddTasks(shoot.Annotations, tasks...)
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

func (c *validationContext) validateManagedServiceAccountIssuer(
	a admission.Attributes,
	secretLister kubecorev1listers.SecretLister,
) error {
	// Skip the validation if no managed service account issuer configuration is involved.
	if !helper.HasManagedIssuer(c.shoot) &&
		(c.oldShoot == nil || !helper.HasManagedIssuer(c.oldShoot)) {
		return nil
	}

	managedIssuerConfigSecrets, err := secretLister.Secrets(v1beta1constants.GardenNamespace).List(labels.SelectorFromSet(labels.Set{
		v1beta1constants.GardenRole: v1beta1constants.GardenRoleShootServiceAccountIssuer,
	}))
	if err != nil {
		return apierrors.NewInternalError(fmt.Errorf("could not retrieve managed service account issuer config secret: %w", err))
	}

	// Skip the validation if no managed service account issuer secrets are found and shoot does not contain managed service account issuer configs.
	if len(managedIssuerConfigSecrets) == 0 {
		// Shoots should not be allowed to enable feature that is not configured for the Gardener installation.
		if helper.HasManagedIssuer(c.shoot) {
			return admission.NewForbidden(a, errors.New("cannot enable managed service account issuer as it is not supported in this Gardener installation"))
		}
		// Old shoot object has the feature enabled but for some reason the configuration is missing.
		// Reconciliation of such cluster can break existing integrations.
		// An intervention from a Gardener admin/operator is required.
		if c.oldShoot != nil && helper.HasManagedIssuer(c.oldShoot) {
			return apierrors.NewInternalError(errors.New("old shoot object has managed service account issuer enabled, but Gardener configuration is missing"))
		}
		return nil
	}

	// Preserve the managed service account issuer enablement during updates.
	if c.oldShoot != nil && helper.HasManagedIssuer(c.oldShoot) && !helper.HasManagedIssuer(c.shoot) {
		return admission.NewForbidden(a, errors.New("once enabled managed service account issuer cannot be disabled"))
	}

	if helper.HasManagedIssuer(c.shoot) {
		if kubeAPIServerConfig := c.shoot.Spec.Kubernetes.KubeAPIServer; kubeAPIServerConfig != nil &&
			kubeAPIServerConfig.ServiceAccountConfig != nil &&
			kubeAPIServerConfig.ServiceAccountConfig.Issuer != nil {
			return admission.NewForbidden(a, errors.New("managed service account issuer cannot be enabled when .kubernetes.kubeAPIServer.serviceAccountConfig.issuer is set"))
		}
	}

	return nil
}

func (c *validationContext) validateLimits(a admission.Attributes) field.ErrorList {
	var allErrs field.ErrorList

	if a.GetOperation() == admission.Delete || c.shoot.DeletionTimestamp != nil || c.cloudProfileSpec.Limits == nil {
		return nil
	}

	if maxNodesTotal := c.cloudProfileSpec.Limits.MaxNodesTotal; maxNodesTotal != nil {
		allErrs = append(allErrs, validateMaxNodesTotal(c.shoot.Spec.Provider.Workers, *maxNodesTotal)...)
	}

	return allErrs
}

func validateMaxNodesTotal(workers []core.Worker, maxNodesTotal int32) field.ErrorList {
	var (
		allErrs      field.ErrorList
		fldPath      = field.NewPath("spec", "provider", "workers")
		totalMinimum int32
	)

	for i, worker := range workers {
		totalMinimum += worker.Minimum
		if worker.Maximum > maxNodesTotal {
			allErrs = append(allErrs, field.Forbidden(fldPath.Index(i).Child("maximum"), fmt.Sprintf("the maximum node count of a worker pool must not exceed the limit of %d configured in the CloudProfile", maxNodesTotal)))
		}
	}

	if totalMinimum > maxNodesTotal {
		allErrs = append(allErrs, field.Forbidden(fldPath, fmt.Sprintf("the total minimum node count of all worker pools must not exceed the limit of %d configured in the CloudProfile", maxNodesTotal)))
	}

	return allErrs
}
