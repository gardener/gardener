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

package kubeapiserver_test

import (
	. "github.com/gardener/gardener/pkg/operation/botanist/component/kubeapiserver"

	scalerapi "github.com/gardener/dependency-watchdog/pkg/scaler/api"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	"k8s.io/utils/pointer"
)

var _ = Describe("DependencyWatchdog", func() {
	Describe("#DependencyWatchdogProbeConfiguration", func() {
		It("should compute the correct configuration", func() {
			config, err := DependencyWatchdogProbeConfiguration()
			Expect(config).To(ConsistOf(scalerapi.ProbeDependants{
				Name: "shoot-kube-apiserver",
				Probe: &scalerapi.ProbeConfig{
					External:      &scalerapi.ProbeDetails{KubeconfigSecretName: "dependency-watchdog-external-probe"},
					Internal:      &scalerapi.ProbeDetails{KubeconfigSecretName: "dependency-watchdog-internal-probe"},
					PeriodSeconds: pointer.Int32(30),
				},
				DependantScales: []*scalerapi.DependantScaleDetails{{
					ScaleRef: autoscalingv1.CrossVersionObjectReference{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
						Name:       "kube-controller-manager",
					},
				}},
			}))
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
