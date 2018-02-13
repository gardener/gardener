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

package quotavalidator

import (
	"errors"
	"fmt"
	"io"

	"github.com/gardener/gardener/pkg/apis/garden"
	"github.com/gardener/gardener/pkg/apis/garden/helper"
	admissioninitializer "github.com/gardener/gardener/pkg/apiserver/admission/initializer"
	informers "github.com/gardener/gardener/pkg/client/garden/informers/internalversion"
	listers "github.com/gardener/gardener/pkg/client/garden/listers/garden/internalversion"
	"github.com/golang/glog"
	"k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apiserver/pkg/admission"
)

const (
	// PluginName is the name of this admission plugin.
	PluginName = "ShootQuotaValidator"
)

type quotaWorker struct {
	garden.Worker
	// VolumeType is the type of the root volumes.
	VolumeType string
	// VolumeSize is the size of the root volume.
	VolumeSize string
}

// Register registers a plugin.
func Register(plugins *admission.Plugins) {
	plugins.Register(PluginName, func(config io.Reader) (admission.Interface, error) {
		return New()
	})
}

// RejectShootIfQuotaExceeded contains listers and and admission handler.
type RejectShootIfQuotaExceeded struct {
	*admission.Handler
	shootLister        listers.ShootLister
	cloudProfileLister listers.CloudProfileLister
	crossSBLister      listers.CrossSecretBindingLister
	quotaLister        listers.QuotaLister
}

var _ = admissioninitializer.WantsInternalGardenInformerFactory(&RejectShootIfQuotaExceeded{})

// New creates a new RejectShootIfQuotaExceeded admission plugin.
func New() (*RejectShootIfQuotaExceeded, error) {
	return &RejectShootIfQuotaExceeded{
		Handler: admission.NewHandler(admission.Create, admission.Update),
	}, nil
}

// SetInternalGardenInformerFactory gets Lister from SharedInformerFactory.
func (h *RejectShootIfQuotaExceeded) SetInternalGardenInformerFactory(f informers.SharedInformerFactory) {
	h.shootLister = f.Garden().InternalVersion().Shoots().Lister()
	h.cloudProfileLister = f.Garden().InternalVersion().CloudProfiles().Lister()
	h.crossSBLister = f.Garden().InternalVersion().CrossSecretBindings().Lister()
	h.quotaLister = f.Garden().InternalVersion().Quotas().Lister()
}

// ValidateInitialization checks whether the plugin was correctly initialized.
func (h *RejectShootIfQuotaExceeded) ValidateInitialization() error {
	if h.shootLister == nil {
		return errors.New("missing shoot lister")
	}
	if h.cloudProfileLister == nil {
		return errors.New("missing cloudProfile lister")
	}
	if h.crossSBLister == nil {
		return errors.New("missing crossSecretBinding lister")
	}
	if h.quotaLister == nil {
		return errors.New("missing quota lister")
	}
	return nil
}

// Admit checks that the requested Shoot resources are within the quota limits.
func (h *RejectShootIfQuotaExceeded) Admit(a admission.Attributes) error {
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

	//TODO support also PrivateSecretBindings
	// Currently only consider quotas for CrossSecretBindings
	if shoot.Spec.Cloud.SecretBindingRef.Kind != "CrossSecretBinding" {
		glog.V(2).Infof("Currently quotas are not supported for %s", shoot.Spec.Cloud.SecretBindingRef.Kind)
		return nil
	}

	// retrieve the secret binding
	crossSBNamespaceLister := h.crossSBLister.CrossSecretBindings(a.GetNamespace())
	usedCrossSB, err := crossSBNamespaceLister.Get(shoot.Spec.Cloud.SecretBindingRef.Name)
	if err != nil {
		return err
	}
	glog.V(2).Infof("CrossSecretBinding %s is referenced in shoot", shoot.Spec.Cloud.SecretBindingRef.Name)

	// retrieve the quota(s) from the secret binding
	var quotaReferences []garden.CrossReference
	quotaReferences = usedCrossSB.Quotas
	if len(quotaReferences) == 0 {
		glog.V(2).Infof("The CrossSecretBinding with the name %s has no quotas referenced", shoot.Spec.Cloud.SecretBindingRef.Name)
		return nil
	}
	if len(quotaReferences) > 2 {
		return apierrors.NewBadRequest(fmt.Sprintf("The CrossSecretBinding with the name %s has more than 2 quotas referenced", shoot.Spec.Cloud.SecretBindingRef.Name))
	}
	glog.V(2).Infof("CrossSecretBinding has %d quotas for the secret %s", len(quotaReferences), usedCrossSB.SecretRef.Name)

	var tempQuotaScope garden.QuotaScope
	for _, quotaReference := range quotaReferences {

		result, quotaScope, err := h.isReferencedQuotaExceeded(a, quotaReference.Namespace, quotaReference.Name)
		if err != nil {
			return apierrors.NewInternalError(err)
		}
		// check that max 1 project and 1 secret quota is referenced
		if tempQuotaScope == "" {
			tempQuotaScope = quotaScope
		} else if quotaScope == tempQuotaScope {
			return apierrors.NewInternalError(fmt.Errorf("The CrossSecretBinding with the name %s has referenced secrets with the same scope", shoot.Spec.Cloud.SecretBindingRef.Name))
		}
		if result {
			return admission.NewForbidden(a, fmt.Errorf("quota limits exceeded for %v", shoot.Name))
		}
	}

	return nil
}

