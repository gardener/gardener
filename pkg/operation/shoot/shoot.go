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

package shoot

import (
	"fmt"

	"github.com/Masterminds/semver"
	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/apis/garden/v1beta1/helper"
	gardeninformers "github.com/gardener/gardener/pkg/client/garden/informers/externalversions/garden/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils"
	corev1 "k8s.io/api/core/v1"
)

// New takes a <k8sGardenClient>, the <k8sGardenInformers> and a <shoot> manifest, and creates a new Shoot representation.
// It will add the CloudProfile, the cloud provider secret, compute the internal cluster domain and identify the cloud provider.
func New(k8sGardenClient kubernetes.Client, k8sGardenInformers gardeninformers.Interface, shoot *gardenv1beta1.Shoot, projectName, internalDomain string) (*Shoot, error) {
	var (
		secret *corev1.Secret
		err    error
	)

	cloudProfile, err := k8sGardenInformers.CloudProfiles().Lister().Get(shoot.Spec.Cloud.Profile)
	if err != nil {
		return nil, err
	}

	binding, err := k8sGardenInformers.SecretBindings().Lister().SecretBindings(shoot.Namespace).Get(shoot.Spec.Cloud.SecretBindingRef.Name)
	if err != nil {
		return nil, err
	}
	secret, err = k8sGardenClient.GetSecret(binding.SecretRef.Namespace, binding.SecretRef.Name)
	if err != nil {
		return nil, err
	}

	shootObj := &Shoot{
		Info:                  shoot,
		Secret:                secret,
		CloudProfile:          cloudProfile,
		SeedNamespace:         ComputeTechnicalID(projectName, shoot),
		InternalClusterDomain: internalDomain,
		Hibernated:            true,
	}

	// Determine the external Shoot cluster domain, i.e. the domain which will be put into the Kubeconfig handed out
	// to the user.
	if *(shoot.Spec.DNS.Domain) != gardenv1beta1.DefaultDomain {
		extDomain := fmt.Sprintf("api.%s", *(shoot.Spec.DNS.Domain))
		shootObj.ExternalClusterDomain = &extDomain
	}

	// Determine the cloud provider kind of this Shoot object.
	cloudProvider, err := helper.DetermineCloudProviderInShoot(shoot.Spec.Cloud)
	if err != nil {
		return nil, err
	}
	shootObj.CloudProvider = cloudProvider

	// Store the Kubernetes version in the format <major>.<minor> on the Shoot object.
	v, err := semver.NewVersion(shoot.Spec.Kubernetes.Version)
	if err != nil {
		return nil, err
	}
	shootObj.KubernetesMajorMinorVersion = fmt.Sprintf("%d.%d", v.Major(), v.Minor())

	// Check whether the Shoot should be hibernated
	workers := shootObj.GetWorkers()
	for _, worker := range workers {
		if worker.AutoScalerMax != 0 {
			shootObj.Hibernated = false
			break
		}
	}

	return shootObj, nil
}

// GetIngressFQDN returns the fully qualified domain name of ingress sub-resource for the Shoot cluster. The
// end result is '<subDomain>.<ingressPrefix>.<clusterDomain>'.
func (s *Shoot) GetIngressFQDN(subDomain string) string {
	return fmt.Sprintf("%s.%s.%s", subDomain, common.IngressPrefix, *(s.Info.Spec.DNS.Domain))
}

// GetWorkers returns a list of worker objects of the worker groups in the Shoot manifest.
func (s *Shoot) GetWorkers() []gardenv1beta1.Worker {
	workers := []gardenv1beta1.Worker{}

	switch s.CloudProvider {
	case gardenv1beta1.CloudProviderAWS:
		for _, worker := range s.Info.Spec.Cloud.AWS.Workers {
			workers = append(workers, worker.Worker)
		}
	case gardenv1beta1.CloudProviderAzure:
		for _, worker := range s.Info.Spec.Cloud.Azure.Workers {
			workers = append(workers, worker.Worker)
		}
	case gardenv1beta1.CloudProviderGCP:
		for _, worker := range s.Info.Spec.Cloud.GCP.Workers {
			workers = append(workers, worker.Worker)
		}
	case gardenv1beta1.CloudProviderOpenStack:
		for _, worker := range s.Info.Spec.Cloud.OpenStack.Workers {
			workers = append(workers, worker.Worker)
		}
	case gardenv1beta1.CloudProviderLocal:
		workers = append(workers, gardenv1beta1.Worker{
			Name:          "local",
			AutoScalerMax: 1,
			AutoScalerMin: 1,
		})
	}

	return workers
}

