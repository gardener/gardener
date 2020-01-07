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
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/apis/garden"
	"github.com/gardener/gardener/pkg/apis/garden/helper"
	admissioninitializer "github.com/gardener/gardener/pkg/apiserver/admission/initializer"
	informers "github.com/gardener/gardener/pkg/client/garden/informers/internalversion"
	listers "github.com/gardener/gardener/pkg/client/garden/listers/garden/internalversion"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/operation/common"
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
	cloudProfileLister listers.CloudProfileLister
	seedLister         listers.SeedLister
	shootLister        listers.ShootLister
	projectLister      listers.ProjectLister
	readyFunc          admission.ReadyFunc
}

var (
	_ = admissioninitializer.WantsInternalGardenInformerFactory(&ValidateShoot{})

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

// SetInternalGardenInformerFactory gets Lister from SharedInformerFactory.
func (v *ValidateShoot) SetInternalGardenInformerFactory(f informers.SharedInformerFactory) {
	seedInformer := f.Garden().InternalVersion().Seeds()
	v.seedLister = seedInformer.Lister()

	shootInformer := f.Garden().InternalVersion().Shoots()
	v.shootLister = shootInformer.Lister()

	cloudProfileInformer := f.Garden().InternalVersion().CloudProfiles()
	v.cloudProfileLister = cloudProfileInformer.Lister()

	projectInformer := f.Garden().InternalVersion().Projects()
	v.projectLister = projectInformer.Lister()

	readyFuncs = append(readyFuncs, seedInformer.Informer().HasSynced, shootInformer.Informer().HasSynced, cloudProfileInformer.Informer().HasSynced, projectInformer.Informer().HasSynced)
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
	if a.GetKind().GroupKind() != garden.Kind("Shoot") && a.GetKind().GroupKind() != core.Kind("Shoot") {
		return nil
	}

	// Ignore updates to shoot status or other subresources
	if a.GetSubresource() != "" {
		return nil
	}

	// Ignore updates if shoot spec hasn't changed
	if a.GetOperation() == admission.Update {
		newShoot, ok := a.GetObject().(*garden.Shoot)
		if !ok {
			return apierrors.NewInternalError(errors.New("could not convert resource into Shoot object"))
		}
		oldShoot, ok := a.GetOldObject().(*garden.Shoot)
		if !ok {
			return apierrors.NewInternalError(errors.New("could not convert old resource into Shoot object"))
		}

		// do not ignore metadata updates to detect and prevent removal of the gardener finalizer or unwanted changes to annotations
		if reflect.DeepEqual(newShoot.Spec, oldShoot.Spec) && reflect.DeepEqual(newShoot.ObjectMeta, oldShoot.ObjectMeta) {
			return nil
		}

		if newShoot.Spec.Provider.Type != oldShoot.Spec.Provider.Type {
			return apierrors.NewBadRequest("shoot provider type was changed which is not allowed")
		}
	}

	shoot, ok := a.GetObject().(*garden.Shoot)
	if !ok {
		return apierrors.NewInternalError(errors.New("could not convert resource into Shoot object"))
	}

	cloudProfile, err := v.cloudProfileLister.Get(shoot.Spec.CloudProfileName)
	if err != nil {
		return apierrors.NewBadRequest(fmt.Sprintf("could not find referenced cloud profile: %+v", err.Error()))
	}
	var seed *garden.Seed
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
		if len(project.Name+shoot.Name) > lengthLimit {
			return apierrors.NewBadRequest(fmt.Sprintf("the length of the shoot name and the project name must not exceed %d characters (project: %s; shoot: %s)", lengthLimit, project.Name, shoot.Name))
		}
		if strings.Contains(project.Name, "--") {
			return apierrors.NewBadRequest(fmt.Sprintf("the project name must not contain two consecutive hyphens (project: %s)", project.Name))
		}
		// We don't want new Shoots to be created in Projects which were already marked for deletion.
		if project.DeletionTimestamp != nil {
			return admission.NewForbidden(a, fmt.Errorf("cannot create shoot '%s' in project '%s' already marked for deletion", shoot.Name, project.Name))
		}
	}

	// Check whether seed is protected or not. In case it is protected then we only allow Shoot resources to reference it which are part of the Garden namespace.
	if shoot.Namespace != v1beta1constants.GardenNamespace && seed != nil && helper.TaintsHave(seed.Spec.Taints, garden.SeedTaintProtected) {
		return admission.NewForbidden(a, fmt.Errorf("forbidden to use a protected seed"))
	}

	// We don't allow shoot to be created on the seed which is already marked to be deleted.
	if seed != nil && seed.DeletionTimestamp != nil && a.GetOperation() == admission.Create {
		return admission.NewForbidden(a, fmt.Errorf("cannot create shoot '%s' on seed '%s' already marked for deletion", shoot.Name, seed.Name))
	}

	if shoot.Spec.Provider.Type != cloudProfile.Spec.Type {
		return apierrors.NewBadRequest(fmt.Sprintf("cloud provider in shoot (%s) is not equal to cloud provider in profile (%s)", shoot.Spec.Provider.Type, cloudProfile.Spec.Type))
	}

	// We only want to validate fields in the Shoot against the CloudProfile/Seed constraints which have changed.
	// On CREATE operations we just use an empty Shoot object, forcing the validator functions to always validate.
	// On UPDATE operations we fetch the current Shoot object.
	var oldShoot *garden.Shoot
	if a.GetOperation() == admission.Create {
		oldShoot = &garden.Shoot{
			Spec: garden.ShootSpec{
				Cloud: garden.Cloud{
					AWS: &garden.AWSCloud{
						MachineImage: &garden.ShootMachineImage{},
					},
					Azure: &garden.AzureCloud{
						MachineImage: &garden.ShootMachineImage{},
					},
					GCP: &garden.GCPCloud{
						MachineImage: &garden.ShootMachineImage{},
					},
					Packet: &garden.PacketCloud{
						MachineImage: &garden.ShootMachineImage{},
					},
					OpenStack: &garden.OpenStackCloud{
						MachineImage: &garden.ShootMachineImage{},
					},
					Alicloud: &garden.Alicloud{
						MachineImage: &garden.ShootMachineImage{},
					},
				},
			},
		}
	} else if a.GetOperation() == admission.Update {
		old, ok := a.GetOldObject().(*garden.Shoot)
		if !ok {
			return apierrors.NewInternalError(errors.New("could not convert old resource into Shoot object"))
		}
		oldShoot = old
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
			newConfirmation, newHasConfirmation := newMeta.Annotations[common.ConfirmationDeletion]

			// copy the new confirmation value to the old annotations to see if
			// anything else was changed other than the confirmation annotation
			if newHasConfirmation {
				if oldMeta.Annotations == nil {
					oldMeta.Annotations = make(map[string]string)
				}
				oldMeta.Annotations[common.ConfirmationDeletion] = newConfirmation
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

		if oldFinalizers.Has(garden.GardenerName) && !newFinalizers.Has(garden.GardenerName) {
			lastOperation := shoot.Status.LastOperation
			deletionSucceeded := lastOperation.Type == garden.LastOperationTypeDelete && lastOperation.State == garden.LastOperationStateSucceeded && lastOperation.Progress == 100

			if !deletionSucceeded {
				return admission.NewForbidden(a, fmt.Errorf("finalizer \"%s\" cannot be removed because shoot deletion has not completed successfully yet", garden.GardenerName))
			}
		}
	}

	// Prevent Shoots from getting hibernated in case they have problematic webhooks.
	// Otherwise, we can never wake up this shoot cluster again.
	oldIsHibernated := oldShoot.Spec.Hibernation != nil && oldShoot.Spec.Hibernation.Enabled != nil && *oldShoot.Spec.Hibernation.Enabled
	newIsHibernated := shoot.Spec.Hibernation != nil && shoot.Spec.Hibernation.Enabled != nil && *shoot.Spec.Hibernation.Enabled

	if !oldIsHibernated && newIsHibernated {
		if hibernationConstraint := helper.GetCondition(shoot.Status.Constraints, garden.ShootHibernationPossible); hibernationConstraint != nil {
			if hibernationConstraint.Status != garden.ConditionTrue {
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

	image, err := getDefaultMachineImage(cloudProfile.Spec.MachineImages)
	if err != nil {
		return apierrors.NewBadRequest(err.Error())
	}

	// General approach with machine image defaulting in this code: Try to keep the machine image
	// from the old shoot object to not accidentally update it to the default machine image.
	// This should only happen in the maintenance time window of shoots and is performed by the
	// shoot maintenance controller.
	switch shoot.Spec.Provider.Type {
	case "aws":
		if shoot.Spec.Cloud.AWS.MachineImage == nil {
			shoot.Spec.Cloud.AWS.MachineImage = image
		}

		for idx, worker := range shoot.Spec.Cloud.AWS.Workers {
			if shoot.DeletionTimestamp == nil && worker.Machine.Image == nil {
				shoot.Spec.Cloud.AWS.Workers[idx].Machine.Image = getOldWorkerMachineImageOrDefault(oldShoot.Spec.Cloud.AWS.Workers, worker.Name, shoot.Spec.Cloud.AWS.MachineImage)
			}
		}

		if seed != nil {
			if shoot.Spec.Cloud.AWS.Networks.Pods == nil {
				if seed.Spec.Networks.ShootDefaults != nil {
					shoot.Spec.Cloud.AWS.Networks.Pods = seed.Spec.Networks.ShootDefaults.Pods
				} else {
					allErrs = append(allErrs, field.Required(field.NewPath("spec", "cloud", "aws", "networks", "pods"), "pods is required"))
				}
			}

			if shoot.Spec.Cloud.AWS.Networks.Services == nil {
				if seed.Spec.Networks.ShootDefaults != nil {
					shoot.Spec.Cloud.AWS.Networks.Services = seed.Spec.Networks.ShootDefaults.Services
				} else {
					allErrs = append(allErrs, field.Required(field.NewPath("spec", "cloud", "aws", "networks", "services"), "services is required"))
				}
			}
		}

		allErrs = validateAWS(validationContext)

	case "azure":
		if shoot.Spec.Cloud.Azure.MachineImage == nil {
			shoot.Spec.Cloud.Azure.MachineImage = image
		}

		for idx, worker := range shoot.Spec.Cloud.Azure.Workers {
			if shoot.DeletionTimestamp == nil && worker.Machine.Image == nil {
				shoot.Spec.Cloud.Azure.Workers[idx].Machine.Image = getOldWorkerMachineImageOrDefault(oldShoot.Spec.Cloud.Azure.Workers, worker.Name, shoot.Spec.Cloud.Azure.MachineImage)
			}
		}

		if seed != nil {
			if shoot.Spec.Cloud.Azure.Networks.Pods == nil {
				if seed.Spec.Networks.ShootDefaults != nil {
					shoot.Spec.Cloud.Azure.Networks.Pods = seed.Spec.Networks.ShootDefaults.Pods
				} else {
					allErrs = append(allErrs, field.Required(field.NewPath("spec", "cloud", "azure", "networks", "pods"), "pods is required"))
				}
			}

			if shoot.Spec.Cloud.Azure.Networks.Services == nil {
				if seed.Spec.Networks.ShootDefaults != nil {
					shoot.Spec.Cloud.Azure.Networks.Services = seed.Spec.Networks.ShootDefaults.Services
				} else {
					allErrs = append(allErrs, field.Required(field.NewPath("spec", "cloud", "azure", "networks", "services"), "services is required"))
				}
			}
		}

		allErrs = validateAzure(validationContext)

	case "gcp":
		if shoot.Spec.Cloud.GCP.MachineImage == nil {
			shoot.Spec.Cloud.GCP.MachineImage = image
		}

		for idx, worker := range shoot.Spec.Cloud.GCP.Workers {
			if shoot.DeletionTimestamp == nil && worker.Machine.Image == nil {
				shoot.Spec.Cloud.GCP.Workers[idx].Machine.Image = getOldWorkerMachineImageOrDefault(oldShoot.Spec.Cloud.GCP.Workers, worker.Name, shoot.Spec.Cloud.GCP.MachineImage)
			}
		}

		if seed != nil {
			if shoot.Spec.Cloud.GCP.Networks.Pods == nil {
				if seed.Spec.Networks.ShootDefaults != nil {
					shoot.Spec.Cloud.GCP.Networks.Pods = seed.Spec.Networks.ShootDefaults.Pods
				} else {
					allErrs = append(allErrs, field.Required(field.NewPath("spec", "cloud", "gcp", "networks", "pods"), "pods is required"))
				}
			}

			if shoot.Spec.Cloud.GCP.Networks.Services == nil {
				if seed.Spec.Networks.ShootDefaults != nil {
					shoot.Spec.Cloud.GCP.Networks.Services = seed.Spec.Networks.ShootDefaults.Services
				} else {
					allErrs = append(allErrs, field.Required(field.NewPath("spec", "cloud", "gcp", "networks", "services"), "services is required"))
				}
			}
		}

		allErrs = validateGCP(validationContext)

	case "openstack":
		if shoot.Spec.Cloud.OpenStack.MachineImage == nil {
			shoot.Spec.Cloud.OpenStack.MachineImage = image
		}

		for idx, worker := range shoot.Spec.Cloud.OpenStack.Workers {
			if shoot.DeletionTimestamp == nil && worker.Machine.Image == nil {
				shoot.Spec.Cloud.OpenStack.Workers[idx].Machine.Image = getOldWorkerMachineImageOrDefault(oldShoot.Spec.Cloud.OpenStack.Workers, worker.Name, shoot.Spec.Cloud.OpenStack.MachineImage)
			}
		}

		if seed != nil {
			if shoot.Spec.Cloud.OpenStack.Networks.Pods == nil {
				if seed.Spec.Networks.ShootDefaults != nil {
					shoot.Spec.Cloud.OpenStack.Networks.Pods = seed.Spec.Networks.ShootDefaults.Pods
				} else {
					allErrs = append(allErrs, field.Required(field.NewPath("spec", "cloud", "openstack", "networks", "pods"), "pods is required"))
				}
			}

			if shoot.Spec.Cloud.OpenStack.Networks.Services == nil {
				if seed.Spec.Networks.ShootDefaults != nil {
					shoot.Spec.Cloud.OpenStack.Networks.Services = seed.Spec.Networks.ShootDefaults.Services
				} else {
					allErrs = append(allErrs, field.Required(field.NewPath("spec", "cloud", "openstack", "networks", "services"), "services is required"))
				}
			}
		}

		allErrs = validateOpenStack(validationContext)

	case "packet":
		if shoot.Spec.Cloud.Packet.MachineImage == nil {
			shoot.Spec.Cloud.Packet.MachineImage = image
		}

		for idx, worker := range shoot.Spec.Cloud.Packet.Workers {
			if shoot.DeletionTimestamp == nil && worker.Machine.Image == nil {
				shoot.Spec.Cloud.Packet.Workers[idx].Machine.Image = getOldWorkerMachineImageOrDefault(oldShoot.Spec.Cloud.Packet.Workers, worker.Name, shoot.Spec.Cloud.Packet.MachineImage)
			}
		}

		if seed != nil {
			if shoot.Spec.Cloud.Packet.Networks.Pods == nil {
				if seed.Spec.Networks.ShootDefaults != nil {
					shoot.Spec.Cloud.Packet.Networks.Pods = seed.Spec.Networks.ShootDefaults.Pods
				} else {
					allErrs = append(allErrs, field.Required(field.NewPath("spec", "cloud", "packet", "networks", "pods"), "pods is required"))
				}
			}

			if shoot.Spec.Cloud.Packet.Networks.Services == nil {
				if seed.Spec.Networks.ShootDefaults != nil {
					shoot.Spec.Cloud.Packet.Networks.Services = seed.Spec.Networks.ShootDefaults.Services
				} else {
					allErrs = append(allErrs, field.Required(field.NewPath("spec", "cloud", "packet", "networks", "services"), "services is required"))
				}
			}
		}

		allErrs = validatePacket(validationContext)

	case "alicloud":
		if shoot.Spec.Cloud.Alicloud.MachineImage == nil {
			shoot.Spec.Cloud.Alicloud.MachineImage = image
		}

		for idx, worker := range shoot.Spec.Cloud.Alicloud.Workers {
			if shoot.DeletionTimestamp == nil && worker.Machine.Image == nil {
				shoot.Spec.Cloud.Alicloud.Workers[idx].Machine.Image = getOldWorkerMachineImageOrDefault(oldShoot.Spec.Cloud.Alicloud.Workers, worker.Name, shoot.Spec.Cloud.Alicloud.MachineImage)
			}
		}

		if seed != nil {
			if shoot.Spec.Cloud.Alicloud.Networks.Pods == nil {
				if seed.Spec.Networks.ShootDefaults != nil {
					shoot.Spec.Cloud.Alicloud.Networks.Pods = seed.Spec.Networks.ShootDefaults.Pods
				} else {
					allErrs = append(allErrs, field.Required(field.NewPath("spec", "cloud", "alicloud", "networks", "pods"), "pods is required"))
				}
			}

			if shoot.Spec.Cloud.Alicloud.Networks.Services == nil {
				if seed.Spec.Networks.ShootDefaults != nil {
					shoot.Spec.Cloud.Alicloud.Networks.Services = seed.Spec.Networks.ShootDefaults.Services
				} else {
					allErrs = append(allErrs, field.Required(field.NewPath("spec", "cloud", "alicloud", "networks", "services"), "services is required"))
				}
			}
		}

		allErrs = validateAlicloud(validationContext)
	}

	if !reflect.DeepEqual(oldShoot.Spec.Provider.InfrastructureConfig, shoot.Spec.Provider.InfrastructureConfig) {
		if shoot.ObjectMeta.Annotations == nil {
			shoot.ObjectMeta.Annotations = make(map[string]string)
		}
		controllerutils.AddTasks(shoot.ObjectMeta.Annotations, common.ShootTaskDeployInfrastructure)
	}

	for idx, worker := range shoot.Spec.Provider.Workers {
		if shoot.DeletionTimestamp == nil && worker.Machine.Image == nil {
			shoot.Spec.Provider.Workers[idx].Machine.Image = getOldWorkerMachineImageOrDefault(oldShoot.Spec.Provider.Workers, worker.Name, image)
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
	cloudProfile *garden.CloudProfile
	seed         *garden.Seed
	shoot        *garden.Shoot
	oldShoot     *garden.Shoot
}

func validateAWS(c *validationContext) field.ErrorList {
	var (
		allErrs = field.ErrorList{}
		path    = field.NewPath("spec", "cloud", "aws")
	)

	if c.seed != nil {
		allErrs = append(allErrs, cidrvalidation.ValidateNetworkDisjointedness(
			path.Child("networks"),
			c.shoot.Spec.Cloud.AWS.Networks.K8SNetworks.Nodes,
			c.shoot.Spec.Cloud.AWS.Networks.K8SNetworks.Pods,
			c.shoot.Spec.Cloud.AWS.Networks.K8SNetworks.Services,
			c.seed.Spec.Networks.Nodes,
			c.seed.Spec.Networks.Pods,
			c.seed.Spec.Networks.Services,
		)...)
	}
	ok, validKubernetesVersions, versionDefault := validateKubernetesVersionConstraints(c.cloudProfile.Spec.Kubernetes.Versions, c.shoot.Spec.Kubernetes.Version, c.oldShoot.Spec.Kubernetes.Version)
	if !ok {
		allErrs = append(allErrs, field.NotSupported(field.NewPath("spec", "kubernetes", "version"), c.shoot.Spec.Kubernetes.Version, validKubernetesVersions))
	} else if versionDefault != nil {
		c.shoot.Spec.Kubernetes.Version = versionDefault.String()
	}
	if ok, validMachineImages := validateMachineImagesConstraints(c.cloudProfile.Spec.MachineImages, c.shoot.Spec.Cloud.AWS.MachineImage, c.oldShoot.Spec.Cloud.AWS.MachineImage); !ok {
		allErrs = append(allErrs, field.NotSupported(path.Child("machine", "image"), *c.shoot.Spec.Cloud.AWS.MachineImage, validMachineImages))
	}

	for i, worker := range c.shoot.Spec.Cloud.AWS.Workers {
		var oldWorker = garden.Worker{}
		for _, ow := range c.oldShoot.Spec.Cloud.AWS.Workers {
			if ow.Name == worker.Name {
				oldWorker = ow
				break
			}
		}

		idxPath := path.Child("workers").Index(i)
		if ok, validMachineTypes := validateMachineTypes(c.cloudProfile.Spec.MachineTypes, worker.Machine.Type, oldWorker.Machine.Type, c.cloudProfile.Spec.Regions, c.shoot.Spec.Region, c.shoot.Spec.Cloud.AWS.Zones); !ok {
			allErrs = append(allErrs, field.NotSupported(idxPath.Child("machine", "type"), worker.Machine.Type, validMachineTypes))
		}
		if ok, validMachineImages := validateMachineImagesConstraints(c.cloudProfile.Spec.MachineImages, worker.Machine.Image, oldWorker.Machine.Image); !ok {
			allErrs = append(allErrs, field.NotSupported(idxPath.Child("machine", "image"), worker.Machine.Image, validMachineImages))
		}
		if ok, validVolumeTypes := validateVolumeTypes(c.cloudProfile.Spec.VolumeTypes, worker.Volume, oldWorker.Volume, c.cloudProfile.Spec.Regions, c.shoot.Spec.Region, c.shoot.Spec.Cloud.AWS.Zones); !ok {
			allErrs = append(allErrs, field.NotSupported(idxPath.Child("volume", "type"), worker.Volume, validVolumeTypes))
		}
	}

	for i, zone := range c.shoot.Spec.Cloud.AWS.Zones {
		idxPath := path.Child("zones").Index(i)
		if ok, validZones := validateZones(c.cloudProfile.Spec.Regions, c.shoot.Spec.Region, c.oldShoot.Spec.Region, zone); !ok {
			if len(validZones) == 0 {
				allErrs = append(allErrs, field.Invalid(idxPath, c.shoot.Spec.Region, "this region is not allowed"))
			} else {
				allErrs = append(allErrs, field.NotSupported(idxPath, zone, validZones))
			}
		}
	}

	return allErrs
}

func validateAzure(c *validationContext) field.ErrorList {
	var (
		allErrs = field.ErrorList{}
		path    = field.NewPath("spec", "cloud", "azure")
	)
	if c.seed != nil {
		allErrs = append(allErrs, cidrvalidation.ValidateNetworkDisjointedness(
			path.Child("networks"),
			c.shoot.Spec.Cloud.Azure.Networks.K8SNetworks.Nodes,
			c.shoot.Spec.Cloud.Azure.Networks.K8SNetworks.Pods,
			c.shoot.Spec.Cloud.Azure.Networks.K8SNetworks.Services,
			c.seed.Spec.Networks.Nodes,
			c.seed.Spec.Networks.Pods,
			c.seed.Spec.Networks.Services,
		)...)
	}
	ok, validKubernetesVersions, versionDefault := validateKubernetesVersionConstraints(c.cloudProfile.Spec.Kubernetes.Versions, c.shoot.Spec.Kubernetes.Version, c.oldShoot.Spec.Kubernetes.Version)
	if !ok {
		allErrs = append(allErrs, field.NotSupported(field.NewPath("spec", "kubernetes", "version"), c.shoot.Spec.Kubernetes.Version, validKubernetesVersions))
	} else if versionDefault != nil {
		c.shoot.Spec.Kubernetes.Version = versionDefault.String()
	}
	if ok, validMachineImages := validateMachineImagesConstraints(c.cloudProfile.Spec.MachineImages, c.shoot.Spec.Cloud.Azure.MachineImage, c.oldShoot.Spec.Cloud.Azure.MachineImage); !ok {
		allErrs = append(allErrs, field.NotSupported(path.Child("machine", "image"), *c.shoot.Spec.Cloud.Azure.MachineImage, validMachineImages))
	}

	for i, worker := range c.shoot.Spec.Cloud.Azure.Workers {
		var oldWorker = garden.Worker{}
		for _, ow := range c.oldShoot.Spec.Cloud.Azure.Workers {
			if ow.Name == worker.Name {
				oldWorker = ow
				break
			}
		}

		idxPath := path.Child("workers").Index(i)
		if ok, validMachineTypes := validateMachineTypes(c.cloudProfile.Spec.MachineTypes, worker.Machine.Type, oldWorker.Machine.Type, c.cloudProfile.Spec.Regions, c.shoot.Spec.Region, nil); !ok {
			allErrs = append(allErrs, field.NotSupported(idxPath.Child("machine", "type"), worker.Machine.Type, validMachineTypes))
		}
		if ok, validMachineImages := validateMachineImagesConstraints(c.cloudProfile.Spec.MachineImages, worker.Machine.Image, oldWorker.Machine.Image); !ok {
			allErrs = append(allErrs, field.NotSupported(idxPath.Child("machine", "image"), worker.Machine.Image, validMachineImages))
		}
		if ok, validVolumeTypes := validateVolumeTypes(c.cloudProfile.Spec.VolumeTypes, worker.Volume, oldWorker.Volume, c.cloudProfile.Spec.Regions, c.shoot.Spec.Region, nil); !ok {
			allErrs = append(allErrs, field.NotSupported(idxPath.Child("volume", "type"), worker.Volume, validVolumeTypes))
		}
	}

	if ok := validateAzureDomainCount(c.cloudProfile.Spec.Azure.CountFaultDomains, c.shoot.Spec.Region); !ok {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "cloud", "region"), c.shoot.Spec.Region, "no fault domain count known for this region"))
	}
	if ok := validateAzureDomainCount(c.cloudProfile.Spec.Azure.CountUpdateDomains, c.shoot.Spec.Region); !ok {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "cloud", "region"), c.shoot.Spec.Region, "no update domain count known for this region"))
	}

	return allErrs
}

