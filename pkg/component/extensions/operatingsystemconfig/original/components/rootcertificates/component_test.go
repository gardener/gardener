// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package rootcertificates_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components"
	. "github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/rootcertificates"
	"github.com/gardener/gardener/pkg/utils"
)

var _ = Describe("Component", func() {
	Describe("#Config", func() {
		var (
			component components.Component
			ctx       components.Context

			caBundle       = "some-non-base64-encoded-ca-bundle"
			caBundleBase64 = utils.EncodeBase64([]byte(caBundle))
		)

		BeforeEach(func() {
			component = New()
			ctx = components.Context{CABundle: &caBundle}
		})

		It("should return nothing because the CABundle is empty", func() {
			ctx.CABundle = nil

			units, files, err := component.Config(ctx)

			Expect(err).NotTo(HaveOccurred())
			Expect(units).To(BeNil())
			Expect(files).To(BeNil())
		})

		It("should return the expected units and files", func() {
			units, files, err := component.Config(ctx)

			Expect(err).NotTo(HaveOccurred())

			updateCACertsUnit := extensionsv1alpha1.Unit{
				Name:    "updatecacerts.service",
				Command: ptr.To(extensionsv1alpha1.CommandStart),
				Content: ptr.To(`[Unit]
Description=Update local certificate authorities
# Since other services depend on the certificate store run this early
DefaultDependencies=no
Wants=systemd-tmpfiles-setup.service clean-ca-certificates.service
After=systemd-tmpfiles-setup.service clean-ca-certificates.service
Before=sysinit.target kubelet.service
ConditionPathIsReadWrite=/etc/ssl/certs
ConditionPathIsReadWrite=/var/lib/ca-certificates-local
[Service]
Type=oneshot
ExecStart=/var/lib/ssl/update-local-ca-certificates.sh
[Install]
WantedBy=multi-user.target`),
				FilePaths: []string{
					"/var/lib/ssl/update-local-ca-certificates.sh",
					"/var/lib/ca-certificates-local/ROOTcerts.crt",
					"/etc/pki/trust/anchors/ROOTcerts.pem",
				},
			}

			updateCACertsFiles := []extensionsv1alpha1.File{
				{
					Path:        "/var/lib/ssl/update-local-ca-certificates.sh",
					Permissions: ptr.To[uint32](0744),
					Content: extensionsv1alpha1.FileContent{
						Inline: &extensionsv1alpha1.FileContentInline{
							Encoding: "b64",
							Data: utils.EncodeBase64([]byte(`#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

if [[ -f "/etc/debian_version" ]]; then
    # Copy certificates from default "localcertsdir" because /usr is mounted read-only in Garden Linux.
    # See https://github.com/gardenlinux/gardenlinux/issues/1490
    mkdir -p "/var/lib/ca-certificates-local"
    if [[ -d "/usr/local/share/ca-certificates" && -n "$(ls -A '/usr/local/share/ca-certificates')" ]]; then
        cp -af /usr/local/share/ca-certificates/* "/var/lib/ca-certificates-local"
    fi
    # localcertsdir is supported on Debian based OS only
    /usr/sbin/update-ca-certificates --fresh --localcertsdir "/var/lib/ca-certificates-local"
    exit
fi

if grep -q flatcar "/etc/os-release"; then
    # Flatcar needs the file in /etc/ssl/certs/ with .pem file extension
    cp "/var/lib/ca-certificates-local/ROOTcerts.crt" /etc/ssl/certs/ROOTcerts.pem
    # Flatcar does not support --fresh
    /usr/sbin/update-ca-certificates
    exit
fi

/usr/sbin/update-ca-certificates --fresh
`)),
						},
					},
				},
				{
					Path:        "/var/lib/ca-certificates-local/ROOTcerts.crt",
					Permissions: ptr.To[uint32](0644),
					Content: extensionsv1alpha1.FileContent{
						Inline: &extensionsv1alpha1.FileContentInline{
							Encoding: "b64",
							Data:     caBundleBase64,
						},
					},
				},
				{
					Path:        "/etc/pki/trust/anchors/ROOTcerts.pem",
					Permissions: ptr.To[uint32](0644),
					Content: extensionsv1alpha1.FileContent{
						Inline: &extensionsv1alpha1.FileContentInline{
							Encoding: "b64",
							Data:     caBundleBase64,
						},
					},
				},
			}

			Expect(units).To(ConsistOf(updateCACertsUnit))
			Expect(files).To(ConsistOf(updateCACertsFiles))
		})
	})
})
