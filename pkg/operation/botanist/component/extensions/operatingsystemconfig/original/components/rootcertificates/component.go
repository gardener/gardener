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

package rootcertificates

import (
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/operatingsystemconfig/original/components"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/operatingsystemconfig/original/components/docker"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/operatingsystemconfig/original/components/kubelet"
	"github.com/gardener/gardener/pkg/utils"

	"k8s.io/utils/pointer"
)

type component struct{}

// New returns a new root CA certificates component.
func New() *component {
	return &component{}
}

func (component) Name() string {
	return "root-certificates"
}

func (component) Config(ctx components.Context) ([]extensionsv1alpha1.Unit, []extensionsv1alpha1.File, error) {
	if ctx.CABundle == nil {
		return nil, nil, nil
	}

	const pathEtcdSSLCerts = "/etc/ssl/certs"
	var caBundleBase64 = utils.EncodeBase64([]byte(*ctx.CABundle))

	return []extensionsv1alpha1.Unit{
			{
				Name:    "updatecacerts.service",
				Command: pointer.String("start"),
				Content: pointer.String(`[Unit]
Description=Update CA bundle at ` + pathEtcdSSLCerts + `/ca-certificates.crt
# Since other services depend on the certificate store run this early
DefaultDependencies=no
Wants=systemd-tmpfiles-setup.service clean-ca-certificates.service
After=systemd-tmpfiles-setup.service clean-ca-certificates.service
Before=sysinit.target ` + kubelet.UnitName + `
ConditionPathIsReadWrite=` + pathEtcdSSLCerts + `
ConditionPathExists=!` + kubelet.PathKubeconfigReal + `
[Service]
Type=oneshot
ExecStart=/usr/sbin/update-ca-certificates --fresh
ExecStartPost=/bin/systemctl restart ` + docker.UnitName + `
[Install]
WantedBy=multi-user.target`),
			},
		},
		[]extensionsv1alpha1.File{
			{
				Path:        pathEtcdSSLCerts + "/ROOTcerts.pem",
				Permissions: pointer.Int32(0644),
				Content: extensionsv1alpha1.FileContent{
					Inline: &extensionsv1alpha1.FileContentInline{
						Encoding: "b64",
						Data:     caBundleBase64,
					},
				},
			},
			{
				Path:        "/etc/pki/trust/anchors/ROOTcerts.pem",
				Permissions: pointer.Int32(0644),
				Content: extensionsv1alpha1.FileContent{
					Inline: &extensionsv1alpha1.FileContentInline{
						Encoding: "b64",
						Data:     caBundleBase64,
					},
				},
			},
		},
		nil
}
