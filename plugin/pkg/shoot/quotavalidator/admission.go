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

package quotavalidator

import (
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/gardener/gardener/pkg/apis/garden"
	"github.com/gardener/gardener/pkg/apis/garden/helper"
	admissioninitializer "github.com/gardener/gardener/pkg/apiserver/admission/initializer"
	informers "github.com/gardener/gardener/pkg/client/garden/informers/internalversion"
	listers "github.com/gardener/gardener/pkg/client/garden/listers/garden/internalversion"
	"github.com/gardener/gardener/pkg/operation/common"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apiserver/pkg/admission"
)

const (
	// PluginName is the name of this admission plugin.
	PluginName = "ShootQuotaValidator"
)

var (
	quotaMetricNames = [6]corev1.ResourceName{
		garden.QuotaMetricCPU,
		garden.QuotaMetricGPU,
		garden.QuotaMetricMemory,
		garden.QuotaMetricStorageStandard,
		garden.QuotaMetricStoragePremium,
		garden.QuotaMetricLoadbalancer}
)

type quotaWorker struct {
	garden.Worker
	// VolumeType is the type of the root volumes.
	VolumeType string
	// VolumeSize is the size of the root volume.
	VolumeSize resource.Quantity
}

// Register registers a plugin.
func Register(plugins *admission.Plugins) {
	plugins.Register(PluginName, func(config io.Reader) (admission.Interface, error) {
		return New()
	})
}

// QuotaValidator contains listers and and admission handler.
type QuotaValidator struct {
	*admission.Handler
	shootLister         listers.ShootLister
	cloudProfileLister  listers.CloudProfileLister
	secretBindingLister listers.SecretBindingLister
	quotaLister         listers.QuotaLister
	readyFunc           admission.ReadyFunc
}

var (
	_ = admissioninitializer.WantsInternalGardenInformerFactory(&QuotaValidator{})

	readyFuncs = []admission.ReadyFunc{}
)

// New creates a new QuotaValidator admission plugin.
func New() (*QuotaValidator, error) {
	return &QuotaValidator{
		Handler: admission.NewHandler(admission.Create, admission.Update),
	}, nil
}

// AssignReadyFunc assigns the ready function to the admission handler.
func (q *QuotaValidator) AssignReadyFunc(f admission.ReadyFunc) {
	q.readyFunc = f
	q.SetReadyFunc(f)
}

// SetInternalGardenInformerFactory gets Lister from SharedInformerFactory.
func (q *QuotaValidator) SetInternalGardenInformerFactory(f informers.SharedInformerFactory) {
	shootInformer := f.Garden().InternalVersion().Shoots()
	q.shootLister = shootInformer.Lister()

	cloudProfileInformer := f.Garden().InternalVersion().CloudProfiles()
	q.cloudProfileLister = cloudProfileInformer.Lister()

	secretBindingInformer := f.Garden().InternalVersion().SecretBindings()
	q.secretBindingLister = secretBindingInformer.Lister()

	quotaInformer := f.Garden().InternalVersion().Quotas()
	q.quotaLister = quotaInformer.Lister()

	readyFuncs = append(readyFuncs, shootInformer.Informer().HasSynced, cloudProfileInformer.Informer().HasSynced, secretBindingInformer.Informer().HasSynced, quotaInformer.Informer().HasSynced)
}

// ValidateInitialization checks whether the plugin was correctly initialized.
func (q *QuotaValidator) ValidateInitialization() error {
	if q.shootLister == nil {
		return errors.New("missing shoot lister")
	}
	if q.cloudProfileLister == nil {
		return errors.New("missing cloudProfile lister")
	}
	if q.secretBindingLister == nil {
		return errors.New("missing secretBinding lister")
	}
	if q.quotaLister == nil {
		return errors.New("missing quota lister")
	}
	return nil
}