func (h *RejectShootIfQuotaExceeded) isReferencedQuotaExceeded(a admission.Attributes, namespace, name string) (bool, garden.QuotaScope, error) {
	quotaNamespaceLister := h.quotaLister.Quotas(namespace)
	usedQuota, err := quotaNamespaceLister.Get(name)
	if err != nil {
		return false, "", err
	}

	switch usedQuota.Spec.Scope {
	case garden.QuotaScopeSecret:
		result, err := h.isSecretQuotaExceeded(a, *usedQuota)
		return result, garden.QuotaScopeSecret, err
	case garden.QuotaScopeProject:
		result, err := isProjectQuotaExceeded(a, *usedQuota)
		return result, garden.QuotaScopeProject, err
	default:
		return false, "", fmt.Errorf("Incorrect scope %s defined in quota with the name %s in namespace %s", usedQuota.Spec.Scope, name, namespace)
	}
}

func (h *RejectShootIfQuotaExceeded) isSecretQuotaExceeded(a admission.Attributes, quota garden.Quota) (bool, error) {
	glog.V(2).Infof("Checking secret quota %s", quota.Name)

	// get the metrics of the quota object
	quotaMetrics := quota.Spec.Metrics

	// calculate the status of the quota object dynamically
	statusMetrics, nil := determineCurrentStatusSecretQuota(quota)

	// get the metrics that the new shoot cluster will use
	shoot, ok := a.GetObject().(*garden.Shoot)
	if !ok {
		return false, apierrors.NewBadRequest("could not convert resource into Shoot object")
	}
	cloudProfile, err := h.cloudProfileLister.Get(shoot.Spec.Cloud.Profile)
	if err != nil {
		return false, apierrors.NewBadRequest("could not find referenced cloud profile")
	}

	shootMetrics, err := convertShootWorkersToMetrics(shoot, cloudProfile)
	if err != nil {
		return false, err
	}

	// check all metrics that are set in the quota object
	for quotaMetricKey, quotaMetricValue := range quotaMetrics {
		newStatusValue := statusMetrics[quotaMetricKey]
		newStatusValue.Add(shootMetrics[quotaMetricKey])
		// check if new cluster would exceed the current metric
		if quotaMetricValue.Cmp(newStatusValue) < 0 {
			glog.V(2).Infof("The quota for %s on shoot %s exceeded; max=%s, new=%s", quotaMetricKey, shoot.Name, quotaMetricValue.String(), newStatusValue.String())
			return true, nil
		}
	}

	// all quota checks passed
	return false, nil
}

func determineCurrentStatusSecretQuota(quota garden.Quota) (v1.ResourceList, error) {
	// TODO get the status dynamically
	return quota.Status.Metrics, nil
}

func isProjectQuotaExceeded(a admission.Attributes, quota garden.Quota) (bool, error) {
	//TODO implement
	return false, nil
}

// This gets the metrics that the new shoot cluster will use
// checked metrics for now are: cpu, memory, storage.standard, storage.premium
func convertShootWorkersToMetrics(shoot *garden.Shoot, cloudProfile *garden.CloudProfile) (v1.ResourceList, error) {
	cloudProviderInShoot, err := helper.DetermineCloudProviderInShoot(shoot.Spec.Cloud)
	if err != nil {
		return nil, apierrors.NewBadRequest("could not identify the cloud provider kind in the Shoot resource")
	}

	metrics := make(v1.ResourceList)
	totalMemory := resource.Quantity{}
	totalCPU := resource.Quantity{}
	totalGPU := resource.Quantity{}
	totalStandardVolume := resource.Quantity{}
	totalPremiumVolume := resource.Quantity{}

	workers := retrieveQuotaWorkersForShoot(shoot, cloudProviderInShoot)
	for _, worker := range workers {
		machineType, err := getMachineTypeByName(cloudProfile, cloudProviderInShoot, worker.MachineType)
		if err != nil {
			return nil, err
		}
		// for now we always use the max. amount of workers for quota calculation
		totalMemory.Add(multiplyQuantity(machineType.Memory, worker.AutoScalerMax))
		totalCPU.Add(*resource.NewQuantity(int64(machineType.CPUs*worker.AutoScalerMax), resource.DecimalSI))
		totalGPU.Add(*resource.NewQuantity(int64(machineType.GPUs*worker.AutoScalerMax), resource.DecimalSI))

		volumeType, err := getVolumeTypeByName(cloudProfile, cloudProviderInShoot, worker.VolumeType)
		if err != nil {
			return nil, err
		}
		volumeSize, err := resource.ParseQuantity(worker.VolumeSize)
		if err != nil {
			return nil, err
		}
		if volumeType.Class == "standard" {
			totalStandardVolume.Add(multiplyQuantity(volumeSize, worker.AutoScalerMax))
		} else {
			if volumeType.Class == "premium" {
				totalPremiumVolume.Add(multiplyQuantity(volumeSize, worker.AutoScalerMax))
			}
		}
	}
	metrics[garden.QuotaMetricMemory] = totalMemory
	metrics[garden.QuotaMetricCPU] = totalCPU
	metrics[garden.QuotaMetricGPU] = totalGPU
	metrics[garden.QuotaMetricStorageStandard] = totalStandardVolume
	metrics[garden.QuotaMetricStoragePremium] = totalPremiumVolume

	return metrics, nil
}

