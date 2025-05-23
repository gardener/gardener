// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package journald_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components"
	. "github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/journald"
)

var _ = Describe("Component", func() {
	Describe("#Config", func() {
		var component components.Component

		BeforeEach(func() {
			component = New()
		})

		It("should return the expected units and files", func() {
			units, files, err := component.Config(components.Context{})

			Expect(err).NotTo(HaveOccurred())
			Expect(units).To(BeNil())
			Expect(files).To(ConsistOf(
				extensionsv1alpha1.File{
					Path:        "/etc/systemd/journald.conf",
					Permissions: ptr.To[uint32](0644),
					Content: extensionsv1alpha1.FileContent{
						Inline: &extensionsv1alpha1.FileContentInline{
							Data: data,
						},
					},
				},
			))
		})
	})
})

// Configure log rotation for all journal logs, which is where kubelet and container runtime  are configured to write
// their log entries. Journald config will:
// * stores individual Journal files for 24 hours before rotating to a new Journal file
// * keep only 14 old Journal files, and will discard older ones
const data = `[Journal]
MaxFileSec=24h
MaxRetentionSec=14day
`
