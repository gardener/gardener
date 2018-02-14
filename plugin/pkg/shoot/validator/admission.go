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
}

var _ = admissioninitializer.WantsInternalGardenInformerFactory(&ValidateShoot{})

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
	h.shootLister = f.Garden().InternalVersion().Shoots().Lister()
}

// ValidateInitialization checks whether the plugin was correctly initialized.
func (h *ValidateShoot) ValidateInitialization() error {
	if h.cloudProfileLister == nil {
		return errors.New("missing cloudProfile lister")
	}
	if h.seedLister == nil {
		return errors.New("missing seed lister")
	}
	if h.shootLister == nil {
		return errors.New("missing c.shoot lister")
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

	cloudProviderInShoot, err := helper.DetermineCloudProviderInShoot(shoot.Spec.Cloud)
	if err != nil {
		return apierrors.NewBadRequest("could not find identify the cloud provider kind in the Shoot resource")
	}
	cloudProviderInProfile, err := helper.DetermineCloudProviderInProfile(cloudProfile.Spec)
	if err != nil {
		return apierrors.NewBadRequest("could not find identify the cloud provider kind in the referenced cloud profile")
	}

	if cloudProviderInShoot != cloudProviderInProfile {
		return apierrors.NewBadRequest("cloud provider in c.shoot is not equal to cloud provder in profile")
	}

	// We only want to validate fields in the Shoot against the CloudProfile/Seed constraints which have changed.
	// On CREATE operations we just use an empty Shoot object, forcing the validator functions to always validate.
	// On UPDATE operations we fetch the current Shoot object.
	var oldShoot *garden.Shoot
	if a.GetOperation() == admission.Create {
		oldShoot = &garden.Shoot{
			Spec: garden.ShootSpec{
				Cloud: garden.Cloud{
					AWS:       &garden.AWSCloud{},
					Azure:     &garden.AzureCloud{},
					GCP:       &garden.GCPCloud{},
					OpenStack: &garden.OpenStackCloud{},
				},
			},
		}
	} else if a.GetOperation() == admission.Update {
		old, err := h.shootLister.Shoots(shoot.Namespace).Get(shoot.Name)
		if err != nil {
			return apierrors.NewInternalError(errors.New("could not fetch the old c.shoot version"))
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
		allErrs = validateAWS(validationContext)
	case garden.CloudProviderAzure:
		allErrs = validateAzure(validationContext)
	case garden.CloudProviderGCP:
		allErrs = validateGCP(validationContext)
	case garden.CloudProviderOpenStack:
		allErrs = validateOpenStack(validationContext)
	}

	if len(allErrs) > 0 {
		return admission.NewForbidden(a, fmt.Errorf("%+v", allErrs))
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

	if ok := validateAWSMachineImage(c.cloudProfile.Spec.AWS.MachineImages, c.shoot.Spec.Cloud.Region); !ok {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "cloud", "region"), c.shoot.Spec.Cloud.Region, "no machine image known for this region"))
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

	for i, worker := range c.shoot.Spec.Cloud.OpenStack.Workers {
		var oldWorker = garden.OpenStackWorker{}
		for _, ow := range c.oldShoot.Spec.Cloud.OpenStack.Workers {
			if ow.Name == worker.Name {
				oldWorker = ow
				break
			}
		}

		idxPath := path.Child("workers").Index(i)
		if ok, validMachineTypes := validateMachineTypes(c.cloudProfile.Spec.OpenStack.Constraints.MachineTypes, worker.MachineType, oldWorker.MachineType); !ok {
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

func validateKubernetesVersionConstraints(constraints []string, version, oldVersion string) (bool, []string) {
	if version == oldVersion {
		return true, nil
	}

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

func validateMachineTypes(constraints []garden.MachineType, machineType, oldMachineType string) (bool, []string) {
	if machineType == oldMachineType {
		return true, nil
	}

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

func validateFloatingPoolConstraints(pools []garden.OpenStackFloatingPool, pool, oldPool string) (bool, []string) {
	if pool == oldPool {
		return true, nil
	}

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

func validateLoadBalancerProviderConstraints(providers []garden.OpenStackLoadBalancerProvider, provider, oldProvider string) (bool, []string) {
	if provider == oldProvider {
		return true, nil
	}

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