func retrieveQuotaWorkersForShoot(shoot *garden.Shoot, cloudProvider garden.CloudProvider) []quotaWorker {
	var workers []quotaWorker

	switch cloudProvider {
	case garden.CloudProviderAWS:
		workers = make([]quotaWorker, len(shoot.Spec.Cloud.AWS.Workers))

		for idx, awsWorker := range shoot.Spec.Cloud.AWS.Workers {
			workers[idx].Worker = awsWorker.Worker
			workers[idx].VolumeSize = awsWorker.VolumeSize
			workers[idx].VolumeType = awsWorker.VolumeType
		}
	case garden.CloudProviderAzure:
		workers = make([]quotaWorker, len(shoot.Spec.Cloud.Azure.Workers))

		for idx, azureWorker := range shoot.Spec.Cloud.Azure.Workers {
			workers[idx].Worker = azureWorker.Worker
			workers[idx].VolumeSize = azureWorker.VolumeSize
			workers[idx].VolumeType = azureWorker.VolumeType
		}
	case garden.CloudProviderGCP:
		workers = make([]quotaWorker, len(shoot.Spec.Cloud.GCP.Workers))

		for idx, gcpWorker := range shoot.Spec.Cloud.GCP.Workers {
			workers[idx].Worker = gcpWorker.Worker
			workers[idx].VolumeSize = gcpWorker.VolumeSize
			workers[idx].VolumeType = gcpWorker.VolumeType
		}
	case garden.CloudProviderOpenStack:
		workers = make([]quotaWorker, len(shoot.Spec.Cloud.OpenStack.Workers))

		for idx, osWorker := range shoot.Spec.Cloud.OpenStack.Workers {
			workers[idx].Worker = osWorker.Worker
			//TODO handle volumes in cloud profile for openstack
		}
	}
	return workers
}

func getMachineTypeByName(cloudProfile *garden.CloudProfile, cloudProvider garden.CloudProvider, machineTypeName string) (*garden.MachineType, error) {
	var machineTypes []garden.MachineType

	switch cloudProvider {
	case garden.CloudProviderAWS:
		machineTypes = cloudProfile.Spec.AWS.Constraints.MachineTypes
	case garden.CloudProviderAzure:
		machineTypes = cloudProfile.Spec.Azure.Constraints.MachineTypes
	case garden.CloudProviderGCP:
		machineTypes = cloudProfile.Spec.GCP.Constraints.MachineTypes
	case garden.CloudProviderOpenStack:
		machineTypes = cloudProfile.Spec.OpenStack.Constraints.MachineTypes
	}

	for _, machineType := range machineTypes {
		if machineType.Name == machineTypeName {
			return &machineType, nil
		}
	}

	return nil, fmt.Errorf("Machine type %s doesn't exist in CloudProfile %s", machineTypeName, cloudProvider)
}

func getVolumeTypeByName(cloudProfile *garden.CloudProfile, cloudProvider garden.CloudProvider, volumeTypeName string) (*garden.VolumeType, error) {
	var volumeTypes []garden.VolumeType

	switch cloudProvider {
	case garden.CloudProviderAWS:
		volumeTypes = cloudProfile.Spec.AWS.Constraints.VolumeTypes
	case garden.CloudProviderAzure:
		volumeTypes = cloudProfile.Spec.Azure.Constraints.VolumeTypes
	case garden.CloudProviderGCP:
		volumeTypes = cloudProfile.Spec.GCP.Constraints.VolumeTypes
	}

	for _, volumeType := range volumeTypes {
		if volumeType.Name == volumeTypeName {
			return &volumeType, nil
		}
	}

	return nil, fmt.Errorf("Volume type %s doesn't exist in CloudProfile %s", volumeTypeName, cloudProvider)
}

func multiplyQuantity(quantity resource.Quantity, multiplier int) resource.Quantity {
	//TODO improve
	result := resource.Quantity{}

	for i := 0; i < multiplier; i++ {
		result.Add(quantity)
	}
	return result
}