// Admit checks that the requested Shoot resources are within the quota limits.
func (q *QuotaValidator) Admit(a admission.Attributes, o admission.ObjectInterfaces) error {
	// Wait until the caches have been synced
	if q.readyFunc == nil {
		q.AssignReadyFunc(func() bool {
			for _, readyFunc := range readyFuncs {
				if !readyFunc() {
					return false
				}
			}
			return true
		})
	}
	if !q.WaitForReady() {
		return admission.NewForbidden(a, errors.New("not yet ready to handle request"))
	}

	// Ignore all kinds other than Shoot
	if a.GetKind().GroupKind() != garden.Kind("Shoot") {
		return nil
	}
	if a.GetSubresource() != "" {
		return nil
	}

	shoot, ok := a.GetObject().(*garden.Shoot)
	if !ok {
		return apierrors.NewBadRequest("could not convert resource into Shoot object")
	}

	// Pass if the shoot is intended to get deleted
	if shoot.DeletionTimestamp != nil {
		return nil
	}

	var (
		oldShoot         *garden.Shoot
		maxShootLifetime *int
		checkLifetime    = false
		checkQuota       = false
	)

	if a.GetOperation() == admission.Create {
		checkQuota = true
	}

	if a.GetOperation() == admission.Update {
		oldShoot, ok = a.GetOldObject().(*garden.Shoot)
		if !ok {
			return apierrors.NewBadRequest("could not convert resource into Shoot object")
		}
		cloudProvider, err := helper.DetermineCloudProviderInShoot(shoot.Spec.Cloud)
		if err != nil {
			return apierrors.NewInternalError(err)
		}

		checkQuota = quotaVerificationNeeded(*shoot, *oldShoot, cloudProvider)
		checkLifetime = lifetimeVerificationNeeded(*shoot, *oldShoot)
	}

	secretBinding, err := q.secretBindingLister.SecretBindings(shoot.Namespace).Get(shoot.Spec.Cloud.SecretBindingRef.Name)
	if err != nil {
		return apierrors.NewInternalError(err)
	}

	// Quotas are cumulative, means each quota must be not exceeded that the admission pass.
	for _, quotaRef := range secretBinding.Quotas {
		quota, err := q.quotaLister.Quotas(quotaRef.Namespace).Get(quotaRef.Name)
		if err != nil {
			return apierrors.NewInternalError(err)
		}

		// Get the max clusterLifeTime
		if checkLifetime && quota.Spec.ClusterLifetimeDays != nil {
			if maxShootLifetime == nil {
				maxShootLifetime = quota.Spec.ClusterLifetimeDays
			}
			if *maxShootLifetime > *quota.Spec.ClusterLifetimeDays {
				maxShootLifetime = quota.Spec.ClusterLifetimeDays
			}
		}

		if checkQuota {
			exceededMetrics, err := q.isQuotaExceeded(*shoot, *quota)
			if err != nil {
				return apierrors.NewInternalError(err)
			}
			if exceededMetrics != nil {
				message := ""
				for _, metric := range *exceededMetrics {
					message = message + metric.String() + " "
				}
				return admission.NewForbidden(a, fmt.Errorf("Quota limits exceeded. Unable to allocate further %s", message))
			}
		}
	}

	// Admit Shoot lifetime changes
	if lifetime, exists := shoot.Annotations[common.ShootExpirationTimestamp]; checkLifetime && exists && maxShootLifetime != nil {
		var (
			plannedExpirationTime     time.Time
			oldExpirationTime         time.Time
			maxPossibleExpirationTime time.Time
		)

		plannedExpirationTime, err = time.Parse(time.RFC3339, lifetime)
		if err != nil {
			return apierrors.NewInternalError(err)
		}

		oldLifetime, exists := oldShoot.Annotations[common.ShootExpirationTimestamp]
		if !exists {
			// The old version of the Shoot has no clusterLifetime annotation yet.
			// Therefore we have to calculate the lifetime based on the maxShootLifetime.
			oldLifetime = oldShoot.CreationTimestamp.Time.Add(time.Duration(*maxShootLifetime*24) * time.Hour).Format(time.RFC3339)
		}
		oldExpirationTime, err = time.Parse(time.RFC3339, oldLifetime)
		if err != nil {
			return apierrors.NewInternalError(err)
		}

		maxPossibleExpirationTime = oldExpirationTime.Add(time.Duration(*maxShootLifetime*24) * time.Hour)
		if plannedExpirationTime.After(maxPossibleExpirationTime) {
			return admission.NewForbidden(a, fmt.Errorf("Requested shoot expiration time to long. Can only be extended by %d day(s)", *maxShootLifetime))
		}
	}

	return nil
}

