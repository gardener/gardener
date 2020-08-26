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

	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apis/core/helper"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	admissioninitializer "github.com/gardener/gardener/pkg/apiserver/admission/initializer"
	coreinformers "github.com/gardener/gardener/pkg/client/core/informers/internalversion"
	corelisters "github.com/gardener/gardener/pkg/client/core/listers/core/internalversion"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils"
	cidrvalidation "github.com/gardener/gardener/pkg/utils/validation/cidr"
	admissionutils "github.com/gardener/gardener/plugin/pkg/utils"

	"github.com/Masterminds/semver"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/admission"
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
	backupBucketLister corelisters.BackupBucketLister
	readyFunc          admission.ReadyFunc
}

var (
	_ = admissioninitializer.WantsInternalCoreInformerFactory(&ValidateShoot{})

	readyFuncs = []admission.ReadyFunc{}
)

// New creates a new ValidateShoot admission plugin.
func New() (*ValidateShoot, error) {
	return &ValidateShoot{
		Handler: admission.NewHandler(admission.Create, admission.Update),
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

	backupBucketInformer := f.Core().InternalVersion().BackupBuckets()
	v.backupBucketLister = backupBucketInformer.Lister()

	readyFuncs = append(
		readyFuncs,
		seedInformer.Informer().HasSynced,
		shootInformer.Informer().HasSynced,
		cloudProfileInformer.Informer().HasSynced,
		projectInformer.Informer().HasSynced,
		backupBucketInformer.Informer().HasSynced,
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
	if v.backupBucketLister == nil {
		return errors.New("missing backupbucket lister")
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

	shoot, ok := a.GetObject().(*core.Shoot)
	if !ok {
		return apierrors.NewInternalError(errors.New("could not convert resource into Shoot object"))
	}

	// We only want to validate fields in the Shoot against the CloudProfile/Seed constraints which have changed.
	// On CREATE operations we just use an empty Shoot object, forcing the validator functions to always validate.
	// On UPDATE operations we fetch the current Shoot object.
	var oldShoot = &core.Shoot{}

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
			return apierrors.NewBadRequest(fmt.Sprintf("could not find referenced seed: %+v", err.Error()))
		}
	}

	project, err := admissionutils.GetProject(shoot.Namespace, v.projectLister)
	if err != nil {
		return apierrors.NewBadRequest(fmt.Sprintf("could not find referenced project: %+v", err.Error()))
	}

	switch a.GetOperation() {
	case admission.Create:
		// We currently use the identifier "shoot-<project-name>-<shoot-name> in nearly all places for old Shoots, but have
		// changed that to "shoot--<project-name>-<shoot-name>": when creating infrastructure resources, Kubernetes resources,
		// DNS names, etc., then this identifier is used to tag/name the resources. Some of those resources have length
		// constraints that this identifier must not exceed 30 characters, thus we need to check whether Shoots do not exceed
		// this limit. The project name is a label on the namespace. If it is not found, the namespace name itself is used as
		// project name. These checks should only be performed for CREATE operations (we do not want to reject changes to existing
		// Shoots in case the limits are changed in the future).
		var lengthLimit = 21
		if len(shoot.Name) == 0 && len(shoot.GenerateName) > 0 {
			var randomLength = 5
			if len(project.Name+shoot.GenerateName) > lengthLimit-randomLength {
				return apierrors.NewBadRequest(fmt.Sprintf("the length of the shoot generateName and the project name must not exceed %d characters (project: %s; shoot with generateName: %s)", lengthLimit-randomLength, project.Name, shoot.GenerateName))
			}
		} else {
			if len(project.Name+shoot.Name) > lengthLimit {
				return apierrors.NewBadRequest(fmt.Sprintf("the length of the shoot name and the project name must not exceed %d characters (project: %s; shoot: %s)", lengthLimit, project.Name, shoot.Name))
			}
		}
		if strings.Contains(project.Name, "--") {
			return apierrors.NewBadRequest(fmt.Sprintf("the project name must not contain two consecutive hyphens (project: %s)", project.Name))
		}
		// We don't want new Shoots to be created in Projects which were already marked for deletion.
		if project.DeletionTimestamp != nil {
			return admission.NewForbidden(a, fmt.Errorf("cannot create shoot '%s' in project '%s' already marked for deletion", shoot.Name, project.Name))
		}
	}

	mustCheckIfTaintsTolerated := a.GetOperation() == admission.Create || (a.GetOperation() == admission.Update && !apiequality.Semantic.DeepEqual(shoot.Spec.SeedName, oldShoot.Spec.SeedName))
	if mustCheckIfTaintsTolerated && seed != nil && !helper.TaintsAreTolerated(seed.Spec.Taints, shoot.Spec.Tolerations) {
		return admission.NewForbidden(a, fmt.Errorf("forbidden to use a seeds whose taints are not tolerated by the shoot"))
	}

	// We don't allow shoot to be created on the seed which is already marked to be deleted.
	if seed != nil && seed.DeletionTimestamp != nil && a.GetOperation() == admission.Create {
		return admission.NewForbidden(a, fmt.Errorf("cannot create shoot '%s' on seed '%s' already marked for deletion", shoot.Name, seed.Name))
	}

	if oldShoot.Spec.SeedName != nil && !apiequality.Semantic.DeepEqual(shoot.Spec.SeedName, oldShoot.Spec.SeedName) &&
		seed != nil && seed.Spec.Backup == nil {
		return admission.NewForbidden(a, fmt.Errorf("cannot change seed name, because seed backup is not configured, for shoot %q", shoot.Name))
	}

	if shoot.Spec.Provider.Type != cloudProfile.Spec.Type {
		return apierrors.NewBadRequest(fmt.Sprintf("cloud provider in shoot (%s) is not equal to cloud provider in profile (%s)", shoot.Spec.Provider.Type, cloudProfile.Spec.Type))
	}

	if err := v.validateShootedSeed(a, shoot, oldShoot); err != nil {
		return err
	}

	var (
		validationContext = &validationContext{
			cloudProfile: cloudProfile,
			seed:         seed,
			shoot:        shoot,
			oldShoot:     oldShoot,
		}
		allErrs field.ErrorList
	)

	if seed != nil && seed.DeletionTimestamp != nil {
		newMeta := shoot.ObjectMeta
		oldMeta := *oldShoot.ObjectMeta.DeepCopy()

		// disallow any changes to the annotations of a shoot that references a seed which is already marked for deletion
		// except changes to the deletion confirmation annotation
		if !reflect.DeepEqual(newMeta.Annotations, oldMeta.Annotations) {
			confimationAnnotations := []string{common.ConfirmationDeletion, common.ConfirmationDeletionDeprecated}
			for _, annotation := range confimationAnnotations {
				newConfirmation, newHasConfirmation := newMeta.Annotations[annotation]

				// copy the new confirmation value to the old annotations to see if
				// anything else was changed other than the confirmation annotation
				if newHasConfirmation {
					if oldMeta.Annotations == nil {
						oldMeta.Annotations = make(map[string]string)
					}
					oldMeta.Annotations[annotation] = newConfirmation
				}
			}

			if !reflect.DeepEqual(newMeta.Annotations, oldMeta.Annotations) {
				return admission.NewForbidden(a, fmt.Errorf("cannot update annotations of shoot '%s' on seed '%s' already marked for deletion: only the '%s' annotation can be changed", shoot.Name, seed.Name, common.ConfirmationDeletion))
			}
		}

		if !reflect.DeepEqual(shoot.Spec, oldShoot.Spec) {
			return admission.NewForbidden(a, fmt.Errorf("cannot update spec of shoot '%s' on seed '%s' already marked for deletion", shoot.Name, seed.Name))
		}
	}

	// Allow removal of `gardener` finalizer only if the Shoot deletion has completed successfully
	if len(shoot.Status.TechnicalID) > 0 && shoot.Status.LastOperation != nil {
		oldFinalizers := sets.NewString(oldShoot.Finalizers...)
		newFinalizers := sets.NewString(shoot.Finalizers...)

		if oldFinalizers.Has(core.GardenerName) && !newFinalizers.Has(core.GardenerName) {
			lastOperation := shoot.Status.LastOperation
			deletionSucceeded := lastOperation.Type == core.LastOperationTypeDelete && lastOperation.State == core.LastOperationStateSucceeded && lastOperation.Progress == 100

			if !deletionSucceeded {
				return admission.NewForbidden(a, fmt.Errorf("finalizer %q cannot be removed because shoot deletion has not completed successfully yet", core.GardenerName))
			}
		}
	}

	// Prevent Shoots from getting hibernated in case they have problematic webhooks.
	// Otherwise, we can never wake up this shoot cluster again.
	oldIsHibernated := oldShoot.Spec.Hibernation != nil && oldShoot.Spec.Hibernation.Enabled != nil && *oldShoot.Spec.Hibernation.Enabled
	newIsHibernated := shoot.Spec.Hibernation != nil && shoot.Spec.Hibernation.Enabled != nil && *shoot.Spec.Hibernation.Enabled

	if !oldIsHibernated && newIsHibernated {
		if hibernationConstraint := helper.GetCondition(shoot.Status.Constraints, core.ShootHibernationPossible); hibernationConstraint != nil {
			if hibernationConstraint.Status != core.ConditionTrue {
				return admission.NewForbidden(a, fmt.Errorf(hibernationConstraint.Message))
			}
		}
	}

	if seed != nil {
		if shoot.Spec.Networking.Pods == nil && seed.Spec.Networks.ShootDefaults != nil {
			shoot.Spec.Networking.Pods = seed.Spec.Networks.ShootDefaults.Pods
		}
		if shoot.Spec.Networking.Services == nil && seed.Spec.Networks.ShootDefaults != nil {
			shoot.Spec.Networking.Services = seed.Spec.Networks.ShootDefaults.Services
		}
	}

	if !reflect.DeepEqual(oldShoot.Spec.Provider.InfrastructureConfig, shoot.Spec.Provider.InfrastructureConfig) {
		if shoot.ObjectMeta.Annotations == nil {
			shoot.ObjectMeta.Annotations = make(map[string]string)
		}
		controllerutils.AddTasks(shoot.ObjectMeta.Annotations, common.ShootTaskDeployInfrastructure)
	}

	if shoot.Spec.Maintenance != nil && utils.IsTrue(shoot.Spec.Maintenance.ConfineSpecUpdateRollout) &&
		!apiequality.Semantic.DeepEqual(oldShoot.Spec, shoot.Spec) &&
		shoot.Status.LastOperation != nil && shoot.Status.LastOperation.State == core.LastOperationStateFailed {
		metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, common.FailedShootNeedsRetryOperation, "true")
	}

	if shoot.DeletionTimestamp == nil {
		for idx, worker := range shoot.Spec.Provider.Workers {
			image, err := ensureMachineImage(oldShoot.Spec.Provider.Workers, worker, cloudProfile.Spec.MachineImages)
			if err != nil {
				return err
			}
			shoot.Spec.Provider.Workers[idx].Machine.Image = image
		}
	}

	if seed != nil {
		if shoot.Spec.Networking.Pods == nil {
			if seed.Spec.Networks.ShootDefaults != nil {
				shoot.Spec.Networking.Pods = seed.Spec.Networks.ShootDefaults.Pods
			} else {
				allErrs = append(allErrs, field.Required(field.NewPath("spec", "networking", "pods"), "pods is required"))
			}
		}

		if shoot.Spec.Networking.Services == nil {
			if seed.Spec.Networks.ShootDefaults != nil {
				shoot.Spec.Networking.Services = seed.Spec.Networks.ShootDefaults.Services
			} else {
				allErrs = append(allErrs, field.Required(field.NewPath("spec", "networking", "services"), "services is required"))
			}
		}
	}

	allErrs = append(allErrs, validateProvider(validationContext)...)

	dnsErrors, err := validateDNSDomainUniqueness(v.shootLister, shoot.Name, shoot.Spec.DNS)
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
	seed         *core.Seed
	shoot        *core.Shoot
	oldShoot     *core.Shoot
}

