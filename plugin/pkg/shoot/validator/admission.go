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
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"time"

	"github.com/gardener/gardener/pkg/apis/garden"
	"github.com/gardener/gardener/pkg/apis/garden/helper"
	admissioninitializer "github.com/gardener/gardener/pkg/apiserver/admission/initializer"
	informers "github.com/gardener/gardener/pkg/client/garden/informers/internalversion"
	listers "github.com/gardener/gardener/pkg/client/garden/listers/garden/internalversion"
	"github.com/gardener/gardener/pkg/operation/common"
	admissionutils "github.com/gardener/gardener/plugin/pkg/utils"

	"github.com/Masterminds/semver"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
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

// Admit validates the Shoot details against the referenced CloudProfile.
func (v *ValidateShoot) Admit(a admission.Attributes, o admission.ObjectInterfaces) error {
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
	if a.GetKind().GroupKind() != garden.Kind("Shoot") {
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
		if reflect.DeepEqual(newShoot.Spec, oldShoot.Spec) {
			return nil
		}
	}

	shoot, ok := a.GetObject().(*garden.Shoot)
	if !ok {
		return apierrors.NewInternalError(errors.New("could not convert resource into Shoot object"))
	}

	cloudProfile, err := v.cloudProfileLister.Get(shoot.Spec.Cloud.Profile)
	if err != nil {
		return apierrors.NewBadRequest(fmt.Sprintf("could not find referenced cloud profile: %+v", err.Error()))
	}
	var seed *garden.Seed
	if shoot.Spec.Cloud.Seed != nil {
		seed, err = v.seedLister.Get(*shoot.Spec.Cloud.Seed)
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
	if shoot.Namespace != common.GardenNamespace && seed != nil && helper.TaintsHave(seed.Spec.Taints, garden.SeedTaintProtected) {
		return admission.NewForbidden(a, fmt.Errorf("forbidden to use a protected seed"))
	}

	// We don't allow shoot to be created on the seed which is already marked to be deleted.
	if seed != nil && seed.DeletionTimestamp != nil {
		return admission.NewForbidden(a, fmt.Errorf("cannot create or update shoot '%s' on seed '%s' already marked for deletion", shoot.Name, seed.Name))
	}

	cloudProviderInShoot, err := helper.DetermineCloudProviderInShoot(shoot.Spec.Cloud)
	if err != nil {
		return apierrors.NewBadRequest("could not identify the cloud provider kind in the Shoot resource")
	}
	cloudProviderInProfile, err := helper.DetermineCloudProviderInProfile(cloudProfile.Spec)
	if err != nil {
		return apierrors.NewBadRequest("could not identify the cloud provider kind in the referenced cloud profile")
	}

	if cloudProviderInShoot != cloudProviderInProfile {
		return apierrors.NewBadRequest("cloud provider in shoot is not equal to cloud provder in profile")
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

	if seed != nil {
		if shoot.Spec.Networking == nil {
			shoot.Spec.Networking = &garden.Networking{}
		}
		if shoot.Spec.Networking.Pods == nil && seed.Spec.Networks.ShootDefaults != nil {
			shoot.Spec.Networking.Pods = seed.Spec.Networks.ShootDefaults.Pods
		}
		if shoot.Spec.Networking.Services == nil && seed.Spec.Networks.ShootDefaults != nil {
			shoot.Spec.Networking.Services = seed.Spec.Networks.ShootDefaults.Services
		}
	}

	switch cloudProviderInShoot {
	case garden.CloudProviderAWS:
		if shoot.Spec.Cloud.AWS.MachineImage == nil {
			image, err := getDefaultMachineImage(cloudProfile.Spec.AWS.Constraints.MachineImages)
			if err != nil {
				return apierrors.NewBadRequest(err.Error())
			}
			shoot.Spec.Cloud.AWS.MachineImage = image
		}

		for idx, worker := range shoot.Spec.Cloud.AWS.Workers {
			if shoot.DeletionTimestamp == nil && worker.MachineImage == nil {
				shoot.Spec.Cloud.AWS.Workers[idx].MachineImage = shoot.Spec.Cloud.AWS.MachineImage
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

	case garden.CloudProviderAzure:
		if shoot.Spec.Cloud.Azure.MachineImage == nil {
			image, err := getDefaultMachineImage(cloudProfile.Spec.Azure.Constraints.MachineImages)
			if err != nil {
				return apierrors.NewBadRequest(err.Error())
			}
			shoot.Spec.Cloud.Azure.MachineImage = image
		}

		for idx, worker := range shoot.Spec.Cloud.Azure.Workers {
			if shoot.DeletionTimestamp == nil && worker.MachineImage == nil {
				shoot.Spec.Cloud.Azure.Workers[idx].MachineImage = shoot.Spec.Cloud.Azure.MachineImage
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

	case garden.CloudProviderGCP:
		if shoot.Spec.Cloud.GCP.MachineImage == nil {
			image, err := getDefaultMachineImage(cloudProfile.Spec.GCP.Constraints.MachineImages)
			if err != nil {
				return apierrors.NewBadRequest(err.Error())
			}
			shoot.Spec.Cloud.GCP.MachineImage = image
		}

		for idx, worker := range shoot.Spec.Cloud.GCP.Workers {
			if shoot.DeletionTimestamp == nil && worker.MachineImage == nil {
				shoot.Spec.Cloud.GCP.Workers[idx].MachineImage = shoot.Spec.Cloud.GCP.MachineImage
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

	case garden.CloudProviderOpenStack:
		if shoot.Spec.Cloud.OpenStack.MachineImage == nil {
			image, err := getDefaultMachineImage(cloudProfile.Spec.OpenStack.Constraints.MachineImages)
			if err != nil {
				return apierrors.NewBadRequest(err.Error())
			}
			shoot.Spec.Cloud.OpenStack.MachineImage = image
		}

		for idx, worker := range shoot.Spec.Cloud.OpenStack.Workers {
			if shoot.DeletionTimestamp == nil && worker.MachineImage == nil {
				shoot.Spec.Cloud.OpenStack.Workers[idx].MachineImage = shoot.Spec.Cloud.OpenStack.MachineImage
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

	case garden.CloudProviderPacket:
		if shoot.Spec.Cloud.Packet.MachineImage == nil {
			image, err := getDefaultMachineImage(cloudProfile.Spec.Packet.Constraints.MachineImages)
			if err != nil {
				return apierrors.NewBadRequest(err.Error())
			}
			shoot.Spec.Cloud.Packet.MachineImage = image
		}

		for idx, worker := range shoot.Spec.Cloud.Packet.Workers {
			if shoot.DeletionTimestamp == nil && worker.MachineImage == nil {
				shoot.Spec.Cloud.Packet.Workers[idx].MachineImage = shoot.Spec.Cloud.Packet.MachineImage
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

	case garden.CloudProviderAlicloud:
		if shoot.Spec.Cloud.Alicloud.MachineImage == nil {
			image, err := getDefaultMachineImage(cloudProfile.Spec.Alicloud.Constraints.MachineImages)
			if err != nil {
				return apierrors.NewBadRequest(err.Error())
			}
			shoot.Spec.Cloud.Alicloud.MachineImage = image
		}

		for idx, worker := range shoot.Spec.Cloud.Alicloud.Workers {
			if shoot.DeletionTimestamp == nil && worker.MachineImage == nil {
				shoot.Spec.Cloud.Alicloud.Workers[idx].MachineImage = shoot.Spec.Cloud.Alicloud.MachineImage
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
		allErrs = append(allErrs, admissionutils.ValidateNetworkDisjointedness(c.seed.Spec.Networks, c.shoot.Spec.Cloud.AWS.Networks.K8SNetworks, path.Child("networks"))...)
	}
	if ok, validDNSProviders := validateDNSConstraints(c.cloudProfile.Spec.AWS.Constraints.DNSProviders, c.shoot.Spec.DNS.Provider, c.oldShoot.Spec.DNS.Provider); !ok {
		allErrs = append(allErrs, field.NotSupported(field.NewPath("spec", "dns", "provider"), c.shoot.Spec.DNS.Provider, validDNSProviders))
	}
	ok, validKubernetesVersions, versionDefault := validateKubernetesVersionConstraints(c.cloudProfile.Spec.AWS.Constraints.Kubernetes.OfferedVersions, c.shoot.Spec.Kubernetes.Version, c.oldShoot.Spec.Kubernetes.Version)
	if !ok {
		allErrs = append(allErrs, field.NotSupported(field.NewPath("spec", "kubernetes", "version"), c.shoot.Spec.Kubernetes.Version, validKubernetesVersions))
	} else if versionDefault != nil {
		c.shoot.Spec.Kubernetes.Version = versionDefault.String()
	}
	if ok, validMachineImages := validateMachineImagesConstraints(c.cloudProfile.Spec.AWS.Constraints.MachineImages, c.shoot.Spec.Cloud.AWS.MachineImage, c.oldShoot.Spec.Cloud.AWS.MachineImage); !ok {
		allErrs = append(allErrs, field.NotSupported(path.Child("machineImage"), *c.shoot.Spec.Cloud.AWS.MachineImage, validMachineImages))
	}

	for i, worker := range c.shoot.Spec.Cloud.AWS.Workers {
		var oldWorker = garden.AWSWorker{}
		for _, ow := range c.oldShoot.Spec.Cloud.AWS.Workers {
			if ow.Name == worker.Name {
				oldWorker = ow
				break
			}
		}

		idxPath := path.Child("workers").Index(i)
		if ok, validMachineTypes := validateMachineTypes(c.cloudProfile.Spec.AWS.Constraints.MachineTypes, worker.MachineType, oldWorker.MachineType); !ok {
			allErrs = append(allErrs, field.NotSupported(idxPath.Child("machineType"), worker.MachineType, validMachineTypes))
		}
		if ok, validMachineImages := validateMachineImagesConstraints(c.cloudProfile.Spec.AWS.Constraints.MachineImages, worker.MachineImage, oldWorker.MachineImage); !ok {
			allErrs = append(allErrs, field.NotSupported(idxPath.Child("machineImage"), worker.MachineImage, validMachineImages))
		}
		if ok, validVolumeTypes := validateVolumeTypes(c.cloudProfile.Spec.AWS.Constraints.VolumeTypes, worker.VolumeType, oldWorker.VolumeType); !ok {
			allErrs = append(allErrs, field.NotSupported(idxPath.Child("volumeType"), worker.VolumeType, validVolumeTypes))
		}
	}

	for i, zone := range c.shoot.Spec.Cloud.AWS.Zones {
		idxPath := path.Child("zones").Index(i)
		if ok, validZones := validateZones(c.cloudProfile.Spec.AWS.Constraints.Zones, c.shoot.Spec.Cloud.Region, zone); !ok {
			if len(validZones) == 0 {
				allErrs = append(allErrs, field.Invalid(idxPath, c.shoot.Spec.Cloud.Region, "this region is not allowed"))
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
		allErrs = append(allErrs, admissionutils.ValidateNetworkDisjointedness(c.seed.Spec.Networks, c.shoot.Spec.Cloud.Azure.Networks.K8SNetworks, path.Child("networks"))...)
	}
	if ok, validDNSProviders := validateDNSConstraints(c.cloudProfile.Spec.Azure.Constraints.DNSProviders, c.shoot.Spec.DNS.Provider, c.oldShoot.Spec.DNS.Provider); !ok {
		allErrs = append(allErrs, field.NotSupported(field.NewPath("spec", "dns", "provider"), c.shoot.Spec.DNS.Provider, validDNSProviders))
	}
	ok, validKubernetesVersions, versionDefault := validateKubernetesVersionConstraints(c.cloudProfile.Spec.Azure.Constraints.Kubernetes.OfferedVersions, c.shoot.Spec.Kubernetes.Version, c.oldShoot.Spec.Kubernetes.Version)
	if !ok {
		allErrs = append(allErrs, field.NotSupported(field.NewPath("spec", "kubernetes", "version"), c.shoot.Spec.Kubernetes.Version, validKubernetesVersions))
	} else if versionDefault != nil {
		c.shoot.Spec.Kubernetes.Version = versionDefault.String()
	}
	if ok, validMachineImages := validateMachineImagesConstraints(c.cloudProfile.Spec.Azure.Constraints.MachineImages, c.shoot.Spec.Cloud.Azure.MachineImage, c.oldShoot.Spec.Cloud.Azure.MachineImage); !ok {
		allErrs = append(allErrs, field.NotSupported(path.Child("machineImage"), *c.shoot.Spec.Cloud.Azure.MachineImage, validMachineImages))
	}

	for i, worker := range c.shoot.Spec.Cloud.Azure.Workers {
		var oldWorker = garden.AzureWorker{}
		for _, ow := range c.oldShoot.Spec.Cloud.Azure.Workers {
			if ow.Name == worker.Name {
				oldWorker = ow
				break
			}
		}

		idxPath := path.Child("workers").Index(i)
		if ok, validMachineTypes := validateMachineTypes(c.cloudProfile.Spec.Azure.Constraints.MachineTypes, worker.MachineType, oldWorker.MachineType); !ok {
			allErrs = append(allErrs, field.NotSupported(idxPath.Child("machineType"), worker.MachineType, validMachineTypes))
		}
		if ok, validMachineImages := validateMachineImagesConstraints(c.cloudProfile.Spec.Azure.Constraints.MachineImages, worker.MachineImage, oldWorker.MachineImage); !ok {
			allErrs = append(allErrs, field.NotSupported(idxPath.Child("machineImage"), worker.MachineImage, validMachineImages))
		}
		if ok, validVolumeTypes := validateVolumeTypes(c.cloudProfile.Spec.Azure.Constraints.VolumeTypes, worker.VolumeType, oldWorker.VolumeType); !ok {
			allErrs = append(allErrs, field.NotSupported(idxPath.Child("volumeType"), worker.VolumeType, validVolumeTypes))
		}
	}

	if ok := validateAzureDomainCount(c.cloudProfile.Spec.Azure.CountFaultDomains, c.shoot.Spec.Cloud.Region); !ok {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "cloud", "region"), c.shoot.Spec.Cloud.Region, "no fault domain count known for this region"))
	}
	if ok := validateAzureDomainCount(c.cloudProfile.Spec.Azure.CountUpdateDomains, c.shoot.Spec.Cloud.Region); !ok {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "cloud", "region"), c.shoot.Spec.Cloud.Region, "no update domain count known for this region"))
	}

	return allErrs
}

func validateGCP(c *validationContext) field.ErrorList {
	var (
		allErrs = field.ErrorList{}
		path    = field.NewPath("spec", "cloud", "gcp")
	)

	if c.seed != nil {
		allErrs = append(allErrs, admissionutils.ValidateNetworkDisjointedness(c.seed.Spec.Networks, c.shoot.Spec.Cloud.GCP.Networks.K8SNetworks, path.Child("networks"))...)
	}
	if ok, validDNSProviders := validateDNSConstraints(c.cloudProfile.Spec.GCP.Constraints.DNSProviders, c.shoot.Spec.DNS.Provider, c.oldShoot.Spec.DNS.Provider); !ok {
		allErrs = append(allErrs, field.NotSupported(field.NewPath("spec", "dns", "provider"), c.shoot.Spec.DNS.Provider, validDNSProviders))
	}
	ok, validKubernetesVersions, versionDefault := validateKubernetesVersionConstraints(c.cloudProfile.Spec.GCP.Constraints.Kubernetes.OfferedVersions, c.shoot.Spec.Kubernetes.Version, c.oldShoot.Spec.Kubernetes.Version)
	if !ok {
		allErrs = append(allErrs, field.NotSupported(field.NewPath("spec", "kubernetes", "version"), c.shoot.Spec.Kubernetes.Version, validKubernetesVersions))
	} else if versionDefault != nil {
		c.shoot.Spec.Kubernetes.Version = versionDefault.String()
	}
	if ok, validMachineImages := validateMachineImagesConstraints(c.cloudProfile.Spec.GCP.Constraints.MachineImages, c.shoot.Spec.Cloud.GCP.MachineImage, c.oldShoot.Spec.Cloud.GCP.MachineImage); !ok {
		allErrs = append(allErrs, field.NotSupported(path.Child("machineImage"), *c.shoot.Spec.Cloud.GCP.MachineImage, validMachineImages))
	}

	for i, worker := range c.shoot.Spec.Cloud.GCP.Workers {
		var oldWorker = garden.GCPWorker{}
		for _, ow := range c.oldShoot.Spec.Cloud.GCP.Workers {
			if ow.Name == worker.Name {
				oldWorker = ow
				break
			}
		}

		idxPath := path.Child("workers").Index(i)
		if ok, validMachineTypes := validateMachineTypes(c.cloudProfile.Spec.GCP.Constraints.MachineTypes, worker.MachineType, oldWorker.MachineType); !ok {
			allErrs = append(allErrs, field.NotSupported(idxPath.Child("machineType"), worker.MachineType, validMachineTypes))
		}
		if ok, validMachineImages := validateMachineImagesConstraints(c.cloudProfile.Spec.GCP.Constraints.MachineImages, worker.MachineImage, oldWorker.MachineImage); !ok {
			allErrs = append(allErrs, field.NotSupported(idxPath.Child("machineImage"), worker.MachineImage, validMachineImages))
		}
		if ok, validVolumeTypes := validateVolumeTypes(c.cloudProfile.Spec.GCP.Constraints.VolumeTypes, worker.VolumeType, oldWorker.MachineType); !ok {
			allErrs = append(allErrs, field.NotSupported(idxPath.Child("volumeType"), worker.VolumeType, validVolumeTypes))
		}
	}

	for i, zone := range c.shoot.Spec.Cloud.GCP.Zones {
		idxPath := path.Child("zones").Index(i)
		if ok, validZones := validateZones(c.cloudProfile.Spec.GCP.Constraints.Zones, c.shoot.Spec.Cloud.Region, zone); !ok {
			if len(validZones) == 0 {
				allErrs = append(allErrs, field.Invalid(idxPath, c.shoot.Spec.Cloud.Region, "this region is not allowed"))
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
		allErrs = append(allErrs, admissionutils.ValidateNetworkDisjointedness(c.seed.Spec.Networks, c.shoot.Spec.Cloud.Packet.Networks.K8SNetworks, path.Child("networks"))...)
	}
	if ok, validDNSProviders := validateDNSConstraints(c.cloudProfile.Spec.Packet.Constraints.DNSProviders, c.shoot.Spec.DNS.Provider, c.oldShoot.Spec.DNS.Provider); !ok {
		allErrs = append(allErrs, field.NotSupported(field.NewPath("spec", "dns", "provider"), c.shoot.Spec.DNS.Provider, validDNSProviders))
	}
	ok, validKubernetesVersions, versionDefault := validateKubernetesVersionConstraints(c.cloudProfile.Spec.Packet.Constraints.Kubernetes.OfferedVersions, c.shoot.Spec.Kubernetes.Version, c.oldShoot.Spec.Kubernetes.Version)
	if !ok {
		allErrs = append(allErrs, field.NotSupported(field.NewPath("spec", "kubernetes", "version"), c.shoot.Spec.Kubernetes.Version, validKubernetesVersions))
	} else if versionDefault != nil {
		c.shoot.Spec.Kubernetes.Version = versionDefault.String()
	}
	if ok, validMachineImages := validateMachineImagesConstraints(c.cloudProfile.Spec.Packet.Constraints.MachineImages, c.shoot.Spec.Cloud.Packet.MachineImage, c.oldShoot.Spec.Cloud.Packet.MachineImage); !ok {
		allErrs = append(allErrs, field.NotSupported(path.Child("machineImage"), *c.shoot.Spec.Cloud.Packet.MachineImage, validMachineImages))
	}

	for i, worker := range c.shoot.Spec.Cloud.Packet.Workers {
		var oldWorker = garden.PacketWorker{}
		for _, ow := range c.oldShoot.Spec.Cloud.Packet.Workers {
			if ow.Name == worker.Name {
				oldWorker = ow
				break
			}
		}

		idxPath := path.Child("workers").Index(i)
		if ok, validMachineTypes := validateMachineTypes(c.cloudProfile.Spec.Packet.Constraints.MachineTypes, worker.MachineType, oldWorker.MachineType); !ok {
			allErrs = append(allErrs, field.NotSupported(idxPath.Child("machineType"), worker.MachineType, validMachineTypes))
		}
		if ok, validMachineImages := validateMachineImagesConstraints(c.cloudProfile.Spec.Packet.Constraints.MachineImages, worker.MachineImage, oldWorker.MachineImage); !ok {
			allErrs = append(allErrs, field.NotSupported(idxPath.Child("machineImage"), worker.MachineImage, validMachineImages))
		}
		if ok, validVolumeTypes := validateVolumeTypes(c.cloudProfile.Spec.Packet.Constraints.VolumeTypes, worker.VolumeType, oldWorker.MachineType); !ok {
			allErrs = append(allErrs, field.NotSupported(idxPath.Child("volumeType"), worker.VolumeType, validVolumeTypes))
		}
	}

	for i, zone := range c.shoot.Spec.Cloud.Packet.Zones {
		idxPath := path.Child("zones").Index(i)
		if ok, validZones := validateZones(c.cloudProfile.Spec.Packet.Constraints.Zones, c.shoot.Spec.Cloud.Region, zone); !ok {
			if len(validZones) == 0 {
				allErrs = append(allErrs, field.Invalid(idxPath, c.shoot.Spec.Cloud.Region, "this region is not allowed"))
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
		allErrs = append(allErrs, admissionutils.ValidateNetworkDisjointedness(c.seed.Spec.Networks, c.shoot.Spec.Cloud.OpenStack.Networks.K8SNetworks, path.Child("networks"))...)
	}
	if ok, validDNSProviders := validateDNSConstraints(c.cloudProfile.Spec.OpenStack.Constraints.DNSProviders, c.shoot.Spec.DNS.Provider, c.oldShoot.Spec.DNS.Provider); !ok {
		allErrs = append(allErrs, field.NotSupported(field.NewPath("spec", "dns", "provider"), c.shoot.Spec.DNS.Provider, validDNSProviders))
	}
	if ok, validFloatingPools := validateFloatingPoolConstraints(c.cloudProfile.Spec.OpenStack.Constraints.FloatingPools, c.shoot.Spec.Cloud.OpenStack.FloatingPoolName, c.oldShoot.Spec.Cloud.OpenStack.FloatingPoolName); !ok {
		allErrs = append(allErrs, field.NotSupported(path.Child("floatingPoolName"), c.shoot.Spec.Cloud.OpenStack.FloatingPoolName, validFloatingPools))
	}
	ok, validKubernetesVersions, versionDefault := validateKubernetesVersionConstraints(c.cloudProfile.Spec.OpenStack.Constraints.Kubernetes.OfferedVersions, c.shoot.Spec.Kubernetes.Version, c.oldShoot.Spec.Kubernetes.Version)
	if !ok {
		allErrs = append(allErrs, field.NotSupported(field.NewPath("spec", "kubernetes", "version"), c.shoot.Spec.Kubernetes.Version, validKubernetesVersions))
	} else if versionDefault != nil {
		c.shoot.Spec.Kubernetes.Version = versionDefault.String()
	}
	if ok, validLoadBalancerProviders := validateLoadBalancerProviderConstraints(c.cloudProfile.Spec.OpenStack.Constraints.LoadBalancerProviders, c.shoot.Spec.Cloud.OpenStack.LoadBalancerProvider, c.oldShoot.Spec.Cloud.OpenStack.LoadBalancerProvider); !ok {
		allErrs = append(allErrs, field.NotSupported(path.Child("floatingPoolName"), c.shoot.Spec.Cloud.OpenStack.LoadBalancerProvider, validLoadBalancerProviders))
	}
	if ok, validMachineImages := validateMachineImagesConstraints(c.cloudProfile.Spec.OpenStack.Constraints.MachineImages, c.shoot.Spec.Cloud.OpenStack.MachineImage, c.oldShoot.Spec.Cloud.OpenStack.MachineImage); !ok {
		allErrs = append(allErrs, field.NotSupported(path.Child("machineImage"), *c.shoot.Spec.Cloud.OpenStack.MachineImage, validMachineImages))
	}

	for i, worker := range c.shoot.Spec.Cloud.OpenStack.Workers {
		var oldWorker = garden.OpenStackWorker{}
		for _, ow := range c.oldShoot.Spec.Cloud.OpenStack.Workers {
			if ow.Name == worker.Name {
				oldWorker = ow
				break
			}
		}

		idxPath := path.Child("workers").Index(i)
		if ok, validMachineTypes := validateOpenStackMachineTypes(c.cloudProfile.Spec.OpenStack.Constraints.MachineTypes, worker.MachineType, oldWorker.MachineType); !ok {
			allErrs = append(allErrs, field.NotSupported(idxPath.Child("machineType"), worker.MachineType, validMachineTypes))
		}
		if ok, validMachineImages := validateMachineImagesConstraints(c.cloudProfile.Spec.OpenStack.Constraints.MachineImages, worker.MachineImage, oldWorker.MachineImage); !ok {
			allErrs = append(allErrs, field.NotSupported(idxPath.Child("machineImage"), worker.MachineImage, validMachineImages))
		}
	}

	for i, zone := range c.shoot.Spec.Cloud.OpenStack.Zones {
		idxPath := path.Child("zones").Index(i)
		if ok, validZones := validateZones(c.cloudProfile.Spec.OpenStack.Constraints.Zones, c.shoot.Spec.Cloud.Region, zone); !ok {
			if len(validZones) == 0 {
				allErrs = append(allErrs, field.Invalid(idxPath, c.shoot.Spec.Cloud.Region, "this region is not allowed"))
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
		allErrs = append(allErrs, admissionutils.ValidateNetworkDisjointedness(c.seed.Spec.Networks, c.shoot.Spec.Cloud.Alicloud.Networks.K8SNetworks, path.Child("networks"))...)
	}
	if ok, validDNSProviders := validateDNSConstraints(c.cloudProfile.Spec.Alicloud.Constraints.DNSProviders, c.shoot.Spec.DNS.Provider, c.oldShoot.Spec.DNS.Provider); !ok {
		allErrs = append(allErrs, field.NotSupported(field.NewPath("spec", "dns", "provider"), c.shoot.Spec.DNS.Provider, validDNSProviders))
	}
	ok, validKubernetesVersions, versionDefault := validateKubernetesVersionConstraints(c.cloudProfile.Spec.Alicloud.Constraints.Kubernetes.OfferedVersions, c.shoot.Spec.Kubernetes.Version, c.oldShoot.Spec.Kubernetes.Version)
	if !ok {
		allErrs = append(allErrs, field.NotSupported(field.NewPath("spec", "kubernetes", "version"), c.shoot.Spec.Kubernetes.Version, validKubernetesVersions))
	} else if versionDefault != nil {
		c.shoot.Spec.Kubernetes.Version = versionDefault.String()
	}
	if ok, validMachineImages := validateMachineImagesConstraints(c.cloudProfile.Spec.Alicloud.Constraints.MachineImages, c.shoot.Spec.Cloud.Alicloud.MachineImage, c.oldShoot.Spec.Cloud.Alicloud.MachineImage); !ok {
		allErrs = append(allErrs, field.NotSupported(path.Child("machineImage"), *c.shoot.Spec.Cloud.Alicloud.MachineImage, validMachineImages))
	}

	for i, worker := range c.shoot.Spec.Cloud.Alicloud.Workers {
		var oldWorker = garden.AlicloudWorker{}
		for _, ow := range c.oldShoot.Spec.Cloud.Alicloud.Workers {
			if ow.Name == worker.Name {
				oldWorker = ow
				break
			}
		}

		idxPath := path.Child("workers").Index(i)
		if ok, validMachineTypes := validateAlicloudMachineTypes(c.cloudProfile.Spec.Alicloud.Constraints.MachineTypes, worker.MachineType, oldWorker.MachineType); !ok {
			allErrs = append(allErrs, field.NotSupported(idxPath.Child("machineType"), worker.MachineType, validMachineTypes))
		}
		if ok, validMachineImages := validateMachineImagesConstraints(c.cloudProfile.Spec.Alicloud.Constraints.MachineImages, worker.MachineImage, oldWorker.MachineImage); !ok {
			allErrs = append(allErrs, field.NotSupported(idxPath.Child("machineImage"), worker.MachineImage, validMachineImages))
		}
		if ok, machineType, validZones := validateAlicloudMachineTypesAvailableInZones(c.cloudProfile.Spec.Alicloud.Constraints.MachineTypes, worker.MachineType, oldWorker.MachineType, c.shoot.Spec.Cloud.Alicloud.Zones); !ok {
			allErrs = append(allErrs, field.Invalid(idxPath.Child("machineType"), worker.MachineType, fmt.Sprintf("only zones %v define machine type %s", validZones, machineType)))
		}
		if ok, validateVolumeTypes := validateAlicloudVolumeTypes(c.cloudProfile.Spec.Alicloud.Constraints.VolumeTypes, worker.VolumeType, oldWorker.VolumeType); !ok {
			allErrs = append(allErrs, field.NotSupported(idxPath.Child("volumeType"), worker.VolumeType, validateVolumeTypes))
		}
		if ok, volumeType, validZones := validateAlicloudVolumeTypesAvailableInZones(c.cloudProfile.Spec.Alicloud.Constraints.VolumeTypes, worker.VolumeType, oldWorker.VolumeType, c.shoot.Spec.Cloud.Alicloud.Zones); !ok {
			allErrs = append(allErrs, field.Invalid(idxPath.Child("volumeType"), worker.VolumeType, fmt.Sprintf("only zones %v define volume type %s", validZones, volumeType)))
		}
	}

	for i, zone := range c.shoot.Spec.Cloud.Alicloud.Zones {
		idxPath := path.Child("zones").Index(i)
		if ok, validZones := validateZones(c.cloudProfile.Spec.Alicloud.Constraints.Zones, c.shoot.Spec.Cloud.Region, zone); !ok {
			if len(validZones) == 0 {
				allErrs = append(allErrs, field.Invalid(idxPath, c.shoot.Spec.Cloud.Region, "this region is not allowed"))
			} else {
				allErrs = append(allErrs, field.NotSupported(idxPath, zone, validZones))
			}
		}
	}

	return allErrs
}

func validateDNSConstraints(constraints []garden.DNSProviderConstraint, provider, oldProvider *string) (bool, []string) {
	if apiequality.Semantic.DeepEqual(provider, oldProvider) {
		return true, nil
	}

	if provider == nil {
		return true, nil
	}

	validValues := []string{}

	for _, p := range constraints {
		validValues = append(validValues, string(p.Name))
		if p.Name == *provider {
			return true, nil
		}
	}

	return false, validValues
}

func validateDNSDomainUniqueness(shootLister listers.ShootLister, name string, dns garden.DNS) (field.ErrorList, error) {
	var (
		allErrs = field.ErrorList{}
		dnsPath = field.NewPath("spec", "dns", "domain")
	)

	if dns.Domain == nil {
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

		domain := shoot.Spec.DNS.Domain
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

func validateKubernetesVersionConstraints(constraints []garden.KubernetesVersion, shootVersion, oldShootVersion string) (bool, []string, *semver.Version) {
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

func validateMachineTypes(constraints []garden.MachineType, machineType, oldMachineType string) (bool, []string) {
	if machineType == oldMachineType {
		return true, nil
	}

	validValues := []string{}

	for _, t := range constraints {
		if t.Usable != nil && !*t.Usable {
			continue
		}
		validValues = append(validValues, t.Name)
		if t.Name == machineType {
			return true, nil
		}
	}

	return false, validValues
}

func validateOpenStackMachineTypes(constraints []garden.OpenStackMachineType, machineType, oldMachineType string) (bool, []string) {
	machineTypes := []garden.MachineType{}
	for _, t := range constraints {
		machineTypes = append(machineTypes, t.MachineType)
	}

	return validateMachineTypes(machineTypes, machineType, oldMachineType)
}

func validateAlicloudMachineTypes(constraints []garden.AlicloudMachineType, machineType, oldMachineType string) (bool, []string) {
	machineTypes := []garden.MachineType{}
	for _, t := range constraints {
		machineTypes = append(machineTypes, t.MachineType)
	}

	return validateMachineTypes(machineTypes, machineType, oldMachineType)
}

// To check whether machine type of worker is available in zones of the shoot,
// because in alicloud different zones may have different machine type
func validateAlicloudMachineTypesAvailableInZones(constraints []garden.AlicloudMachineType, machineType, oldMachineType string, zones []string) (bool, string, []string) {
	if machineType == oldMachineType {
		return true, "", nil
	}

	for _, constraint := range constraints {
		if constraint.Name == machineType {
			ok, validValues := zonesCovered(zones, constraint.Zones)
			if !ok {
				return ok, machineType, validValues
			}
		}
	}

	return true, "", nil
}

// To check whether subzones are all covered by zones
func zonesCovered(subzones, zones []string) (bool, []string) {
	var (
		covered     bool
		validValues []string
	)

	for _, zoneS := range subzones {
		covered = false
		validValues = []string{}
		for _, zoneL := range zones {
			validValues = append(validValues, zoneL)
			if zoneS == zoneL {
				covered = true
				break
			}
		}
	}

	return covered, validValues
}

func validateVolumeTypes(constraints []garden.VolumeType, volumeType, oldVolumeType string) (bool, []string) {
	if volumeType == oldVolumeType {
		return true, nil
	}

	validValues := []string{}

	for _, v := range constraints {
		if v.Usable != nil && !*v.Usable {
			continue
		}
		validValues = append(validValues, v.Name)
		if v.Name == volumeType {
			return true, nil
		}
	}

	return false, validValues
}

func validateAlicloudVolumeTypes(constraints []garden.AlicloudVolumeType, volumeType, oldVolumeType string) (bool, []string) {
	volumeTypes := []garden.VolumeType{}
	for _, t := range constraints {
		volumeTypes = append(volumeTypes, t.VolumeType)
	}

	return validateVolumeTypes(volumeTypes, volumeType, oldVolumeType)
}

//To check whether volume type of worker is available in zones of the shoot,
//because in alicloud different zones may have different volume type
func validateAlicloudVolumeTypesAvailableInZones(constraints []garden.AlicloudVolumeType, volumeType, oldVolumeType string, zones []string) (bool, string, []string) {
	if volumeType == oldVolumeType {
		return true, "", nil
	}

	for _, constraint := range constraints {
		if constraint.Name == volumeType {
			ok, validValues := zonesCovered(zones, constraint.Zones)
			if !ok {
				return ok, volumeType, validValues
			}
		}
	}

	return true, "", nil
}

func validateZones(constraints []garden.Zone, region, zone string) (bool, []string) {
	validValues := []string{}

	for _, z := range constraints {
		if z.Region == region {
			for _, n := range z.Names {
				validValues = append(validValues, n)
				if n == zone {
					return true, nil
				}
			}
		}
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
func getDefaultMachineImage(machineImages []garden.MachineImage) (*garden.ShootMachineImage, error) {
	if len(machineImages) == 0 {
		return nil, errors.New("the cloud profile does not contain any machine image - cannot create shoot cluster")
	}
	firstMachineImageInCloudProfile := machineImages[0]
	latestMachineImageVersion, err := helper.DetermineLatestMachineImageVersion(firstMachineImageInCloudProfile)
	if err != nil {
		return nil, fmt.Errorf("failed to determine latest machine image from cloud profile: %s", err.Error())
	}
	return &garden.ShootMachineImage{Name: firstMachineImageInCloudProfile.Name, Version: latestMachineImageVersion.Version}, nil
}

func validateMachineImagesConstraints(constraints []garden.MachineImage, image, oldImage *garden.ShootMachineImage) (bool, []string) {
	if oldImage == nil || apiequality.Semantic.DeepEqual(*image, *oldImage) {
		return true, nil
	}

	validValues := []string{}
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
