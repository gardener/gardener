// Copyright 2024 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package namespacedcloudprofile_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gardener/gardener/pkg/apis/core"
	namespacedcloudprofileregistry "github.com/gardener/gardener/pkg/apiserver/registry/core/namespacedcloudprofile"
)

var _ = Describe("PrepareForCreate", func() {
	var namespacedCloudProfile *core.NamespacedCloudProfile

	It("should drop the expired Kubernetes and MachineImage versions from the namespacedcloudprofile", func() {
		var (
			validExpirationDate1   = &metav1.Time{Time: time.Now().Add(144 * time.Hour)}
			validExpirationDate2   = &metav1.Time{Time: time.Now().Add(24 * time.Hour)}
			expiredExpirationDate1 = &metav1.Time{Time: time.Now().Add(-time.Hour)}
			expiredExpirationDate2 = &metav1.Time{Time: time.Now().Add(-24 * time.Hour)}
		)

		namespacedCloudProfile = &core.NamespacedCloudProfile{
			Spec: core.NamespacedCloudProfileSpec{
				Kubernetes: &core.KubernetesSettings{
					Versions: []core.ExpirableVersion{
						{
							Version: "1.27.3",
						},
						{
							Version:        "1.26.4",
							ExpirationDate: validExpirationDate1,
						},
						{
							Version:        "1.25.6",
							ExpirationDate: validExpirationDate2,
						},
						{
							Version:        "1.24.8",
							ExpirationDate: expiredExpirationDate1,
						},
						{
							Version:        "1.24.6",
							ExpirationDate: expiredExpirationDate2,
						},
					},
				},
				MachineImages: []core.MachineImage{
					{
						Name: "machineImage1",
						Versions: []core.MachineImageVersion{
							{
								ExpirableVersion: core.ExpirableVersion{
									Version: "2.1.0",
								},
							},
							{
								ExpirableVersion: core.ExpirableVersion{
									Version:        "2.0.3",
									ExpirationDate: validExpirationDate1,
								},
							},
							{
								ExpirableVersion: core.ExpirableVersion{
									Version:        "1.9.7",
									ExpirationDate: expiredExpirationDate2,
								},
							},
						},
					},
					{
						Name: "machineImage2",
						Versions: []core.MachineImageVersion{
							{
								ExpirableVersion: core.ExpirableVersion{
									Version:        "4.3.0",
									ExpirationDate: validExpirationDate2,
								},
							},
							{
								ExpirableVersion: core.ExpirableVersion{
									Version: "4.2.3",
								},
							},
							{
								ExpirableVersion: core.ExpirableVersion{
									Version:        "4.1.8",
									ExpirationDate: expiredExpirationDate1,
								},
							},
						},
					},
				},
			},
		}

		namespacedcloudprofileregistry.Strategy.PrepareForCreate(context.TODO(), namespacedCloudProfile)

		Expect(namespacedCloudProfile.Spec.Kubernetes.Versions).To(ConsistOf(
			MatchFields(IgnoreExtras, Fields{
				"Version": Equal("1.27.3"),
			}), MatchFields(IgnoreExtras, Fields{
				"Version": Equal("1.26.4"),
			}), MatchFields(IgnoreExtras, Fields{
				"Version": Equal("1.25.6"),
			}),
		))

		Expect(namespacedCloudProfile.Spec.MachineImages).To(ConsistOf(
			MatchFields(IgnoreExtras, Fields{
				"Name": Equal("machineImage1"),
				"Versions": ConsistOf(MatchFields(IgnoreExtras, Fields{
					"ExpirableVersion": MatchFields(IgnoreExtras, Fields{
						"Version": Equal("2.1.0"),
					})},
				), MatchFields(IgnoreExtras, Fields{
					"ExpirableVersion": MatchFields(IgnoreExtras, Fields{
						"Version": Equal("2.0.3"),
					})},
				)),
			}), MatchFields(IgnoreExtras, Fields{
				"Name": Equal("machineImage2"),
				"Versions": ConsistOf(MatchFields(IgnoreExtras, Fields{
					"ExpirableVersion": MatchFields(IgnoreExtras, Fields{
						"Version": Equal("4.3.0"),
					})},
				), MatchFields(IgnoreExtras, Fields{
					"ExpirableVersion": MatchFields(IgnoreExtras, Fields{
						"Version": Equal("4.2.3"),
					})},
				)),
			}),
		))
	})
})
