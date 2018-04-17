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
	"net"

	"github.com/gardener/gardener/pkg/apis/garden"
	"github.com/gardener/gardener/pkg/apis/garden/helper"
	admissioninitializer "github.com/gardener/gardener/pkg/apiserver/admission/initializer"
	informers "github.com/gardener/gardener/pkg/client/garden/informers/internalversion"
	listers "github.com/gardener/gardener/pkg/client/garden/listers/garden/internalversion"
	"github.com/gardener/gardener/pkg/operation/common"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/admission"
	kubeinformers "k8s.io/client-go/informers"
	kubecorev1listers "k8s.io/client-go/listers/core/v1"
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
	namespaceLister    kubecorev1listers.NamespaceLister
}

var _ = admissioninitializer.WantsInternalGardenInformerFactory(&ValidateShoot{})
var _ = admissioninitializer.WantsKubeInformerFactory(&ValidateShoot{})

// New creates a new ValidateShoot admission plugin.
func New() (*ValidateShoot, error) {
	return &ValidateShoot{
		Handler: admission.NewHandler(admission.Create, admission.Update),
	}, nil
}

// SetInternalGardenInformerFactory gets Lister from SharedInformerFactory.
func (h *ValidateShoot) SetInternalGardenInformerFactory(f informers.SharedInformerFactory) {
	h.cloudProfileLister = f.Garden().InternalVersion().CloudProfiles().Lister()
	h.seedLister = f.Garden().InternalVersion().Seeds().Lister()
}

// SetKubeInformerFactory gets Lister from SharedInformerFactory.
func (h *ValidateShoot) SetKubeInformerFactory(f kubeinformers.SharedInformerFactory) {
	h.namespaceLister = f.Core().V1().Namespaces().Lister()
}

// ValidateInitialization checks whether the plugin was correctly initialized.
func (h *ValidateShoot) ValidateInitialization() error {
	if h.cloudProfileLister == nil {
		return errors.New("missing cloudProfile lister")
	}
	if h.seedLister == nil {
		return errors.New("missing seed lister")
	}
	if h.namespaceLister == nil {
		return errors.New("missing namespace lister")
	}
	return nil
}

