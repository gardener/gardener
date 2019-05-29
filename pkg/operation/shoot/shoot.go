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
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Masterminds/semver"

	corev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/apis/garden/v1beta1/helper"
	gardeninformers "github.com/gardener/gardener/pkg/client/garden/informers/externalversions/garden/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/operation/garden"
	"github.com/gardener/gardener/pkg/utils"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// New takes a <k8sGardenClient>, the <k8sGardenInformers> and a <shoot> manifest, and creates a new Shoot representation.
// It will add the CloudProfile, the cloud provider secret, compute the internal cluster domain and identify the cloud provider.
func New(k8sGardenClient kubernetes.Interface, k8sGardenInformers gardeninformers.Interface, shoot *gardenv1beta1.Shoot, projectName, internalDomain string, defaultDomains []*garden.DefaultDomain) (*Shoot, error) {
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

	seedNamespace := ComputeTechnicalID(projectName, shoot)

	extensions, err := calculateExtensions(k8sGardenClient.Client(), shoot, seedNamespace)
	if err != nil {
		return nil, fmt.Errorf("Cannot calculate required extensions for shoot %s: %v", shoot.Name, err)
	}

	shootObj := &Shoot{
		Info:         shoot,
		Secret:       secret,
		CloudProfile: cloudProfile,

		SeedNamespace: seedNamespace,

		InternalClusterDomain: ConstructInternalClusterDomain(shoot.Name, projectName, internalDomain),
		ExternalClusterDomain: ConstructExternalClusterDomain(shoot),

		IsHibernated:           helper.IsShootHibernated(shoot),
		WantsClusterAutoscaler: false,

		Extensions: extensions,
	}
	shootObj.CloudConfigMap = make(map[string]CloudConfig, len(shootObj.GetWorkerNames()))

	// Determine information about external domain for shoot cluster.
	externalDomain, err := ConstructExternalDomain(context.TODO(), k8sGardenClient.Client(), shoot, secret, defaultDomains)
	if err != nil {
		return nil, err
	}
	shootObj.ExternalDomain = externalDomain

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

	needsAutoscaler, err := helper.ShootWantsClusterAutoscaler(shoot)
	if err != nil {
		return nil, err
	}
	shootObj.WantsClusterAutoscaler = needsAutoscaler

	return shootObj, nil
}

func calculateExtensions(gardenClient client.Client, shoot *gardenv1beta1.Shoot, seedNamespace string) (map[string]Extension, error) {
	var controllerRegistrations = &corev1alpha1.ControllerRegistrationList{}
	if err := gardenClient.List(context.TODO(), nil, controllerRegistrations); err != nil {
		return nil, err
	}
	return MergeExtensions(controllerRegistrations.Items, shoot.Spec.Extensions, seedNamespace)
}

// GetIngressFQDN returns the fully qualified domain name of ingress sub-resource for the Shoot cluster. The
// end result is '<subDomain>.<ingressPrefix>.<clusterDomain>'.
func (s *Shoot) GetIngressFQDN(subDomain string) string {
	return fmt.Sprintf("%s.%s.%s", subDomain, common.IngressPrefix, *(s.Info.Spec.DNS.Domain))
}

// GetWorkers returns a list of worker objects of the worker groups in the Shoot manifest.
func (s *Shoot) GetWorkers() []gardenv1beta1.Worker {
	return helper.GetShootCloudProviderWorkers(s.CloudProvider, s.Info)
}

// GetMachineTypesFromCloudProfile returns a list of machine types in the cloud profile.
func (s *Shoot) GetMachineTypesFromCloudProfile() []gardenv1beta1.MachineType {
	return helper.GetMachineTypesFromCloudProfile(s.CloudProvider, s.CloudProfile)
}

// GetWorkerNames returns a list of names of the worker groups in the Shoot manifest.
func (s *Shoot) GetWorkerNames() []string {
	workerNames := []string{}
	for _, worker := range s.GetWorkers() {
		workerNames = append(workerNames, worker.Name)
	}
	return workerNames
}

// GetNodeCount returns the sum of all 'autoScalerMax' fields of all worker groups of the Shoot.
func (s *Shoot) GetNodeCount() int {
	nodeCount := 0
	for _, worker := range s.GetWorkers() {
		nodeCount += worker.AutoScalerMax
	}
	return nodeCount
}