func validateProvider(c *validationContext) field.ErrorList {
	var (
		allErrs = field.ErrorList{}
		path    = field.NewPath("spec", "provider")
	)

	if c.seed != nil {
		allErrs = append(allErrs, cidrvalidation.ValidateNetworkDisjointedness(
			path.Child("networks"),
			c.shoot.Spec.Networking.Nodes,
			c.shoot.Spec.Networking.Pods,
			c.shoot.Spec.Networking.Services,
			c.seed.Spec.Networks.Nodes,
			c.seed.Spec.Networks.Pods,
			c.seed.Spec.Networks.Services,
		)...)
	}

	ok, isDefaulted, validKubernetesVersions, versionDefault := validateKubernetesVersionConstraints(c.cloudProfile.Spec.Kubernetes.Versions, c.shoot.Spec.Kubernetes.Version, c.oldShoot.Spec.Kubernetes.Version)
	if !ok {
		err := field.NotSupported(field.NewPath("spec", "kubernetes", "version"), c.shoot.Spec.Kubernetes.Version, validKubernetesVersions)
		if isDefaulted {
			err.Detail = fmt.Sprintf("unable to default version - couldn't find a suitable patch version for %s. Suitable patch versions have a non-expired expiration date and are no 'preview' versions. 'Preview'-classified versions have to be selected explicitly -  %s", c.shoot.Spec.Kubernetes.Version, err.Detail)
		}
		allErrs = append(allErrs, err)
	} else if versionDefault != nil {
		c.shoot.Spec.Kubernetes.Version = versionDefault.String()
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

		allErrs = append(allErrs, validateZones(c.cloudProfile.Spec.Regions, c.shoot.Spec.Region, c.oldShoot.Spec.Region, worker, oldWorker, idxPath)...)
	}

	return allErrs
}

