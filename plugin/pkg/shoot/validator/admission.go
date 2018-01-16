// Copyright 2018 The Gardener Authors.
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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/admission"
)

// Register registers a plugin.
func Register(plugins *admission.Plugins) {
	plugins.Register("ShootValidator", func(config io.Reader) (admission.Interface, error) {
		return New()
	})
}

// ValidateShoot contains listers and and admission handler.
type ValidateShoot struct {
	*admission.Handler
	cloudProfileLister listers.CloudProfileLister
	seedLister         listers.SeedLister
}

var _ = admissioninitializer.WantsInternalGardenInformerFactory(&ValidateShoot{})

// New creates a new ValidateShoot admission plugin.
func New() (*ValidateShoot, error) {
	return &ValidateShoot{
		Handler: admission.NewHandler(admission.Create),
	}, nil
}

// SetInternalGardenInformerFactory gets Lister from SharedInformerFactory.
func (h *ValidateShoot) SetInternalGardenInformerFactory(f informers.SharedInformerFactory) {
	h.cloudProfileLister = f.Garden().InternalVersion().CloudProfiles().Lister()
	h.seedLister = f.Garden().InternalVersion().Seeds().Lister()
}

// ValidateInitialization checks whether the plugin was correctly initialized.
func (h *ValidateShoot) ValidateInitialization() error {
	if h.cloudProfileLister == nil {
		return errors.New("missing cloudProfile lister")
	}
	if h.seedLister == nil {
		return errors.New("missing seed lister")
	}
	return nil
}

// Admit ensures that the object in-flight is of kind Shoot.
// In addition it checks that the request resources are within the quota limits.
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
		return apierrors.NewBadRequest("could not convert resource into Shoot object")
	}

	cloudProfile, err := h.cloudProfileLister.Get(shoot.Spec.Cloud.Profile)
	if err != nil {
		return apierrors.NewBadRequest("could not find referenced cloud profile")
	}
	seed, err := h.seedLister.Get(*shoot.Spec.Cloud.Seed)
	if err != nil {
		return apierrors.NewBadRequest("could not find referenced seed")
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

	var allErrs field.ErrorList

	switch cloudProviderInShoot {
	case garden.CloudProviderAWS:
		allErrs = validateAWS(cloudProfile, seed, shoot)
	case garden.CloudProviderAzure:
		allErrs = validateAzure(cloudProfile, seed, shoot)
	case garden.CloudProviderGCP:
		allErrs = validateGCP(cloudProfile, seed, shoot)
	case garden.CloudProviderOpenStack:
		allErrs = validateOpenStack(cloudProfile, seed, shoot)
	}

	if len(allErrs) > 0 {
		return admission.NewForbidden(a, fmt.Errorf("%+v", allErrs))
	}

	return nil
}

// Cloud specific validation

