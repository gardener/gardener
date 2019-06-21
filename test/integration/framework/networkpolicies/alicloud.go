// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package networkpolicies

import (
	"github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
)

var (

	// AlicloudCloudControllerManagerInfoNotSecured points to alicloud-specific cloud-controller-manager.
	// For now it listens only on HTTP for all Shoot versions.
	AlicloudCloudControllerManagerInfoNotSecured = &SourcePod{
		Ports: NewSinglePort(10253),
		Pod: NewPod("cloud-controller-manager-http", labels.Set{
			"app":                     "kubernetes",
			"garden.sapcloud.io/role": "controlplane",
			"role":                    "cloud-controller-manager",
		}),
		ExpectedPolicies: sets.NewString(
			"allow-from-prometheus",
			"allow-to-dns",
			"allow-to-public-networks",
			"allow-to-shoot-apiserver",
			"deny-all",
		),
	}

	// AlicloudKubeControllerManagerInfoSecured points to alicloud-specific kube-controller-manager.
	AlicloudKubeControllerManagerInfoSecured = &SourcePod{
		Ports: NewSinglePort(10257),
		Pod: NewPod("kube-controller-manager-https", labels.Set{
			"app":                     "kubernetes",
			"garden.sapcloud.io/role": "controlplane",
			"role":                    "controller-manager",
		}, ">= 1.13"),
		ExpectedPolicies: sets.NewString(
			"allow-from-prometheus",
			"allow-to-dns",
			"allow-to-shoot-apiserver",
			"deny-all",
		),
	}

	// AlicloudKubeControllerManagerInfoNotSecured points to alicloud-specific kube-controller-manager.
	AlicloudKubeControllerManagerInfoNotSecured = &SourcePod{
		Ports: NewSinglePort(10252),
		Pod: NewPod("kube-controller-manager-http", labels.Set{
			"app":                     "kubernetes",
			"garden.sapcloud.io/role": "controlplane",
			"role":                    "controller-manager",
		}, "< 1.13"),
		ExpectedPolicies: sets.NewString(
			"allow-from-prometheus",
			"allow-to-dns",
			"allow-to-shoot-apiserver",
			"deny-all",
		),
	}

	// AlicloudCSIPluginInfo points to alicloud-specific CSI Plugin.
	AlicloudCSIPluginInfo = &SourcePod{
		Ports: NewSinglePort(80),
		Pod: NewPod("csi-plugin-controller", labels.Set{
			"app":                     "kubernetes",
			"garden.sapcloud.io/role": "controlplane",
			"role":                    "csi-plugin-controller",
		}),
		ExpectedPolicies: sets.NewString(
			"allow-to-public-networks",
			"allow-to-dns",
			"allow-to-shoot-apiserver",
			"deny-all",
		),
	}

	// AlicloudMetadataServiceHost points to alicloud-specific Metadata service.
	AlicloudMetadataServiceHost = &Host{
		Description: "Metadata service",
		HostName:    "100.100.100.200",
		Port:        80,
	}
)

var _ CloudAwarePodInfo = &AlicloudNetworkPolicy{}

// AlicloudNetworkPolicy holds alicloud-specific network policy settings.
// +gen-netpoltests=true
// +gen-packagename=alicloud
type AlicloudNetworkPolicy struct {
}