// GetWorkerNames returns a list of names of the worker groups in the Shoot manifest.
func (s *Shoot) GetWorkerNames() []string {
	var (
		workers     = s.GetWorkers()
		workerNames = []string{}
	)

	for _, worker := range workers {
		workerNames = append(workerNames, worker.Name)
	}

	return workerNames
}

// GetNodeCount returns the sum of all 'autoScalerMax' fields of all worker groups of the Shoot.
func (s *Shoot) GetNodeCount() int {
	nodeCount := 0

	switch s.CloudProvider {
	case gardenv1beta1.CloudProviderAWS:
		for _, worker := range s.Info.Spec.Cloud.AWS.Workers {
			nodeCount += worker.AutoScalerMax
		}
	case gardenv1beta1.CloudProviderAzure:
		for _, worker := range s.Info.Spec.Cloud.Azure.Workers {
			nodeCount += worker.AutoScalerMax
		}
	case gardenv1beta1.CloudProviderGCP:
		for _, worker := range s.Info.Spec.Cloud.GCP.Workers {
			nodeCount += worker.AutoScalerMax
		}
	case gardenv1beta1.CloudProviderOpenStack:
		for _, worker := range s.Info.Spec.Cloud.OpenStack.Workers {
			nodeCount += worker.AutoScalerMax
		}
	case gardenv1beta1.CloudProviderLocal:
		nodeCount = 1
	}

	return nodeCount
}

// GetK8SNetworks returns the Kubernetes network CIDRs for the Shoot cluster.
func (s *Shoot) GetK8SNetworks() *gardenv1beta1.K8SNetworks {
	switch s.CloudProvider {
	case gardenv1beta1.CloudProviderAWS:
		return &s.Info.Spec.Cloud.AWS.Networks.K8SNetworks
	case gardenv1beta1.CloudProviderAzure:
		return &s.Info.Spec.Cloud.Azure.Networks.K8SNetworks
	case gardenv1beta1.CloudProviderGCP:
		return &s.Info.Spec.Cloud.GCP.Networks.K8SNetworks
	case gardenv1beta1.CloudProviderOpenStack:
		return &s.Info.Spec.Cloud.OpenStack.Networks.K8SNetworks
	case gardenv1beta1.CloudProviderLocal:
		return &s.Info.Spec.Cloud.Local.Networks.K8SNetworks
	}
	return nil
}

// GetPodNetwork returns the pod network CIDR for the Shoot cluster.
func (s *Shoot) GetPodNetwork() gardenv1beta1.CIDR {
	if k8sNetworks := s.GetK8SNetworks(); k8sNetworks != nil {
		return *k8sNetworks.Pods
	}
	return ""
}

// GetServiceNetwork returns the service network CIDR for the Shoot cluster.
func (s *Shoot) GetServiceNetwork() gardenv1beta1.CIDR {
	if k8sNetworks := s.GetK8SNetworks(); k8sNetworks != nil {
		return *k8sNetworks.Services
	}
	return ""
}

// GetNodeNetwork returns the node network CIDR for the Shoot cluster.
func (s *Shoot) GetNodeNetwork() gardenv1beta1.CIDR {
	if k8sNetworks := s.GetK8SNetworks(); k8sNetworks != nil {
		return *k8sNetworks.Nodes
	}
	return ""
}