func validateAWS(cloudProfile *garden.CloudProfile, seed *garden.Seed, shoot *garden.Shoot) field.ErrorList {
	var (
		allErrs field.ErrorList
		path    = field.NewPath("spec", "cloud", "aws")
	)

	if yes := networksIntersect(seed.Spec.Networks.Nodes, shoot.Spec.Cloud.AWS.Networks.K8SNetworks.Nodes); yes {
		allErrs = append(allErrs, field.Invalid(path.Child("networks", "nodes"), shoot.Spec.Cloud.AWS.Networks.K8SNetworks.Nodes, "shoot node network intersects with seed node network"))
	}
	if yes := networksIntersect(seed.Spec.Networks.Pods, shoot.Spec.Cloud.AWS.Networks.K8SNetworks.Pods); yes {
		allErrs = append(allErrs, field.Invalid(path.Child("networks", "nodes"), shoot.Spec.Cloud.AWS.Networks.K8SNetworks.Pods, "shoot pod network intersects with seed pod network"))
	}
	if yes := networksIntersect(seed.Spec.Networks.Services, shoot.Spec.Cloud.AWS.Networks.K8SNetworks.Services); yes {
		allErrs = append(allErrs, field.Invalid(path.Child("networks", "nodes"), shoot.Spec.Cloud.AWS.Networks.K8SNetworks.Services, "shoot service network intersects with seed service network"))
	}

	if ok, validDNSProviders := validateDNSConstraints(cloudProfile.Spec.AWS.Constraints.DNSProviders, shoot.Spec.DNS.Provider); !ok {
		allErrs = append(allErrs, field.NotSupported(field.NewPath("spec", "dns", "provider"), shoot.Spec.DNS.Provider, validDNSProviders))
	}
	if ok, validKubernetesVersions := validateKubernetesVersionConstraints(cloudProfile.Spec.AWS.Constraints.Kubernetes.Versions, shoot.Spec.Kubernetes.Version); !ok {
		allErrs = append(allErrs, field.NotSupported(field.NewPath("spec", "kubernetes", "version"), shoot.Spec.Kubernetes.Version, validKubernetesVersions))
	}

	for i, worker := range shoot.Spec.Cloud.AWS.Workers {
		idxPath := path.Child("workers").Index(i)
		if ok, validMachineTypes := validateMachineTypes(cloudProfile.Spec.AWS.Constraints.MachineTypes, worker.MachineType); !ok {
			allErrs = append(allErrs, field.NotSupported(idxPath.Child("machineType"), worker.MachineType, validMachineTypes))
		}
		if ok, validVolumeTypes := validateVolumeTypes(cloudProfile.Spec.AWS.Constraints.VolumeTypes, worker.VolumeType); !ok {
			allErrs = append(allErrs, field.NotSupported(idxPath.Child("volumeType"), worker.VolumeType, validVolumeTypes))
		}
	}

	for i, zone := range shoot.Spec.Cloud.AWS.Zones {
		idxPath := path.Child("zones").Index(i)
		if ok, validZones := validateZones(cloudProfile.Spec.AWS.Constraints.Zones, shoot.Spec.Cloud.Region, zone); !ok {
			if len(validZones) == 0 {
				allErrs = append(allErrs, field.Invalid(idxPath, shoot.Spec.Cloud.Region, "this region is not allowed"))
			} else {
				allErrs = append(allErrs, field.NotSupported(idxPath, zone, validZones))
			}
		}
	}

	if ok := validateAWSMachineImage(cloudProfile.Spec.AWS.MachineImages, shoot.Spec.Cloud.Region); !ok {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "cloud", "region"), shoot.Spec.Cloud.Region, "no machine image known for this region"))
	}

	return allErrs
}