// ToSources returns list of all alicloud-specific sources and targets.
func (a *AlicloudNetworkPolicy) ToSources() []Rule {

	return []Rule{
		a.newSource(KubeAPIServerInfo).AllowPod(EtcdMainInfo, EtcdEventsInfo).AllowHost(SeedKubeAPIServer, ExternalHost).Build(),
		a.newSource(EtcdMainInfo).AllowHost(ExternalHost).Build(),
		a.newSource(EtcdEventsInfo).AllowHost(ExternalHost).Build(),
		a.newSource(AlicloudCloudControllerManagerInfoNotSecured).AllowPod(KubeAPIServerInfo).AllowHost(ExternalHost).Build(),
		a.newSource(DependencyWatchdog).AllowHost(SeedKubeAPIServer, ExternalHost).Build(),
		a.newSource(ElasticSearchInfo).Build(),
		a.newSource(GrafanaInfo).AllowPod(PrometheusInfo).Build(),
		a.newSource(KibanaInfo).AllowTargetPod(ElasticSearchInfo.FromPort("http")).Build(),
		a.newSource(AddonManagerInfo).AllowPod(KubeAPIServerInfo).AllowHost(SeedKubeAPIServer, ExternalHost).Build(),
		a.newSource(AlicloudKubeControllerManagerInfoSecured).AllowPod(KubeAPIServerInfo).Build(),
		a.newSource(AlicloudKubeControllerManagerInfoNotSecured).AllowPod(KubeAPIServerInfo).Build(),
		a.newSource(KubeSchedulerInfoNotSecured).AllowPod(KubeAPIServerInfo).Build(),
		a.newSource(KubeSchedulerInfoSecured).AllowPod(KubeAPIServerInfo).Build(),
		a.newSource(KubeStateMetricsShootInfo).AllowPod(KubeAPIServerInfo).Build(),
		a.newSource(KubeStateMetricsSeedInfo).AllowHost(SeedKubeAPIServer, ExternalHost).Build(),
		a.newSource(MachineControllerManagerInfo).AllowPod(KubeAPIServerInfo).AllowHost(SeedKubeAPIServer, ExternalHost).Build(),
		a.newSource(AlicloudCSIPluginInfo).AllowPod(KubeAPIServerInfo).AllowHost(ExternalHost).Build(),
		a.newSource(PrometheusInfo).AllowPod(
			AlicloudCloudControllerManagerInfoNotSecured,
			AlicloudKubeControllerManagerInfoNotSecured,
			AlicloudKubeControllerManagerInfoSecured,
			EtcdEventsInfo,
			EtcdMainInfo,
			KubeAPIServerInfo,
			KubeSchedulerInfoNotSecured,
			KubeSchedulerInfoSecured,
			KubeStateMetricsSeedInfo,
			KubeStateMetricsShootInfo,
			MachineControllerManagerInfo,
		).AllowTargetPod(ElasticSearchInfo.FromPort("metrics")).AllowHost(SeedKubeAPIServer, ExternalHost, GardenPrometheus).Build(),
	}
}

// EgressFromOtherNamespaces returns list of all alicloud-specific sources and targets.
func (a *AlicloudNetworkPolicy) EgressFromOtherNamespaces(sourcePod *SourcePod) Rule {
	return NewSource(sourcePod).DenyPod(a.allPods()...).AllowPod(KubeAPIServerInfo).Build()
}

func (a *AlicloudNetworkPolicy) newSource(sourcePod *SourcePod) *RuleBuilder {
	return NewSource(sourcePod).DenyPod(a.allPods()...).DenyHost(AlicloudMetadataServiceHost, ExternalHost, GardenPrometheus)
}

func (a *AlicloudNetworkPolicy) allPods() []*SourcePod {
	return []*SourcePod{
		AddonManagerInfo,
		AlicloudCloudControllerManagerInfoNotSecured,
		AlicloudCSIPluginInfo,
		AlicloudKubeControllerManagerInfoNotSecured,
		AlicloudKubeControllerManagerInfoSecured,
		DependencyWatchdog,
		ElasticSearchInfo,
		EtcdEventsInfo,
		EtcdMainInfo,
		GrafanaInfo,
		KibanaInfo,
		KubeAPIServerInfo,
		KubeSchedulerInfoNotSecured,
		KubeSchedulerInfoSecured,
		KubeStateMetricsSeedInfo,
		KubeStateMetricsShootInfo,
		MachineControllerManagerInfo,
		PrometheusInfo,
	}
}

// Provider returns Alicloud cloud provider.
func (a *AlicloudNetworkPolicy) Provider() v1beta1.CloudProvider {
	return v1beta1.CloudProviderAlicloud
}