func validateDNSDomainUniqueness(shootLister corelisters.ShootLister, name string, dns *core.DNS) (field.ErrorList, error) {
	var (
		allErrs = field.ErrorList{}
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
		if shoot.Name == name {
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

			// defaulting on patch level: version has to have the the same major and minor kubernetes version
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
	if apiequality.Semantic.DeepEqual(worker.CRI, oldWorker.CRI) {
		return nil
	}

	if worker.CRI == nil || worker.Machine.Image == nil {
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

func (v ValidateShoot) validateShootedSeed(a admission.Attributes, shoot, oldShoot *core.Shoot) error {
	if shoot.Namespace != constants.GardenNamespace {
		return nil
	}

	oldVal, oldOk := constants.GetShootUseAsSeedAnnotation(oldShoot.Annotations)
	if !oldOk || len(oldVal) == 0 {
		return nil
	}

	val, ok := constants.GetShootUseAsSeedAnnotation(shoot.Annotations)
	if ok && len(val) != 0 {
		return nil
	}

	if _, err := v.seedLister.Get(shoot.Name); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return apierrors.NewInternalError(fmt.Errorf("could not get seed '%s' to verify that annotation '%s' can be removed: %v", shoot.Name, constants.AnnotationShootUseAsSeed, err))
	}

	shoots, err := v.shootLister.List(labels.Everything())
	if err != nil {
		return apierrors.NewInternalError(fmt.Errorf("could not list shoots to verify that annotation '%s' can be removed: %v", constants.AnnotationShootUseAsSeed, err))
	}

	backupbuckets, err := v.backupBucketLister.List(labels.Everything())
	if err != nil {
		return apierrors.NewInternalError(fmt.Errorf("could not list backupbuckets to verify that annotation '%s' can be removed: %v", constants.AnnotationShootUseAsSeed, err))
	}

	if admissionutils.IsSeedUsedByShoot(shoot.Name, shoots) {
		return admission.NewForbidden(a, fmt.Errorf("cannot delete seed '%s' which is still used by shoot(s)", shoot.Name))
	}

	if admissionutils.IsSeedUsedByBackupBucket(shoot.Name, backupbuckets) {
		return admission.NewForbidden(a, fmt.Errorf("cannot delete seed '%s' which is still used by backupbucket(s)", shoot.Name))
	}

	return nil
}