func (q *QuotaValidator) isQuotaExceeded(shoot garden.Shoot, quota garden.Quota) (*[]corev1.ResourceName, error) {
	allocatedResources, err := q.determineAllocatedResources(quota, shoot)
	if err != nil {
		return nil, err
	}
	requiredResources, err := q.determineRequiredResources(allocatedResources, shoot)
	if err != nil {
		return nil, err
	}

	exceededMetrics := make([]corev1.ResourceName, 0)
	for _, metric := range quotaMetricNames {
		if _, ok := quota.Spec.Metrics[metric]; !ok {
			continue
		}
		if !hasSufficientQuota(quota.Spec.Metrics[metric], requiredResources[metric]) {
			exceededMetrics = append(exceededMetrics, metric)
		}
	}
	if len(exceededMetrics) != 0 {
		return &exceededMetrics, nil
	}
	return nil, nil
}

func (q *QuotaValidator) determineAllocatedResources(quota garden.Quota, shoot garden.Shoot) (corev1.ResourceList, error) {
	shoots, err := q.findShootsReferQuota(quota, shoot)
	if err != nil {
		return nil, err
	}

	// Collect the resources which are allocated according to the shoot specs
	allocatedResources := make(corev1.ResourceList)
	for _, s := range shoots {
		shootResources, err := q.getShootResources(s)
		if err != nil {
			return nil, err
		}
		for _, metric := range quotaMetricNames {
			allocatedResources[metric] = sumQuantity(allocatedResources[metric], shootResources[metric])
		}
	}

	// TODO: We have to determine and add the amount of storage, which is allocated by manually created persistent volumes
	// and the count of loadbalancer, which are created due to manually created services of type loadbalancer

	return allocatedResources, nil
}

func (q *QuotaValidator) findShootsReferQuota(quota garden.Quota, shoot garden.Shoot) ([]garden.Shoot, error) {
	var (
		shootsReferQuota []garden.Shoot
		secretBindings   []garden.SecretBinding
	)

	namespace := corev1.NamespaceAll
	if quota.Spec.Scope == garden.QuotaScopeProject {
		namespace = shoot.Namespace
	}
	allSecretBindings, err := q.secretBindingLister.SecretBindings(namespace).List(labels.Everything())
	if err != nil {
		return nil, err
	}
	for _, binding := range allSecretBindings {
		for _, quotaRef := range binding.Quotas {
			if quota.Name == quotaRef.Name && quota.Namespace == quotaRef.Namespace {
				secretBindings = append(secretBindings, *binding)
			}
		}
	}

	for _, binding := range secretBindings {
		shoots, err := q.shootLister.Shoots(binding.Namespace).List(labels.Everything())
		if err != nil {
			return nil, err
		}
		for _, s := range shoots {
			if shoot.Namespace == s.Namespace && shoot.Name == s.Name {
				continue
			}
			if s.Spec.Cloud.SecretBindingRef.Name == binding.Name {
				shootsReferQuota = append(shootsReferQuota, *s)
			}
		}
	}
	return shootsReferQuota, nil
}

func (q *QuotaValidator) determineRequiredResources(allocatedResources corev1.ResourceList, shoot garden.Shoot) (corev1.ResourceList, error) {
	shootResources, err := q.getShootResources(shoot)
	if err != nil {
		return nil, err
	}

	requiredResources := make(corev1.ResourceList)
	for _, metric := range quotaMetricNames {
		requiredResources[metric] = sumQuantity(allocatedResources[metric], shootResources[metric])
	}
	return requiredResources, nil
}

