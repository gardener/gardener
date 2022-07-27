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

package util_test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/utils/pointer"

	"github.com/gardener/gardener/extensions/pkg/util"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

var _ = Describe("Shoot", func() {
	Describe("#VersionMajorMinor", func() {
		It("should return an error due to an invalid version format", func() {
			v, err := util.VersionMajorMinor("invalid-semver")

			Expect(v).To(BeEmpty())
			Expect(err).To(HaveOccurred())
		})

		It("should return the major/minor part of the given version", func() {
			var (
				major = 14
				minor = 123

				expectedVersion = fmt.Sprintf("%d.%d", major, minor)
			)

			v, err := util.VersionMajorMinor(fmt.Sprintf("%s.88", expectedVersion))

			Expect(v).To(Equal(expectedVersion))
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("#VersionInfo", func() {
		It("should return an error due to an invalid version format", func() {
			v, err := util.VersionInfo("invalid-semver")

			Expect(v).To(BeNil())
			Expect(err).To(HaveOccurred())
		})

		It("should convert the given version to a correct version.Info", func() {
			var (
				expectedVersionInfo = &version.Info{
					Major:      "14",
					Minor:      "123",
					GitVersion: "v14.123.42",
				}
			)

			v, err := util.VersionInfo("14.123.42")

			Expect(v).To(Equal(expectedVersionInfo))
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("#IsPSPDisabled", func() {
		var shoot = &gardencorev1beta1.Shoot{
			Spec: gardencorev1beta1.ShootSpec{
				Kubernetes: gardencorev1beta1.Kubernetes{
					KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{},
				},
			},
		}

		It("should return true if PodSecurityPolicy admissionPlugin is disabled", func() {
			shoot.Spec.Kubernetes.KubeAPIServer.AdmissionPlugins = []gardencorev1beta1.AdmissionPlugin{
				{
					Name:     "PodSecurityPolicy",
					Disabled: pointer.Bool(true),
				},
			}
			Expect(util.IsPSPDisabled(shoot)).To(BeTrue())
		})

		It("should return false if PodSecurityPolicy admissionPlugin is not disabled", func() {
			shoot.Spec.Kubernetes.KubeAPIServer.AdmissionPlugins = []gardencorev1beta1.AdmissionPlugin{
				{
					Name: "PodSecurityPolicy",
				},
			}
			Expect(util.IsPSPDisabled(shoot)).To(BeFalse())
		})

		It("should return false if PodSecurityPolicy admissionPlugin is not specified in the shootSpec", func() {
			shoot.Spec.Kubernetes.KubeAPIServer.AdmissionPlugins = []gardencorev1beta1.AdmissionPlugin{
				{
					Name: "NamespaceLifecycle",
				},
			}
			Expect(util.IsPSPDisabled(shoot)).To(BeFalse())
		})

		It("should return false if KubeAPIServerConfig is nil", func() {
			shoot.Spec.Kubernetes.KubeAPIServer = nil

			Expect(util.IsPSPDisabled(shoot)).To(BeFalse())
		})
	})
})