// GetMachineImageName returns the name of the used machine image.
func (s *Shoot) GetMachineImageName() gardenv1beta1.MachineImageName {
	switch s.CloudProvider {
	case gardenv1beta1.CloudProviderAWS:
		return s.Info.Spec.Cloud.AWS.MachineImage.Name
	case gardenv1beta1.CloudProviderAzure:
		return s.Info.Spec.Cloud.Azure.MachineImage.Name
	case gardenv1beta1.CloudProviderGCP:
		return s.Info.Spec.Cloud.GCP.MachineImage.Name
	case gardenv1beta1.CloudProviderOpenStack:
		return s.Info.Spec.Cloud.OpenStack.MachineImage.Name
	}
	return ""
}

// ClusterAutoscalerEnabled returns true if the cluster-autoscaler addon is enabled in the Shoot manifest.
func (s *Shoot) ClusterAutoscalerEnabled() bool {
	return s.Info.Spec.Addons != nil && s.Info.Spec.Addons.ClusterAutoscaler != nil && s.Info.Spec.Addons.ClusterAutoscaler.Enabled
}

// HeapsterEnabled returns true if the heapster addon is enabled in the Shoot manifest.
func (s *Shoot) HeapsterEnabled() bool {
	return s.Info.Spec.Addons != nil && s.Info.Spec.Addons.Heapster != nil && s.Info.Spec.Addons.Heapster.Enabled
}

// Kube2IAMEnabled returns true if the kube2iam addon is enabled in the Shoot manifest.
func (s *Shoot) Kube2IAMEnabled() bool {
	return s.Info.Spec.Addons != nil && s.Info.Spec.Addons.Kube2IAM != nil && s.Info.Spec.Addons.Kube2IAM.Enabled
}

// KubeLegoEnabled returns true if the kube-lego addon is enabled in the Shoot manifest.
func (s *Shoot) KubeLegoEnabled() bool {
	return s.Info.Spec.Addons != nil && s.Info.Spec.Addons.KubeLego != nil && s.Info.Spec.Addons.KubeLego.Enabled
}

// KubernetesDashboardEnabled returns true if the kubernetes-dashboard addon is enabled in the Shoot manifest.
func (s *Shoot) KubernetesDashboardEnabled() bool {
	return s.Info.Spec.Addons != nil && s.Info.Spec.Addons.KubernetesDashboard != nil && s.Info.Spec.Addons.KubernetesDashboard.Enabled
}

// NginxIngressEnabled returns true if the nginx-ingress addon is enabled in the Shoot manifest.
func (s *Shoot) NginxIngressEnabled() bool {
	return s.Info.Spec.Addons != nil && s.Info.Spec.Addons.NginxIngress != nil && s.Info.Spec.Addons.NginxIngress.Enabled
}

// MonocularEnabled returns true if the monocular addon is enabled in the Shoot manifest.
func (s *Shoot) MonocularEnabled() bool {
	return s.Info.Spec.Addons != nil && s.Info.Spec.Addons.Monocular != nil && s.Info.Spec.Addons.Monocular.Enabled
}

// ComputeCloudConfigSecretName computes the name for a secret which contains the original cloud config for
// the worker group with the given <workerName>. It is build by the cloud config secret prefix, the worker
// name itself and a hash of the minor Kubernetes version of the Shoot cluster.
func (s *Shoot) ComputeCloudConfigSecretName(workerName string) string {
	return fmt.Sprintf("%s-%s-%s", common.CloudConfigPrefix, workerName, utils.ComputeSHA256Hex([]byte(s.KubernetesMajorMinorVersion))[:5])
}

// ComputeTechnicalID determines the technical id of that Shoot which is later used for the name of the
// namespace and for tagging all the resources created in the infrastructure.
func ComputeTechnicalID(projectName string, shoot *gardenv1beta1.Shoot) string {
	// Use the stored technical ID in the Shoot's status field if it's there.
	if len(shoot.Status.TechnicalID) > 0 {
		return shoot.Status.TechnicalID
	}

	// Otherwise, existing clusters definitely have set the last operation on the Shoot status.
	// For backwards compatibility we keep the pattern as it was before we had to change it
	// (double hyphens).
	if shoot.Status.LastOperation != nil {
		return fmt.Sprintf("shoot-%s-%s", projectName, shoot.Name)
	}

	// New clusters shall be created with the new technical id (double hyphens).
	return fmt.Sprintf("shoot--%s--%s", projectName, shoot.Name)
}
