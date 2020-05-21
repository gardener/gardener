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
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
)

// Agnostic contains cloud agnostic settings.
type Agnostic struct{}

// KubeAPIServer points to cloud-agnostic kube-apiserver.
func (a *Agnostic) KubeAPIServer() *SourcePod {
	return &SourcePod{
		Ports: NewSinglePort(443),
		Pod: NewPod("kube-apiserver", labels.Set{
			"app":  "kubernetes",
			"role": "apiserver",
		}),
		ExpectedPolicies: sets.NewString(
			"allow-from-prometheus",
			"allow-kube-apiserver",
			"allow-to-dns",
			"allow-to-public-networks",
			"allow-to-private-networks",
			"allow-to-shoot-networks",
			"deny-all",
		),
	}
}

// KubeControllerManagerSecured points to cloud-agnostic kube-controller-manager running on HTTPS port.
func (a *Agnostic) KubeControllerManagerSecured() *SourcePod {
	return &SourcePod{
		Ports: NewSinglePort(10257),
		Pod: NewPod("kube-controller-manager-https", labels.Set{
			"app":                     "kubernetes",
			"garden.sapcloud.io/role": "controlplane",
			"role":                    "controller-manager",
		}, ">= 1.13"),
		ExpectedPolicies: sets.NewString(
			"allow-to-public-networks",
			"allow-to-private-networks",
			"allow-from-prometheus",
			"allow-to-dns",
			"allow-to-blocked-cidrs",
			"allow-to-shoot-apiserver",
			"deny-all",
		),
	}
}

// KubeControllerManagerNotSecured points to cloud-agnostic kube-controller-manager running on HTTP port.
func (a *Agnostic) KubeControllerManagerNotSecured() *SourcePod {
	return &SourcePod{
		Ports: NewSinglePort(10252),
		Pod: NewPod("kube-controller-manager-http", labels.Set{
			"app":                     "kubernetes",
			"garden.sapcloud.io/role": "controlplane",
			"role":                    "controller-manager",
		}, "< 1.13"),
		ExpectedPolicies: sets.NewString(
			"allow-to-public-networks",
			"allow-to-private-networks",
			"allow-from-prometheus",
			"allow-to-dns",
			"allow-to-blocked-cidrs",
			"allow-to-shoot-apiserver",
			"deny-all",
		),
	}
}

// KubeSchedulerSecured points to cloud-agnostic kube-scheduler running on HTTPS port.
func (a *Agnostic) KubeSchedulerSecured() *SourcePod {
	return &SourcePod{
		Ports: NewSinglePort(10259),
		Pod: NewPod("kube-scheduler-https", labels.Set{
			"app":                     "kubernetes",
			"garden.sapcloud.io/role": "controlplane",
			"role":                    "scheduler",
		}, ">= 1.13"),
		ExpectedPolicies: sets.NewString(
			"allow-from-prometheus",
			"allow-to-shoot-apiserver",
			"allow-to-dns",
			"deny-all",
		),
	}
}

// KubeSchedulerNotSecured points to cloud-agnostic kube-scheduler running on HTTP port.
func (a *Agnostic) KubeSchedulerNotSecured() *SourcePod {
	return &SourcePod{
		Ports: NewSinglePort(10251),
		Pod: NewPod("kube-scheduler-http", labels.Set{
			"app":                     "kubernetes",
			"garden.sapcloud.io/role": "controlplane",
			"role":                    "scheduler",
		}, "< 1.13"),
		ExpectedPolicies: sets.NewString(
			"allow-from-prometheus",
			"allow-to-shoot-apiserver",
			"allow-to-dns",
			"deny-all",
		),
	}
}

// EtcdMain points to cloud-agnostic etcd-main instance.
func (a *Agnostic) EtcdMain() *SourcePod {
	return &SourcePod{
		Ports: NewSinglePort(2379),
		Pod: NewPod("etcd-main", labels.Set{
			"app":                     "etcd-statefulset",
			"garden.sapcloud.io/role": "controlplane",
			"role":                    "main",
		}),
		ExpectedPolicies: sets.NewString(
			"allow-etcd",
			"allow-to-dns",
			"allow-to-public-networks",
			"allow-to-private-networks",
			"deny-all",
		),
	}
}

// EtcdEvents points to cloud-agnostic etcd-main instance.
func (a *Agnostic) EtcdEvents() *SourcePod {
	return &SourcePod{
		Ports: NewSinglePort(2379),
		Pod: NewPod("etcd-events", labels.Set{
			"app":                     "etcd-statefulset",
			"garden.sapcloud.io/role": "controlplane",
			"role":                    "events",
		}),
		ExpectedPolicies: sets.NewString(
			"allow-etcd",
			"allow-to-dns",
			"allow-to-public-networks",
			"allow-to-private-networks",
			"deny-all",
		),
	}
}

// CloudControllerManagerNotSecured points to cloud-agnostic cloud-controller-manager running on HTTP port.
func (a *Agnostic) CloudControllerManagerNotSecured() *SourcePod {
	return &SourcePod{
		Ports: NewSinglePort(10253),
		Pod: NewPod("cloud-controller-manager-http", labels.Set{
			"app":                     "kubernetes",
			"garden.sapcloud.io/role": "controlplane",
			"role":                    "cloud-controller-manager",
		}, "< 1.13"),
		ExpectedPolicies: sets.NewString(
			"allow-from-prometheus",
			"allow-to-shoot-apiserver",
			"allow-to-dns",
			"allow-to-public-networks",
			"deny-all",
		),
	}
}

