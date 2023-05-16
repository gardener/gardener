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

package rootcertificates_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"

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
			Expect(units).To(ConsistOf(
				extensionsv1alpha1.Unit{
					Name:    "updatecacerts.service",
					Command: pointer.String("start"),
					Content: pointer.String(`[Unit]
Description=Update local certificate authorities
# Since other services depend on the certificate store run this early
DefaultDependencies=no
Wants=systemd-tmpfiles-setup.service clean-ca-certificates.service
After=systemd-tmpfiles-setup.service clean-ca-certificates.service
Before=sysinit.target kubelet.service
ConditionPathIsReadWrite=/etc/ssl/certs
ConditionPathIsReadWrite=/var/lib/ca-certificates-local
ConditionPathExists=!/var/lib/kubelet/kubeconfig-real
[Service]
Type=oneshot
ExecStart=/var/lib/ssl/update-local-ca-certificates.sh
ExecStartPost=/bin/systemctl restart docker.service
[Install]
WantedBy=multi-user.target`),
				},
			))
			Expect(files).To(ConsistOf(
				extensionsv1alpha1.File{
					Path:        "/var/lib/ssl/update-local-ca-certificates.sh",
					Permissions: pointer.Int32(0744),
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
    if [[ -d "/usr/local/share/ca-certificates" ]]; then
        cp -af /usr/local/share/ca-certificates/* "/var/lib/ca-certificates-local"
    fi
    # localcertsdir is supported on Debian based OS only
    /usr/sbin/update-ca-certificates --fresh --localcertsdir "/var/lib/ca-certificates-local"
else
    /usr/sbin/update-ca-certificates --fresh
fi
`)),
						},
					},
				},
				extensionsv1alpha1.File{
					Path:        "/var/lib/ca-certificates-local/ROOTcerts.crt",
					Permissions: pointer.Int32(0644),
					Content: extensionsv1alpha1.FileContent{
						Inline: &extensionsv1alpha1.FileContentInline{
							Encoding: "b64",
							Data:     caBundleBase64,
						},
					},
				},
				extensionsv1alpha1.File{
					Path:        "/etc/pki/trust/anchors/ROOTcerts.pem",
					Permissions: pointer.Int32(0644),
					Content: extensionsv1alpha1.FileContent{
						Inline: &extensionsv1alpha1.FileContentInline{
							Encoding: "b64",
							Data:     caBundleBase64,
						},
					},
				},
			))
		})
	})
})