func validateAzure(cloudProfile *garden.CloudProfile, seed *garden.Seed, shoot *garden.Shoot) field.ErrorList {
	var (
		allErrs field.ErrorList
		path    = field.NewPath("spec", "cloud", "azure")
	)

	if yes := networksIntersect(seed.Spec.Networks.Nodes, shoot.Spec.Cloud.Azure.Networks.K8SNetworks.Nodes); yes {
		allErrs = append(allErrs, field.Invalid(path.Child("networks", "nodes"), shoot.Spec.Cloud.Azure.Networks.K8SNetworks.Nodes, "shoot node network intersects with seed node network"))
	}
	if yes := networksIntersect(seed.Spec.Networks.Pods, shoot.Spec.Cloud.Azure.Networks.K8SNetworks.Pods); yes {
		allErrs = append(allErrs, field.Invalid(path.Child("networks", "nodes"), shoot.Spec.Cloud.Azure.Networks.K8SNetworks.Pods, "shoot pod network intersects with seed pod network"))
	}
	if yes := networksIntersect(seed.Spec.Networks.Services, shoot.Spec.Cloud.Azure.Networks.K8SNetworks.Services); yes {
		allErrs = append(allErrs, field.Invalid(path.Child("networks", "nodes"), shoot.Spec.Cloud.Azure.Networks.K8SNetworks.Services, "shoot service network intersects with seed service network"))
	}

	if ok, validDNSProviders := validateDNSConstraints(cloudProfile.Spec.Azure.Constraints.DNSProviders, shoot.Spec.DNS.Provider); !ok {
		allErrs = append(allErrs, field.NotSupported(field.NewPath("spec", "dns", "provider"), shoot.Spec.DNS.Provider, validDNSProviders))
	}
	if ok, validKubernetesVersions := validateKubernetesVersionConstraints(cloudProfile.Spec.Azure.Constraints.Kubernetes.Versions, shoot.Spec.Kubernetes.Version); !ok {
		allErrs = append(allErrs, field.NotSupported(field.NewPath("spec", "kubernetes", "version"), shoot.Spec.Kubernetes.Version, validKubernetesVersions))
	}

	for i, worker := range shoot.Spec.Cloud.Azure.Workers {
		idxPath := path.Child("workers").Index(i)
		if ok, validMachineTypes := validateMachineTypes(cloudProfile.Spec.Azure.Constraints.MachineTypes, worker.MachineType); !ok {
			allErrs = append(allErrs, field.NotSupported(idxPath.Child("machineType"), worker.MachineType, validMachineTypes))
		}
		if ok, validVolumeTypes := validateVolumeTypes(cloudProfile.Spec.Azure.Constraints.VolumeTypes, worker.VolumeType); !ok {
			allErrs = append(allErrs, field.NotSupported(idxPath.Child("volumeType"), worker.VolumeType, validVolumeTypes))
		}
	}

	if ok := validateAzureDomainCount(cloudProfile.Spec.Azure.CountFaultDomains, shoot.Spec.Cloud.Region); !ok {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "cloud", "region"), shoot.Spec.Cloud.Region, "no fault domain count known for this region"))
	}
	if ok := validateAzureDomainCount(cloudProfile.Spec.Azure.CountUpdateDomains, shoot.Spec.Cloud.Region); !ok {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "cloud", "region"), shoot.Spec.Cloud.Region, "no update domain count known for this region"))
	}

	return allErrs
}

func validateGCP(cloudProfile *garden.CloudProfile, seed *garden.Seed, shoot *garden.Shoot) field.ErrorList {
	var (
		allErrs field.ErrorList
		path    = field.NewPath("spec", "cloud", "gcp")
	)

	if yes := networksIntersect(seed.Spec.Networks.Nodes, shoot.Spec.Cloud.GCP.Networks.K8SNetworks.Nodes); yes {
		allErrs = append(allErrs, field.Invalid(path.Child("networks", "nodes"), shoot.Spec.Cloud.GCP.Networks.K8SNetworks.Nodes, "shoot node network intersects with seed node network"))
	}
	if yes := networksIntersect(seed.Spec.Networks.Pods, shoot.Spec.Cloud.GCP.Networks.K8SNetworks.Pods); yes {
		allErrs = append(allErrs, field.Invalid(path.Child("networks", "nodes"), shoot.Spec.Cloud.GCP.Networks.K8SNetworks.Pods, "shoot pod network intersects with seed pod network"))
	}
	if yes := networksIntersect(seed.Spec.Networks.Services, shoot.Spec.Cloud.GCP.Networks.K8SNetworks.Services); yes {
		allErrs = append(allErrs, field.Invalid(path.Child("networks", "nodes"), shoot.Spec.Cloud.GCP.Networks.K8SNetworks.Services, "shoot service network intersects with seed service network"))
	}

	if ok, validDNSProviders := validateDNSConstraints(cloudProfile.Spec.GCP.Constraints.DNSProviders, shoot.Spec.DNS.Provider); !ok {
		allErrs = append(allErrs, field.NotSupported(field.NewPath("spec", "dns", "provider"), shoot.Spec.DNS.Provider, validDNSProviders))
	}
	if ok, validKubernetesVersions := validateKubernetesVersionConstraints(cloudProfile.Spec.GCP.Constraints.Kubernetes.Versions, shoot.Spec.Kubernetes.Version); !ok {
		allErrs = append(allErrs, field.NotSupported(field.NewPath("spec", "kubernetes", "version"), shoot.Spec.Kubernetes.Version, validKubernetesVersions))
	}

	for i, worker := range shoot.Spec.Cloud.GCP.Workers {
		idxPath := path.Child("workers").Index(i)
		if ok, validMachineTypes := validateMachineTypes(cloudProfile.Spec.GCP.Constraints.MachineTypes, worker.MachineType); !ok {
			allErrs = append(allErrs, field.NotSupported(idxPath.Child("machineType"), worker.MachineType, validMachineTypes))
		}
		if ok, validVolumeTypes := validateVolumeTypes(cloudProfile.Spec.GCP.Constraints.VolumeTypes, worker.VolumeType); !ok {
			allErrs = append(allErrs, field.NotSupported(idxPath.Child("volumeType"), worker.VolumeType, validVolumeTypes))
		}
	}

	for i, zone := range shoot.Spec.Cloud.GCP.Zones {
		idxPath := path.Child("zones").Index(i)
		if ok, validZones := validateZones(cloudProfile.Spec.GCP.Constraints.Zones, shoot.Spec.Cloud.Region, zone); !ok {
			if len(validZones) == 0 {
				allErrs = append(allErrs, field.Invalid(idxPath, shoot.Spec.Cloud.Region, "this region is not allowed"))
			} else {
				allErrs = append(allErrs, field.NotSupported(idxPath, zone, validZones))
			}
		}
	}

	return allErrs
}

