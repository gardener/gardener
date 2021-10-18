// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package seed

import (
	"fmt"
	"net"
	"time"

	"github.com/gardener/gardener/charts"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/botanist/component/dependencywatchdog"
	"github.com/gardener/gardener/pkg/operation/botanist/component/etcd"
	"github.com/gardener/gardener/pkg/operation/botanist/component/gardenerkubescheduler"
	"github.com/gardener/gardener/pkg/operation/botanist/component/kubeapiserver"
	"github.com/gardener/gardener/pkg/operation/botanist/component/networkpolicies"
	"github.com/gardener/gardener/pkg/operation/botanist/component/resourcemanager"
	"github.com/gardener/gardener/pkg/operation/botanist/component/seedadmissioncontroller"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/imagevector"

	"github.com/Masterminds/semver"
	restarterapi "github.com/gardener/dependency-watchdog/pkg/restarter/api"
	scalerapi "github.com/gardener/dependency-watchdog/pkg/scaler/api"
	"k8s.io/component-base/version"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func defaultEtcdDruid(
	c client.Client,
	seedVersion string,
	imageVector imagevector.ImageVector,
	imageVectorOverwrites map[string]string,
) (component.DeployWaiter, error) {
	image, err := imageVector.FindImage(charts.ImageNameEtcdDruid, imagevector.RuntimeVersion(seedVersion), imagevector.TargetVersion(seedVersion))
	if err != nil {
		return nil, err
	}

	var imageVectorOverwrite *string
	if val, ok := imageVectorOverwrites[etcd.Druid]; ok {
		imageVectorOverwrite = &val
	}

	return etcd.NewBootstrapper(c, v1beta1constants.GardenNamespace, image.String(), imageVectorOverwrite), nil
}

func defaultKubeScheduler(c client.Client, imageVector imagevector.ImageVector, kubernetesVersion *semver.Version) (component.DeployWaiter, error) {
	image, err := imageVector.FindImage(charts.ImageNameKubeScheduler, imagevector.TargetVersion(kubernetesVersion.String()))
	if err != nil {
		return nil, err
	}

	scheduler, err := gardenerkubescheduler.Bootstrap(c, v1beta1constants.GardenNamespace, image, kubernetesVersion)
	if err != nil {
		return nil, err
	}

	return scheduler, nil
}

func defaultGardenerSeedAdmissionController(c client.Client, imageVector imagevector.ImageVector) (component.DeployWaiter, error) {
	image, err := imageVector.FindImage(charts.ImageNameGardenerSeedAdmissionController)
	if err != nil {
		return nil, err
	}

	repository, tag := image.String(), version.Get().GitVersion
	if image.Tag != nil {
		repository, tag = image.Repository, *image.Tag
	}
	image = &imagevector.Image{Repository: repository, Tag: &tag}

	return seedadmissioncontroller.New(c, v1beta1constants.GardenNamespace, image.String()), nil
}

func defaultGardenerResourceManager(c client.Client, seedVersion string, imageVector imagevector.ImageVector) (component.DeployWaiter, error) {
	image, err := imageVector.FindImage(charts.ImageNameGardenerResourceManager)
	if err != nil {
		return nil, err
	}

	repository, tag := image.String(), version.Get().GitVersion
	if image.Tag != nil {
		repository, tag = image.Repository, *image.Tag
	}
	image = &imagevector.Image{Repository: repository, Tag: &tag}

	return resourcemanager.New(c, v1beta1constants.GardenNamespace, image.String(), 1, resourcemanager.Values{
		ConcurrentSyncs:  pointer.Int32(20),
		HealthSyncPeriod: utils.DurationPtr(time.Minute),
		ResourceClass:    pointer.String(v1beta1constants.SeedResourceManagerClass),
		SyncPeriod:       utils.DurationPtr(time.Hour),
	}), nil
}

func defaultNetworkPolicies(c client.Client, seed *gardencorev1beta1.Seed, sniEnabled bool) (component.DeployWaiter, error) {
	networks := []string{seed.Spec.Networks.Pods, seed.Spec.Networks.Services}
	if v := seed.Spec.Networks.Nodes; v != nil {
		networks = append(networks, *v)
	}
	privateNetworkPeers, err := networkpolicies.ToNetworkPolicyPeersWithExceptions(networkpolicies.AllPrivateNetworkBlocks(), networks...)
	if err != nil {
		return nil, err
	}

	_, seedServiceCIDR, err := net.ParseCIDR(seed.Spec.Networks.Services)
	if err != nil {
		return nil, err
	}
	seedDNSServerAddress, err := common.ComputeOffsetIP(seedServiceCIDR, 10)
	if err != nil {
		return nil, fmt.Errorf("cannot calculate CoreDNS ClusterIP: %v", err)
	}

	return networkpolicies.NewBootstrapper(c, v1beta1constants.GardenNamespace, networkpolicies.GlobalValues{
		SNIEnabled:           sniEnabled,
		DenyAllTraffic:       false,
		PrivateNetworkPeers:  privateNetworkPeers,
		NodeLocalIPVSAddress: pointer.String(common.NodeLocalIPVSAddress),
		DNSServerAddress:     pointer.String(seedDNSServerAddress.String()),
	}), nil
}

func defaultDependencyWatchdogs(
	c client.Client,
	seedVersion string,
	imageVector imagevector.ImageVector,
) (
	dwdEndpoint component.DeployWaiter,
	dwdProbe component.DeployWaiter,
	err error,
) {
	// Fetch component-specific dependency-watchdog configuration
	var (
		dependencyWatchdogEndpointConfigurationFuncs = []dependencywatchdog.EndpointConfigurationFunc{
			func() (map[string]restarterapi.Service, error) {
				return etcd.DependencyWatchdogEndpointConfiguration(v1beta1constants.ETCDRoleMain)
			},
			kubeapiserver.DependencyWatchdogEndpointConfiguration,
		}
		dependencyWatchdogEndpointConfigurations = restarterapi.ServiceDependants{
			Services: make(map[string]restarterapi.Service, len(dependencyWatchdogEndpointConfigurationFuncs)),
		}

		dependencyWatchdogProbeConfigurationFuncs = []dependencywatchdog.ProbeConfigurationFunc{
			kubeapiserver.DependencyWatchdogProbeConfiguration,
		}
		dependencyWatchdogProbeConfigurations = scalerapi.ProbeDependantsList{
			Probes: make([]scalerapi.ProbeDependants, 0, len(dependencyWatchdogProbeConfigurationFuncs)),
		}
	)

	for _, componentFn := range dependencyWatchdogEndpointConfigurationFuncs {
		dwdConfig, err := componentFn()
		if err != nil {
			return nil, nil, err
		}
		for k, v := range dwdConfig {
			dependencyWatchdogEndpointConfigurations.Services[k] = v
		}
	}

	for _, componentFn := range dependencyWatchdogProbeConfigurationFuncs {
		dwdConfig, err := componentFn()
		if err != nil {
			return nil, nil, err
		}
		dependencyWatchdogProbeConfigurations.Probes = append(dependencyWatchdogProbeConfigurations.Probes, dwdConfig...)
	}

	image, err := imageVector.FindImage(charts.ImageNameDependencyWatchdog, imagevector.RuntimeVersion(seedVersion), imagevector.TargetVersion(seedVersion))
	if err != nil {
		return nil, nil, err
	}

	dwdEndpoint = dependencywatchdog.New(c, v1beta1constants.GardenNamespace, dependencywatchdog.Values{
		Role:           dependencywatchdog.RoleEndpoint,
		Image:          image.String(),
		ValuesEndpoint: dependencywatchdog.ValuesEndpoint{ServiceDependants: dependencyWatchdogEndpointConfigurations},
	})
	dwdProbe = dependencywatchdog.New(c, v1beta1constants.GardenNamespace, dependencywatchdog.Values{
		Role:        dependencywatchdog.RoleProbe,
		Image:       image.String(),
		ValuesProbe: dependencywatchdog.ValuesProbe{ProbeDependantsList: dependencyWatchdogProbeConfigurations},
	})
	return
}
