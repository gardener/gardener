// Copyright 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package etcd_test

import (
	restarterapi "github.com/gardener/dependency-watchdog/pkg/restarter/api"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gardener/gardener/pkg/operation/botanist/component/etcd"
)

var _ = Describe("DependencyWatchdog", func() {
	Describe("#DependencyWatchdogEndpointConfiguration", func() {
		It("should compute the correct configuration", func() {
			config, err := etcd.DependencyWatchdogEndpointConfiguration(testRole)
			Expect(config).To(Equal(map[string]restarterapi.Service{
				"etcd-" + testRole + "-client": {
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
										Operator: "In",
										Values:   []string{"apiserver"},
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
})
