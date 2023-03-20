// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package kubeapiserver_test

import (
	restarterapi "github.com/gardener/dependency-watchdog/pkg/restarter/api"
	scalerapi "github.com/gardener/dependency-watchdog/pkg/scaler/api"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	. "github.com/gardener/gardener/pkg/operation/botanist/component/kubeapiserver"
)

var _ = Describe("DependencyWatchdog", func() {
	Describe("#DependencyWatchdogEndpointConfiguration", func() {
		It("should compute the correct configuration", func() {
			config, err := DependencyWatchdogEndpointConfiguration()
			Expect(config).To(Equal(map[string]restarterapi.Service{
				"kube-apiserver": {
					Dependants: []restarterapi.DependantPods{
						{
							Name: "controlplane",
							Selector: &metav1.LabelSelector{
								MatchExpressions: []metav1.LabelSelectorRequirement{
									{
										Key:      "gardener.cloud/role",
										Operator: "In",
										Values:   []string{"controlplane"},
									},
									{
										Key:      "role",
										Operator: "NotIn",
										Values:   []string{"main", "apiserver"},
									},
								},
							},
						},
					},
				},
			}))
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("#DependencyWatchdogProbeConfiguration", func() {
		It("should compute the correct configuration", func() {
			config, err := DependencyWatchdogProbeConfiguration()
			Expect(config).To(ConsistOf(scalerapi.ProbeDependants{
				Name: "shoot-kube-apiserver",
				Probe: &scalerapi.ProbeConfig{
					External:      &scalerapi.ProbeDetails{KubeconfigSecretName: "shoot-access-dependency-watchdog-external-probe"},
					Internal:      &scalerapi.ProbeDetails{KubeconfigSecretName: "shoot-access-dependency-watchdog-internal-probe"},
					PeriodSeconds: pointer.Int32(30),
				},
				DependantScales: []*scalerapi.DependantScaleDetails{
					{
						ScaleRef: autoscalingv1.CrossVersionObjectReference{
							APIVersion: "apps/v1",
							Kind:       "Deployment",
							Name:       "kube-controller-manager",
						},
						ScaleUpDelaySeconds: pointer.Int32(120),
					},
					{
						ScaleRef: autoscalingv1.CrossVersionObjectReference{
							APIVersion: "apps/v1",
							Kind:       "Deployment",
							Name:       "machine-controller-manager",
						},
						ScaleUpDelaySeconds: pointer.Int32(60),
						ScaleRefDependsOn: []autoscalingv1.CrossVersionObjectReference{
							{
								APIVersion: "apps/v1",
								Kind:       "Deployment",
								Name:       "kube-controller-manager",
							},
						},
					},
					{
						ScaleRef: autoscalingv1.CrossVersionObjectReference{
							APIVersion: "apps/v1",
							Kind:       "Deployment",
							Name:       "cluster-autoscaler",
						},
						ScaleRefDependsOn: []autoscalingv1.CrossVersionObjectReference{
							{
								APIVersion: "apps/v1",
								Kind:       "Deployment",
								Name:       "machine-controller-manager",
							},
						},
					},
				},
			}))
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