func (q *QuotaValidator) getShootResources(shoot garden.Shoot) (corev1.ResourceList, error) {
	cloudProfile, err := q.cloudProfileLister.Get(shoot.Spec.Cloud.Profile)
	if err != nil {
		return nil, apierrors.NewBadRequest("could not find referenced cloud profile")
	}

	cloudProvider, err := helper.DetermineCloudProviderInShoot(shoot.Spec.Cloud)
	if err != nil {
		return nil, apierrors.NewBadRequest("could not identify the cloud provider kind in the Shoot resource")
	}

	var (
		countLB      int64 = 1
		resources          = make(corev1.ResourceList)
		workers            = getShootWorkerResources(shoot, cloudProvider, *cloudProfile)
		machineTypes       = getMachineTypes(cloudProvider, *cloudProfile)
		volumeTypes        = getVolumeTypes(cloudProvider, *cloudProfile)
	)

	for _, worker := range workers {
		var (
			machineType *garden.MachineType
			volumeType  *garden.VolumeType
		)

		// Get the proper machineType
		for _, element := range machineTypes {
			if element.Name == worker.MachineType {
				machineType = &element
				break
			}
		}
		if machineType == nil {
			return nil, fmt.Errorf("MachineType %s not found in CloudProfile %s", worker.MachineType, cloudProfile.Name)
		}

		// Get the proper VolumeType
		for _, element := range volumeTypes {
			if element.Name == worker.VolumeType {
				volumeType = &element
				break
			}
		}
		if volumeType == nil {
			return nil, fmt.Errorf("VolumeType %s not found in CloudProfile %s", worker.MachineType, cloudProfile.Name)
		}

		// For now we always use the max. amount of resources for quota calculation
		resources[garden.QuotaMetricCPU] = sumQuantity(resources[garden.QuotaMetricCPU], multiplyQuantity(machineType.CPU, worker.AutoScalerMax))
		resources[garden.QuotaMetricGPU] = sumQuantity(resources[garden.QuotaMetricGPU], multiplyQuantity(machineType.GPU, worker.AutoScalerMax))
		resources[garden.QuotaMetricMemory] = sumQuantity(resources[garden.QuotaMetricMemory], multiplyQuantity(machineType.Memory, worker.AutoScalerMax))

		switch volumeType.Class {
		case garden.VolumeClassStandard:
			resources[garden.QuotaMetricStorageStandard] = sumQuantity(resources[garden.QuotaMetricStorageStandard], multiplyQuantity(worker.VolumeSize, worker.AutoScalerMax))
		case garden.VolumeClassPremium:
			resources[garden.QuotaMetricStoragePremium] = sumQuantity(resources[garden.QuotaMetricStoragePremium], multiplyQuantity(worker.VolumeSize, worker.AutoScalerMax))
		default:
			return nil, fmt.Errorf("Unknown volumeType class %s", volumeType.Class)
		}
	}

	if shoot.Spec.Addons != nil && shoot.Spec.Addons.NginxIngress != nil && shoot.Spec.Addons.NginxIngress.Addon.Enabled {
		countLB++
	}
	resources[garden.QuotaMetricLoadbalancer] = *resource.NewQuantity(countLB, resource.DecimalSI)

	return resources, nil
}

func getShootWorkerResources(shoot garden.Shoot, cloudProvider garden.CloudProvider, cloudProfile garden.CloudProfile) []quotaWorker {
	var workers []quotaWorker

	switch cloudProvider {
	case garden.CloudProviderAWS:
		workers = make([]quotaWorker, len(shoot.Spec.Cloud.AWS.Workers))

		for idx, awsWorker := range shoot.Spec.Cloud.AWS.Workers {
			workers[idx].Worker = awsWorker.Worker
			workers[idx].VolumeType = awsWorker.VolumeType
			workers[idx].VolumeSize = resource.MustParse(awsWorker.VolumeSize)
		}
	case garden.CloudProviderAzure:
		workers = make([]quotaWorker, len(shoot.Spec.Cloud.Azure.Workers))

		for idx, azureWorker := range shoot.Spec.Cloud.Azure.Workers {
			workers[idx].Worker = azureWorker.Worker
			workers[idx].VolumeType = azureWorker.VolumeType
			workers[idx].VolumeSize = resource.MustParse(azureWorker.VolumeSize)
		}
	case garden.CloudProviderGCP:
		workers = make([]quotaWorker, len(shoot.Spec.Cloud.GCP.Workers))

		for idx, gcpWorker := range shoot.Spec.Cloud.GCP.Workers {
			workers[idx].Worker = gcpWorker.Worker
			workers[idx].VolumeType = gcpWorker.VolumeType
			workers[idx].VolumeSize = resource.MustParse(gcpWorker.VolumeSize)
		}
	case garden.CloudProviderOpenStack:
		workers = make([]quotaWorker, len(shoot.Spec.Cloud.OpenStack.Workers))

		for idx, osWorker := range shoot.Spec.Cloud.OpenStack.Workers {
			workers[idx].Worker = osWorker.Worker
			for _, machineType := range cloudProfile.Spec.OpenStack.Constraints.MachineTypes {
				if osWorker.MachineType == machineType.Name {
					workers[idx].VolumeType = machineType.MachineType.Name
					workers[idx].VolumeSize = machineType.VolumeSize
				}
			}
		}
	case garden.CloudProviderAlicloud:
		workers = make([]quotaWorker, len(shoot.Spec.Cloud.Alicloud.Workers))

		for idx, aliWorker := range shoot.Spec.Cloud.Alicloud.Workers {
			workers[idx].Worker = aliWorker.Worker
			workers[idx].VolumeType = aliWorker.VolumeType
			workers[idx].VolumeSize = resource.MustParse(aliWorker.VolumeSize)
		}

	case garden.CloudProviderPacket:
		workers = make([]quotaWorker, len(shoot.Spec.Cloud.Packet.Workers))

		for idx, packetWorker := range shoot.Spec.Cloud.Packet.Workers {
			workers[idx].Worker = packetWorker.Worker
			workers[idx].VolumeType = packetWorker.VolumeType
			workers[idx].VolumeSize = resource.MustParse(packetWorker.VolumeSize)
		}
	}
	return workers
}

