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
)

var (

	// GCPMetadataServiceHost points to gcp-specific Metadata service.
	GCPMetadataServiceHost = &Host{
		Description: "Metadata service",
		HostName:    "169.254.169.254",
		Port:        80,
	}
)

// GCPNetworkPolicy holds gcp-specific network policy settings.
// +gen-netpoltests=true
// +gen-packagename=gcp
type GCPNetworkPolicy struct {
}

// ToSources returns list of all gcp-specific sources and targets.
func (a *GCPNetworkPolicy) ToSources() []Rule {

	return []Rule{
		a.newSource(KubeAPIServerInfo).AllowPod(EtcdMainInfo, EtcdEventsInfo).AllowHost(SeedKubeAPIServer, ExternalHost).Build(),
		a.newSource(EtcdMainInfo).AllowHost(ExternalHost).Build(),
		a.newSource(EtcdEventsInfo).AllowHost(ExternalHost).Build(),
		a.newSource(CloudControllerManagerInfoNotSecured).AllowPod(KubeAPIServerInfo).AllowHost(ExternalHost).Build(),
		a.newSource(CloudControllerManagerInfoSecured).AllowPod(KubeAPIServerInfo).AllowHost(ExternalHost).Build(),
		a.newSource(DependencyWatchdog).AllowHost(SeedKubeAPIServer, ExternalHost).Build(),
		a.newSource(ElasticSearchInfo).Build(),
		a.newSource(GrafanaInfo).AllowPod(PrometheusInfo).Build(),
		a.newSource(KibanaInfo).AllowTargetPod(ElasticSearchInfo.FromPort("http")).Build(),
		a.newSource(AddonManagerInfo).AllowPod(KubeAPIServerInfo).AllowHost(SeedKubeAPIServer, ExternalHost).Build(),
		a.newSource(KubeControllerManagerInfoNotSecured).AllowPod(KubeAPIServerInfo).AllowHost(GCPMetadataServiceHost, ExternalHost).Build(),
		a.newSource(KubeControllerManagerInfoSecured).AllowPod(KubeAPIServerInfo).AllowHost(GCPMetadataServiceHost, ExternalHost).Build(),
		a.newSource(KubeSchedulerInfoNotSecured).AllowPod(KubeAPIServerInfo).Build(),
		a.newSource(KubeSchedulerInfoSecured).AllowPod(KubeAPIServerInfo).Build(),
		a.newSource(KubeStateMetricsShootInfo).AllowPod(KubeAPIServerInfo).Build(),
		a.newSource(KubeStateMetricsSeedInfo).AllowHost(SeedKubeAPIServer, ExternalHost).Build(),
		a.newSource(MachineControllerManagerInfo).AllowPod(KubeAPIServerInfo).AllowHost(SeedKubeAPIServer, ExternalHost).Build(),
		a.newSource(PrometheusInfo).AllowPod(
			CloudControllerManagerInfoNotSecured,
			CloudControllerManagerInfoSecured,
			EtcdEventsInfo,
			EtcdMainInfo,
			KubeAPIServerInfo,
			KubeControllerManagerInfoNotSecured,
			KubeControllerManagerInfoSecured,
			KubeSchedulerInfoNotSecured,
			KubeSchedulerInfoSecured,
			KubeStateMetricsSeedInfo,
			KubeStateMetricsShootInfo,
			MachineControllerManagerInfo,
		).AllowTargetPod(ElasticSearchInfo.FromPort("metrics")).AllowHost(SeedKubeAPIServer, ExternalHost, GardenPrometheus).Build(),
	}
}

// EgressFromOtherNamespaces returns list of all gcp-specific sources and targets.
func (a *GCPNetworkPolicy) EgressFromOtherNamespaces(sourcePod *SourcePod) Rule {
	return NewSource(sourcePod).DenyPod(a.allPods()...).AllowPod(KubeAPIServerInfo).Build()
}

func (a *GCPNetworkPolicy) newSource(sourcePod *SourcePod) *RuleBuilder {
	return NewSource(sourcePod).DenyPod(a.allPods()...).DenyHost(GCPMetadataServiceHost, ExternalHost, GardenPrometheus)
}

func (a *GCPNetworkPolicy) allPods() []*SourcePod {
	return []*SourcePod{
		AddonManagerInfo,
		CloudControllerManagerInfoNotSecured,
		CloudControllerManagerInfoSecured,
		DependencyWatchdog,
		ElasticSearchInfo,
		EtcdEventsInfo,
		EtcdMainInfo,
		GrafanaInfo,
		KibanaInfo,
		KubeAPIServerInfo,
		KubeControllerManagerInfoNotSecured,
		KubeSchedulerInfoSecured,
		KubeStateMetricsSeedInfo,
		KubeStateMetricsShootInfo,
		MachineControllerManagerInfo,
		PrometheusInfo,
	}
}

// Provider returns GCP cloud provider.
func (a *GCPNetworkPolicy) Provider() v1beta1.CloudProvider {
	return v1beta1.CloudProviderGCP
}