func validateGCP(c *validationContext) field.ErrorList {
	var (
		allErrs = field.ErrorList{}
		path    = field.NewPath("spec", "cloud", "gcp")
	)

	if c.seed != nil {
		allErrs = append(allErrs, cidrvalidation.ValidateNetworkDisjointedness(
			path.Child("networks"),
			c.shoot.Spec.Cloud.GCP.Networks.K8SNetworks.Nodes,
			c.shoot.Spec.Cloud.GCP.Networks.K8SNetworks.Pods,
			c.shoot.Spec.Cloud.GCP.Networks.K8SNetworks.Services,
			c.seed.Spec.Networks.Nodes,
			c.seed.Spec.Networks.Pods,
			c.seed.Spec.Networks.Services,
		)...)
	}
	ok, validKubernetesVersions, versionDefault := validateKubernetesVersionConstraints(c.cloudProfile.Spec.Kubernetes.Versions, c.shoot.Spec.Kubernetes.Version, c.oldShoot.Spec.Kubernetes.Version)
	if !ok {
		allErrs = append(allErrs, field.NotSupported(field.NewPath("spec", "kubernetes", "version"), c.shoot.Spec.Kubernetes.Version, validKubernetesVersions))
	} else if versionDefault != nil {
		c.shoot.Spec.Kubernetes.Version = versionDefault.String()
	}
	if ok, validMachineImages := validateMachineImagesConstraints(c.cloudProfile.Spec.MachineImages, c.shoot.Spec.Cloud.GCP.MachineImage, c.oldShoot.Spec.Cloud.GCP.MachineImage); !ok {
		allErrs = append(allErrs, field.NotSupported(path.Child("machine", "image"), *c.shoot.Spec.Cloud.GCP.MachineImage, validMachineImages))
	}

	for i, worker := range c.shoot.Spec.Cloud.GCP.Workers {
		var oldWorker = garden.Worker{}
		for _, ow := range c.oldShoot.Spec.Cloud.GCP.Workers {
			if ow.Name == worker.Name {
				oldWorker = ow
				break
			}
		}

		idxPath := path.Child("workers").Index(i)
		if ok, validMachineTypes := validateMachineTypes(c.cloudProfile.Spec.MachineTypes, worker.Machine.Type, oldWorker.Machine.Type, c.cloudProfile.Spec.Regions, c.shoot.Spec.Region, c.shoot.Spec.Cloud.GCP.Zones); !ok {
			allErrs = append(allErrs, field.NotSupported(idxPath.Child("machine", "type"), worker.Machine.Type, validMachineTypes))
		}
		if ok, validMachineImages := validateMachineImagesConstraints(c.cloudProfile.Spec.MachineImages, worker.Machine.Image, oldWorker.Machine.Image); !ok {
			allErrs = append(allErrs, field.NotSupported(idxPath.Child("machine", "image"), worker.Machine.Image, validMachineImages))
		}
		if ok, validVolumeTypes := validateVolumeTypes(c.cloudProfile.Spec.VolumeTypes, worker.Volume, oldWorker.Volume, c.cloudProfile.Spec.Regions, c.shoot.Spec.Region, c.shoot.Spec.Cloud.GCP.Zones); !ok {
			allErrs = append(allErrs, field.NotSupported(idxPath.Child("volume", "type"), worker.Volume, validVolumeTypes))
		}
	}

	for i, zone := range c.shoot.Spec.Cloud.GCP.Zones {
		idxPath := path.Child("zones").Index(i)
		if ok, validZones := validateZones(c.cloudProfile.Spec.Regions, c.shoot.Spec.Region, c.oldShoot.Spec.Region, zone); !ok {
			if len(validZones) == 0 {
				allErrs = append(allErrs, field.Invalid(idxPath, c.shoot.Spec.Region, "this region is not allowed"))
			} else {
				allErrs = append(allErrs, field.NotSupported(idxPath, zone, validZones))
			}
		}
	}

	return allErrs
}