func validateOpenStack(cloudProfile *garden.CloudProfile, seed *garden.Seed, shoot *garden.Shoot) field.ErrorList {
	var (
		allErrs field.ErrorList
		path    = field.NewPath("spec", "cloud", "openstack")
	)

	if yes := networksIntersect(seed.Spec.Networks.Nodes, shoot.Spec.Cloud.OpenStack.Networks.K8SNetworks.Nodes); yes {
		allErrs = append(allErrs, field.Invalid(path.Child("networks", "nodes"), shoot.Spec.Cloud.OpenStack.Networks.K8SNetworks.Nodes, "shoot node network intersects with seed node network"))
	}
	if yes := networksIntersect(seed.Spec.Networks.Pods, shoot.Spec.Cloud.OpenStack.Networks.K8SNetworks.Pods); yes {
		allErrs = append(allErrs, field.Invalid(path.Child("networks", "nodes"), shoot.Spec.Cloud.OpenStack.Networks.K8SNetworks.Pods, "shoot pod network intersects with seed pod network"))
	}
	if yes := networksIntersect(seed.Spec.Networks.Services, shoot.Spec.Cloud.OpenStack.Networks.K8SNetworks.Services); yes {
		allErrs = append(allErrs, field.Invalid(path.Child("networks", "nodes"), shoot.Spec.Cloud.OpenStack.Networks.K8SNetworks.Services, "shoot service network intersects with seed service network"))
	}

	if ok, validDNSProviders := validateDNSConstraints(cloudProfile.Spec.OpenStack.Constraints.DNSProviders, shoot.Spec.DNS.Provider); !ok {
		allErrs = append(allErrs, field.NotSupported(field.NewPath("spec", "dns", "provider"), shoot.Spec.DNS.Provider, validDNSProviders))
	}
	if ok, validFloatingPools := validateFloatingPoolConstraints(cloudProfile.Spec.OpenStack.Constraints.FloatingPools, shoot.Spec.Cloud.OpenStack.FloatingPoolName); !ok {
		allErrs = append(allErrs, field.NotSupported(path.Child("floatingPoolName"), shoot.Spec.Cloud.OpenStack.FloatingPoolName, validFloatingPools))
	}
	if ok, validKubernetesVersions := validateKubernetesVersionConstraints(cloudProfile.Spec.OpenStack.Constraints.Kubernetes.Versions, shoot.Spec.Kubernetes.Version); !ok {
		allErrs = append(allErrs, field.NotSupported(field.NewPath("spec", "kubernetes", "version"), shoot.Spec.Kubernetes.Version, validKubernetesVersions))
	}
	if ok, validLoadBalancerProviders := validateLoadBalancerProviderConstraints(cloudProfile.Spec.OpenStack.Constraints.LoadBalancerProviders, shoot.Spec.Cloud.OpenStack.LoadBalancerProvider); !ok {
		allErrs = append(allErrs, field.NotSupported(path.Child("floatingPoolName"), shoot.Spec.Cloud.OpenStack.LoadBalancerProvider, validLoadBalancerProviders))
	}

	for i, worker := range shoot.Spec.Cloud.OpenStack.Workers {
		idxPath := path.Child("workers").Index(i)
		if ok, validMachineTypes := validateMachineTypes(cloudProfile.Spec.OpenStack.Constraints.MachineTypes, worker.MachineType); !ok {
			allErrs = append(allErrs, field.NotSupported(idxPath.Child("machineType"), worker.MachineType, validMachineTypes))
		}
	}

	for i, zone := range shoot.Spec.Cloud.OpenStack.Zones {
		idxPath := path.Child("zones").Index(i)
		if ok, validZones := validateZones(cloudProfile.Spec.OpenStack.Constraints.Zones, shoot.Spec.Cloud.Region, zone); !ok {
			if len(validZones) == 0 {
				allErrs = append(allErrs, field.Invalid(idxPath, shoot.Spec.Cloud.Region, "this region is not allowed"))
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

func validateDNSConstraints(constraints []garden.DNSProviderConstraint, provider garden.DNSProvider) (bool, []string) {
	var (
		validValues = []string{}
		ok          = false
	)

	for _, p := range constraints {
		validValues = append(validValues, string(p.Name))
		if p.Name == provider {
			ok = true
		}
	}

	return ok, validValues
}

func validateKubernetesVersionConstraints(constraints []string, version string) (bool, []string) {
	var (
		validValues = []string{}
		ok          = false
	)

	for _, v := range constraints {
		validValues = append(validValues, v)
		if v == version {
			ok = true
		}
	}

	return ok, validValues
}

func validateMachineTypes(constraints []garden.MachineType, machineType string) (bool, []string) {
	var (
		validValues = []string{}
		ok          = false
	)

	for _, t := range constraints {
		validValues = append(validValues, t.Name)
		if t.Name == machineType {
			ok = true
		}
	}

	return ok, validValues
}

func validateVolumeTypes(constraints []garden.VolumeType, volumeType string) (bool, []string) {
	var (
		validValues = []string{}
		ok          = false
	)

	for _, v := range constraints {
		validValues = append(validValues, v.Name)
		if v.Name == volumeType {
			ok = true
		}
	}

	return ok, validValues
}

func validateZones(constraints []garden.Zone, region, zone string) (bool, []string) {
	var (
		validValues = []string{}
		ok          = false
	)

	for _, z := range constraints {
		if z.Region == region {
			for _, n := range z.Names {
				validValues = append(validValues, n)
				if n == zone {
					ok = true
				}
			}
		}
	}

	return ok, validValues
}

func validateAWSMachineImage(images []garden.AWSMachineImage, region string) bool {
	for _, i := range images {
		if i.Region == region {
			return true
		}
	}
	return false
}

func validateAzureDomainCount(count []garden.AzureDomainCount, region string) bool {
	for _, c := range count {
		if c.Region == region {
			return true
		}
	}
	return false
}

func validateFloatingPoolConstraints(pools []garden.OpenStackFloatingPool, pool string) (bool, []string) {
	var (
		validValues = []string{}
		ok          = false
	)

	for _, p := range pools {
		validValues = append(validValues, p.Name)
		if p.Name == pool {
			ok = true
		}
	}

	return ok, validValues
}

func validateLoadBalancerProviderConstraints(providers []garden.OpenStackLoadBalancerProvider, provider string) (bool, []string) {
	var (
		validValues = []string{}
		ok          = false
	)

	for _, p := range providers {
		validValues = append(validValues, p.Name)
		if p.Name == provider {
			ok = true
		}
	}

	return ok, validValues
}