func getMachineTypes(provider garden.CloudProvider, cloudProfile garden.CloudProfile) []garden.MachineType {
	var machineTypes []garden.MachineType
	switch provider {
	case garden.CloudProviderAWS:
		machineTypes = cloudProfile.Spec.AWS.Constraints.MachineTypes
	case garden.CloudProviderAzure:
		machineTypes = cloudProfile.Spec.Azure.Constraints.MachineTypes
	case garden.CloudProviderGCP:
		machineTypes = cloudProfile.Spec.GCP.Constraints.MachineTypes
	case garden.CloudProviderPacket:
		machineTypes = cloudProfile.Spec.Packet.Constraints.MachineTypes
	case garden.CloudProviderOpenStack:
		machineTypes = make([]garden.MachineType, 0)
		for _, element := range cloudProfile.Spec.OpenStack.Constraints.MachineTypes {
			machineTypes = append(machineTypes, element.MachineType)
		}
	case garden.CloudProviderAlicloud:
		machineTypes = make([]garden.MachineType, 0)
		for _, element := range cloudProfile.Spec.Alicloud.Constraints.MachineTypes {
			machineTypes = append(machineTypes, element.MachineType)
		}
	}
	return machineTypes
}

func getVolumeTypes(provider garden.CloudProvider, cloudProfile garden.CloudProfile) []garden.VolumeType {
	var volumeTypes []garden.VolumeType
	switch provider {
	case garden.CloudProviderAWS:
		volumeTypes = cloudProfile.Spec.AWS.Constraints.VolumeTypes
	case garden.CloudProviderAzure:
		volumeTypes = cloudProfile.Spec.Azure.Constraints.VolumeTypes
	case garden.CloudProviderGCP:
		volumeTypes = cloudProfile.Spec.GCP.Constraints.VolumeTypes
	case garden.CloudProviderPacket:
		volumeTypes = cloudProfile.Spec.Packet.Constraints.VolumeTypes
	case garden.CloudProviderOpenStack:
		volumeTypes = make([]garden.VolumeType, 0)
		contains := func(types []garden.VolumeType, volumeType string) bool {
			for _, element := range types {
				if element.Name == volumeType {
					return true
				}
			}
			return false
		}

		for _, machineType := range cloudProfile.Spec.OpenStack.Constraints.MachineTypes {
			if !contains(volumeTypes, machineType.MachineType.Name) {
				volumeTypes = append(volumeTypes, garden.VolumeType{
					Name:  machineType.MachineType.Name,
					Class: machineType.VolumeType,
				})
			}
		}
	case garden.CloudProviderAlicloud:
		volumeTypes = make([]garden.VolumeType, 0)
		for _, element := range cloudProfile.Spec.Alicloud.Constraints.VolumeTypes {
			volumeTypes = append(volumeTypes, element.VolumeType)
		}
	}
	return volumeTypes
}

func lifetimeVerificationNeeded(new, old garden.Shoot) bool {
	oldLifetime, exits := old.Annotations[common.ShootExpirationTimestamp]
	if !exits {
		oldLifetime = old.CreationTimestamp.String()
	}
	if oldLifetime != new.Annotations[common.ShootExpirationTimestamp] {
		return true
	}
	return false
}