func validatePacket(c *validationContext) field.ErrorList {
	var (
		allErrs = field.ErrorList{}
		path    = field.NewPath("spec", "cloud", "packet")
	)

	if c.seed != nil {
		allErrs = append(allErrs, cidrvalidation.ValidateNetworkDisjointedness(
			path.Child("networks"),
			c.shoot.Spec.Cloud.Packet.Networks.K8SNetworks.Nodes,
			c.shoot.Spec.Cloud.Packet.Networks.K8SNetworks.Pods,
			c.shoot.Spec.Cloud.Packet.Networks.K8SNetworks.Services,
			c.seed.Spec.Networks.Nodes,
			c.seed.Spec.Networks.Pods,
			c.seed.Spec.Networks.Services,
		)...)
	}
	ok, validKubernetesVersions, versionDefault := validateKubernetesVersionConstraints(c.cloudProfile.Spec.Kubernetes.Versions, c.shoot.Spec.Kubernetes.Version, c.oldShoot.Spec.Kubernetes.Version)
	if !ok {
		allErrs = append(allErrs, field.NotSupported(field.NewPath("spec", "kubernetes", "version"), c.shoot.Spec.Kubernetes.Version, validKubernetesVersions))
	} else if versionDefault != nil {
		c.shoot.Spec.Kubernetes.Version = versionDefault.String()
	}
	if ok, validMachineImages := validateMachineImagesConstraints(c.cloudProfile.Spec.MachineImages, c.shoot.Spec.Cloud.Packet.MachineImage, c.oldShoot.Spec.Cloud.Packet.MachineImage); !ok {
		allErrs = append(allErrs, field.NotSupported(path.Child("machine", "image"), *c.shoot.Spec.Cloud.Packet.MachineImage, validMachineImages))
	}

	for i, worker := range c.shoot.Spec.Cloud.Packet.Workers {
		var oldWorker = garden.Worker{}
		for _, ow := range c.oldShoot.Spec.Cloud.Packet.Workers {
			if ow.Name == worker.Name {
				oldWorker = ow
				break
			}
		}

		idxPath := path.Child("workers").Index(i)
		if ok, validMachineTypes := validateMachineTypes(c.cloudProfile.Spec.MachineTypes, worker.Machine.Type, oldWorker.Machine.Type, c.cloudProfile.Spec.Regions, c.shoot.Spec.Region, c.shoot.Spec.Cloud.Packet.Zones); !ok {
			allErrs = append(allErrs, field.NotSupported(idxPath.Child("machine", "type"), worker.Machine.Type, validMachineTypes))
		}
		if ok, validMachineImages := validateMachineImagesConstraints(c.cloudProfile.Spec.MachineImages, worker.Machine.Image, oldWorker.Machine.Image); !ok {
			allErrs = append(allErrs, field.NotSupported(idxPath.Child("machine", "image"), worker.Machine.Image, validMachineImages))
		}
		if ok, validVolumeTypes := validateVolumeTypes(c.cloudProfile.Spec.VolumeTypes, worker.Volume, oldWorker.Volume, c.cloudProfile.Spec.Regions, c.shoot.Spec.Region, c.shoot.Spec.Cloud.Packet.Zones); !ok {
			allErrs = append(allErrs, field.NotSupported(idxPath.Child("volume", "type"), worker.Volume, validVolumeTypes))
		}
	}

	for i, zone := range c.shoot.Spec.Cloud.Packet.Zones {
		idxPath := path.Child("zones").Index(i)
		if ok, validZones := validateZones(c.cloudProfile.Spec.Regions, c.shoot.Spec.Region, c.oldShoot.Spec.Region, zone); !ok {
			if len(validZones) == 0 {
				allErrs = append(allErrs, field.Invalid(idxPath, c.shoot.Spec.Region, "this region is not allowed"))
			} else {
				allErrs = append(allErrs, field.NotSupported(idxPath, zone, validZones))
			}
		}
	}

	return allErrs
}

