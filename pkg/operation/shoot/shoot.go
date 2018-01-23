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

package shoot

import (
	"errors"
	"fmt"

	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/apis/garden/v1beta1/helper"
	gardeninformers "github.com/gardener/gardener/pkg/client/garden/informers/externalversions/garden/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	corev1 "k8s.io/api/core/v1"
)

// New takes a <k8sGardenClient>, the <k8sGardenInformers> and a <shoot> manifest, and creates a new Shoot representation.
// It will add the CloudProfile, the cloud provider secret, compute the internal cluster domain and identify the cloud provider.
func New(k8sGardenClient kubernetes.Client, k8sGardenInformers gardeninformers.Interface, shoot *gardenv1beta1.Shoot, internalDomain string) (*Shoot, error) {
	var (
		secret *corev1.Secret
		err    error
	)

	cloudProfile, err := k8sGardenInformers.CloudProfiles().Lister().Get(shoot.Spec.Cloud.Profile)
	if err != nil {
		return nil, err
	}

	bindingRef := shoot.Spec.Cloud.SecretBindingRef
	switch bindingRef.Kind {
	case "PrivateSecretBinding":
		binding, err := k8sGardenInformers.PrivateSecretBindings().Lister().PrivateSecretBindings(shoot.Namespace).Get(bindingRef.Name)
		if err != nil {
			return nil, err
		}
		secret, err = k8sGardenClient.GetSecret(binding.Namespace, binding.SecretRef.Name)
		if err != nil {
			return nil, err
		}
	case "CrossSecretBinding":
		binding, err := k8sGardenInformers.CrossSecretBindings().Lister().CrossSecretBindings(shoot.Namespace).Get(bindingRef.Name)
		if err != nil {
			return nil, err
		}
		secret, err = k8sGardenClient.GetSecret(binding.SecretRef.Namespace, binding.SecretRef.Name)
		if err != nil {
			return nil, err
		}
	default:
		return nil, errors.New("cannot create new shoot object: unknown secret binding reference kind")
	}

	shootObj := &Shoot{
		Info:                  shoot,
		Secret:                secret,
		CloudProfile:          cloudProfile,
		SeedNamespace:         fmt.Sprintf("shoot-%s-%s", shoot.Namespace, shoot.Name),
		InternalClusterDomain: internalDomain,
	}

	if shoot.Spec.DNS.Domain != nil {
		extDomain := fmt.Sprintf("api.%s", *(shoot.Spec.DNS.Domain))
		shootObj.ExternalClusterDomain = &extDomain
	}

	cloudProvider, err := helper.DetermineCloudProviderInShoot(shoot.Spec.Cloud)
	if err != nil {
		return nil, err
	}
	shootObj.CloudProvider = cloudProvider

	return shootObj, nil
}

// GetIngressFQDN returns the fully qualified domain name of ingress sub-resource for the Shoot cluster. The
// end result is '<subDomain>.ingress.<clusterDomain>'. It must not exceed 64 characters in length (see RFC-5280
// for details).
func (s *Shoot) GetIngressFQDN(subDomain string) (string, error) {
	result := fmt.Sprintf("%s.ingress.%s", subDomain, *(s.Info.Spec.DNS.Domain))
	if len(result) > 64 {
		return "", fmt.Errorf("the FQDN for '%s' cannot be longer than 64 characters", result)
	}
	return result, nil
}

// GetWorkerNames returns a list of names of the worker groups in the Shoot manifest.
func (s *Shoot) GetWorkerNames() []string {
	workerNames := []string{}

	switch s.CloudProvider {
	case gardenv1beta1.CloudProviderAWS:
		for _, worker := range s.Info.Spec.Cloud.AWS.Workers {
			workerNames = append(workerNames, worker.Name)
		}
	case gardenv1beta1.CloudProviderAzure:
		for _, worker := range s.Info.Spec.Cloud.Azure.Workers {
			workerNames = append(workerNames, worker.Name)
		}
	case gardenv1beta1.CloudProviderGCP:
		for _, worker := range s.Info.Spec.Cloud.GCP.Workers {
			workerNames = append(workerNames, worker.Name)
		}
	case gardenv1beta1.CloudProviderOpenStack:
		for _, worker := range s.Info.Spec.Cloud.OpenStack.Workers {
			workerNames = append(workerNames, worker.Name)
		}
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
	}
	return nil
}

// GetPodNetwork returns the pod network CIDR for the Shoot cluster.
func (s *Shoot) GetPodNetwork() gardenv1beta1.CIDR {
	if k8sNetworks := s.GetK8SNetworks(); k8sNetworks != nil {
		return k8sNetworks.Pods
	}
	return ""
}

// GetServiceNetwork returns the service network CIDR for the Shoot cluster.
func (s *Shoot) GetServiceNetwork() gardenv1beta1.CIDR {
	if k8sNetworks := s.GetK8SNetworks(); k8sNetworks != nil {
		return k8sNetworks.Services
	}
	return ""
}

// GetNodeNetwork returns the node network CIDR for the Shoot cluster.
func (s *Shoot) GetNodeNetwork() gardenv1beta1.CIDR {
	if k8sNetworks := s.GetK8SNetworks(); k8sNetworks != nil {
		return k8sNetworks.Nodes
	}
	return ""
}
