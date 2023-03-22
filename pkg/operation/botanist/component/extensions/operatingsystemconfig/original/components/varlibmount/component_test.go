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

package varlibmount_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/operatingsystemconfig/original/components"
	. "github.com/gardener/gardener/pkg/operation/botanist/component/extensions/operatingsystemconfig/original/components/varlibmount"
)

var _ = Describe("Component", func() {
	Describe("#Config", func() {
		var component components.Component

		BeforeEach(func() {
			component = New()
		})

		It("should do nothing because kubelet data volume name is not set", func() {
			units, files, err := component.Config(components.Context{})

			Expect(err).NotTo(HaveOccurred())
			Expect(units).To(BeNil())
			Expect(files).To(BeNil())
		})

		It("should return the expected units and files", func() {
			units, files, err := component.Config(components.Context{KubeletDataVolumeName: pointer.String("foo")})

			Expect(err).NotTo(HaveOccurred())
			Expect(units).To(ConsistOf(
				extensionsv1alpha1.Unit{
					Name: "var-lib.mount",
					Content: pointer.String(`[Unit]
Description=mount /var/lib on kubelet data device
Before=kubelet.service
[Mount]
What=/dev/disk/by-label/kubeletdev
Where=/var/lib
Type=xfs
Options=defaults
[Install]
WantedBy=local-fs.target`),
				},
			))
			Expect(files).To(BeNil())
		})
	})
})