func validateOpenStack(c *validationContext) field.ErrorList {
	var (
		allErrs = field.ErrorList{}
		path    = field.NewPath("spec", "cloud", "openstack")
	)

	if c.seed != nil {
		allErrs = append(allErrs, cidrvalidation.ValidateNetworkDisjointedness(
			path.Child("networks"),
			c.shoot.Spec.Cloud.OpenStack.Networks.K8SNetworks.Nodes,
			c.shoot.Spec.Cloud.OpenStack.Networks.K8SNetworks.Pods,
			c.shoot.Spec.Cloud.OpenStack.Networks.K8SNetworks.Services,
			c.seed.Spec.Networks.Nodes,
			c.seed.Spec.Networks.Pods,
			c.seed.Spec.Networks.Services,
		)...)
	}
	if ok, validFloatingPools := validateFloatingPoolConstraints(c.cloudProfile.Spec.OpenStack.Constraints.FloatingPools, c.shoot.Spec.Cloud.OpenStack.FloatingPoolName, c.oldShoot.Spec.Cloud.OpenStack.FloatingPoolName); !ok {
		allErrs = append(allErrs, field.NotSupported(path.Child("floatingPoolName"), c.shoot.Spec.Cloud.OpenStack.FloatingPoolName, validFloatingPools))
	}
	ok, validKubernetesVersions, versionDefault := validateKubernetesVersionConstraints(c.cloudProfile.Spec.Kubernetes.Versions, c.shoot.Spec.Kubernetes.Version, c.oldShoot.Spec.Kubernetes.Version)
	if !ok {
		allErrs = append(allErrs, field.NotSupported(field.NewPath("spec", "kubernetes", "version"), c.shoot.Spec.Kubernetes.Version, validKubernetesVersions))
	} else if versionDefault != nil {
		c.shoot.Spec.Kubernetes.Version = versionDefault.String()
	}
	if ok, validLoadBalancerProviders := validateLoadBalancerProviderConstraints(c.cloudProfile.Spec.OpenStack.Constraints.LoadBalancerProviders, c.shoot.Spec.Cloud.OpenStack.LoadBalancerProvider, c.oldShoot.Spec.Cloud.OpenStack.LoadBalancerProvider); !ok {
		allErrs = append(allErrs, field.NotSupported(path.Child("floatingPoolName"), c.shoot.Spec.Cloud.OpenStack.LoadBalancerProvider, validLoadBalancerProviders))
	}
	if ok, validMachineImages := validateMachineImagesConstraints(c.cloudProfile.Spec.MachineImages, c.shoot.Spec.Cloud.OpenStack.MachineImage, c.oldShoot.Spec.Cloud.OpenStack.MachineImage); !ok {
		allErrs = append(allErrs, field.NotSupported(path.Child("machine", "image"), *c.shoot.Spec.Cloud.OpenStack.MachineImage, validMachineImages))
	}

	for i, worker := range c.shoot.Spec.Cloud.OpenStack.Workers {
		var oldWorker = garden.Worker{}
		for _, ow := range c.oldShoot.Spec.Cloud.OpenStack.Workers {
			if ow.Name == worker.Name {
				oldWorker = ow
				break
			}
		}

		idxPath := path.Child("workers").Index(i)
		if ok, validMachineTypes := validateMachineTypes(c.cloudProfile.Spec.MachineTypes, worker.Machine.Type, oldWorker.Machine.Type, c.cloudProfile.Spec.Regions, c.shoot.Spec.Region, c.shoot.Spec.Cloud.OpenStack.Zones); !ok {
			allErrs = append(allErrs, field.NotSupported(idxPath.Child("machine", "type"), worker.Machine.Type, validMachineTypes))
		}
		if ok, validMachineImages := validateMachineImagesConstraints(c.cloudProfile.Spec.MachineImages, worker.Machine.Image, oldWorker.Machine.Image); !ok {
			allErrs = append(allErrs, field.NotSupported(idxPath.Child("machine", "image"), worker.Machine.Image, validMachineImages))
		}
	}

	for i, zone := range c.shoot.Spec.Cloud.OpenStack.Zones {
		idxPath := path.Child("zones").Index(i)
		if ok, validZones := validateZones(c.cloudProfile.Spec.Regions, c.shoot.Spec.Region, c.oldShoot.Spec.Region, zone); !ok {
			if len(validZones) == 0 {
				allErrs = append(allErrs, field.Invalid(idxPath, c.shoot.Spec.Region, "this region is not allowed"))
			} else {
				allErrs = append(allErrs, field.NotSupported(idxPath, zone, validZones))
			}
		}
	}

	return allErrs
}