// GetK8SNetworks returns the Kubernetes network CIDRs for the Shoot cluster.
func (s *Shoot) GetK8SNetworks() *gardencorev1alpha1.K8SNetworks {
	switch s.CloudProvider {
	case gardenv1beta1.CloudProviderAWS:
		return &s.Info.Spec.Cloud.AWS.Networks.K8SNetworks
	case gardenv1beta1.CloudProviderAzure:
		return &s.Info.Spec.Cloud.Azure.Networks.K8SNetworks
	case gardenv1beta1.CloudProviderGCP:
		return &s.Info.Spec.Cloud.GCP.Networks.K8SNetworks
	case gardenv1beta1.CloudProviderOpenStack:
		return &s.Info.Spec.Cloud.OpenStack.Networks.K8SNetworks
	case gardenv1beta1.CloudProviderAlicloud:
		return &s.Info.Spec.Cloud.Alicloud.Networks.K8SNetworks
	}
	return nil
}

// GetWorkerVolumesByName returns the volume information for the specific worker pool (if there
// is any volume information).
func (s *Shoot) GetWorkerVolumesByName(workerName string) (ok bool, volumeType, volumeSize string, err error) {
	switch s.CloudProvider {
	case gardenv1beta1.CloudProviderAWS:
		for _, worker := range s.Info.Spec.Cloud.AWS.Workers {
			if worker.Name == workerName {
				ok = true
				volumeType = worker.VolumeType
				volumeSize = worker.VolumeSize
				return
			}
		}
	case gardenv1beta1.CloudProviderAzure:
		for _, worker := range s.Info.Spec.Cloud.Azure.Workers {
			if worker.Name == workerName {
				ok = true
				volumeType = worker.VolumeType
				volumeSize = worker.VolumeSize
				return
			}
		}
	case gardenv1beta1.CloudProviderGCP:
		for _, worker := range s.Info.Spec.Cloud.GCP.Workers {
			if worker.Name == workerName {
				ok = true
				volumeType = worker.VolumeType
				volumeSize = worker.VolumeSize
				return
			}
		}
	case gardenv1beta1.CloudProviderOpenStack:
		return
	case gardenv1beta1.CloudProviderAlicloud:
		for _, worker := range s.Info.Spec.Cloud.Alicloud.Workers {
			if worker.Name == workerName {
				ok = true
				volumeType = worker.VolumeType
				volumeSize = worker.VolumeSize
				return
			}
		}
	case gardenv1beta1.CloudProviderPacket:
		for _, worker := range s.Info.Spec.Cloud.Packet.Workers {
			if worker.Name == workerName {
				ok = true
				volumeType = worker.VolumeType
				volumeSize = worker.VolumeSize
				return
			}
		}
	}

	return false, "", "", fmt.Errorf("could not find worker with name %q", workerName)
}

// GetZones returns the zones of the shoot cluster.
func (s *Shoot) GetZones() []string {
	switch s.CloudProvider {
	case gardenv1beta1.CloudProviderAWS:
		return s.Info.Spec.Cloud.AWS.Zones
	case gardenv1beta1.CloudProviderAzure:
		return nil
	case gardenv1beta1.CloudProviderGCP:
		return s.Info.Spec.Cloud.GCP.Zones
	case gardenv1beta1.CloudProviderOpenStack:
		return s.Info.Spec.Cloud.OpenStack.Zones
	case gardenv1beta1.CloudProviderAlicloud:
		return s.Info.Spec.Cloud.Alicloud.Zones
	}
	return nil
}

// GetPodNetwork returns the pod network CIDR for the Shoot cluster.
func (s *Shoot) GetPodNetwork() gardencorev1alpha1.CIDR {
	if k8sNetworks := s.GetK8SNetworks(); k8sNetworks != nil {
		return *k8sNetworks.Pods
	}
	return ""
}

// GetServiceNetwork returns the service network CIDR for the Shoot cluster.
func (s *Shoot) GetServiceNetwork() gardencorev1alpha1.CIDR {
	if k8sNetworks := s.GetK8SNetworks(); k8sNetworks != nil {
		return *k8sNetworks.Services
	}
	return ""
}