func quotaVerificationNeeded(new, old garden.Shoot, provider garden.CloudProvider) bool {
	// Check for diff on addon nginx-ingress (addon requires to deploy a load balancer)
	var (
		oldNginxIngressEnabled bool
		newNginxIngressEnabled bool
	)

	if old.Spec.Addons != nil && old.Spec.Addons.NginxIngress != nil {
		oldNginxIngressEnabled = old.Spec.Addons.NginxIngress.Enabled
	}

	if new.Spec.Addons != nil && new.Spec.Addons.NginxIngress != nil {
		newNginxIngressEnabled = new.Spec.Addons.NginxIngress.Enabled
	}

	if oldNginxIngressEnabled == false && newNginxIngressEnabled == true {
		return true
	}

	// Check for diffs on workers
	switch provider {
	case garden.CloudProviderAWS:
		for _, worker := range new.Spec.Cloud.AWS.Workers {
			oldHasWorker := false
			for _, oldWorker := range old.Spec.Cloud.AWS.Workers {
				if worker.Name == oldWorker.Name {
					oldHasWorker = true
					if hasWorkerDiff(worker.Worker, oldWorker.Worker) || worker.VolumeType != oldWorker.VolumeType || worker.VolumeSize != oldWorker.VolumeSize {
						return true
					}
				}
			}
			if !oldHasWorker {
				return true
			}
		}
	case garden.CloudProviderAzure:
		for _, worker := range new.Spec.Cloud.Azure.Workers {
			oldHasWorker := false
			for _, oldWorker := range old.Spec.Cloud.Azure.Workers {
				if worker.Name == oldWorker.Name {
					oldHasWorker = true
					if hasWorkerDiff(worker.Worker, oldWorker.Worker) || worker.VolumeType != oldWorker.VolumeType || worker.VolumeSize != oldWorker.VolumeSize {
						return true
					}
				}
			}
			if !oldHasWorker {
				return true
			}
		}
	case garden.CloudProviderGCP:
		for _, worker := range new.Spec.Cloud.GCP.Workers {
			oldHasWorker := false
			for _, oldWorker := range old.Spec.Cloud.GCP.Workers {
				if worker.Name == oldWorker.Name {
					oldHasWorker = true
					if hasWorkerDiff(worker.Worker, oldWorker.Worker) || worker.VolumeType != oldWorker.VolumeType || worker.VolumeSize != oldWorker.VolumeSize {
						return true
					}
				}
			}
			if !oldHasWorker {
				return true
			}
		}
	case garden.CloudProviderPacket:
		for _, worker := range new.Spec.Cloud.Packet.Workers {
			oldHasWorker := false
			for _, oldWorker := range old.Spec.Cloud.Packet.Workers {
				if worker.Name == oldWorker.Name {
					oldHasWorker = true
					if hasWorkerDiff(worker.Worker, oldWorker.Worker) || worker.VolumeType != oldWorker.VolumeType || worker.VolumeSize != oldWorker.VolumeSize {
						return true
					}
				}
			}
			if !oldHasWorker {
				return true
			}
		}
	case garden.CloudProviderOpenStack:
		for _, worker := range new.Spec.Cloud.OpenStack.Workers {
			oldHasWorker := false
			for _, oldWorker := range old.Spec.Cloud.OpenStack.Workers {
				if worker.Name == oldWorker.Name {
					oldHasWorker = true
					if hasWorkerDiff(worker.Worker, oldWorker.Worker) {
						return true
					}
				}
			}
			if !oldHasWorker {
				return true
			}
		}
	case garden.CloudProviderAlicloud:
		for _, worker := range new.Spec.Cloud.Alicloud.Workers {
			oldHasWorker := false
			for _, oldWorker := range old.Spec.Cloud.Alicloud.Workers {
				if worker.Name == oldWorker.Name {
					oldHasWorker = true
					if hasWorkerDiff(worker.Worker, oldWorker.Worker) || worker.VolumeType != oldWorker.VolumeType || worker.VolumeSize != oldWorker.VolumeSize {
						return true
					}
				}
			}
			if !oldHasWorker {
				return true
			}
		}
	}

	return false
}

func hasWorkerDiff(new, old garden.Worker) bool {
	if new.MachineType != old.MachineType || new.AutoScalerMax != old.AutoScalerMax {
		return true
	}
	return false
}

func hasSufficientQuota(limit, required resource.Quantity) bool {
	compareCode := limit.Cmp(required)
	if compareCode == -1 {
		return false
	}
	return true
}

func sumQuantity(values ...resource.Quantity) resource.Quantity {
	res := resource.Quantity{}
	for _, v := range values {
		res.Add(v)
	}
	return res
}

func multiplyQuantity(quantity resource.Quantity, multiplier int) resource.Quantity {
	res := resource.Quantity{}
	for i := 0; i < multiplier; i++ {
		res.Add(quantity)
	}
	return res
}