func validateAlicloud(c *validationContext) field.ErrorList {
	var (
		allErrs = field.ErrorList{}
		path    = field.NewPath("spec", "cloud", "alicloud")
	)

	if c.seed != nil {
		allErrs = append(allErrs, cidrvalidation.ValidateNetworkDisjointedness(
			path.Child("networks"),
			c.shoot.Spec.Cloud.Alicloud.Networks.K8SNetworks.Nodes,
			c.shoot.Spec.Cloud.Alicloud.Networks.K8SNetworks.Pods,
			c.shoot.Spec.Cloud.Alicloud.Networks.K8SNetworks.Services,
			c.seed.Spec.Networks.Nodes,
			c.seed.Spec.Networks.Pods,
			c.seed.Spec.Networks.Services,
		)...)
	}
	ok, validKubernetesVersions, versionDefault := validateKubernetesVersionConstraints(c.cloudProfile.Spec.Kubernetes.Versions, c.shoot.Spec.Kubernetes.Version, c.oldShoot.Spec.Kubernetes.Version)
	if !ok {
		allErrs = append(allErrs, field.NotSupported(field.NewPath("spec", "kubernetes", "version"), c.shoot.Spec.Kubernetes.Version, validKubernetesVersions))
	} else if versionDefault != nil {
		c.shoot.Spec.Kubernetes.Version = versionDefault.String()
	}
	if ok, validMachineImages := validateMachineImagesConstraints(c.cloudProfile.Spec.MachineImages, c.shoot.Spec.Cloud.Alicloud.MachineImage, c.oldShoot.Spec.Cloud.Alicloud.MachineImage); !ok {
		allErrs = append(allErrs, field.NotSupported(path.Child("machine", "image"), *c.shoot.Spec.Cloud.Alicloud.MachineImage, validMachineImages))
	}

	for i, worker := range c.shoot.Spec.Cloud.Alicloud.Workers {
		var oldWorker = garden.Worker{}
		for _, ow := range c.oldShoot.Spec.Cloud.Alicloud.Workers {
			if ow.Name == worker.Name {
				oldWorker = ow
				break
			}
		}

		idxPath := path.Child("workers").Index(i)
		if ok, validMachineImages := validateMachineImagesConstraints(c.cloudProfile.Spec.MachineImages, worker.Machine.Image, oldWorker.Machine.Image); !ok {
			allErrs = append(allErrs, field.NotSupported(idxPath.Child("machine", "image"), worker.Machine.Image, validMachineImages))
		}
		if ok, validMachineTypes := validateMachineTypes(c.cloudProfile.Spec.MachineTypes, worker.Machine.Type, oldWorker.Machine.Type, c.cloudProfile.Spec.Regions, c.shoot.Spec.Region, c.shoot.Spec.Cloud.Alicloud.Zones); !ok {
			allErrs = append(allErrs, field.NotSupported(idxPath.Child("machine", "type"), worker.Machine.Type, validMachineTypes))
		}
		if ok, validateVolumeTypes := validateVolumeTypes(c.cloudProfile.Spec.VolumeTypes, worker.Volume, oldWorker.Volume, c.cloudProfile.Spec.Regions, c.shoot.Spec.Region, c.shoot.Spec.Cloud.Alicloud.Zones); !ok {
			allErrs = append(allErrs, field.NotSupported(idxPath.Child("volume", "type"), worker.Volume, validateVolumeTypes))
		}
	}

	for i, zone := range c.shoot.Spec.Cloud.Alicloud.Zones {
		idxPath := path.Child("zones").Index(i)
		if ok, validZones := validateZones(c.cloudProfile.Spec.Regions, c.shoot.Spec.Region, c.oldShoot.Spec.Region, zone); !ok {
			if len(validZones) == 0 {
				allErrs = append(allErrs, field.Invalid(idxPath, c.shoot.Spec.Region, "this region is not allowed"))
			} else {
				allErrs = append(allErrs, field.NotSupported(idxPath, zone, validZones))
			}
		}
	}

	return allErrs
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

	ok, validKubernetesVersions, versionDefault := validateKubernetesVersionConstraints(c.cloudProfile.Spec.Kubernetes.Versions, c.shoot.Spec.Kubernetes.Version, c.oldShoot.Spec.Kubernetes.Version)
	if !ok {
		allErrs = append(allErrs, field.NotSupported(field.NewPath("spec", "kubernetes", "version"), c.shoot.Spec.Kubernetes.Version, validKubernetesVersions))
	} else if versionDefault != nil {
		c.shoot.Spec.Kubernetes.Version = versionDefault.String()
	}

	for i, worker := range c.shoot.Spec.Provider.Workers {
		var oldWorker = garden.Worker{Machine: garden.Machine{Image: &garden.ShootMachineImage{}}}
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
		}
		if ok, validVolumeTypes := validateVolumeTypes(c.cloudProfile.Spec.VolumeTypes, worker.Volume, oldWorker.Volume, c.cloudProfile.Spec.Regions, c.shoot.Spec.Region, worker.Zones); !ok {
			allErrs = append(allErrs, field.NotSupported(idxPath.Child("volume", "type"), worker.Volume, validVolumeTypes))
		}

		for j, zone := range worker.Zones {
			jdxPath := idxPath.Child("zones").Index(j)
			if ok, validZones := validateZones(c.cloudProfile.Spec.Regions, c.shoot.Spec.Region, c.oldShoot.Spec.Region, zone); !ok {
				if len(validZones) == 0 {
					allErrs = append(allErrs, field.Invalid(jdxPath, c.shoot.Spec.Region, "this region is not allowed"))
				} else {
					allErrs = append(allErrs, field.NotSupported(jdxPath, zone, validZones))
				}
			}
		}
	}

	return allErrs
}

