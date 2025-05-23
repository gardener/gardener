// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package varlibkubeletmount_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components"
	. "github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/varlibkubeletmount"
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
			units, files, err := component.Config(components.Context{KubeletDataVolumeName: ptr.To("foo")})

			Expect(err).NotTo(HaveOccurred())
			Expect(units).To(ConsistOf(
				extensionsv1alpha1.Unit{
					Name: "var-lib-kubelet.mount",
					Content: ptr.To(`[Unit]
Description=mount /var/lib/kubelet on kubelet data device
Before=kubelet.service
[Mount]
What=/dev/disk/by-label/KUBEDEV
Where=/var/lib/kubelet
Type=ext4
Options=defaults,prjquota,errors=remount-ro
[Install]
WantedBy=local-fs.target`),
				},
			))
			Expect(files).To(BeNil())
		})
	})
})
