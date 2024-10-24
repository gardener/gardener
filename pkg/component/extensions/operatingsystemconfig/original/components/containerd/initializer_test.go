// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package containerd_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components"
	. "github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/containerd"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/imagevector"
)

var _ = Describe("Initializer", func() {
	Describe("#Config", func() {
		var (
			component components.Component
			ctx       components.Context

			images = map[string]*imagevector.Image{
				"pause-container": {
					Name:       "pause-container",
					Repository: ptr.To(pauseContainerImageRepo),
					Tag:        ptr.To(pauseContainerImageTag),
				},
			}
		)

		BeforeEach(func() {
			component = NewInitializer()
			ctx.Images = images
		})

		It("should return the expected units and files", func() {
			units, files, err := component.Config(ctx)

			Expect(err).NotTo(HaveOccurred())
			Expect(units).To(ConsistOf(
				extensionsv1alpha1.Unit{
					Name:    "containerd-initializer.service",
					Command: ptr.To(extensionsv1alpha1.CommandStart),
					Enable:  ptr.To(true),
					Content: ptr.To(`[Unit]
Description=Containerd initializer
[Install]
WantedBy=multi-user.target
[Service]
Type=oneshot
RemainAfterExit=yes
ExecStart=/opt/bin/init-containerd`),
				},
			))
			Expect(files).To(ConsistOf(
				extensionsv1alpha1.File{
					Path:        "/opt/bin/init-containerd",
					Permissions: ptr.To[uint32](744),
					Content: extensionsv1alpha1.FileContent{
						Inline: &extensionsv1alpha1.FileContentInline{
							Encoding: "b64",
							Data:     utils.EncodeBase64([]byte(initScript)),
						},
					},
				},
			))
		})
	})
})

const (
	pauseContainerImageRepo = "foo.io"
	pauseContainerImageTag  = "v1.2.3"
	initScript              = `#!/bin/bash

# The containerd-initializer is deprecated, functionless and only kept on this node for compatibility reasons.
`
)