func validateDNSDomainUniqueness(shootLister listers.ShootLister, name string, dns *garden.DNS) (field.ErrorList, error) {
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

func validateKubernetesVersionConstraints(constraints []garden.ExpirableVersion, shootVersion, oldShootVersion string) (bool, []string, *semver.Version) {
	if shootVersion == oldShootVersion {
		return true, nil, nil
	}

	shootVersionSplit := strings.Split(shootVersion, ".")
	var (
		shootVersionMajor, shootVersionMinor int64
		getLatestPatchVersion                bool
	)
	if len(shootVersionSplit) == 2 {
		// add a fake patch version to avoid manual parsing
		fakeShootVersion := shootVersion + ".0"
		version, err := semver.NewVersion(fakeShootVersion)
		if err == nil {
			getLatestPatchVersion = true
			shootVersionMajor = version.Major()
			shootVersionMinor = version.Minor()
		}
	}

	validValues := []string{}
	var latestVersion *semver.Version
	for _, versionConstraint := range constraints {
		if versionConstraint.ExpirationDate != nil && versionConstraint.ExpirationDate.Time.Before(time.Now()) {
			continue
		}

		validValues = append(validValues, versionConstraint.Version)

		if versionConstraint.Version == shootVersion {
			return true, nil, nil
		}

		if getLatestPatchVersion {
			// CloudProfile cannot contain invalid semVer shootVersion
			cpVersion, _ := semver.NewVersion(versionConstraint.Version)

			if cpVersion.Major() != shootVersionMajor || cpVersion.Minor() != shootVersionMinor {
				continue
			}

			if latestVersion == nil || cpVersion.GreaterThan(latestVersion) {
				latestVersion = cpVersion
			}
		}
	}

	if latestVersion != nil {
		return true, nil, latestVersion
	}

	return false, validValues, nil
}

func validateMachineTypes(constraints []garden.MachineType, machineType, oldMachineType string, regions []garden.Region, region string, zones []string) (bool, []string) {
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

func validateVolumeTypes(constraints []garden.VolumeType, volume, oldVolume *garden.Volume, regions []garden.Region, region string, zones []string) (bool, []string) {
	if (volume == nil && oldVolume == nil) || volume.Type == nil || (volume != nil && oldVolume != nil && volume.Type != nil && oldVolume.Type != nil && *volume.Type == *oldVolume.Type) {
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

func validateZones(constraints []garden.Region, region, oldRegion, zone string) (bool, []string) {
	var (
		validValues = []string{}
		regionFound = false
	)

	for _, r := range constraints {
		if r.Name == region {
			for _, z := range r.Zones {
				validValues = append(validValues, z.Name)
				if z.Name == zone {
					return true, nil
				}
			}
			regionFound = true
			break
		}
	}

	if !regionFound && region == oldRegion {
		return true, nil
	}

	return false, validValues
}

func validateAzureDomainCount(count []garden.AzureDomainCount, region string) bool {
	for _, c := range count {
		if c.Region == region {
			return true
		}
	}
	return false
}

func validateFloatingPoolConstraints(pools []garden.OpenStackFloatingPool, pool, oldPool string) (bool, []string) {
	if pool == oldPool {
		return true, nil
	}

	validValues := []string{}

	for _, p := range pools {
		validValues = append(validValues, p.Name)
		if p.Name == pool {
			return true, nil
		}
	}

	return false, validValues
}

func validateLoadBalancerProviderConstraints(providers []garden.OpenStackLoadBalancerProvider, provider, oldProvider string) (bool, []string) {
	if provider == oldProvider {
		return true, nil
	}

	validValues := []string{}

	for _, p := range providers {
		validValues = append(validValues, p.Name)
		if p.Name == provider {
			return true, nil
		}
	}

	return false, validValues
}

// getDefaultMachineImage determines the latest machine image version from the first machine image in the CloudProfile and considers that as the default image
func getDefaultMachineImage(machineImages []garden.CloudProfileMachineImage) (*garden.ShootMachineImage, error) {
	if len(machineImages) == 0 {
		return nil, errors.New("the cloud profile does not contain any machine image - cannot create shoot cluster")
	}
	firstMachineImageInCloudProfile := machineImages[0]
	latestMachineImageVersion, err := helper.DetermineLatestCloudProfileMachineImageVersion(firstMachineImageInCloudProfile)
	if err != nil {
		return nil, fmt.Errorf("failed to determine latest machine image from cloud profile: %s", err.Error())
	}
	return &garden.ShootMachineImage{Name: firstMachineImageInCloudProfile.Name, Version: latestMachineImageVersion.Version}, nil
}

func validateMachineImagesConstraints(constraints []garden.CloudProfileMachineImage, image, oldImage *garden.ShootMachineImage) (bool, []string) {
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

func getOldWorkerMachineImageOrDefault(workers []garden.Worker, name string, defaultImage *garden.ShootMachineImage) *garden.ShootMachineImage {
	if oldWorker := helper.FindWorkerByName(workers, name); oldWorker != nil && oldWorker.Machine.Image != nil {
		return oldWorker.Machine.Image
	}
	return defaultImage
}