// CloudControllerManagerSecured points to cloud-agnostic cloud-controller-manager running on HTTPS port.
func (a *Agnostic) CloudControllerManagerSecured() *SourcePod {
	return &SourcePod{
		Ports: NewSinglePort(10258),
		Pod: NewPod("cloud-controller-manager-https", labels.Set{
			"app":                     "kubernetes",
			"garden.sapcloud.io/role": "controlplane",
			"role":                    "cloud-controller-manager",
		}, ">= 1.13"),
		ExpectedPolicies: sets.NewString(
			"allow-from-prometheus",
			"allow-to-shoot-apiserver",
			"allow-to-dns",
			"allow-to-public-networks",
			"deny-all",
		),
	}
}

// LokiSearch points to cloud-agnostic loki instance.
func (a *Agnostic) LokiSearch() *SourcePod {
	return &SourcePod{
		Ports: []Port{
			{Name: "metrics", Port: 3100},
		},
		Pod: NewPod("loki", labels.Set{
			"app":                 "loki",
			"gardener.cloud/role": "logging",
			"role":                "logging",
		}),
		ExpectedPolicies: sets.NewString(
			"allow-from-prometheus",
			"allow-loki",
			"deny-all",
		),
	}
}

// Grafana points to cloud-agnostic grafana instance.
func (a *Agnostic) Grafana() *SourcePod {
	return &SourcePod{
		Ports: NewSinglePort(3000),
		Pod: NewPod("grafana", labels.Set{
			"component":               "grafana",
			"garden.sapcloud.io/role": "monitoring",
		}),
		ExpectedPolicies: sets.NewString(
			"allow-grafana",
			"allow-to-dns",
			"deny-all",
		),
	}
}

// KubeStateMetricsSeed points to cloud-agnostic kube-state-metrics-seed instance.
func (a *Agnostic) KubeStateMetricsSeed() *SourcePod {
	return &SourcePod{
		Ports: NewSinglePort(8080),
		Pod: NewPod("kube-state-metrics-seed", labels.Set{
			"component":               "kube-state-metrics",
			"garden.sapcloud.io/role": "monitoring",
			"type":                    "seed",
		}),
		ExpectedPolicies: sets.NewString(
			"allow-from-prometheus",
			"allow-to-dns",
			"allow-to-seed-apiserver",
			"deny-all",
		),
	}
}

// KubeStateMetricsShoot points to cloud-agnostic kube-state-metrics-shoot instance.
func (a *Agnostic) KubeStateMetricsShoot() *SourcePod {
	return &SourcePod{
		Ports: NewSinglePort(8080),
		Pod: NewPod("kube-state-metrics-shoot", labels.Set{
			"component":               "kube-state-metrics",
			"garden.sapcloud.io/role": "monitoring",
			"type":                    "shoot",
		}),
		ExpectedPolicies: sets.NewString(
			"allow-from-prometheus",
			"allow-to-dns",
			"allow-to-shoot-apiserver",
			"deny-all",
		),
	}
}

// MachineControllerManager points to cloud-agnostic machine-controller-manager instance.
func (a *Agnostic) MachineControllerManager() *SourcePod {
	return &SourcePod{
		Ports: NewSinglePort(10258),
		Pod: NewPod("machine-controller-manager", labels.Set{
			"app":                     "kubernetes",
			"garden.sapcloud.io/role": "controlplane",
			"role":                    "machine-controller-manager",
		}),
		ExpectedPolicies: sets.NewString(
			"allow-from-prometheus",
			"allow-to-dns",
			"allow-to-public-networks",
			"allow-to-private-networks",
			"allow-to-seed-apiserver",
			"allow-to-shoot-apiserver",
			"deny-all",
		),
	}
}

// Prometheus points to cloud-agnostic prometheus instance.
func (a *Agnostic) Prometheus() *SourcePod {
	return &SourcePod{
		Ports: NewSinglePort(9090),
		Pod: NewPod("prometheus", labels.Set{
			"app":                     "prometheus",
			"garden.sapcloud.io/role": "monitoring",
			"role":                    "monitoring",
		}),
		ExpectedPolicies: sets.NewString(
			"allow-prometheus",
			"allow-to-dns",
			"allow-to-public-networks",
			"allow-to-seed-apiserver",
			"allow-to-shoot-apiserver",
			"allow-to-shoot-networks",
			"deny-all",
		),
	}
}

// AddonManager points to gardener-resource-manager instance.
func (a *Agnostic) AddonManager() *SourcePod {
	return &SourcePod{
		Pod: NewPod("gardener-resource-manager", labels.Set{
			"app":                     "gardener-resource-manager",
			"garden.sapcloud.io/role": "controlplane",
		}),
		ExpectedPolicies: sets.NewString(
			"allow-to-dns",
			"allow-to-seed-apiserver",
			"allow-to-shoot-apiserver",
			"deny-all",
		),
	}
}

// Busybox points to busybox instance.
func (a *Agnostic) Busybox() *SourcePod {
	return &SourcePod{
		Pod: NewPod("busybox", labels.Set{
			"app":  "busybox",
			"role": "testing",
		}),
	}
}

// External points external host.
func (a *Agnostic) External() *Host {
	return &Host{
		Description: "External host",
		HostName:    "8.8.8.8",
		Port:        53,
	}
}

// SeedKubeAPIServer points the Seed Kube APIServer.
func (a *Agnostic) SeedKubeAPIServer() *Host {
	return &Host{
		Description: "Seed Kube APIServer",
		HostName:    "kubernetes.default",
		Port:        443,
	}
}

// GardenPrometheus points the Gardener Prometheus running in the seed cluster.
func (a *Agnostic) GardenPrometheus() *Host {
	return &Host{
		Description: "Garden Prometheus",
		HostName:    "prometheus-web.garden",
		Port:        80,
	}
}