// Admit validates the Shoot details against the referenced CloudProfile.
func (h *ValidateShoot) Admit(a admission.Attributes) error {
	// Wait until the caches have been synced
	if !h.WaitForReady() {
		return admission.NewForbidden(a, errors.New("not yet ready to handle request"))
	}

	// Ignore all kinds other than Shoot
	if a.GetKind().GroupKind() != garden.Kind("Shoot") {
		return nil
	}
	shoot, ok := a.GetObject().(*garden.Shoot)
	if !ok {
		return apierrors.NewInternalError(errors.New("could not convert resource into Shoot object"))
	}

	cloudProfile, err := h.cloudProfileLister.Get(shoot.Spec.Cloud.Profile)
	if err != nil {
		return apierrors.NewBadRequest("could not find referenced cloud profile")
	}
	seed, err := h.seedLister.Get(*shoot.Spec.Cloud.Seed)
	if err != nil {
		return apierrors.NewBadRequest("could not find referenced seed")
	}
	namespace, err := h.namespaceLister.Get(shoot.Namespace)
	if err != nil {
		return apierrors.NewBadRequest("could not find referenced namespace")
	}

	// We use the identifier "shoot-<project-name>-<shoot-name> in nearly all places: when creating infrastructure
	// resources, Kubernetes resources, DNS names, etc. Some infrastructure resources have length constraints that
	// this identifier must not exceed 30 characters, thus we need to check whether Shoots do not exceed this limit.
	// The project name is a label on the namespace. If it is not found, the namespace name itself is used as project
	// name.
	var (
		projectName = shoot.Namespace
		lengthLimit = 23
	)
	if projectNameLabel, ok := namespace.Labels[common.ProjectName]; ok {
		projectName = projectNameLabel
	}
	if len(projectName+shoot.Name) > lengthLimit {
		return apierrors.NewBadRequest(fmt.Sprintf("the length of the shoot name and the project name must not exceed %d characters (project: %s; shoot: %s)", lengthLimit, projectName, shoot.Name))
	}

	cloudProviderInShoot, err := helper.DetermineCloudProviderInShoot(shoot.Spec.Cloud)
	if err != nil {
		return apierrors.NewBadRequest("could not find identify the cloud provider kind in the Shoot resource")
	}
	cloudProviderInProfile, err := helper.DetermineCloudProviderInProfile(cloudProfile.Spec)
	if err != nil {
		return apierrors.NewBadRequest("could not find identify the cloud provider kind in the referenced cloud profile")
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
						MachineImage: &garden.AWSMachineImage{},
					},
					Azure: &garden.AzureCloud{
						MachineImage: &garden.AzureMachineImage{},
					},
					GCP: &garden.GCPCloud{
						MachineImage: &garden.GCPMachineImage{},
					},
					OpenStack: &garden.OpenStackCloud{
						MachineImage: &garden.OpenStackMachineImage{},
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

	switch cloudProviderInShoot {
	case garden.CloudProviderAWS:
		if shoot.Spec.Cloud.AWS.MachineImage == nil {
			image, err := getAWSMachineImage(shoot, cloudProfile)
			if err != nil {
				return apierrors.NewBadRequest(err.Error())
			}
			shoot.Spec.Cloud.AWS.MachineImage = image
		}
		allErrs = validateAWS(validationContext)

	case garden.CloudProviderAzure:
		if shoot.Spec.Cloud.Azure.MachineImage == nil {
			image, err := getAzureMachineImage(shoot, cloudProfile)
			if err != nil {
				return apierrors.NewBadRequest(err.Error())
			}
			shoot.Spec.Cloud.Azure.MachineImage = image
		}
		allErrs = validateAzure(validationContext)

	case garden.CloudProviderGCP:
		if shoot.Spec.Cloud.GCP.MachineImage == nil {
			image, err := getGCPMachineImage(shoot, cloudProfile)
			if err != nil {
				return apierrors.NewBadRequest(err.Error())
			}
			shoot.Spec.Cloud.GCP.MachineImage = image
		}
		allErrs = validateGCP(validationContext)

	case garden.CloudProviderOpenStack:
		if shoot.Spec.Cloud.OpenStack.MachineImage == nil {
			image, err := getOpenStackMachineImage(shoot, cloudProfile)
			if err != nil {
				return apierrors.NewBadRequest(err.Error())
			}
			shoot.Spec.Cloud.OpenStack.MachineImage = image
		}
		allErrs = validateOpenStack(validationContext)
	}

	if len(allErrs) > 0 {
		return admission.NewForbidden(a, fmt.Errorf("%+v", allErrs))
	}

	// Add createdBy annotation to Shoot
	if a.GetOperation() == admission.Create {
		annotations := shoot.Annotations
		if annotations == nil {
			annotations = map[string]string{}
		}
		annotations[common.GardenCreatedBy] = a.GetUserInfo().GetName()
		shoot.Annotations = annotations
	}

	return nil
}

// Cloud specific validation

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

	allErrs = append(allErrs, validateNetworkDisjointedness(c.seed.Spec.Networks, c.shoot.Spec.Cloud.AWS.Networks.K8SNetworks, path.Child("networks"))...)

	if ok, validDNSProviders := validateDNSConstraints(c.cloudProfile.Spec.AWS.Constraints.DNSProviders, c.shoot.Spec.DNS.Provider, c.oldShoot.Spec.DNS.Provider); !ok {
		allErrs = append(allErrs, field.NotSupported(field.NewPath("spec", "dns", "provider"), c.shoot.Spec.DNS.Provider, validDNSProviders))
	}
	if ok, validKubernetesVersions := validateKubernetesVersionConstraints(c.cloudProfile.Spec.AWS.Constraints.Kubernetes.Versions, c.shoot.Spec.Kubernetes.Version, c.oldShoot.Spec.Kubernetes.Version); !ok {
		allErrs = append(allErrs, field.NotSupported(field.NewPath("spec", "kubernetes", "version"), c.shoot.Spec.Kubernetes.Version, validKubernetesVersions))
	}
	if ok, validMachineImages := validateAWSMachineImagesConstraints(c.cloudProfile.Spec.AWS.Constraints.MachineImages, c.shoot.Spec.Cloud.Region, c.shoot.Spec.Cloud.AWS.MachineImage, c.oldShoot.Spec.Cloud.AWS.MachineImage); !ok {
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

	allErrs = append(allErrs, validateNetworkDisjointedness(c.seed.Spec.Networks, c.shoot.Spec.Cloud.Azure.Networks.K8SNetworks, path.Child("networks"))...)

	if ok, validDNSProviders := validateDNSConstraints(c.cloudProfile.Spec.Azure.Constraints.DNSProviders, c.shoot.Spec.DNS.Provider, c.oldShoot.Spec.DNS.Provider); !ok {
		allErrs = append(allErrs, field.NotSupported(field.NewPath("spec", "dns", "provider"), c.shoot.Spec.DNS.Provider, validDNSProviders))
	}
	if ok, validKubernetesVersions := validateKubernetesVersionConstraints(c.cloudProfile.Spec.Azure.Constraints.Kubernetes.Versions, c.shoot.Spec.Kubernetes.Version, c.oldShoot.Spec.Kubernetes.Version); !ok {
		allErrs = append(allErrs, field.NotSupported(field.NewPath("spec", "kubernetes", "version"), c.shoot.Spec.Kubernetes.Version, validKubernetesVersions))
	}
	if ok, validMachineImages := validateAzureMachineImagesConstraints(c.cloudProfile.Spec.Azure.Constraints.MachineImages, c.shoot.Spec.Cloud.Azure.MachineImage, c.oldShoot.Spec.Cloud.Azure.MachineImage); !ok {
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

	allErrs = append(allErrs, validateNetworkDisjointedness(c.seed.Spec.Networks, c.shoot.Spec.Cloud.GCP.Networks.K8SNetworks, path.Child("networks"))...)

	if ok, validDNSProviders := validateDNSConstraints(c.cloudProfile.Spec.GCP.Constraints.DNSProviders, c.shoot.Spec.DNS.Provider, c.oldShoot.Spec.DNS.Provider); !ok {
		allErrs = append(allErrs, field.NotSupported(field.NewPath("spec", "dns", "provider"), c.shoot.Spec.DNS.Provider, validDNSProviders))
	}
	if ok, validKubernetesVersions := validateKubernetesVersionConstraints(c.cloudProfile.Spec.GCP.Constraints.Kubernetes.Versions, c.shoot.Spec.Kubernetes.Version, c.oldShoot.Spec.Kubernetes.Version); !ok {
		allErrs = append(allErrs, field.NotSupported(field.NewPath("spec", "kubernetes", "version"), c.shoot.Spec.Kubernetes.Version, validKubernetesVersions))
	}
	if ok, validMachineImages := validateGCPMachineImagesConstraints(c.cloudProfile.Spec.GCP.Constraints.MachineImages, c.shoot.Spec.Cloud.GCP.MachineImage, c.oldShoot.Spec.Cloud.GCP.MachineImage); !ok {
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

func validateOpenStack(c *validationContext) field.ErrorList {
	var (
		allErrs = field.ErrorList{}
		path    = field.NewPath("spec", "cloud", "openstack")
	)

	allErrs = append(allErrs, validateNetworkDisjointedness(c.seed.Spec.Networks, c.shoot.Spec.Cloud.OpenStack.Networks.K8SNetworks, path.Child("networks"))...)

	if ok, validDNSProviders := validateDNSConstraints(c.cloudProfile.Spec.OpenStack.Constraints.DNSProviders, c.shoot.Spec.DNS.Provider, c.oldShoot.Spec.DNS.Provider); !ok {
		allErrs = append(allErrs, field.NotSupported(field.NewPath("spec", "dns", "provider"), c.shoot.Spec.DNS.Provider, validDNSProviders))
	}
	if ok, validFloatingPools := validateFloatingPoolConstraints(c.cloudProfile.Spec.OpenStack.Constraints.FloatingPools, c.shoot.Spec.Cloud.OpenStack.FloatingPoolName, c.oldShoot.Spec.Cloud.OpenStack.FloatingPoolName); !ok {
		allErrs = append(allErrs, field.NotSupported(path.Child("floatingPoolName"), c.shoot.Spec.Cloud.OpenStack.FloatingPoolName, validFloatingPools))
	}
	if ok, validKubernetesVersions := validateKubernetesVersionConstraints(c.cloudProfile.Spec.OpenStack.Constraints.Kubernetes.Versions, c.shoot.Spec.Kubernetes.Version, c.oldShoot.Spec.Kubernetes.Version); !ok {
		allErrs = append(allErrs, field.NotSupported(field.NewPath("spec", "kubernetes", "version"), c.shoot.Spec.Kubernetes.Version, validKubernetesVersions))
	}
	if ok, validLoadBalancerProviders := validateLoadBalancerProviderConstraints(c.cloudProfile.Spec.OpenStack.Constraints.LoadBalancerProviders, c.shoot.Spec.Cloud.OpenStack.LoadBalancerProvider, c.oldShoot.Spec.Cloud.OpenStack.LoadBalancerProvider); !ok {
		allErrs = append(allErrs, field.NotSupported(path.Child("floatingPoolName"), c.shoot.Spec.Cloud.OpenStack.LoadBalancerProvider, validLoadBalancerProviders))
	}
	if ok, validMachineImages := validateOpenStackMachineImagesConstraints(c.cloudProfile.Spec.OpenStack.Constraints.MachineImages, c.shoot.Spec.Cloud.OpenStack.MachineImage, c.oldShoot.Spec.Cloud.OpenStack.MachineImage); !ok {
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

// Helper functions

func networksIntersect(cidr1, cidr2 garden.CIDR) bool {
	_, net1, err1 := net.ParseCIDR(string(cidr1))
	_, net2, err2 := net.ParseCIDR(string(cidr2))
	return err1 != nil || err2 != nil || net2.Contains(net1.IP) || net1.Contains(net2.IP)
}

func validateDNSConstraints(constraints []garden.DNSProviderConstraint, provider, oldProvider garden.DNSProvider) (bool, []string) {
	if provider == oldProvider {
		return true, nil
	}

	validValues := []string{}

	for _, p := range constraints {
		validValues = append(validValues, string(p.Name))
		if p.Name == provider {
			return true, nil
		}
	}

	return false, validValues
}

func validateKubernetesVersionConstraints(constraints []string, version, oldVersion string) (bool, []string) {
	if version == oldVersion {
		return true, nil
	}

	validValues := []string{}

	for _, v := range constraints {
		validValues = append(validValues, v)
		if v == version {
			return true, nil
		}
	}

	return false, validValues
}

func validateMachineTypes(constraints []garden.MachineType, machineType, oldMachineType string) (bool, []string) {
	if machineType == oldMachineType {
		return true, nil
	}

	validValues := []string{}

	for _, t := range constraints {
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

func validateNetworkDisjointedness(seedNetworks garden.SeedNetworks, k8sNetworks garden.K8SNetworks, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if yes := networksIntersect(seedNetworks.Nodes, *k8sNetworks.Nodes); yes {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("nodes"), *k8sNetworks.Nodes, "shoot node network intersects with seed node network"))
	}
	if yes := networksIntersect(seedNetworks.Pods, *k8sNetworks.Pods); yes {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("nodes"), *k8sNetworks.Pods, "shoot pod network intersects with seed pod network"))
	}
	if yes := networksIntersect(seedNetworks.Services, *k8sNetworks.Services); yes {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("nodes"), *k8sNetworks.Services, "shoot service network intersects with seed service network"))
	}

	return allErrs
}

func validateVolumeTypes(constraints []garden.VolumeType, volumeType, oldVolumeType string) (bool, []string) {
	if volumeType == oldVolumeType {
		return true, nil
	}

	validValues := []string{}

	for _, v := range constraints {
		validValues = append(validValues, v.Name)
		if v.Name == volumeType {
			return true, nil
		}
	}

	return false, validValues
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

// Machine Image Helper functions

func getAWSMachineImage(shoot *garden.Shoot, cloudProfile *garden.CloudProfile) (*garden.AWSMachineImage, error) {
	machineImageMappings := cloudProfile.Spec.AWS.Constraints.MachineImages
	if len(machineImageMappings) != 1 {
		return nil, errors.New("must provide a value for .spec.cloud.aws.machineImage as the referenced cloud profile contains more than one")
	}

	return findAWSMachineImageForRegion(machineImageMappings[0], shoot.Spec.Cloud.Region)
}

func findAWSMachineImageForRegion(machineImageMapping garden.AWSMachineImageMapping, region string) (*garden.AWSMachineImage, error) {
	for _, regionalMachineImage := range machineImageMapping.Regions {
		if regionalMachineImage.Name == region {
			return &garden.AWSMachineImage{
				Name: machineImageMapping.Name,
				AMI:  regionalMachineImage.AMI,
			}, nil
		}
	}
	return nil, fmt.Errorf("could not find an AMI for region %s and machine image %s", region, machineImageMapping.Name)
}

func validateAWSMachineImagesConstraints(constraints []garden.AWSMachineImageMapping, region string, image, oldImage *garden.AWSMachineImage) (bool, []string) {
	if apiequality.Semantic.DeepEqual(*image, *oldImage) {
		return true, nil
	}

	validValues := []string{}

	for _, v := range constraints {
		machineImage, err := findAWSMachineImageForRegion(v, region)
		if err != nil {
			return false, nil
		}

		validValues = append(validValues, fmt.Sprintf("%+v", *machineImage))

		if apiequality.Semantic.DeepEqual(*machineImage, *image) {
			return true, nil
		}
	}

	return false, validValues
}

func getAzureMachineImage(shoot *garden.Shoot, cloudProfile *garden.CloudProfile) (*garden.AzureMachineImage, error) {
	machineImages := cloudProfile.Spec.Azure.Constraints.MachineImages
	if len(machineImages) != 1 {
		return nil, errors.New("must provide a value for .spec.cloud.azure.machineImage as the referenced cloud profile contains more than one")
	}
	return &machineImages[0], nil
}

func validateAzureMachineImagesConstraints(constraints []garden.AzureMachineImage, image, oldImage *garden.AzureMachineImage) (bool, []string) {
	if apiequality.Semantic.DeepEqual(*image, *oldImage) {
		return true, nil
	}

	validValues := []string{}

	for _, v := range constraints {
		validValues = append(validValues, fmt.Sprintf("%+v", v))
		if apiequality.Semantic.DeepEqual(v, *image) {
			return true, nil
		}
	}

	return false, validValues
}

func getGCPMachineImage(shoot *garden.Shoot, cloudProfile *garden.CloudProfile) (*garden.GCPMachineImage, error) {
	machineImages := cloudProfile.Spec.GCP.Constraints.MachineImages
	if len(machineImages) != 1 {
		return nil, errors.New("must provide a value for .spec.cloud.gcp.machineImage as the referenced cloud profile contains more than one")
	}
	return &machineImages[0], nil
}

func validateGCPMachineImagesConstraints(constraints []garden.GCPMachineImage, image, oldImage *garden.GCPMachineImage) (bool, []string) {
	if apiequality.Semantic.DeepEqual(*image, *oldImage) {
		return true, nil
	}

	validValues := []string{}

	for _, v := range constraints {
		validValues = append(validValues, fmt.Sprintf("%+v", v))
		if apiequality.Semantic.DeepEqual(v, *image) {
			return true, nil
		}
	}

	return false, validValues
}

func getOpenStackMachineImage(shoot *garden.Shoot, cloudProfile *garden.CloudProfile) (*garden.OpenStackMachineImage, error) {
	machineImages := cloudProfile.Spec.OpenStack.Constraints.MachineImages
	if len(machineImages) != 1 {
		return nil, errors.New("must provide a value for .spec.cloud.openstack.machineImage as the referenced cloud profile contains more than one")
	}
	return &machineImages[0], nil
}

func validateOpenStackMachineImagesConstraints(constraints []garden.OpenStackMachineImage, image, oldImage *garden.OpenStackMachineImage) (bool, []string) {
	if apiequality.Semantic.DeepEqual(*image, *oldImage) {
		return true, nil
	}

	validValues := []string{}

	for _, v := range constraints {
		validValues = append(validValues, fmt.Sprintf("%+v", v))
		if apiequality.Semantic.DeepEqual(v, *image) {
			return true, nil
		}
	}

	return false, validValues
}