// GetNodeNetwork returns the node network CIDR for the Shoot cluster.
func (s *Shoot) GetNodeNetwork() gardencorev1alpha1.CIDR {
	if k8sNetworks := s.GetK8SNetworks(); k8sNetworks != nil {
		return *k8sNetworks.Nodes
	}
	return ""
}

// GetMachineImage returns the name of the used machine image.
func (s *Shoot) GetMachineImage() *gardenv1beta1.MachineImage {
	return helper.GetMachineImageFromShoot(s.CloudProvider, s.Info)
}

// ClusterAutoscalerEnabled returns true if the cluster-autoscaler addon is enabled in the Shoot manifest.
func (s *Shoot) ClusterAutoscalerEnabled() bool {
	return s.Info.Spec.Addons != nil && s.Info.Spec.Addons.ClusterAutoscaler != nil && s.Info.Spec.Addons.ClusterAutoscaler.Enabled
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

// ComputeCloudConfigSecretName computes the name for a secret which contains the original cloud config for
// the worker group with the given <workerName>. It is build by the cloud config secret prefix, the worker
// name itself and a hash of the minor Kubernetes version of the Shoot cluster.
func (s *Shoot) ComputeCloudConfigSecretName(workerName string) string {
	return fmt.Sprintf("%s-%s-%s", common.CloudConfigPrefix, workerName, utils.ComputeSHA256Hex([]byte(s.KubernetesMajorMinorVersion))[:5])
}

// GetReplicas returns the given <wokenUp> number if the shoot is not hibernated, or zero otherwise.
func (s *Shoot) GetReplicas(wokenUp int) int {
	if s.IsHibernated {
		return 0
	}
	return wokenUp
}

// UsesCSI returns whether the given shoot uses CSI volume plugins.
func (s *Shoot) UsesCSI() bool {
	return s.CloudProvider == gardenv1beta1.CloudProviderAlicloud
}

// ComputeAPIServerURL takes a boolean value identifying whether the component connecting to the API server
// runs in the Seed cluster <runsInSeed>, and a boolean value <useInternalClusterDomain> which determines whether the
// internal or the external cluster domain should be used.
func (s *Shoot) ComputeAPIServerURL(runsInSeed, useInternalClusterDomain bool) string {
	if runsInSeed {
		return common.KubeAPIServerDeploymentName
	}

	if dnsProvider := s.Info.Spec.DNS.Provider; dnsProvider != nil && *dnsProvider == gardenv1beta1.DNSUnmanaged {
		return s.InternalClusterDomain
	}

	if useInternalClusterDomain {
		return s.InternalClusterDomain
	}

	return *(s.ExternalClusterDomain)
}

// IPVSEnabled returns true if IPVS is enabled for the shoot.
func (s *Shoot) IPVSEnabled() bool {
	return s.Info.Spec.Kubernetes.KubeProxy != nil &&
		s.Info.Spec.Kubernetes.KubeProxy.Mode != nil &&
		*s.Info.Spec.Kubernetes.KubeProxy.Mode == gardenv1beta1.ProxyModeIPVS
}

// ComputeTechnicalID determines the technical id of that Shoot which is later used for the name of the
// namespace and for tagging all the resources created in the infrastructure.
func ComputeTechnicalID(projectName string, shoot *gardenv1beta1.Shoot) string {
	// Use the stored technical ID in the Shoot's status field if it's there.
	// For backwards compatibility we keep the pattern as it was before we had to change it
	// (double hyphens).
	if len(shoot.Status.TechnicalID) > 0 {
		return shoot.Status.TechnicalID
	}

	// New clusters shall be created with the new technical id (double hyphens).
	return fmt.Sprintf("shoot--%s--%s", projectName, shoot.Name)
}

// ConstructInternalClusterDomain constructs the domain pointing to the kube-apiserver of a Shoot cluster
// which is only used for internal purposes (all kubeconfigs except the one which is received by the
// user will only talk with the kube-apiserver via this domain). In case the given <internalDomain>
// already contains "internal", the result is constructed as "api.<shootName>.<shootProject>.<internalDomain>."
// In case it does not, the word "internal" will be appended, resulting in
// "api.<shootName>.<shootProject>.internal.<internalDomain>".
func ConstructInternalClusterDomain(shootName, shootProject, internalDomain string) string {
	if strings.Contains(internalDomain, common.InternalDomainKey) {
		return fmt.Sprintf("api.%s.%s.%s", shootName, shootProject, internalDomain)
	}
	return fmt.Sprintf("api.%s.%s.%s.%s", shootName, shootProject, common.InternalDomainKey, internalDomain)
}

// ConstructExternalClusterDomain constructs the external Shoot cluster domain, i.e. the domain which will be put
// into the Kubeconfig handed out to the user.
func ConstructExternalClusterDomain(shoot *gardenv1beta1.Shoot) *string {
	if shoot.Spec.DNS.Domain == nil {
		return nil
	}

	domain := fmt.Sprintf("api.%s", *(shoot.Spec.DNS.Domain))
	return &domain
}

// ConstructExternalDomain constructs an object containing all relevant information of the external domain that
// shall be used for a shoot cluster - based on the configuration of the Garden cluster and the shoot itself.
func ConstructExternalDomain(ctx context.Context, client client.Client, shoot *gardenv1beta1.Shoot, shootSecret *corev1.Secret, defaultDomains []*garden.DefaultDomain) (*ExternalDomain, error) {
	externalClusterDomain := ConstructExternalClusterDomain(shoot)
	if externalClusterDomain == nil {
		return nil, nil
	}

	var (
		externalDomain = &ExternalDomain{Domain: *shoot.Spec.DNS.Domain}
		defaultDomain  = garden.DomainIsDefaultDomain(*externalClusterDomain, defaultDomains)
	)

	switch {
	case shoot.Spec.DNS.SecretName != nil && shoot.Spec.DNS.Provider != nil:
		secret := &corev1.Secret{}
		if err := client.Get(ctx, kutil.Key(shoot.Namespace, *shoot.Spec.DNS.SecretName), secret); err != nil {
			return nil, err
		}
		externalDomain.SecretData = secret.Data
		externalDomain.Provider = *shoot.Spec.DNS.Provider

	case defaultDomain != nil:
		externalDomain.SecretData = defaultDomain.SecretData
		externalDomain.Provider = defaultDomain.Provider

	case shoot.Spec.DNS.Provider != nil && shoot.Spec.DNS.SecretName == nil:
		externalDomain.SecretData = shootSecret.Data
		externalDomain.Provider = *shoot.Spec.DNS.Provider

	default:
		return nil, fmt.Errorf("unable to figure out which secret should be used for dns")
	}

	return externalDomain, nil
}

// ExtensionDefaultTimeout is the default timeout and defines how long Gardener should wait
// for a successful reconcilation of this extension resource.
const ExtensionDefaultTimeout = 10 * time.Minute

// MergeExtensions merges the given controller registrations with the given extensions, expecting that each type in extensions is also represented in the registration.
func MergeExtensions(registrations []corev1alpha1.ControllerRegistration, extensions []gardenv1beta1.Extension, namespace string) (map[string]Extension, error) {
	var (
		typeToExtension    = make(map[string]Extension)
		requiredExtensions = make(map[string]Extension)
	)
	// Extensions enabled by default for all Shoot clusters.
	for _, reg := range registrations {
		for _, res := range reg.Spec.Resources {
			if res.Kind != extensionsv1alpha1.ExtensionResource {
				continue
			}

			var timeout time.Duration
			if res.ReconcileTimeout != nil {
				timeout = res.ReconcileTimeout.Duration
			} else {
				timeout = ExtensionDefaultTimeout
			}

			typeToExtension[res.Type] = Extension{
				Extension: extensionsv1alpha1.Extension{
					ObjectMeta: metav1.ObjectMeta{
						Name:      res.Type,
						Namespace: namespace,
					},
					Spec: extensionsv1alpha1.ExtensionSpec{
						DefaultSpec: extensionsv1alpha1.DefaultSpec{
							Type: res.Type,
						},
					},
				},
				Timeout: timeout,
			}

			if res.GloballyEnabled != nil && *res.GloballyEnabled {
				requiredExtensions[res.Type] = typeToExtension[res.Type]
			}
		}
	}

	// Extensions defined in Shoot resource.
	for _, extension := range extensions {
		obj, ok := typeToExtension[extension.Type]
		if ok {
			if extension.ProviderConfig != nil {
				providerConfig := extension.ProviderConfig.RawExtension
				obj.Spec.ProviderConfig = &providerConfig
			}
			requiredExtensions[extension.Type] = obj
			continue
		}
	}

	return requiredExtensions, nil
}
